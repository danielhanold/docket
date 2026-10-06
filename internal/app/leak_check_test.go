package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/leakscan"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// fakeLeakGit records the base fetch and the outgoing read runLeakCheck makes,
// and returns scripted values or errors.
type fakeLeakGit struct {
	fetchRemote gitcli.RemoteName
	fetchBranch gitcli.RefName
	fetchCalls  int
	fetchRev    gitcli.Revision
	fetchErr    error

	readCalls int
	readBase  gitcli.ObjectID
	readHead  gitcli.ObjectID
	out       gitcli.Outgoing
	readErr   error
}

func (f *fakeLeakGit) FetchBranch(_ context.Context, _ gitcli.Repository, remote gitcli.RemoteName, branch gitcli.RefName) (gitcli.Revision, error) {
	f.fetchCalls++
	f.fetchRemote, f.fetchBranch = remote, branch
	return f.fetchRev, f.fetchErr
}

func (f *fakeLeakGit) ReadOutgoing(_ context.Context, _ gitcli.Repository, base, head gitcli.ObjectID) (gitcli.Outgoing, error) {
	f.readCalls++
	f.readBase, f.readHead = base, head
	return f.out, f.readErr
}

const (
	leakBase = gitcli.ObjectID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	leakHead = gitcli.ObjectID("cccccccccccccccccccccccccccccccccccccccc")
)

// leakContext builds the workspace context and target runLeakCheck reads: the
// prReader pin with leak_check.match_word set explicitly, the snapshot over its
// corpus (change 7 exists), a fixed repository, and a main base ref.
func leakContext(t *testing.T, matchWord bool) (workspaceContext, workspace.Target) {
	t.Helper()
	reader := prReader(t)
	pin := reader.pin
	pin.Config.Effective.LeakCheck.MatchWord = config.Value[bool]{Value: matchWord, Explicit: true}
	inputs, _ := parseCorpus(reader.corpus)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	wc := workspaceContext{
		pin:  pin,
		snap: build.Snapshot,
		repo: gitcli.Repository{PrimaryWorktree: "/r", CommonDir: "/r/.git"},
	}
	return wc, workspace.Target{BaseRef: "refs/heads/main"}
}

func TestRunLeakCheckScansFetchedBaseToHead(t *testing.T) {
	wc, target := leakContext(t, true)
	git := &fakeLeakGit{
		fetchRev: gitcli.Revision{Commit: leakBase},
		out: gitcli.Outgoing{
			Commits:    []gitcli.OutgoingCommit{{Commit: "c1", Message: "Fix it (0007)\n"}},
			AddedPaths: []string{"a.go"},
			AddedLines: []gitcli.OutgoingLine{{Path: "a.go", Line: 3, Text: "x := \".docket/y\""}},
		},
	}
	hits, err := runLeakCheck(context.Background(), git, wc, target, leakHead, &leakscan.PRText{Title: "change 0007", Body: "ok\n"})
	if err != nil {
		t.Fatalf("runLeakCheck: %v", err)
	}
	if git.fetchCalls != 1 || git.fetchRemote != "origin" || git.fetchBranch != "refs/heads/main" {
		t.Errorf("fetch = %d call(s) of (%q, %q), want one of (origin, refs/heads/main)", git.fetchCalls, git.fetchRemote, git.fetchBranch)
	}
	if git.readCalls != 1 || git.readBase != leakBase || git.readHead != leakHead {
		t.Errorf("read = %d call(s) of (%q, %q), want one of the fetched base and the head", git.readCalls, git.readBase, git.readHead)
	}
	want := []LeakHit{
		{Source: "commit-message", Commit: "c1", Line: 1, Text: "(0007)", Rule: "change-ref"},
		{Source: "added-line", File: "a.go", Line: 3, Text: ".docket", Rule: "path"},
		{Source: "pr-title", Line: 1, Text: "change 0007", Rule: "change-ref"},
	}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("hits =\n  %+v\nwant\n  %+v", hits, want)
	}
}

func TestRunLeakCheckFetchFailureIsUnverified(t *testing.T) {
	wc, target := leakContext(t, true)
	git := &fakeLeakGit{fetchErr: errors.New("network down")}
	hits, err := runLeakCheck(context.Background(), git, wc, target, leakHead, nil)
	if err == nil {
		t.Fatalf("a failed base fetch returned no error (hits %v); an unverifiable scan must never read as clean", hits)
	}
	if git.readCalls != 0 {
		t.Errorf("a failed base fetch still read %d outgoing set(s)", git.readCalls)
	}
}

func TestRunLeakCheckReadFailureIsUnverified(t *testing.T) {
	wc, target := leakContext(t, true)
	git := &fakeLeakGit{fetchRev: gitcli.Revision{Commit: leakBase}, readErr: errors.New("no merge base")}
	hits, err := runLeakCheck(context.Background(), git, wc, target, leakHead, nil)
	if err == nil {
		t.Fatalf("a failed outgoing read returned no error (hits %v); an unverifiable scan must never read as clean", hits)
	}
}

func TestRunLeakCheckMatchWordOff(t *testing.T) {
	wc, target := leakContext(t, false)
	git := &fakeLeakGit{
		fetchRev: gitcli.Revision{Commit: leakBase},
		out: gitcli.Outgoing{Commits: []gitcli.OutgoingCommit{
			{Commit: "c1", Message: "the docket number"},
			{Commit: "c2", Message: "dckt"},
		}},
	}
	hits, err := runLeakCheck(context.Background(), git, wc, target, leakHead, nil)
	if err != nil {
		t.Fatalf("runLeakCheck: %v", err)
	}
	want := []LeakHit{{Source: "commit-message", Commit: "c2", Line: 1, Text: "dckt", Rule: "alias"}}
	if !reflect.DeepEqual(hits, want) {
		t.Errorf("hits under match_word false = %+v, want %+v", hits, want)
	}
}

func TestLeakCheckAppliesOnlyPrivate(t *testing.T) {
	for _, tc := range []struct {
		mode layout.Mode
		want bool
	}{{layout.Shared, false}, {"", false}, {layout.Private, true}} {
		if got := leakCheckApplies(layout.Layout{Mode: tc.mode}); got != tc.want {
			t.Errorf("leakCheckApplies(mode %q) = %v, want %v", tc.mode, got, tc.want)
		}
	}
}

func TestLeakMessageBoundsAndNamesHits(t *testing.T) {
	hits := []LeakHit{
		{Source: "commit-message", Commit: "0123456789abcdef0123", Line: 2, Text: "(0007)", Rule: "change-ref"},
		{Source: "added-line", File: "a.go", Line: 3, Text: ".docket", Rule: "path"},
		{Source: "added-path", File: "x/.docket/y", Text: ".docket", Rule: "path"},
		{Source: "pr-body", Line: 4, Text: "dckt", Rule: "alias"},
		{Source: "pr-title", Line: 1, Text: "docket", Rule: "word"},
		{Source: "added-line", File: "sixth.go", Line: 6, Text: "docket", Rule: "word"},
		{Source: "added-line", File: "seventh.go", Line: 7, Text: "docket", Rule: "word"},
	}
	msg := leakMessage(hits)
	for _, want := range []string{
		"7 docket fingerprint(s) would reach a shared surface; nothing was published: ",
		`commit 0123456789ab message line 2 "(0007)" (change-ref)`,
		`a.go:3 ".docket" (path)`,
		`path x/.docket/y ".docket" (path)`,
		`pr-body line 4 "dckt" (alias)`,
		`pr-title line 1 "docket" (word)`,
		" … and 2 more (see leaks)",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("leak message lacks %q:\n%s", want, msg)
		}
	}
	for _, absent := range []string{"sixth.go", "seventh.go"} {
		if strings.Contains(msg, absent) {
			t.Errorf("leak message names %q past the first five hits:\n%s", absent, msg)
		}
	}
	if short := leakMessage(hits[:2]); strings.Contains(short, "more") {
		t.Errorf("a two-hit message claims more hits:\n%s", short)
	}
}
