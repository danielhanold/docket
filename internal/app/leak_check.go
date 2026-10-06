package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/leakscan"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the private-repository leak check. Before a push or PR write in a
// private repository, everything that write would expose — the feature branch
// name, the outgoing commit messages, the paths and lines each outgoing commit
// adds and the merge-base diff adds, and any PR title and body — is scanned for
// docket fingerprints (internal/leakscan), and any hit refuses the write with
// nothing published. A shared repository never runs it.
// A failure to fetch the base or read the outgoing set is leak-check-unverified,
// never a clean scan. Each caller scans the exact head it publishes.

// The stable machine reasons a leak-checked publish reports.
const (
	// ReasonLeakDetected is a blocked publish: the scan found at least one
	// fingerprint, and nothing was published.
	ReasonLeakDetected = "leak-detected"
	// ReasonLeakCheckUnverified is an external-failed publish: the check could
	// not run (the base fetch or the outgoing read failed), so nothing was
	// published.
	ReasonLeakCheckUnverified = "leak-check-unverified"
)

// LeakHit is one fingerprint a publish would expose. Commit is set for a
// commit-message hit, and for an added line or path that a known outgoing commit
// added; File is set for an added line or path; Line is 1-based (0 for a path). Text is the matched text only, never the surrounding line, so a hit
// never carries the rest of a PR body or a file.
type LeakHit struct {
	Source string `json:"source"`
	Commit string `json:"commit,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Text   string `json:"text"`
	Rule   string `json:"rule"`
}

// leakGit is the Git seam runLeakCheck reads through.
type leakGit interface {
	FetchBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, branch gitcli.RefName) (gitcli.Revision, error)
	ReadOutgoing(ctx context.Context, repo gitcli.Repository, base, head gitcli.ObjectID) (gitcli.Outgoing, error)
}

var _ leakGit = (*gitcli.Client)(nil)

// leakCheckShown bounds how many hits leakMessage names; the rest ride in the
// result's leaks list.
const leakCheckShown = 5

// leakCheckApplies reports whether the leak check runs: in a private
// repository only.
func leakCheckApplies(lay layout.Layout) bool { return lay.Mode == layout.Private }

// leakOptions builds the scan options from the pinned configuration and the
// snapshot's change ids.
func leakOptions(pin StatusPin, snap domain.Snapshot) leakscan.Options {
	ids := map[int]bool{}
	for _, c := range snap.Changes() {
		ids[int(c.ID())] = true
	}
	return leakscan.Options{MatchWord: pin.Config.Effective.LeakCheck.MatchWord.Value, ChangeIDs: ids}
}

// runLeakCheck fetches target's base from origin and scans what publishing head
// over it would expose — target's feature branch name included, since origin and
// the PR show it — plus pr when non-nil. Any fetch or read failure is an
// error — the caller refuses as unverified — and never an empty hit list.
func runLeakCheck(ctx context.Context, git leakGit, wc workspaceContext, target workspace.Target, head gitcli.ObjectID, pr *leakscan.PRText) ([]LeakHit, error) {
	base, err := git.FetchBranch(ctx, wc.repo, originRemote, target.BaseRef)
	if err != nil {
		return nil, fmt.Errorf("fetching the base %s from origin: %w", target.BaseRef, err)
	}
	out, err := git.ReadOutgoing(ctx, wc.repo, base.Commit, head)
	if err != nil {
		return nil, fmt.Errorf("reading what %s over %s would publish: %w", shortCommit(string(head)), shortCommit(string(base.Commit)), err)
	}
	in := leakscan.Input{Branch: target.FeatureBranch(), PR: pr}
	for _, c := range out.Commits {
		in.Commits = append(in.Commits, leakscan.Commit{ID: string(c.Commit), Message: c.Message})
	}
	for _, p := range out.AddedPaths {
		in.AddedPaths = append(in.AddedPaths, leakscan.AddedPath{Path: p.Path, Commit: string(p.Commit)})
	}
	for _, l := range out.AddedLines {
		in.AddedLines = append(in.AddedLines, leakscan.AddedLine{Path: l.Path, Line: l.Line, Text: l.Text, Commit: string(l.Commit)})
	}
	var hits []LeakHit
	for _, h := range leakscan.Scan(in, leakOptions(wc.pin, wc.snap)) {
		hits = append(hits, LeakHit{Source: string(h.Source), Commit: h.Commit, File: h.File, Line: h.Line, Text: h.Text, Rule: string(h.Rule)})
	}
	return hits, nil
}

// leakMessage renders a bounded refusal message naming the first hits by where
// they are, their matched text, and their rule.
func leakMessage(hits []LeakHit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d docket fingerprint(s) would reach a shared surface; nothing was published: ", len(hits))
	for i, h := range hits {
		if i == leakCheckShown {
			break
		}
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s %q (%s)", leakWhere(h), h.Text, h.Rule)
	}
	if more := len(hits) - leakCheckShown; more > 0 {
		fmt.Fprintf(&b, " … and %d more (see leaks)", more)
	}
	return b.String()
}

// leakWhere names where a hit sits, with the adding commit for an added line or
// path when it is known.
func leakWhere(h LeakHit) string {
	in := ""
	if h.Commit != "" {
		in = " in commit " + shortCommit(h.Commit)
	}
	switch leakscan.Source(h.Source) {
	case leakscan.SourceBranchName:
		return "the feature branch name"
	case leakscan.SourceCommitMessage:
		return fmt.Sprintf("commit %s message line %d", shortCommit(h.Commit), h.Line)
	case leakscan.SourceAddedLine:
		return fmt.Sprintf("%s:%d%s", h.File, h.Line, in)
	case leakscan.SourceAddedPath:
		return "path " + h.File + in
	default:
		return fmt.Sprintf("%s line %d", h.Source, h.Line)
	}
}
