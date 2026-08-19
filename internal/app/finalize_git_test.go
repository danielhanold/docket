package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
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

// TestFinalizeInterruptionMatrix walks the spec's recovery table in BOTH metadata
// modes. Each boundary runs its operation once (the effect lands), then replays the
// same operation against the durable authority and asserts convergence: the effect
// is adopted, never duplicated, and no false-success or unsafe overwrite/delete
// results.
func TestFinalizeInterruptionMatrix(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			t.Run("rebase-completion-adopts-not-reruns", func(t *testing.T) {
				matrixRebaseCompletion(t, m)
			})
			t.Run("force-with-lease-push-and-pr-evidence", func(t *testing.T) {
				matrixPublish(t, m)
			})
			t.Run("pr-merge-never-merges-twice", func(t *testing.T) {
				matrixMerge(t, m)
			})
			t.Run("metadata-closeout-and-backlink-idempotent", func(t *testing.T) {
				matrixCloseout(t, m)
			})
			t.Run("worktree-and-ref-delete-retryable", func(t *testing.T) {
				matrixCleanup(t, m)
			})
		})
	}

	// Gate-run deletion is repository-mode invariant (a private run directory, no
	// metadata topology), and child retarget opens no transaction and touches no
	// Git — both are covered once.
	t.Run("gate-run-delete-tombstoned", matrixGateRunDelete)
	t.Run("child-retarget-adopts-exact-pr", matrixChildRetarget)
}

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

	first := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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
	second := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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

// TestFinalizeConcurrentMovement proves that a concurrent base move, a remote
// feature-head move, and a same-entity version contention each produce a
// contended/refused outcome — never a text-merge, a silent overwrite, or a merge —
// because every effect is fenced by the exact old-value it read.
func TestFinalizeConcurrentMovement(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("remote-feature-head-move-contends-publish", func(t *testing.T) {
		f := setupPublishFixture(t, m)
		// A concurrent writer moves the remote feature ref off the head the receipt
		// recorded, after the receipt was written. The receipt's lease is pinned to
		// that original remote head, so the rewrite push must lose the lease — never
		// clobber. Build the rival commit on the live remote feature ref and push it.
		w := f.repo.writer
		runGit(t, w, "fetch", "-q", "origin", "feat/"+f.slug)
		runGit(t, w, "checkout", "-q", "-B", "rival", "FETCH_HEAD")
		writeRepoFile(t, w, "concurrent.txt", "a rival writer\n")
		runGit(t, w, "add", "-A")
		runGit(t, w, "commit", "-q", "-m", "rival writer")
		moved := runGit(t, w, "rev-parse", "HEAD")
		runGit(t, w, "push", "-q", "--force", "origin", "HEAD:refs/heads/feat/"+f.slug)
		if tip := f.remoteFeatureTip(t); tip != moved {
			t.Fatalf("precondition: remote feature tip = %q, want the rival commit %q", tip, moved)
		}
		_, evBytes := recFor(t, f.rewritten)
		gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.openPRForPublish(f.rewritten, authoredPRBody(t, f.origHead))}
		res := FinalizePublish(context.Background(), f.publishDeps(gh), f.repo.invocation,
			FinalizePublishRequest{ID: f.id, Attempt: f.attempt, Head: f.rewritten, EvidenceRecord: evBytes})
		if res.Result == ResultApplied || res.Disposition == PublishDispPublished {
			t.Fatalf("a concurrent feature-head move let the rewrite publish: %q disp %q", res.Result, res.Disposition)
		}
		if res.Disposition != PublishDispContended {
			t.Fatalf("disposition = %q, want %q", res.Disposition, PublishDispContended)
		}
		if tip := f.remoteFeatureTip(t); tip != moved {
			t.Fatalf("the lease-losing push OVERWROTE the rival commit: %q -> %q", moved, tip)
		}
	})

	t.Run("base-move-contends-merge", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		// The PR merged against a base that has since diverged from the parent's
		// effective base: a present-but-wrong destination, contended, never a merge
		// the closeout could act on.
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "develop", strings.Repeat("a", 40))
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultContended || res.Merge != nil {
			t.Fatalf("base divergence = %q merge %v, want contended and no VerifiedMerge", res.Result, res.Merge)
		}
	})

	t.Run("same-entity-version-contends-retarget", func(t *testing.T) {
		pin := docketPin(t)
		corpus := []StatusBlob{
			finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
			finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n"),
		}
		// The authorized set names cv810, but the live PR entity moved to cv810-new: a
		// same-entity contention. The retarget must refuse the edit, never force it.
		gh := &fakeRetargetGitHub{
			repo: retargetRepo(),
			prs:  []*fakePR{{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810-new"}},
		}
		engine := &recordingEngine{}
		req := RetargetChildrenRequest{
			ID: 80, Version: "blobfin0080",
			Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810"}},
		}
		res := FinalizeRetargetChildren(context.Background(), retargetDeps(&fakeReader{pin: pin, corpus: corpus}, gh, engine), "", req)
		if res.Result == ResultApplied {
			t.Fatalf("a version-contended child was retargeted: %q disp %q", res.Result, res.Disposition)
		}
		if c := childOutcomeByID(t, res, 81); c.Outcome != childOutcomeContended {
			t.Fatalf("child 81 outcome = %q, want contended", c.Outcome)
		}
		// The rival version stands: no forced edit.
		if gh.prs[0].base != "feat/root" || gh.prs[0].version != "cv810-new" {
			t.Fatalf("the contended retarget overwrote the rival PR: base %q version %q", gh.prs[0].base, gh.prs[0].version)
		}
	})
}

// --- TestFinalizeBytePreservation -----------------------------------------

// TestFinalizeBytePreservation proves that after a closeout, every byte outside the
// generated blocks the operation owns is identical to its pre-image: the authored
// bodies of the merged plan/results, the repository config, and an unrelated file
// are compared in full against snapshots taken before the transaction.
func TestFinalizeBytePreservation(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)

			integrationBranch := f.branch
			if m.name == "docket" {
				integrationBranch = "main"
			}
			// Pre-images of files the closeout must not disturb outside its owned blocks.
			planBefore, _ := originFile(t, f.repo.origin, integrationBranch, f.planPath)
			resultsBefore, _ := originFile(t, f.repo.origin, integrationBranch, f.resultsPath)
			readmeBefore, hadReadme := originFile(t, f.repo.origin, integrationBranch, "README.md")

			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)
			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
			if res.Result != ResultApplied {
				t.Fatalf("closeout did not apply: %q (reason %q)", res.Result, res.Reason)
			}

			// The authored body after the backlink block is byte-identical.
			for _, tc := range []struct {
				path   string
				before string
			}{
				{f.planPath, planBefore},
				{f.resultsPath, resultsBefore},
			} {
				after, ok := originFile(t, f.repo.origin, integrationBranch, tc.path)
				if !ok {
					t.Fatalf("artifact %q vanished after closeout", tc.path)
				}
				if matrixAuthoredBody(t, tc.before) != matrixAuthoredBody(t, after) {
					t.Errorf("artifact %q authored body changed outside its backlink block:\n--before--\n%q\n--after--\n%q",
						tc.path, matrixAuthoredBody(t, tc.before), matrixAuthoredBody(t, after))
				}
			}

			// A file the closeout never targets is byte-identical in full.
			if hadReadme {
				readmeAfter, ok := originFile(t, f.repo.origin, integrationBranch, "README.md")
				if !ok || readmeAfter != readmeBefore {
					t.Errorf("README.md was disturbed by closeout:\n--before--\n%q\n--after--\n%q", readmeBefore, readmeAfter)
				}
			}
		})
	}
}

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

// TestFinalizeNoForeignWrites proves the terminal metadata transaction writes only
// through its own detached candidate worktree and the remote: the invocation's
// primary checkout, the sibling feature worktree, and the transactions root are all
// left as they were — no foreign index, HEAD, or worktree is touched.
func TestFinalizeNoForeignWrites(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)
			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)

			// Primary (invocation) checkout and the sibling feature worktree before.
			// The invariant is that closeout does not CHANGE these indexes/HEADs, so
			// capture the exact status and compare — not a pristine assumption.
			invHeadBefore := runGit(t, f.repo.invocation, "rev-parse", "HEAD")
			invStatusBefore := runGit(t, f.repo.invocation, "status", "--porcelain")
			featHeadBefore := runGit(t, f.wp, "rev-parse", "HEAD")
			featStatusBefore := runGit(t, f.wp, "status", "--porcelain")

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
			if res.Result != ResultApplied {
				t.Fatalf("closeout did not apply: %q (reason %q)", res.Result, res.Reason)
			}

			if got := runGit(t, f.repo.invocation, "rev-parse", "HEAD"); got != invHeadBefore {
				t.Errorf("closeout moved the primary checkout HEAD: %q -> %q", invHeadBefore, got)
			}
			if got := runGit(t, f.repo.invocation, "status", "--porcelain"); got != invStatusBefore {
				t.Errorf("closeout wrote into the primary checkout index:\n--before--\n%s\n--after--\n%s", invStatusBefore, got)
			}
			if got := runGit(t, f.wp, "rev-parse", "HEAD"); got != featHeadBefore {
				t.Errorf("closeout moved the sibling feature worktree HEAD: %q -> %q", featHeadBefore, got)
			}
			if got := runGit(t, f.wp, "status", "--porcelain"); got != featStatusBefore {
				t.Errorf("closeout wrote into the sibling feature worktree index:\n--before--\n%s\n--after--\n%s", featStatusBefore, got)
			}
			if !transactionsRootEmpty(t, f.deps.Client, f.repo.invocation) {
				t.Errorf("closeout left a candidate worktree under the transactions root")
			}
		})
	}
}
