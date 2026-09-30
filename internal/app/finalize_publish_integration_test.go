//go:build integration

package app

import (
	"context"
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

// --- TestFinalizePublishCrashReplay ---------------------------------------

// --- TestFinalizePublishUnknownStops --------------------------------------

// --- TestFinalizePublishRefusesForeignAttempt -----------------------------

// --- TestFinalizePublishShapeAndEvidenceRefusals --------------------------

// TestIntegrationFinalizeOpsFinalizePublishAcceptsSkippedEvidence: a build.gate: off repository's
// truthful skipped evidence certifying the exact rewritten head passes
// FinalizePublish's evidence condition — the operation proceeds PAST it
// (VerdictSkipped is accepted exactly as VerdictVerified). Reverting the
// green-or-skipped acceptance would refuse here with ReasonPublishEvidenceUnverified,
// so this pins the verify-site change. (PR-body weaving of a skipped block is a
// separate concern: evidence.Upsert is green-only today, so the operation still
// fails later at body assembly — see the change notes.)
func TestIntegrationFinalizeOpsFinalizePublishAcceptsSkippedEvidence(t *testing.T) {
	requireRealGit(t)
	f := setupPublishFixture(t, planRepoModes()[0])
	// Land the push out of band so publish resumes only the PR update, exactly as
	// the green after-push replay case does.
	runGit(t, f.wp, "push", "--force", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
	if tip := f.remoteFeatureTip(t); tip != f.rewritten {
		t.Fatalf("precondition: remote tip = %q, want the rewritten head", tip)
	}
	skipped, err := evidence.NewSkippedRecord(f.rewritten, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewSkippedRecord: %v", err)
	}
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, authoredPRBody(t, f.origHead))}
	res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
		FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: []byte(evidence.Render(skipped))})
	if res.Reason == ReasonPublishEvidenceUnverified {
		t.Fatalf("skipped evidence at the exact head was refused at the evidence condition: %q", res.Message)
	}
}

// TestIntegrationFinalizeOpsFinalizePublishAfterCheckpointResume proves the reuse path end to end:
// a completed rewrite whose PASSED gate recorded a publish checkpoint, a
// denied publish (nothing pushed, PR untouched), a finalize resume that reuses
// the checkpoint WITHOUT re-running the suite, and a FinalizePublish driven by
// the reused evidence that lands the rewritten head under the exact lease and
// converges the PR body while preserving every authored byte outside the
// managed evidence block.
func TestIntegrationFinalizeOpsFinalizePublishAfterCheckpointResume(t *testing.T) {
	requireRealGit(t)
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.advanceBase(t)
	rebaseGH := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &headEvidenceGate{t: t}
	deps := f.finalizeDeps(rebaseGH, gate)

	// The rebase completes, the gate passes, the checkpoint is recorded — and
	// then the publish is denied: no push happens, the remote and PR still hold
	// the original head.
	first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Revision: f.revision, Head: f.head})
	if first.Disposition != RebaseDispRebased || gate.calls != 1 {
		t.Fatalf("setup rebase = disp %q calls %d, want rebased/1", first.Disposition, gate.calls)
	}
	rewritten := f.localHead()

	// The resume reuses the checkpoint: skipped compose, evidence returned, no
	// second suite run.
	resume := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Revision: f.revision, Head: f.head})
	if resume.Gate == nil || resume.Gate.Compose != gateComposeSkipped || gate.calls != 1 {
		t.Fatalf("resume gate = %+v calls %d, want skipped with no re-run", resume.Gate, gate.calls)
	}

	// The reused evidence drives finalize publish, exactly as the skill would.
	authored := "## Summary\n\nAuthored intro prose.\n\nAuthored outro prose.\n"
	pubGH := &fakePublishGitHub{repo: retargetRepo(), pr: githubcli.PullRequest{
		Number: 1, URL: "https://example.test/pr/1", State: githubcli.StateOpen,
		HeadBranch: "feat/" + f.slug, HeadCommit: rewritten, BaseBranch: "main",
		Title: "Add the widget", Body: authored, Revision: "sha256:" + strings.Repeat("d", 64),
	}}
	pres := FinalizePublish(context.Background(), FinalizeDeps{Planning: f.deps, GitHub: pubGH, Workspace: f.svc},
		f.repo.invocation, FinalizePublishRequest{
			ID: f.id, Attempt: resume.Attempt, Head: rewritten,
			EvidenceRecord: []byte(resume.Gate.Evidence),
		})
	if pres.Result != ResultApplied || pres.Disposition != PublishDispPublished {
		t.Fatalf("publish = %q disp %q (reason %q msg %q), want applied/published", pres.Result, pres.Disposition, pres.Reason, pres.Message)
	}
	if tip := runGit(t, f.repo.origin, "rev-parse", "refs/heads/feat/"+f.slug); tip != rewritten {
		t.Errorf("origin feature tip = %q, want the rewritten head %q", tip, rewritten)
	}
	// The authored bytes survived; the managed block certifies the rewritten head.
	body := pubGH.lastEnsuredBody()
	if !strings.Contains(body, "Authored intro prose.") || !strings.Contains(body, "Authored outro prose.") {
		t.Errorf("authored PR-body bytes were not preserved:\n%s", body)
	}
	got, err := evidence.Extract([]byte(body))
	if err != nil || got.Head != rewritten {
		t.Errorf("converged evidence head = %q err=%v, want %q", got.Head, err, rewritten)
	}
}
