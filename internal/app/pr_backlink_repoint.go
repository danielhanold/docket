package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
)

// This file is the one place that decides whether a merged pull request's
// docket:backlink block must be repointed at the change's archived record, and
// the one edit sequence that does it. Close-out (runCloseoutPRBacklinkLeg),
// cleanup (finalizeCleanupPRBacklinkRepair), the sweep assessment
// (sweepAssessPRBacklinkLeg), and `repository repair --pr-backlinks` all call
// it; none keeps a copy of the predicate.
//
// The decision is path-keyed: a block whose interior already names the archive
// path is a no-op whatever its title wording, and a body with no block is never
// given one. Only the docket-owned block is replaced, through the loss-preserving
// document patch path, so every authored byte (CRLF endings and the build-
// evidence block included) survives. The leg is best-effort: a failure is a
// pr-backlink-pending warning, never a stop. PR body bytes never reach a finding.

// ReasonPRBacklinkPending: a merged PR's backlink block could not be repointed at
// the archived record (a read, edit, or marker problem, or a lost race); the
// change stays truthfully done and cleanup / the sweep retry it.
const ReasonPRBacklinkPending = "pr-backlink-pending"

// The closed outcomes of one repoint attempt.
const (
	prBacklinkRepointed = "repointed" // the block now names the archive path
	prBacklinkAlready   = "already"   // the block already named it; no edit
	prBacklinkNoBlock   = "no-block"  // a hand-written body; never given a block
	prBacklinkContended = "contended" // the body moved under us; not overwritten
	prBacklinkUnknown   = "unknown"   // unreadable, malformed, or unverified
)

// PRBodyEditor is the one GitHub write the repoint needs.
type PRBodyEditor interface {
	EditPullRequestBody(ctx context.Context, repo githubcli.Repository, number int, expectedRevision, body string) (githubcli.BodyEditOutcome, githubcli.PullRequest, error)
}

// FinalizePRBody is the read+write seam close-out and cleanup use.
type FinalizePRBody interface {
	ViewPullRequest(ctx context.Context, repo githubcli.Repository, number int) (githubcli.PullRequest, error)
	PRBodyEditor
}

// The production client satisfies the seam, so prBodyEditor's fallback is live.
var _ FinalizePRBody = (*githubcli.Client)(nil)

// prBodyEditor returns the injected PRBody seam, else deps.GitHub when it
// satisfies FinalizePRBody (production: the real client), else nil (a test
// fake GitHub that cannot edit bodies — the leg is then not run).
func prBodyEditor(deps FinalizeDeps) FinalizePRBody {
	if deps.PRBody != nil {
		return deps.PRBody
	}
	if ed, ok := deps.GitHub.(FinalizePRBody); ok {
		return ed
	}
	return nil
}

// prBacklinkResult is one attempt's outcome, the block interior it found
// (generated text only), and a redacted diagnostic.
type prBacklinkResult struct {
	outcome string
	current string
	detail  string
}

// planPRBacklinkRepoint is the pure decision over one PR body. present reports
// whether a docket:backlink block exists; needs reports whether its interior
// fails to name archivePath; updated is the body with only that block's interior
// replaced by archivedInterior (the input bytes unchanged when !needs). A parse
// or patch failure — malformed, unbalanced, or nested markers — is an error.
func planPRBacklinkRepoint(body []byte, archivePath, archivedInterior string) (updated []byte, current string, present, needs bool, err error) {
	doc, err := document.Parse(body)
	if err != nil {
		return nil, "", false, false, err
	}
	blk, ok := doc.Block(backlinkBlockName)
	if !ok {
		return body, "", false, false, nil
	}
	current = strings.TrimRight(string(body[blk.Interior.Start:blk.Interior.End]), "\r\n")
	if strings.Contains(current, archivePath) {
		return body, current, true, false, nil
	}
	var ps document.PatchSet
	ps.ReplaceBlock(backlinkBlockName, archivedInterior)
	updated, err = doc.Apply(ps)
	if err != nil {
		return nil, current, true, false, err
	}
	return updated, current, true, true, nil
}

// repointPRBacklink reads one PR by number and repoints its block.
func repointPRBacklink(ctx context.Context, ed FinalizePRBody, repo githubcli.Repository, number int, archivePath, archivedInterior string) prBacklinkResult {
	pr, err := ed.ViewPullRequest(ctx, repo, number)
	if err != nil {
		return prBacklinkResult{outcome: prBacklinkUnknown, detail: "the pull-request body could not be read: " + err.Error()}
	}
	return applyPRBacklinkRepoint(ctx, ed, repo, number, pr.Body, pr.Revision, archivePath, archivedInterior)
}

// applyPRBacklinkRepoint repoints a body already in hand (the repair reads it in
// a batch), editing only under the revision that body was read at.
func applyPRBacklinkRepoint(ctx context.Context, ed PRBodyEditor, repo githubcli.Repository, number int, body, revision, archivePath, archivedInterior string) prBacklinkResult {
	updated, current, present, needs, err := planPRBacklinkRepoint([]byte(body), archivePath, archivedInterior)
	if err != nil {
		return prBacklinkResult{outcome: prBacklinkUnknown, detail: "the pull-request body carries a malformed docket:backlink block"}
	}
	if !present {
		return prBacklinkResult{outcome: prBacklinkNoBlock}
	}
	if !needs {
		return prBacklinkResult{outcome: prBacklinkAlready, current: current}
	}
	out, _, err := ed.EditPullRequestBody(ctx, repo, number, revision, string(updated))
	switch {
	case err != nil:
		return prBacklinkResult{outcome: prBacklinkUnknown, current: current, detail: "the edit could not be verified: " + err.Error()}
	case out == githubcli.BodyEdited || out == githubcli.BodyAlready:
		return prBacklinkResult{outcome: prBacklinkRepointed, current: current}
	case out == githubcli.BodyContended:
		return prBacklinkResult{outcome: prBacklinkContended, current: current, detail: "the body changed since it was read"}
	default:
		return prBacklinkResult{outcome: prBacklinkUnknown, current: current, detail: "the edit outcome is unknown"}
	}
}

// prBacklinkFinding maps an attempt to a retryable warning, or nil when the PR is
// in its promised state (repointed, already, or a block-less body).
func prBacklinkFinding(id, number int, r prBacklinkResult) *StatusFinding {
	switch r.outcome {
	case prBacklinkRepointed, prBacklinkAlready, prBacklinkNoBlock:
		return nil
	}
	msg := fmt.Sprintf("change %04d is done, but PR #%d's change backlink was not repointed to the archived record (%s)", id, number, r.outcome)
	if r.detail != "" {
		msg += ": " + r.detail
	}
	msg += "; cleanup and the maintenance sweep will retry it"
	return &StatusFinding{Code: ReasonPRBacklinkPending, Severity: string(domain.SeverityWarning), Message: msg}
}
