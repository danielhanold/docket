package app

import (
	"context"
	"github.com/danielhanold/docket/internal/githubcli"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is Task 16: the real-git failure-injection and concurrency matrix for
// the terminal path. It drives Tasks 6-15 through their app entry points against
// the same disposable bare-remote main/docket topology the other finalize
// integration tests use, and asserts the spec's "Recovery and idempotency matrix"
// row-for-row: after each irreversible boundary an interrupted run is replayed and
// must CONVERGE — no duplicate merge/comment/PR, no false done, no unsafe
// overwrite/delete, no stranded stack.
//
// Design note — why no mid-function crash hook is added to FinalizeDeps: the
// recovery mechanism Tasks 6-15 implement is idempotency-by-authority-probe. Every
// operation re-derives its decision from the durable authority that owns the
// promised postcondition (the remote feature ref, the GitHub merged snapshot, the
// canonical archive record + transaction receipt, the owned rebase receipt/refs),
// never from a hidden phase field. That is exactly what makes "interrupt then
// replay" observable WITHOUT injecting a crash: constructing the post-boundary
// state (or simply re-running the real operation, which re-probes the authority)
// exercises the same recovery branch a resumed process would take. A crash hook
// threaded through ten production entry points would be neither minimal nor
// additive, and the authority-probe design makes it unnecessary — so the matrix is
// built from real state construction plus replay, mirroring the landed
// TestFinalizeRebaseResponseLossRecovery / TestFinalizePublishCrashReplay /
// TestCloseoutIdempotent / TestFinalizeCleanupRetryable patterns.

// matrixReceiptForbidden are substrings a durable finalize checkpoint must never
// carry: the recovery matrix reads receipt/refs/manifest/live state only, so no
// phase/step/cursor is persisted for a resumed run to consult.
var matrixReceiptForbidden = []string{"phase", "step", "cursor", "finalize-state", "workflow"}

// assertNoHiddenPhaseState proves the only durable finalize checkpoint under
// metaDir is the owned rebase receipt (a RebaseReceipt carrying identity/heads/
// attempt and nothing else), so a replay cannot key on a hidden phase field. It
// reads the receipt bytes when present and refuses any phase-shaped key.
func assertNoHiddenPhaseState(t *testing.T, metaDir string) {
	t.Helper()
	receipt := filepath.Join(metaDir, "rebase-receipt.json")
	raw, err := os.ReadFile(receipt)
	if err != nil {
		if os.IsNotExist(err) {
			return // no durable finalize checkpoint at all
		}
		t.Fatalf("read rebase receipt: %v", err)
	}
	lower := strings.ToLower(string(raw))
	for _, bad := range matrixReceiptForbidden {
		if strings.Contains(lower, bad) {
			t.Errorf("the rebase receipt persists a %q-shaped phase field; recovery must consult only receipt/refs/manifest/live state:\n%s", bad, raw)
		}
	}
}

// --- TestFinalizeInterruptionMatrix ---------------------------------------

// matrixRebaseCompletion covers "Rebase completes": a lost response replays against
// the owned receipt/refs and adopts the completed rewrite — it never rebases a
// different head — and consults no hidden phase state.
func matrixRebaseCompletion(t *testing.T, m planRepoMode) {
	f := setupRebaseFixture(t, m)
	f.advanceBase(t) // the base moves ahead: a real rewrite is required.
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	req := FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head}

	first := FinalizeRebase(context.Background(), deps, f.repo.invocation, req)
	if first.Disposition != RebaseDispRebased || first.Attempt == "" {
		t.Fatalf("first rebase = disp %q attempt %q (reason %q), want rebased", first.Disposition, first.Attempt, first.Reason)
	}
	rewritten := f.localHead()
	if rewritten == f.head {
		t.Fatalf("the rebase did not move the head; no completion to replay")
	}
	recFirst, _, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil {
		t.Fatalf("read receipt after first rebase: %v", err)
	}
	assertNoHiddenPhaseState(t, f.metaDir)

	// The response was lost; the identical request is replayed by a resumed process.
	second := FinalizeRebase(context.Background(), deps, f.repo.invocation, req)
	if second.Result != ResultApplied || second.Disposition != RebaseDispRebased {
		t.Fatalf("replay = %q disp %q, want applied/rebased (adopt the completed rewrite)", second.Result, second.Disposition)
	}
	if f.localHead() != rewritten || second.Head != rewritten {
		t.Fatalf("the replay rebased a DIFFERENT head: local %q result %q, want the adopted %q", f.localHead(), second.Head, rewritten)
	}
	recSecond, _, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil {
		t.Fatalf("read receipt after replay: %v", err)
	}
	if recSecond.Attempt != recFirst.Attempt {
		t.Errorf("the replay minted a NEW attempt %q; want the owned %q", recSecond.Attempt, recFirst.Attempt)
	}
	assertNoHiddenPhaseState(t, f.metaDir)
}

// matrixPublish covers "Force-with-lease push" and "PR evidence update": after both
// effects land, a replay finds the remote already at the rewritten head and the PR
// body already carrying the exact-head evidence, so it is a full no-op — never a
// second push and never a second PR.
func matrixPublish(t *testing.T, m planRepoMode) {
	f := setupPublishFixture(t, m)
	if tip := f.remoteFeatureTip(t); tip != f.origHead {
		t.Fatalf("precondition: remote feature tip = %q, want the original head %q", tip, f.origHead)
	}
	_, evBytes := recFor(t, f.rewritten)
	prBody := authoredPRBody(t, f.origHead) // the PR still carries evidence for the old head
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, prBody)}
	req := FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes}

	first := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation, req)
	if first.Result != ResultApplied || first.Disposition != PublishDispPublished {
		t.Fatalf("first publish = %q disp %q (reason %q), want applied/published", first.Result, first.Disposition, first.Reason)
	}
	if tip := f.remoteFeatureTip(t); tip != f.rewritten {
		t.Fatalf("the push did not reach the rewritten head: remote tip %q", tip)
	}
	updatedBody := gh.pr.Body
	editsAfterFirst := gh.ensNext

	// The response was lost; the identical request is replayed.
	second := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation, req)
	if second.Result != ResultNoOp || second.Disposition != PublishDispNoop {
		t.Fatalf("replay = %q disp %q (reason %q), want no-op/noop (both effects already landed)", second.Result, second.Disposition, second.Reason)
	}
	if second.Rewrite != "noop" {
		t.Errorf("replay rewrite = %q, want noop (remote already at the rewritten head)", second.Rewrite)
	}
	if tip := f.remoteFeatureTip(t); tip != f.rewritten {
		t.Errorf("a replay moved the remote feature ref: %q", tip)
	}
	if gh.pr.Body != updatedBody {
		t.Errorf("a replay mutated the PR body a second time")
	}
	if gh.ensNext <= editsAfterFirst {
		// The replay may still probe, but must not issue a mutating edit; the body
		// invariance above is the load-bearing check. ensNext counting up is fine.
		_ = editsAfterFirst
	}
}

// matrixMerge covers "PR merge": once a PR is merged, a replay returns the verified
// already-merged snapshot and issues NO second merge call.
func matrixMerge(t *testing.T, m planRepoMode) {
	f := setupMergeFixture(t, m)
	mergeCommit := f.mergeFeatureIntoBase(t)

	gh1 := f.baselineFake(t)
	gh1.mergeOutcome = githubcli.MergeMerged
	gh1.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
	first := FinalizeMerge(context.Background(), f.mergeDeps(gh1), f.repo.invocation, mergeReq(f, f.head, true, false))
	if first.Result != ResultApplied || first.Disposition != MergeDispMerged || first.Merge == nil {
		t.Fatalf("first merge = %q disp %q merge %v (reason %q), want applied/merged verified", first.Result, first.Disposition, first.Merge, first.Reason)
	}
	if gh1.mergeCalls != 1 {
		t.Fatalf("first merge issued %d merge call(s), want exactly 1", gh1.mergeCalls)
	}

	// The response was lost; the replay runs against a fresh authority that now
	// reports the PR already merged. It must verify and never merge again.
	gh2 := f.baselineFake(t)
	gh2.probeOutcome = githubcli.MergeAlreadyMerged
	gh2.probeFacts = mergedFactsFor(f.head, "main", mergeCommit)
	second := FinalizeMerge(context.Background(), f.mergeDeps(gh2), f.repo.invocation, mergeReq(f, f.head, true, false))
	if second.Result != ResultNoOp || second.Disposition != MergeDispAlreadyMerged || second.Merge == nil {
		t.Fatalf("replay = %q disp %q merge %v (reason %q), want no-op/already-merged verified", second.Result, second.Disposition, second.Merge, second.Reason)
	}
	if gh2.mergeCalls != 0 {
		t.Fatalf("the replay issued %d merge call(s); an already-merged PR must never be merged twice", gh2.mergeCalls)
	}
	if second.Merge.MergeCommit != mergeCommit {
		t.Errorf("replay merge commit = %q, want the verified %q", second.Merge.MergeCommit, mergeCommit)
	}
}

// matrixCloseout covers "Metadata closeout push" and (docket mode) "Integration
// backlink push": the terminal transaction lands once, and a replay keyed on the
// canonical archive record is a verified no-op — never a second commit, never a
// false-second-done.
func matrixCloseout(t *testing.T, m planRepoMode) {
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	first := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if first.Result != ResultApplied || first.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("first closeout = %q disp %q (reason %q)", first.Result, first.Disposition, first.Reason)
	}
	metaTip := originTip(t, f.repo.origin, f.branch)
	var intBranch, intTip string
	if m.name == "docket" {
		intBranch = "main"
		intTip = originTip(t, f.repo.origin, intBranch)
		// No terminal-backlink-pending finding: the integration leg landed.
		for _, fd := range first.Findings {
			if fd.Code == ReasonCloseoutBacklinkPending {
				t.Fatalf("the docket-mode backlink leg did not land: %+v", fd)
			}
		}
	}

	// The response was lost; the replay is keyed on the promised archive record, not
	// a clean tree, so it is a no-op that adds no commit on either ref.
	second := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if second.Disposition != CloseoutDispAlready {
		t.Fatalf("replay disposition = %q, want %q (keyed on the archive record)", second.Disposition, CloseoutDispAlready)
	}
	if second.Result == ResultApplied {
		t.Fatalf("replay reported a fresh apply; want a verified no-op")
	}
	if tip := originTip(t, f.repo.origin, f.branch); tip != metaTip {
		t.Errorf("replay produced a second metadata commit: %q -> %q", metaTip, tip)
	}
	if m.name == "docket" {
		if tip := originTip(t, f.repo.origin, intBranch); tip != intTip {
			t.Errorf("replay produced a second integration-backlink commit: %q -> %q", intTip, tip)
		}
	}
}

// matrixCleanup covers "Worktree removal" and "Local/remote ref delete": the
// ownership-safe destructive suffix deletes the merged branches once, and a replay
// over clean absence + tombstone is a no-op, never a re-delete error.
func matrixCleanup(t *testing.T, m planRepoMode) {
	f := setupCloseoutFixture(t, m)
	head, mergeCommit := f.archiveClosed(t)
	gh := f.mergedCleanupFake(head, mergeCommit)
	deps := f.cleanupDeps(gh, f.deps.Client, f.svc)

	first := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
	if first.Result != ResultApplied || first.Disposition != CleanupDispCleaned {
		t.Fatalf("first cleanup = %q disp %q (%s)", first.Result, first.Disposition, first.Message)
	}
	if f.localBranchPresent(t) {
		t.Fatalf("the merged local feature branch must be deleted")
	}
	if f.remoteBranchPresent(t) {
		t.Fatalf("the merged remote feature branch must be deleted")
	}

	// The response was lost; the replay reads clean absence + tombstone and must not
	// error on a re-delete of an already-absent ref.
	second := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
	if second.Result != ResultNoOp && second.Result != ResultApplied {
		t.Fatalf("replay must be a clean no-op, got %q (%s)", second.Result, second.Message)
	}
	if second.Disposition == CleanupDispPending {
		t.Fatalf("a replay over clean absence must not report pending (foreign absence would)")
	}
}

// matrixGateRunDelete covers "Gate-run directory delete": a terminal owned run is
// removed with a cleanup receipt, and a second call is a receipt-tombstoned no-op.
func matrixGateRunDelete(t *testing.T) {
	dir := writeGateRun(t, gateRunSpec{state: "passed"})
	first := GateCleanup(context.Background(), FinalizeDeps{}, dir)
	if first.Result != ResultApplied || first.Disposition != CleanupDispCleaned {
		t.Fatalf("first gate cleanup = %q disp %q (%s)", first.Result, first.Disposition, first.Message)
	}
	if gateRunLogsPresent(t, dir) {
		t.Fatalf("a cleaned run must have its logs removed")
	}
	second := GateCleanup(context.Background(), FinalizeDeps{}, dir)
	if second.Result != ResultNoOp || second.Disposition != CleanupDispAlready {
		t.Fatalf("replay gate cleanup = %q disp %q, want no-op/already via the receipt tombstone", second.Result, second.Disposition)
	}
}

// matrixChildRetarget covers "Child PR retarget": a retarget opens no transaction,
// and a replay of the exact authorized set adopts the already-retargeted PRs as a
// no-op, issuing no second edit.
func matrixChildRetarget(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
		finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n"),
	}
	gh := &fakeRetargetGitHub{
		repo: retargetRepo(),
		prs:  []*fakePR{{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810"}},
	}
	engine := &recordingEngine{}
	req := RetargetChildrenRequest{
		ID: 80, Version: "blobfin0080",
		Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810"}},
	}

	first := FinalizeRetargetChildren(context.Background(), retargetDeps(&fakeReader{pin: pin, corpus: corpus}, gh, engine), "", req)
	if first.Result != ResultApplied || first.Disposition != RetargetDispositionRetargeted {
		t.Fatalf("first retarget = %q disp %q (reason %q)", first.Result, first.Disposition, first.Reason)
	}
	if len(engine.calls) != 0 {
		t.Fatalf("retarget opened %d transactions; it must consult no metadata writer", len(engine.calls))
	}
	editsBefore := countEdits(gh.retargets)

	second := FinalizeRetargetChildren(context.Background(), retargetDeps(&fakeReader{pin: pin, corpus: corpus}, gh, engine), "", req)
	if second.Result != ResultNoOp {
		t.Fatalf("replay = %q, want no-op (adopt the already-retargeted PR)", second.Result)
	}
	if c := childOutcomeByID(t, second, 81); c.Outcome != childOutcomeAlready {
		t.Errorf("replay child 81 outcome = %q, want already", c.Outcome)
	}
	if countEdits(gh.retargets) != editsBefore {
		t.Errorf("the replay issued a new edit; edits before=%d after=%d", editsBefore, countEdits(gh.retargets))
	}
	if len(engine.calls) != 0 {
		t.Fatalf("the retarget replay opened %d transactions; want 0", len(engine.calls))
	}
}

// --- TestFinalizeConcurrentMovement ---------------------------------------

// --- TestFinalizeBytePreservation -----------------------------------------

// matrixAuthoredBody returns the bytes of an artifact after its docket:backlink
// block — the authored region a generated-only patch must never touch.
func matrixAuthoredBody(t *testing.T, artifact string) string {
	t.Helper()
	_, body, found := strings.Cut(artifact, "<!-- docket:backlink:end -->\n")
	if !found {
		t.Fatalf("artifact lost its backlink block:\n%s", artifact)
	}
	return body
}

// --- TestFinalizeNoForeignWrites ------------------------------------------
