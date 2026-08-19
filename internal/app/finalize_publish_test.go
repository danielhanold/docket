package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/githubcli"
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
func (f *fakePublishGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
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

// TestFinalizePublishOrder proves the ordered composition: the rewrite is pushed
// under the receipt lease (the remote moves from the original head to the
// rewritten head), then the PR build-evidence block is loss-preservingly replaced
// with the exact current-head record — every authored byte and the title
// preserved — and no second PR is ever created.
func TestFinalizePublishOrder(t *testing.T) {
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupPublishFixture(t, m)
			if tip := f.remoteFeatureTip(t); tip != f.origHead {
				t.Fatalf("precondition: remote feature tip = %q, want the original head %q", tip, f.origHead)
			}

			evRec, evBytes := recFor(t, f.rewritten)
			prBody := authoredPRBody(t, f.origHead) // the PR still carries evidence for the old head
			gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, prBody)}

			res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
				FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})

			if res.Result != ResultApplied || res.Disposition != PublishDispPublished {
				t.Fatalf("publish = %q disp %q (reason %q msg %q), want applied/published", res.Result, res.Disposition, res.Reason, res.Message)
			}
			// The rewrite reached the remote under the lease.
			if tip := f.remoteFeatureTip(t); tip != f.rewritten {
				t.Fatalf("remote feature tip = %q, want the rewritten head %q", tip, f.rewritten)
			}
			if res.Rewrite != "published" {
				t.Errorf("rewrite outcome = %q, want published", res.Rewrite)
			}
			// Exactly one PR edit; never a create.
			if gh.ensNext != 1 {
				t.Fatalf("EnsurePullRequest called %d time(s), want exactly 1", gh.ensNext)
			}
			// The edit converged the exact expected head and version.
			if gh.ensLast.ExpectedHead != f.rewritten {
				t.Errorf("edit expected head = %q, want the rewritten head %q", gh.ensLast.ExpectedHead, f.rewritten)
			}
			// Loss preservation: the full body equals the authored body with ONLY its
			// evidence block replaced, and the title and base are byte-identical.
			wantBody, err := evidence.Upsert([]byte(prBody), evRec)
			if err != nil {
				t.Fatalf("evidence.Upsert: %v", err)
			}
			if gh.ensLast.Body != string(wantBody) {
				t.Errorf("edited body mismatch:\n got %q\nwant %q", gh.ensLast.Body, string(wantBody))
			}
			if gh.ensLast.Title != publishPRTitle {
				t.Errorf("edited title = %q, want the authored title unchanged %q", gh.ensLast.Title, publishPRTitle)
			}
			if gh.ensLast.BaseBranch != "main" {
				t.Errorf("edited base = %q, want the authored base unchanged", gh.ensLast.BaseBranch)
			}
			// The replaced block certifies the exact rewritten head, and the authored
			// prose survived.
			got, err := evidence.Extract([]byte(gh.ensLast.Body))
			if err != nil || got.Head != f.rewritten {
				t.Errorf("edited body evidence head = %q (err %v), want the rewritten head %q", got.Head, err, f.rewritten)
			}
			if !strings.Contains(gh.ensLast.Body, "Authored intro prose.") || !strings.Contains(gh.ensLast.Body, "Authored outro prose.") {
				t.Errorf("the authored prose was not preserved in the edited body: %q", gh.ensLast.Body)
			}
			// The result names the PR without leaking a body byte.
			if res.Number != 7 || !strings.HasSuffix(res.Reference, "#7") {
				t.Errorf("result PR identity = number %d ref %q, want #7", res.Number, res.Reference)
			}
		})
	}
}

// --- TestFinalizePublishCrashReplay ---------------------------------------

// TestFinalizePublishCrashReplay proves the two replay faces. A crash after the
// push but before the PR update is a no-op rewrite (the remote already holds the
// rewritten head) that still resumes the PR update. A crash after both is a full
// no-op.
func TestFinalizePublishCrashReplay(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("after-push-before-pr-update", func(t *testing.T) {
		f := setupPublishFixture(t, main)
		// The push already landed: force the rewritten head onto the remote out of band.
		runGit(t, f.wp, "push", "--force", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
		if tip := f.remoteFeatureTip(t); tip != f.rewritten {
			t.Fatalf("precondition: remote tip = %q, want the rewritten head", tip)
		}
		_, evBytes := recFor(t, f.rewritten)
		prBody := authoredPRBody(t, f.origHead) // the PR still carries the old-head evidence
		gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, prBody)}

		res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
			FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})

		if res.Result != ResultApplied || res.Disposition != PublishDispPublished {
			t.Fatalf("replay = %q disp %q, want applied/published (the PR update resumes)", res.Result, res.Disposition)
		}
		if res.Rewrite != "noop" {
			t.Errorf("rewrite outcome = %q, want noop (the remote already held the head)", res.Rewrite)
		}
		if gh.ensNext != 1 {
			t.Errorf("EnsurePullRequest called %d time(s), want 1 (the PR update resumes)", gh.ensNext)
		}
	})

	t.Run("after-both-is-full-noop", func(t *testing.T) {
		f := setupPublishFixture(t, main)
		runGit(t, f.wp, "push", "--force", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
		evRec, evBytes := recFor(t, f.rewritten)
		// The PR already carries the exact current-head evidence.
		converged, err := evidence.Upsert([]byte(authoredPRBody(t, f.origHead)), evRec)
		if err != nil {
			t.Fatalf("evidence.Upsert: %v", err)
		}
		gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, string(converged))}

		res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
			FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})

		if res.Result != ResultNoOp || res.Disposition != PublishDispNoop {
			t.Fatalf("full replay = %q disp %q (reason %q), want no-op/noop", res.Result, res.Disposition, res.Reason)
		}
		if res.Rewrite != "noop" {
			t.Errorf("rewrite outcome = %q, want noop", res.Rewrite)
		}
		// The PR body was not mutated (the edit was a no-op).
		if gh.pr.Body != string(converged) {
			t.Errorf("a full replay mutated the PR body")
		}
	})
}

// --- TestFinalizePublishUnknownStops --------------------------------------

// TestFinalizePublishUnknownStops proves a PR reprobe that cannot be established
// (after the rewrite already published) is unknown: retained, no PR edit issued,
// and never a merge-enabling success.
func TestFinalizePublishUnknownStops(t *testing.T) {
	requireRealGit(t)
	f := setupPublishFixture(t, planRepoModes()[0])
	_, evBytes := recFor(t, f.rewritten)
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, authoredPRBody(t, f.origHead)), findErr: errors.New("gh list boom")}

	res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
		FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})

	if res.Result != ResultExternalFailed || res.Disposition != PublishDispUnknown {
		t.Fatalf("unknown reprobe = %q disp %q, want external-failed/unknown", res.Result, res.Disposition)
	}
	if res.Reason != ReasonPublishPRProbeFailed {
		t.Errorf("reason = %q, want %q", res.Reason, ReasonPublishPRProbeFailed)
	}
	if gh.ensNext != 0 {
		t.Errorf("EnsurePullRequest was called %d time(s) after an unknown reprobe; want 0 (no second mutation)", gh.ensNext)
	}
	// The rewrite still landed on the remote (the push is not rolled back), but the
	// PR was not touched.
	if tip := f.remoteFeatureTip(t); tip != f.rewritten {
		t.Errorf("remote tip = %q, want the rewritten head (the push is not rolled back)", tip)
	}
}

// --- TestFinalizePublishRefusesForeignAttempt -----------------------------

// TestFinalizePublishRefusesForeignAttempt proves an attempt token that does not
// match the owned receipt is refused before any push: the remote is untouched and
// no PR edit is issued.
func TestFinalizePublishRefusesForeignAttempt(t *testing.T) {
	requireRealGit(t)
	f := setupPublishFixture(t, planRepoModes()[0])
	_, evBytes := recFor(t, f.rewritten)
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, authoredPRBody(t, f.origHead))}

	res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
		FinalizePublishRequest{ID: f.id, Attempt: "not-the-owned-attempt", Head: f.rewritten, EvidenceRecord: evBytes})

	if res.Result != ResultBlocked || res.Reason != ReasonPublishForeignAttempt {
		t.Fatalf("foreign attempt = (%q, %q), want blocked/attempt-token-mismatch", res.Result, res.Reason)
	}
	// No push happened: the remote still holds the original head.
	if tip := f.remoteFeatureTip(t); tip != f.origHead {
		t.Errorf("a foreign-attempt refusal pushed to the remote: tip %q, want the original head %q", tip, f.origHead)
	}
	if gh.ensNext != 0 {
		t.Errorf("a foreign-attempt refusal issued %d PR edit(s); want 0", gh.ensNext)
	}
}

// --- TestFinalizePublishShapeAndEvidenceRefusals --------------------------

// TestFinalizePublishShapeAndEvidenceRefusals proves the pre-effect gates: a
// malformed request shape, and evidence that does not certify the requested head,
// both refuse before any workspace or GitHub effect.
func TestFinalizePublishShapeAndEvidenceRefusals(t *testing.T) {
	requireRealGit(t)
	f := setupPublishFixture(t, planRepoModes()[0])
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, authoredPRBody(t, f.origHead))}

	// A malformed head is a shape refusal carrying findings, before any effect.
	bad := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
		FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: "not-hex", EvidenceRecord: []byte("x")})
	if bad.Result != ResultInvalidInput || len(bad.Findings) == 0 {
		t.Fatalf("malformed head = %q findings %v, want invalid-input with findings", bad.Result, bad.Findings)
	}

	// Evidence for a DIFFERENT head does not certify the requested head: refused.
	_, staleBytes := recFor(t, f.origHead) // certifies the old head, not the rewritten one
	stale := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
		FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: staleBytes})
	if stale.Result != ResultInvalidState || stale.Reason != ReasonPublishEvidenceUnverified {
		t.Fatalf("stale evidence = (%q, %q), want invalid-state/evidence-unverified", stale.Result, stale.Reason)
	}
	// Neither refusal pushed or edited anything.
	if tip := f.remoteFeatureTip(t); tip != f.origHead {
		t.Errorf("a pre-effect refusal pushed to the remote: tip %q", tip)
	}
	if gh.ensNext != 0 {
		t.Errorf("a pre-effect refusal issued %d PR edit(s); want 0", gh.ensNext)
	}
}
