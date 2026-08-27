package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/githubcli"
	"strings"
	"testing"
	"time"
)

// This file drives `finalize publish` over a REAL feature workspace (the same
// bare-remote topology, gitcli.Client, and workspace.Service the rebase tests
// use, including its owned rebase receipt and its receipt-scoped PublishRewrite)
// plus a faithful-enough fake FinalizeGitHub that also implements the PR
// create-or-edit face. The receipt-scoped force-with-lease push and the
// loss-preserving PR body update are only meaningful against real Git and a real
// receipt, so nothing about the rewrite publication is stubbed; only the GitHub
// PR effect a hermetic suite cannot reach is injected.

// --- fake FinalizeGitHub + PR editor --------------------------------------

// fakePublishGitHub answers the GitHub calls `finalize publish` makes
// (DiscoverRepository, FindOpenPullRequestsByHead, EnsurePullRequest) from a
// single in-memory PR. It models EnsurePullRequest's load-bearing behaviors — the
// expected-head gate, the already-equal no-op, the version CAS, and the edit —
// so one fake serves the happy, replay, and no-op cases, and records every edit
// so a test can assert the full body it received. Every other finalize-half
// GitHub method panics so an accidental call is loud.
type fakePublishGitHub struct {
	repo    githubcli.Repository
	pr      githubcli.PullRequest // the single open PR for the feature head
	findErr error                 // forces the PR reprobe to be unknown
	ensNext int                   // count of EnsurePullRequest calls
	ensLast githubcli.EnsurePullRequestRequest
}

func (f *fakePublishGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	return f.repo, nil
}

func (f *fakePublishGitHub) ViewPullRequest(context.Context, githubcli.Repository, int) (githubcli.PullRequest, error) {
	panic("ViewPullRequest: publish must not call this")
}
func (f *fakePublishGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, head string) ([]githubcli.PullRequest, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.pr.HeadBranch == head && f.pr.State == githubcli.StateOpen {
		return []githubcli.PullRequest{f.pr}, nil
	}
	return nil, nil
}

func (f *fakePublishGitHub) EnsurePullRequest(_ context.Context, req githubcli.EnsurePullRequestRequest) (githubcli.EnsureResult, error) {
	f.ensNext++
	f.ensLast = req
	// The expected-head gate: refuse a PR at any other head (never create/adopt).
	if f.pr.HeadCommit != req.ExpectedHead {
		return githubcli.EnsureResult{Disposition: githubcli.EnsureFailed}, errors.New("head mismatch")
	}
	// Already in the desired end state — no mutation.
	if f.pr.Body == req.Body && f.pr.Title == req.Title && f.pr.BaseBranch == req.BaseBranch {
		if req.ExpectedVersion == "" {
			return githubcli.EnsureResult{Disposition: githubcli.EnsureAdopted, PR: f.pr}, nil
		}
		return githubcli.EnsureResult{Disposition: githubcli.EnsureUnchanged, PR: f.pr}, nil
	}
	// An edit is authorized only by the exact live version.
	if req.ExpectedVersion == "" || req.ExpectedVersion != f.pr.Version {
		return githubcli.EnsureResult{Disposition: githubcli.EnsureContended}, nil
	}
	f.pr.Body = req.Body
	f.pr.Title = req.Title
	f.pr.BaseBranch = req.BaseBranch
	f.pr.Version = "sha256:" + strings.Repeat("e", 64)
	return githubcli.EnsureResult{Disposition: githubcli.EnsureUpdated, PR: f.pr}, nil
}

func (f *fakePublishGitHub) ProbeMerged(context.Context, githubcli.Repository, int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	panic("ProbeMerged: publish must not call this")
}
func (f *fakePublishGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: publish must not call this")
}
func (f *fakePublishGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: publish must not call this")
}
func (f *fakePublishGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: publish must not call this")
}
func (f *fakePublishGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeResult, error) {
	panic("MergePullRequest: publish must not call this")
}

// --- publish fixture ------------------------------------------------------

// publishFixture is a real feature workspace already carrying a completed owned
// rebase: the local head is the rewritten head, an owned receipt keyed to the
// pre-rewrite remote head exists, and the remote feature ref still holds the
// original (pre-rewrite) head. It is the exact state `finalize publish` consumes.
type publishFixture struct {
	*rebaseFixture
	rewritten string // the rewritten local head PublishRewrite must publish
	attempt   string // the owned rebase attempt token
	origHead  string // the pre-rewrite head the remote still holds
}

// setupPublishFixture drives a real rebase to completion, leaving the fixture in
// the publish-ready state (rewritten local head, owned receipt, remote at the
// original head).
func setupPublishFixture(t *testing.T, m planRepoMode) *publishFixture {
	t.Helper()
	f := setupRebaseFixture(t, m)
	f.advanceBase(t) // the base moves ahead: a real rewrite is required.
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Disposition != RebaseDispRebased || res.Attempt == "" {
		t.Fatalf("rebase setup = disp %q attempt %q (reason %q), want rebased with an attempt", res.Disposition, res.Attempt, res.Reason)
	}
	rewritten := f.localHead()
	if rewritten == f.head {
		t.Fatalf("the rebase did not move the head; no rewrite to publish")
	}
	return &publishFixture{rebaseFixture: f, rewritten: rewritten, attempt: res.Attempt, origHead: f.head}
}

// publishDeps assembles the FinalizeDeps a publish test drives: the real planning
// seams and workspace service, plus the fake publish GitHub.
func (f *publishFixture) publishDeps(gh FinalizeGitHub) FinalizeDeps {
	return FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}
}

// remoteFeatureTip reads the current tip of the remote feature ref on the bare
// origin — the authoritative published head the rewrite must reach.
func (f *publishFixture) remoteFeatureTip(t *testing.T) string {
	t.Helper()
	return runGit(t, f.repo.origin, "rev-parse", "refs/heads/feat/"+f.slug)
}

// recFor extracts the canonical green record certifying head from freshly
// rendered evidence bytes — the same record `finalize publish` reparses.
func recFor(t *testing.T, head string) (evidence.Record, []byte) {
	t.Helper()
	body := greenEvidenceFor(t, head)
	rec, err := evidence.Extract([]byte(body))
	if err != nil {
		t.Fatalf("evidence.Extract: %v", err)
	}
	return rec, []byte(body)
}

// authoredPRBody builds a PR body with authored prose surrounding a build-evidence
// block that certifies head — the loss-preservation target.
func authoredPRBody(t *testing.T, head string) string {
	t.Helper()
	rec, err := evidence.NewRecord("go test ./...", head, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("evidence.NewRecord: %v", err)
	}
	return "## Summary\n\nAuthored intro prose.\n\n" + evidence.Render(rec) + "\n\nAuthored outro prose.\n"
}

const publishPRTitle = "Add the widget"

// openPRForPublish builds the open PR the reprobe finds: at the rewritten head
// (GitHub reflects the pushed head), with the authored body and title.
func (f *publishFixture) openPRForPublish(head, body string) githubcli.PullRequest {
	return githubcli.PullRequest{
		Number: 7, URL: "https://example.test/pr/7", State: githubcli.StateOpen,
		HeadBranch: "feat/" + f.slug, HeadCommit: head, BaseBranch: "main",
		Title: publishPRTitle, Body: body, Version: "sha256:" + strings.Repeat("d", 64),
	}
}

// --- TestFinalizePublishOrder ---------------------------------------------

// --- TestFinalizePublishCrashReplay ---------------------------------------

// --- TestFinalizePublishUnknownStops --------------------------------------

// --- TestFinalizePublishRefusesForeignAttempt -----------------------------

// --- TestFinalizePublishShapeAndEvidenceRefusals --------------------------
