//go:build integration

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
	"strings"
	"testing"
)

// TestFinalizeRebaseAbortVerifiesRestore proves abort proves the owned attempt,
// restores the recorded original head, clears the owned scratch, and returns a
// blocked disposition recommending the human finalize-block — with the report body
// never echoed.
func TestIntegrationFinalizeRebaseRecoveryAbortVerifiesRestore(t *testing.T) {
	requireRealGit(t)
	f, conflicted, deps := setupConflictedRebase(t, planRepoModes()[0])
	attempt := conflicted.Attempt
	ctx := context.Background()

	// A wrong attempt cannot abort someone else's rewrite.
	wrong := FinalizeRebaseAbort(ctx, deps, f.repo.invocation, f.id, "not-the-attempt",
		ResolverReport{ChangeID: f.id, Attempt: "not-the-attempt", Disposition: ResolverStuck})
	assertRebaseRefused(t, wrong, ResultBlocked, ReasonRebaseAttemptMismatch)

	secret := "SECRET internal path detail that must never be echoed"
	res := FinalizeRebaseAbort(ctx, deps, f.repo.invocation, f.id, attempt,
		ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverStuck, Summary: secret, RecommendedAction: secret})
	if res.Result != ResultApplied || res.Disposition != RebaseDispBlocked {
		t.Fatalf("abort = %q disp %q, want applied/blocked", res.Result, res.Disposition)
	}
	if strings.Contains(res.Message, secret) {
		t.Errorf("the abort result echoed the report body: %q", res.Message)
	}
	// The original head is restored and the rebase is no longer in progress.
	if f.localHead() != f.head {
		t.Errorf("abort did not restore the original head: %q != %q", f.localHead(), f.head)
	}
	if st, _ := f.deps.Client.RebaseState(ctx, f.wp); st.Disposition != gitcli.RebaseUnchanged {
		t.Errorf("a rebase is still in progress after abort: %q", st.Disposition)
	}
	// The owned scratch is cleared.
	f.receiptAbsent(t)
	if _, err := tryGit(f.wp, "rev-parse", "--verify", "refs/docket/finalize/5/orig"); err == nil {
		t.Errorf("the owned orig ref survived abort")
	}
}

// TestFinalizeRebaseContinueValidatesReport proves the resolver report is verified
// against the live unmerged set: a wrong attempt, a non-resolved disposition, and
// a path outside the unmerged set all refuse without staging, while a valid report
// stages exactly the named paths and completes the rebase.
func TestIntegrationFinalizeRebaseRecoveryContinueValidatesReport(t *testing.T) {
	requireRealGit(t)
	f, conflicted, deps := setupConflictedRebase(t, planRepoModes()[0])
	attempt := conflicted.Attempt
	ctx := context.Background()

	// A budgeted continue requires an outstanding reservation (change 0349): reserve
	// one dispatch first, then echo its token in the report.
	reserve := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, attempt)
	if reserve.Disposition != ReserveReserved || reserve.Reservation == "" {
		t.Fatalf("reserve = disp %q token %q (reason %q), want reserved with a token", reserve.Disposition, reserve.Reservation, reserve.Reason)
	}
	token := reserve.Reservation
	goodReport := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}

	// A wrong attempt token refuses.
	wrong := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, "not-the-attempt", goodReport)
	assertRebaseRefused(t, wrong, ResultBlocked, ReasonRebaseAttemptMismatch)

	// A non-resolved report refuses (route through abort).
	stuck := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt,
		ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverStuck, ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token})
	assertRebaseRefused(t, stuck, ResultInvalidInput, ReasonRebaseReportDisposition)

	// A path outside the live unmerged set refuses (the reservation is valid).
	badPaths := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt,
		ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved, ConflictedPaths: []string{"not-conflicted.txt"}, ResolverReservation: token})
	assertRebaseRefused(t, badPaths, ResultInvalidInput, ReasonRebaseReportPaths)

	// The rebase is still live and conflicted: the refusals staged nothing.
	if st, _ := f.deps.Client.RebaseState(ctx, f.wp); st.Disposition != gitcli.RebaseConflicted {
		t.Fatalf("a refused continue changed the rebase state to %q", st.Disposition)
	}

	// The resolver resolves the file; a valid report stages exactly it and completes.
	writeRepoFile(t, f.wp, "feature.txt", "reconciled content\n")
	done := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, goodReport)
	if done.Result != ResultApplied || done.Disposition != RebaseDispRebased {
		t.Fatalf("valid continue = %q disp %q (reason %q msg %q), want applied/rebased", done.Result, done.Disposition, done.Reason, done.Message)
	}
	if st, _ := f.deps.Client.RebaseState(ctx, f.wp); st.Disposition != gitcli.RebaseUnchanged {
		t.Errorf("the rebase did not complete; state %q", st.Disposition)
	}
	// The completed continue reconciled (cleared) the reservation, keeping used.
	rec, _, _ := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
	if rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("a completed continue left the reservation outstanding: token %q cont %q", rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
	if rec.ResolverUsed != "1" {
		t.Errorf("used = %q after continue, want 1 preserved", rec.ResolverUsed)
	}
}

// TestFinalizeRebaseForeignStateBlocked proves a moved base (a resumed rewrite
// whose base drifted) and a pre-existing foreign rebase are both retained and
// blocked, never reset or adopted.
func TestIntegrationFinalizeRebaseGateForeignStateBlocked(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("divergent-base", func(t *testing.T) {
		// A REWRITTEN (divergent) base — one that does NOT descend the recorded base —
		// stays retained and blocked with base-moved-under-receipt: change 0438 only
		// forward-refreshes onto a base that descends the recorded one; a divergent
		// base is out of scope (spec §1). (A forward base advance under a completed
		// rewrite now refreshes instead of blocking; see the ForwardRefresh tests.)
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
		deps := f.finalizeDeps(gh, gate)
		first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if first.Disposition != RebaseDispRebased {
			t.Fatalf("first call = %q, want rebased", first.Disposition)
		}
		rewritten := f.localHead()

		// The base tip is amended and force-pushed, so the new base does NOT descend
		// the recorded base.
		writeRepoFile(t, f.repo.writer, "divergent.txt", "rewritten base tip\n")
		runGit(t, f.repo.writer, "add", "-A")
		runGit(t, f.repo.writer, "commit", "-q", "--amend", "--no-edit")
		runGit(t, f.repo.writer, "push", "-q", "-f", "origin", "main")

		res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseMovedBase)
		if f.localHead() != rewritten {
			t.Errorf("a divergent-base refusal reset the head: %q -> %q", rewritten, f.localHead())
		}
	})

	t.Run("foreign-rebase-in-progress", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		// A foreign rebase, started outside this operation, is left conflicted.
		f.repo.writerAdvance(t, "main", map[string]string{"feature.txt": "base-version conflicting\n"})
		runGit(t, f.wp, "fetch", "-q", "origin", "main")
		_, _ = tryGit(f.wp, "rebase", "origin/main") // conflicts, leaving a rebase in progress
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseForeignInProgress)
		f.receiptAbsent(t)
	})
}

// TestFinalizeRebaseGateOutcomes proves the gate composition maps each terminal:
// skip on a no-op with exact-head green evidence, passed to evidence, failed to
// repair work, and every non-decidable observation to a retained halt — never a
// fabricated red.
func TestIntegrationFinalizeRebaseGateOutcomes(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("skip-on-noop-exact-green", func(t *testing.T) {
		f := setupRebaseFixture(t, main) // base unmoved: the rebase is a no-op.
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, greenEvidenceFor(t, f.head))}}
		gate := &fakeGate{}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispUnchanged || res.Gate == nil || res.Gate.Compose != gateComposeSkipped {
			t.Fatalf("noop+green = disp %q gate %+v, want unchanged/skipped", res.Disposition, res.Gate)
		}
		if gate.calls != 0 {
			t.Errorf("the suite was launched %d time(s) on a skip; want 0", gate.calls)
		}
		if res.Gate.Permit != f.head {
			t.Errorf("skip permit = %q, want the exact evidence head %q", res.Gate.Permit, f.head)
		}
	})

	t.Run("passed-produces-evidence", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultApplied || res.Disposition != RebaseDispRebased || res.Gate.Evidence == "" {
			t.Fatalf("passed = %q disp %q evidence?%v, want applied/rebased with evidence", res.Result, res.Disposition, res.Gate.Evidence != "")
		}
	})

	t.Run("failed-is-repair-work", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateFailed, RunDir: "/run/x"}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultGateFailed || res.Disposition != RebaseDispFailed || res.Reason != ReasonRebaseGateFailed {
			t.Fatalf("failed = (%q, %q, %q), want gate-failed/failed/gate-failed", res.Result, res.Disposition, res.Reason)
		}
		if res.Gate.Evidence != "" {
			t.Errorf("a failed gate produced evidence: %q", res.Gate.Evidence)
		}
	})

	t.Run("halt-at-budget-not-red", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltRunningAtBudget, RunDir: "/run/x"}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultBlocked || res.Disposition != RebaseDispBlocked || res.Reason != ReasonRebaseGateHalted {
			t.Fatalf("halt = (%q, %q, %q), want blocked/blocked/gate-halted", res.Result, res.Disposition, res.Reason)
		}
		if res.Gate.HaltCause != GateHaltRunningAtBudget {
			t.Errorf("halt cause = %q, want running-at-budget", res.Gate.HaltCause)
		}
	})

	t.Run("seam-error-is-halt-unavailable", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{err: errRebaseGateSeam}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultBlocked || res.Gate.HaltCause != GateHaltUnavailable {
			t.Fatalf("seam error = %q halt %q, want blocked/unavailable", res.Result, res.Gate.HaltCause)
		}
	})
}

// TestFinalizeRebaseGateWaiting proves the slice-bounded driver contract: a
// nonterminal WAITING slice returns a waiting disposition carrying the opaque
// continuation, mints NO evidence, and is NOT routed to integration-repair;
// re-entering the same local-gate phase with that continuation advances the SAME
// drive without repeating the completed rewrite, and a subsequent PASSED slice is
// the only outcome that mints evidence.
func TestIntegrationFinalizeRebaseGateWaiting(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("waiting-returns-continuation-no-evidence-no-repair", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		cont := GateContinuation{DriveID: "drive-1", Generation: "gen-1"}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateWaiting, Continuation: cont}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispWaiting {
			t.Fatalf("waiting disposition = %q, want %q (result %q reason %q)", res.Disposition, RebaseDispWaiting, res.Result, res.Reason)
		}
		if res.Result == ResultGateFailed || res.Disposition == RebaseDispFailed {
			t.Errorf("a WAITING slice was routed to repair; waiting is neither repair nor a red terminal")
		}
		if res.Gate == nil || res.Gate.Outcome != string(FinalizeGateWaiting) {
			t.Fatalf("gate report = %+v, want ran/waiting", res.Gate)
		}
		if res.Gate.Evidence != "" {
			t.Errorf("a WAITING slice minted evidence: %q", res.Gate.Evidence)
		}
		if res.Gate.Continuation == nil || *res.Gate.Continuation != cont {
			t.Fatalf("gate continuation = %+v, want %+v", res.Gate.Continuation, cont)
		}
		// The WAITING slice persisted the continuation pair into the owned receipt.
		rec, found, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if err != nil || !found {
			t.Fatalf("receipt after WAITING: found=%v err=%v", found, err)
		}
		if rec.GateDriveID != "drive-1" || rec.GateOwnerGeneration != "gen-1" {
			t.Errorf("WAITING did not persist the continuation pair: %q/%q", rec.GateDriveID, rec.GateOwnerGeneration)
		}
		if rec.Attempt != res.Attempt {
			t.Errorf("receipt attempt %q != result attempt %q", rec.Attempt, res.Attempt)
		}
		// With the gate pair cleared, every remaining (non-pair) field of the
		// persisted receipt must match what the result advertised: the WAITING
		// persist changed ONLY the pair, nothing else.
		bare := rec
		bare.GateDriveID, bare.GateOwnerGeneration = "", ""
		if bare.OrigHead != res.OrigHead || bare.BaseHead != res.BaseHead || bare.Attempt != res.Attempt {
			t.Errorf("WAITING persist altered a non-pair receipt field: receipt %+v vs result orig=%q base_head=%q attempt=%q",
				bare, res.OrigHead, res.BaseHead, res.Attempt)
		}
	})

	t.Run("resume-advances-same-drive-without-repeating-rebase-then-mints-on-passed", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t) // the base moved: a real rewrite is required.
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		cont := GateContinuation{DriveID: "drive-9", Generation: "gen-9"}
		gate := &seqGate{results: []LocalGateResult{
			{Outcome: FinalizeGateWaiting, Continuation: cont},
			{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"},
		}}
		deps := f.finalizeDeps(gh, gate)

		first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if first.Disposition != RebaseDispWaiting || first.Gate == nil || first.Gate.Continuation == nil {
			t.Fatalf("first call = disp %q gate %+v, want waiting with a continuation", first.Disposition, first.Gate)
		}
		rewritten := f.localHead()
		recFirst, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)

		// Re-enter with a BARE identical request: no caller-held continuation. The
		// owned receipt carries the drive, so the rebase must not be repeated; only
		// the gate advances.
		second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if second.Result != ResultApplied || second.Disposition != RebaseDispRebased {
			t.Fatalf("resume = %q disp %q (reason %q msg %q), want applied/rebased", second.Result, second.Disposition, second.Reason, second.Message)
		}
		if second.Gate == nil || second.Gate.Evidence == "" {
			t.Fatalf("resume produced no evidence on PASSED: %+v", second.Gate)
		}
		// The completed rewrite was NOT repeated: the head and the owned attempt token
		// are unchanged across the re-entry.
		if f.localHead() != rewritten {
			t.Fatalf("resume repeated the rewrite: %q -> %q", rewritten, f.localHead())
		}
		recSecond, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if recSecond.Attempt != recFirst.Attempt {
			t.Errorf("resume minted a new rebase attempt %q; want the owned %q", recSecond.Attempt, recFirst.Attempt)
		}
		// The gate saw exactly two slices: the first STARTED (no continuation), the
		// second RESUMED with the exact continuation (Advance semantics).
		if len(gate.reqs) != 2 {
			t.Fatalf("gate slices = %d, want 2 (one per re-entry)", len(gate.reqs))
		}
		if gate.reqs[0].Continuation != (GateContinuation{}) {
			t.Errorf("the first slice carried a continuation %+v; it must start a fresh drive", gate.reqs[0].Continuation)
		}
		if gate.reqs[1].Continuation != cont {
			t.Errorf("resume slice continuation = %+v, want the receipt-recorded %+v", gate.reqs[1].Continuation, cont)
		}
		// cleared in the same call that maps any terminal (Task 3)
		recAfter, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if recAfter.GateDriveID != "" || recAfter.GateOwnerGeneration != "" {
			t.Errorf("PASSED did not clear the continuation pair: %q/%q", recAfter.GateDriveID, recAfter.GateOwnerGeneration)
		}
	})

	t.Run("waiting-then-waiting-keeps-the-pair-and-resumes-the-same-drive", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		cont := GateContinuation{DriveID: "drive-5", Generation: "gen-5"}
		gate := &seqGate{results: []LocalGateResult{
			{Outcome: FinalizeGateWaiting, Continuation: cont},
			{Outcome: FinalizeGateWaiting, Continuation: cont},
			{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"},
		}}
		deps := f.finalizeDeps(gh, gate)
		req := FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head}
		ctx := context.Background()

		if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
			t.Fatalf("first = %q, want waiting (reason %q msg %q)", r.Disposition, r.Reason, r.Message)
		}
		if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
			t.Fatalf("second = %q, want waiting again (reason %q msg %q)", r.Disposition, r.Reason, r.Message)
		}
		rec, _, _ := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
		if rec.GateDriveID != "drive-5" || rec.GateOwnerGeneration != "gen-5" {
			t.Fatalf("WAITING->WAITING lost the pair: %q/%q", rec.GateDriveID, rec.GateOwnerGeneration)
		}
		third := FinalizeRebase(ctx, deps, f.repo.invocation, req)
		if third.Result != ResultApplied || third.Gate == nil || third.Gate.Evidence == "" {
			t.Fatalf("third = %q gate %+v, want applied with evidence", third.Result, third.Gate)
		}
		// Slices 2 and 3 both resumed the SAME recorded drive.
		if len(gate.reqs) != 3 || gate.reqs[1].Continuation != cont || gate.reqs[2].Continuation != cont {
			t.Fatalf("slice continuations = %+v, want the recorded %+v on slices 2 and 3", gate.reqs, cont)
		}
	})

	t.Run("failed-and-halted-terminals-clear-the-pair-next-run-starts-fresh", func(t *testing.T) {
		for name, terminal := range map[string]LocalGateResult{
			"failed": {Outcome: FinalizeGateFailed, RunDir: "/run/x"},
			"halted": {Outcome: FinalizeGateHalted, HaltCause: GateHaltRunningAtBudget},
		} {
			t.Run(name, func(t *testing.T) {
				f := setupRebaseFixture(t, main)
				f.advanceBase(t)
				gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
				cont := GateContinuation{DriveID: "drive-7", Generation: "gen-7"}
				gate := &seqGate{results: []LocalGateResult{
					{Outcome: FinalizeGateWaiting, Continuation: cont},
					terminal,
					{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "drive-8", Generation: "gen-8"}},
				}}
				deps := f.finalizeDeps(gh, gate)
				req := FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head}
				ctx := context.Background()

				if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
					t.Fatalf("first = %q, want waiting", r.Disposition)
				}
				second := FinalizeRebase(ctx, deps, f.repo.invocation, req)
				if name == "failed" && (second.Disposition != RebaseDispFailed || second.Reason != ReasonRebaseGateFailed) {
					t.Fatalf("failed terminal = %q/%q, want failed/gate-failed", second.Disposition, second.Reason)
				}
				if name == "halted" && (second.Disposition != RebaseDispBlocked || second.Reason != ReasonRebaseGateHalted) {
					t.Fatalf("halted terminal = %q/%q, want blocked/gate-halted", second.Disposition, second.Reason)
				}
				rec, _, _ := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
				if rec.GateDriveID != "" || rec.GateOwnerGeneration != "" {
					t.Fatalf("%s terminal did not clear the pair: %q/%q", name, rec.GateDriveID, rec.GateOwnerGeneration)
				}
				// A halt keeps blocked (a human is needed) but does not wedge the
				// receipt: the next deliberate re-run starts a FRESH drive.
				if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
					t.Fatalf("post-terminal re-run = %q, want a fresh waiting drive", r.Disposition)
				}
				if gate.reqs[2].Continuation != (GateContinuation{}) {
					t.Fatalf("post-terminal slice carried %+v; a cleared pair must start a fresh drive", gate.reqs[2].Continuation)
				}
			})
		}
	})

	t.Run("seam-error-clears-the-pair-too", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &seqGate{results: []LocalGateResult{
			{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "drive-2", Generation: "gen-2"}},
		}}
		deps := f.finalizeDeps(gh, gate)
		req := FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head}
		ctx := context.Background()
		if r := FinalizeRebase(ctx, deps, f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
			t.Fatalf("first = %q, want waiting", r.Disposition)
		}
		// Swap in an erroring seam for the resume slice: an unrecoverable seam
		// failure is a halt (unavailable) — a terminal for the recorded drive.
		deps2 := f.finalizeDeps(gh, &fakeGate{err: errRebaseGateSeam})
		second := FinalizeRebase(ctx, deps2, f.repo.invocation, req)
		if second.Result != ResultBlocked || second.Gate == nil || second.Gate.HaltCause != GateHaltUnavailable {
			t.Fatalf("seam error = %q gate %+v, want blocked/unavailable", second.Result, second.Gate)
		}
		rec, _, _ := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
		if rec.GateDriveID != "" || rec.GateOwnerGeneration != "" {
			t.Errorf("seam-error halt did not clear the pair: %q/%q", rec.GateDriveID, rec.GateOwnerGeneration)
		}
	})

	t.Run("recorded-live-continuation-overrides-the-evidence-skip", func(t *testing.T) {
		// A pair recorded by a WAITING slice means a drive is LIVE for this
		// attempt; goal 3 forbids leaving it dangling. Even if the PR body now
		// carries exact-head green evidence (which would skip on a first call),
		// the re-entry must ADVANCE the recorded drive, not skip past it —
		// otherwise the pair encodes a state nothing transitions out of
		// (learnings: presence-encoded-state).
		f := setupRebaseFixture(t, main)
		// No advanceBase: the rebase is a no-op, the skip's first conjunct.
		cont := GateContinuation{DriveID: "drive-3", Generation: "gen-3"}
		gate := &seqGate{results: []LocalGateResult{
			{Outcome: FinalizeGateWaiting, Continuation: cont},
			{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"},
		}}
		// First call: no PR evidence -> the suite runs -> WAITING records the pair.
		ghNone := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		req := FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head}
		ctx := context.Background()
		if r := FinalizeRebase(ctx, f.finalizeDeps(ghNone, gate), f.repo.invocation, req); r.Disposition != RebaseDispWaiting {
			t.Fatalf("first = %q, want waiting", r.Disposition)
		}
		// Second call: the PR body NOW carries exact-head green evidence.
		ghGreen := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, greenEvidenceFor(t, f.head))}}
		second := FinalizeRebase(ctx, f.finalizeDeps(ghGreen, gate), f.repo.invocation, req)
		if second.Gate == nil || second.Gate.Compose != "ran" {
			t.Fatalf("re-entry with a recorded live drive skipped: gate %+v", second.Gate)
		}
		if len(gate.reqs) != 2 || gate.reqs[1].Continuation != cont {
			t.Fatalf("re-entry did not advance the recorded drive: %+v", gate.reqs)
		}
	})

	t.Run("waiting-document-carries-drive-id-and-never-the-generation", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateWaiting,
			Continuation: GateContinuation{DriveID: "drive-4", Generation: "SECRET-gen-4"}}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispWaiting {
			t.Fatalf("disposition = %q, want waiting", res.Disposition)
		}
		// The same marshal internal/cli/presenter.go performs ("json.Marshal(r)").
		buf, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(buf), `"drive_id":"drive-4"`) {
			t.Errorf("waiting document lost drive_id: %s", buf)
		}
		if strings.Contains(string(buf), "SECRET-gen-4") || strings.Contains(string(buf), `"generation"`) {
			t.Errorf("the owner generation leaked into the CLI document: %s", buf)
		}
	})
}

// TestIntegrationFinalizeRebaseRecoveryAttemptRoundTrip settles the stub's unverified
// attempt-token-truncation claim (spec §6; learnings:
// groomed-root-cause-is-a-hypothesis): the finalize.rebase JSON document's
// `attempt` must equal the on-disk receipt's `attempt` byte for byte, through
// the exact marshal internal/cli/presenter.go performs ("json.Marshal(r)").
// newRebaseAttempt mints `<stamp>-<12 hex>`; if this test never reddens, the
// claim did not reproduce and this test stands as the guard.
func TestIntegrationFinalizeRebaseRecoveryAttemptRoundTrip(t *testing.T) {
	requireRealGit(t)
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.advanceBase(t)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Result != ResultApplied {
		t.Fatalf("rebase = %q (reason %q msg %q)", res.Result, res.Reason, res.Message)
	}
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		Attempt string `json:"attempt"`
	}
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rec, found, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !found {
		t.Fatalf("receipt: found=%v err=%v", found, err)
	}
	if doc.Attempt != rec.Attempt {
		t.Fatalf("document attempt %q != receipt attempt %q", doc.Attempt, rec.Attempt)
	}
	// The base suffix is the full 12 hex characters newRebaseAttempt mints.
	if i := strings.LastIndex(doc.Attempt, "-"); i < 0 || len(doc.Attempt)-i-1 != 12 {
		t.Fatalf("attempt %q does not carry a 12-character base suffix", doc.Attempt)
	}
}

func TestIntegrationFinalizeRebaseGateHappyAndReceipt(t *testing.T) {
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupRebaseFixture(t, m)
			f.advanceBase(t) // the base moves ahead: a real rewrite is required.

			gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
			gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
			deps := f.finalizeDeps(gh, gate)

			res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
				FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})

			if res.Result != ResultApplied || res.Disposition != RebaseDispRebased {
				t.Fatalf("rebase = %q disp %q (reason %q msg %q), want applied/rebased", res.Result, res.Disposition, res.Reason, res.Message)
			}
			// The rewrite actually moved the head.
			if newHead := f.localHead(); newHead == f.head {
				t.Fatalf("the feature head did not move; no rewrite happened")
			}
			// The receipt was written with the exact orig/base identities.
			rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
			if err != nil || !present {
				t.Fatalf("expected an owned receipt after the rewrite (present=%v err=%v)", present, err)
			}
			if rec.OrigHead != f.head {
				t.Errorf("receipt orig head = %q, want the pre-rebase head %q", rec.OrigHead, f.head)
			}
			if rec.BaseRef != string(f.target.BaseRef) {
				t.Errorf("receipt base ref = %q, want %q", rec.BaseRef, f.target.BaseRef)
			}
			if rec.OrigRemoteHead != f.head {
				t.Errorf("receipt orig remote head = %q, want the published head %q", rec.OrigRemoteHead, f.head)
			}
			// The owned recovery refs exist.
			if orig, err := tryGit(f.wp, "rev-parse", "refs/docket/finalize/5/orig"); err != nil || strings.TrimSpace(orig) != f.head {
				t.Errorf("owned orig ref = %q (err %v), want the pre-rebase head", orig, err)
			}
			if _, err := tryGit(f.wp, "rev-parse", "refs/docket/finalize/5/base"); err != nil {
				t.Errorf("owned base ref missing: %v", err)
			}
			// The gate ran (a real rewrite is never skipped) and produced evidence.
			if gate.calls != 1 {
				t.Errorf("gate calls = %d, want exactly 1 (a real rewrite runs the suite)", gate.calls)
			}
			if res.Gate == nil || res.Gate.Compose != gateComposeRan || res.Gate.Outcome != string(FinalizeGatePassed) || res.Gate.Evidence == "" {
				t.Errorf("gate report = %+v, want ran/passed with evidence", res.Gate)
			}
		})
	}
}

// TestFinalizeRebasePreconditions proves every precondition refusal leaves the
// receipt unwritten and Git untouched, with a closed reason. Preconditions are
// mode-independent, so the table runs in main mode.
func TestIntegrationFinalizeRebaseGatePreconditions(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("not-implemented", func(t *testing.T) {
		f := setupRebaseFixtureStatus(t, main, "in-progress")
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseNotImplemented)
		f.receiptAbsent(t)
		if f.localHead() != f.head {
			t.Errorf("a refused precondition moved the feature head")
		}
	})

	t.Run("version-drift", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: "sha256:" + strings.Repeat("b", 64), Head: f.head})
		assertRebaseRefused(t, res, ResultContended, ReasonRebaseVersionDrift)
		f.receiptAbsent(t)
	})

	t.Run("pr-base-mismatch", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		badPR := f.prForHead(f.head, "")
		badPR.BaseBranch = "some-other-branch"
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{badPR}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebasePRBaseMismatch)
		f.receiptAbsent(t)
	})

	t.Run("pr-head-mismatch", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		badPR := f.prForHead(strings.Repeat("c", 40), "")
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{badPR}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebasePRHeadMismatch)
		f.receiptAbsent(t)
	})

	t.Run("dirty-workspace", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		writeRepoFile(t, f.wp, "scratch.txt", "uncommitted\n")
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseWorkspaceDirty)
		f.receiptAbsent(t)
	})

	t.Run("local-head-mismatch", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		// Advance the local head off the expected head, but keep the PR (and req)
		// naming the old head: the workspace head no longer matches the authorization.
		writeRepoFile(t, f.wp, "more.txt", "more work\n")
		runGit(t, f.wp, "add", "-A")
		runGit(t, f.wp, "commit", "-q", "-m", "extra")
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultContended, ReasonRebaseLocalHeadMismatch)
		f.receiptAbsent(t)
	})

	t.Run("remote-head-mismatch", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		// Advance the remote feature head out of band, leaving the local head at the
		// expected head: local and remote no longer agree.
		runGit(t, f.wp, "commit", "-q", "--allow-empty", "-m", "remote-only")
		runGit(t, f.wp, "push", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
		runGit(t, f.wp, "reset", "--hard", f.head)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseRemoteHeadMismatch)
		f.receiptAbsent(t)
	})
}

// TestFinalizeRebaseResponseLossRecovery proves a replay after a completed rewrite
// (a lost response) adopts the same outcome from the receipt, the owned refs, the
// head, and the ancestry — and never rebases a different head.
func TestIntegrationFinalizeRebaseRecoveryResponseLossRecovery(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]
	f := setupRebaseFixture(t, main)
	f.advanceBase(t)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)

	first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if first.Disposition != RebaseDispRebased {
		t.Fatalf("first call = %q, want rebased", first.Disposition)
	}
	rewritten := f.localHead()
	recFirst, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)

	// The response was lost; the same request is replayed. It must recover, not
	// rebase again.
	second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if second.Result != ResultApplied || second.Disposition != RebaseDispRebased {
		t.Fatalf("replay = %q disp %q, want applied/rebased", second.Result, second.Disposition)
	}
	if f.localHead() != rewritten {
		t.Fatalf("the replay rebased a different head: %q -> %q", rewritten, f.localHead())
	}
	if second.Head != rewritten {
		t.Errorf("replay head = %q, want the already-rewritten head %q", second.Head, rewritten)
	}
	recSecond, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if recSecond.Attempt != recFirst.Attempt {
		t.Errorf("the replay minted a new attempt token %q; want the owned %q", recSecond.Attempt, recFirst.Attempt)
	}
}

// --- resolver budget end-to-end (change 0349, Task 8) ---------------------

// beginSuccessiveConflicts builds a real feature workspace whose feature branch
// carries one commit per (setup + extra) entry — each rewriting feature.txt (and
// whatever else the entry names) — over a base that conflictingly wrote the same
// file, so a rebase onto the base stops on each feature commit in turn. limit > 0
// resolves finalize.resolver_max_attempts through the repository-local layer; limit
// == 0 leaves the built-in default (10) in force. It publishes the multi-commit head,
// begins the owned rebase from it, and returns the fixture, the deps, the authorized
// head, and the conflicted begin result.
func beginSuccessiveConflicts(t *testing.T, limit int, extra []map[string]string, baseFiles map[string]string) (*rebaseFixture, FinalizeDeps, string, FinalizeRebaseResult) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0]) // c1: feature.txt="feature work\n", published at f.head
	for i, files := range extra {
		for name, content := range files {
			writeRepoFile(t, f.wp, name, content)
		}
		runGit(t, f.wp, "add", "-A")
		runGit(t, f.wp, "commit", "-q", "-m", fmt.Sprintf("feature step %d", i+2))
	}
	head := runGit(t, f.wp, "rev-parse", "HEAD")
	// Re-publish the (possibly multi-commit) head so the remote feature ref agrees
	// with the local head the request authorizes.
	runGit(t, f.wp, "push", "-f", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
	// The base conflictingly writes the same file(s) the feature commits touch.
	f.repo.writerAdvance(t, "main", baseFiles)
	if limit > 0 {
		setResolverConfig(t, f, limit)
	}
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	begin := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	if begin.Disposition != RebaseDispConflicted {
		t.Fatalf("begin = disp %q (reason %q msg %q), want conflicted", begin.Disposition, begin.Reason, begin.Message)
	}
	return f, deps, head, begin
}

// reserveResolveContinue runs one full reserve -> resolve -> continue cycle against a
// live conflicted rebase: it durably reserves one dispatch, writes reconciled content
// to every reported unmerged path, echoes the reservation token in the resolver
// report, and feeds it back through the real FinalizeRebaseContinue. It returns both
// results so a caller can assert the disposition of each.
func reserveResolveContinue(t *testing.T, f *rebaseFixture, deps FinalizeDeps, attempt string, unmerged []string, cycle int) (FinalizeReserveResult, FinalizeRebaseResult) {
	t.Helper()
	ctx := context.Background()
	reserve := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, attempt)
	if reserve.Disposition != ReserveReserved || reserve.Reservation == "" {
		t.Fatalf("cycle %d reserve = disp %q token %q (reason %q), want reserved with a token", cycle, reserve.Disposition, reserve.Reservation, reserve.Reason)
	}
	resolved := fmt.Sprintf("reconciled content for cycle %d\n", cycle)
	for _, p := range unmerged {
		writeRepoFile(t, f.wp, p, resolved)
	}
	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: unmerged, ResolverReservation: reserve.Reservation}
	cont := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	return reserve, cont
}

// TestIntegrationResolverBudgetSuccessiveConflicts drives the whole reserve ->
// dispatch-once -> verified-continue loop end to end over a real multi-commit
// conflicting rebase (spec test 2): the default budget completes three successive
// conflicts, a lowered budget exhausts and then aborts cleanly, a raised budget
// completes a fourth, a single multi-file resolution consumes exactly one
// reservation, and the reservation survives a process restart via the on-disk
// receipt.
func TestIntegrationResolverBudgetSuccessiveConflicts(t *testing.T) {
	requireRealGit(t)

	// Default limit 10: three successive conflicts, reserve->resolve->continue each,
	// the rebase completes and the gate composes. (0419 raised the built-in default
	// from 3 to 10; three conflicts still consume only 3 reservations.)
	t.Run("default-limit-10-three-conflicts-completes", func(t *testing.T) {
		f, deps, _, begin := beginSuccessiveConflicts(t, 0,
			[]map[string]string{{"feature.txt": "feature v2\n"}, {"feature.txt": "feature v3\n"}},
			map[string]string{"feature.txt": "conflicting base content\n"})
		if begin.ResolverLimit != 10 || begin.ResolverUsed != 0 {
			t.Fatalf("begin counts = %d/%d, want limit 10 (built-in default) used 0", begin.ResolverLimit, begin.ResolverUsed)
		}
		attempt := begin.Attempt
		unmerged := begin.UnmergedPaths
		var cont FinalizeRebaseResult
		for cycle := 1; cycle <= 3; cycle++ {
			_, cont = reserveResolveContinue(t, f, deps, attempt, unmerged, cycle)
			if cycle < 3 {
				if cont.Disposition != RebaseDispConflicted {
					t.Fatalf("cycle %d continue = disp %q (reason %q), want conflicted", cycle, cont.Disposition, cont.Reason)
				}
				unmerged = cont.UnmergedPaths
			}
		}
		if cont.Result != ResultApplied || cont.Disposition != RebaseDispRebased {
			t.Fatalf("final continue = (%q, %q) reason %q msg %q, want applied/rebased", cont.Result, cont.Disposition, cont.Reason, cont.Message)
		}
		if cont.Gate == nil || cont.Gate.Evidence == "" {
			t.Errorf("the completed rebase did not compose the gate: %+v", cont.Gate)
		}
		if st, _ := f.deps.Client.RebaseState(context.Background(), f.wp); st.Disposition != gitcli.RebaseUnchanged {
			t.Errorf("a rebase is still in progress after completion: %q", st.Disposition)
		}
		rec := reloadReceipt(t, f)
		if rec.ResolverUsed != "3" || rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
			t.Errorf("final receipt = used %q token %q cont %q, want used 3 and no outstanding reservation",
				rec.ResolverUsed, rec.ResolverReservationToken, rec.ResolverContinuationStarted)
		}
	})

	// Lowered limit 2: the second continuation's next (third) conflict is refused
	// blocked/resolver-budget-exhausted, a fresh reservation is exhausted, and abort
	// restores the original head.
	t.Run("limit-2-exhausts-then-abort-restores-orig-head", func(t *testing.T) {
		f, deps, head, begin := beginSuccessiveConflicts(t, 2,
			[]map[string]string{{"feature.txt": "feature v2\n"}, {"feature.txt": "feature v3\n"}},
			map[string]string{"feature.txt": "conflicting base content\n"})
		if begin.ResolverLimit != 2 {
			t.Fatalf("begin limit = %d, want the resolved non-default 2", begin.ResolverLimit)
		}
		attempt := begin.Attempt
		ctx := context.Background()

		_, c1 := reserveResolveContinue(t, f, deps, attempt, begin.UnmergedPaths, 1)
		if c1.Disposition != RebaseDispConflicted || c1.ResolverUsed != 1 {
			t.Fatalf("cycle 1 continue = disp %q used %d (reason %q), want conflicted used 1", c1.Disposition, c1.ResolverUsed, c1.Reason)
		}
		_, c2 := reserveResolveContinue(t, f, deps, attempt, c1.UnmergedPaths, 2)
		if c2.Result != ResultBlocked || c2.Disposition != RebaseDispBlocked || c2.Reason != ReasonResolverBudgetExhausted {
			t.Fatalf("cycle 2 continue = (%q, %q, %q), want blocked/blocked/%q", c2.Result, c2.Disposition, c2.Reason, ReasonResolverBudgetExhausted)
		}
		if c2.ResolverLimit != 2 || c2.ResolverUsed != 2 || c2.ResolverRemaining != 0 {
			t.Errorf("exhausted counts = %d/%d/%d, want 2/2/0", c2.ResolverLimit, c2.ResolverUsed, c2.ResolverRemaining)
		}
		// A fresh reservation refuses: the stored budget is spent.
		fresh := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, attempt)
		if fresh.Disposition != ReserveExhausted || fresh.Reason != ReasonResolverBudgetExhausted {
			t.Fatalf("fresh reserve = disp %q reason %q, want exhausted/%q", fresh.Disposition, fresh.Reason, ReasonResolverBudgetExhausted)
		}
		// Abort is always available: it restores the recorded original head.
		abort := FinalizeRebaseAbort(ctx, deps, f.repo.invocation, f.id, attempt,
			ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverStuck})
		if abort.Result != ResultApplied || abort.Disposition != RebaseDispBlocked {
			t.Fatalf("abort = (%q, %q) reason %q, want applied/blocked", abort.Result, abort.Disposition, abort.Reason)
		}
		if f.localHead() != head {
			t.Errorf("abort did not restore the original head: %q != %q", f.localHead(), head)
		}
		if st, _ := f.deps.Client.RebaseState(ctx, f.wp); st.Disposition != gitcli.RebaseUnchanged {
			t.Errorf("a rebase is still in progress after abort: %q", st.Disposition)
		}
	})

	// Raised limit 4 with a fourth conflicting commit completes.
	t.Run("limit-4-with-a-fourth-conflict-completes", func(t *testing.T) {
		f, deps, _, begin := beginSuccessiveConflicts(t, 4,
			[]map[string]string{{"feature.txt": "feature v2\n"}, {"feature.txt": "feature v3\n"}, {"feature.txt": "feature v4\n"}},
			map[string]string{"feature.txt": "conflicting base content\n"})
		if begin.ResolverLimit != 4 {
			t.Fatalf("begin limit = %d, want the resolved 4", begin.ResolverLimit)
		}
		attempt := begin.Attempt
		unmerged := begin.UnmergedPaths
		var cont FinalizeRebaseResult
		for cycle := 1; cycle <= 4; cycle++ {
			_, cont = reserveResolveContinue(t, f, deps, attempt, unmerged, cycle)
			if cycle < 4 {
				if cont.Disposition != RebaseDispConflicted {
					t.Fatalf("cycle %d continue = disp %q (reason %q), want conflicted", cycle, cont.Disposition, cont.Reason)
				}
				unmerged = cont.UnmergedPaths
			}
		}
		if cont.Result != ResultApplied || cont.Disposition != RebaseDispRebased {
			t.Fatalf("final continue = (%q, %q) reason %q, want applied/rebased", cont.Result, cont.Disposition, cont.Reason)
		}
		if rec := reloadReceipt(t, f); rec.ResolverUsed != "4" {
			t.Errorf("used = %q after four resolutions, want 4", rec.ResolverUsed)
		}
	})

	// One resolver report listing multiple conflicted files consumes exactly one
	// reservation: the fixture folds a second file into the single feature commit.
	t.Run("one-report-multiple-files-consumes-one-reservation", func(t *testing.T) {
		f := setupRebaseFixture(t, planRepoModes()[0])
		// Fold a second file into the single feature commit (amend) so one conflicting
		// commit touches two files.
		writeRepoFile(t, f.wp, "other.txt", "feature other\n")
		runGit(t, f.wp, "add", "-A")
		runGit(t, f.wp, "commit", "-q", "--amend", "--no-edit")
		head := runGit(t, f.wp, "rev-parse", "HEAD")
		runGit(t, f.wp, "push", "-f", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
		// The base conflictingly writes BOTH files.
		f.repo.writerAdvance(t, "main", map[string]string{
			"feature.txt": "conflicting base content\n", "other.txt": "conflicting base other\n"})
		setResolverConfig(t, f, 3)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, head), RunDir: "/run/x"}}
		deps := f.finalizeDeps(gh, gate)
		begin := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		if begin.Disposition != RebaseDispConflicted {
			t.Fatalf("begin = %q (reason %q), want conflicted", begin.Disposition, begin.Reason)
		}
		if len(begin.UnmergedPaths) != 2 {
			t.Fatalf("begin unmerged = %v, want two conflicted files", begin.UnmergedPaths)
		}
		_, cont := reserveResolveContinue(t, f, deps, begin.Attempt, begin.UnmergedPaths, 1)
		if cont.Result != ResultApplied || cont.Disposition != RebaseDispRebased {
			t.Fatalf("continue = (%q, %q) reason %q, want applied/rebased", cont.Result, cont.Disposition, cont.Reason)
		}
		if rec := reloadReceipt(t, f); rec.ResolverUsed != "1" {
			t.Errorf("used = %q after one multi-file resolution, want exactly 1 reservation", rec.ResolverUsed)
		}
	})

	// A process restart between reserve and continue: rebuilding the workspace
	// Service (and every dep) drops all in-memory state, yet the reservation survives
	// via the on-disk receipt and the continue completes with the pre-restart token.
	t.Run("process-restart-preserves-limit-used-reservation", func(t *testing.T) {
		f, deps, head, begin := beginSuccessiveConflicts(t, 3, nil,
			map[string]string{"feature.txt": "conflicting base content\n"})
		attempt := begin.Attempt
		ctx := context.Background()

		reserve := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, attempt)
		if reserve.Disposition != ReserveReserved || reserve.Reservation == "" {
			t.Fatalf("reserve = disp %q token %q (reason %q), want reserved with a token", reserve.Disposition, reserve.Reservation, reserve.Reason)
		}

		// Simulate a process restart: a brand-new workspace.Service and fresh deps,
		// so nothing in memory carries limit/used/reservation across.
		svc2, err := workspace.NewService(f.deps.Client)
		if err != nil {
			t.Fatalf("rebuild workspace service: %v", err)
		}
		gh2 := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(head, "")}}
		gate2 := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, head), RunDir: "/run/x"}}
		deps2 := FinalizeDeps{Planning: f.deps, GitHub: gh2, Workspace: svc2, Gate: gate2}

		// The reservation survived the restart via the receipt.
		rec := reloadReceipt(t, f)
		if rec.ResolverLimit != "3" || rec.ResolverUsed != "1" || rec.ResolverReservationToken != reserve.Reservation {
			t.Fatalf("post-restart receipt = limit %q used %q token %q, want 3/1/%q",
				rec.ResolverLimit, rec.ResolverUsed, rec.ResolverReservationToken, reserve.Reservation)
		}

		// Continue with the fresh deps and the pre-restart reservation.
		for _, p := range begin.UnmergedPaths {
			writeRepoFile(t, f.wp, p, "reconciled after restart\n")
		}
		report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
			ConflictedPaths: begin.UnmergedPaths, ResolverReservation: reserve.Reservation}
		cont := FinalizeRebaseContinue(ctx, deps2, f.repo.invocation, f.id, attempt, report)
		if cont.Result != ResultApplied || cont.Disposition != RebaseDispRebased {
			t.Fatalf("post-restart continue = (%q, %q) reason %q, want applied/rebased", cont.Result, cont.Disposition, cont.Reason)
		}
		if after := reloadReceipt(t, f); after.ResolverUsed != "1" || after.ResolverReservationToken != "" {
			t.Errorf("post-continue receipt = used %q token %q, want used 1 preserved and reservation cleared", after.ResolverUsed, after.ResolverReservationToken)
		}
	})
}
func TestIntegrationFinalizeRebaseGatePassedRecordsPublishCheckpoint(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("real-rewrite-records", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &headEvidenceGate{t: t}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultApplied || res.Disposition != RebaseDispRebased {
			t.Fatalf("rebase = %q disp %q (reason %q msg %q), want applied/rebased", res.Result, res.Disposition, res.Reason, res.Message)
		}
		rewritten := f.localHead()
		rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if err != nil || !present {
			t.Fatalf("receipt after PASSED gate: present=%v err=%v", present, err)
		}
		if rec.PublishCheckpointHead != rewritten {
			t.Errorf("checkpoint head = %q, want the rewritten head %q", rec.PublishCheckpointHead, rewritten)
		}
		if rec.PublishCheckpointBaseHead != rec.BaseHead {
			t.Errorf("checkpoint base head = %q, want the receipt base head %q", rec.PublishCheckpointBaseHead, rec.BaseHead)
		}
		if rec.PublishCheckpointCommand != "go test ./..." {
			t.Errorf("checkpoint command = %q, want the resolved finalize.test_command", rec.PublishCheckpointCommand)
		}
		if rec.PublishCheckpointGate != "local" {
			t.Errorf("checkpoint gate policy = %q, want %q", rec.PublishCheckpointGate, "local")
		}
		if rec.PublishCheckpointPRNumber != "1" {
			t.Errorf("checkpoint pr number = %q, want %q", rec.PublishCheckpointPRNumber, "1")
		}
		if v := evidence.Verify([]byte(rec.PublishCheckpointEvidence), rewritten); v != evidence.VerdictVerified {
			t.Errorf("checkpoint evidence verdict for the rewritten head = %q, want verified", v)
		}
		if rec.GateDriveID != "" || rec.GateOwnerGeneration != "" {
			t.Errorf("gate pair not cleared at the PASSED terminal: (%q, %q)", rec.GateDriveID, rec.GateOwnerGeneration)
		}
	})

	t.Run("noop-rebase-records-nothing", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		// No advanceBase: the feature already sits on the base; the gate still runs
		// (no PR evidence waives it) but the pass is for a no-op rebase.
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &headEvidenceGate{t: t}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispUnchanged {
			t.Fatalf("disp = %q (reason %q), want unchanged", res.Disposition, res.Reason)
		}
		rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if err != nil || !present {
			t.Fatalf("receipt: present=%v err=%v", present, err)
		}
		if rec.PublishCheckpointHead != "" || rec.PublishCheckpointEvidence != "" {
			t.Errorf("a no-op rebase recorded a publish checkpoint: %+v", rec)
		}
	})
}

// setupPassedRebaseCheckpoint drives a real rewrite to a PASSED gate whose
// publish checkpoint is recorded (the state a denied publish leaves behind),
// returning the fixture, the gate fake (for call counting), the deps, and the
// rewritten head.
func setupPassedRebaseCheckpoint(t *testing.T) (*rebaseFixture, *headEvidenceGate, FinalizeDeps, string) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.advanceBase(t)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &headEvidenceGate{t: t}
	deps := f.finalizeDeps(gh, gate)
	res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Disposition != RebaseDispRebased || gate.calls != 1 {
		t.Fatalf("setup rebase = disp %q gate calls %d (reason %q), want rebased with one gate run", res.Disposition, gate.calls, res.Reason)
	}
	rewritten := f.localHead()
	rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present || rec.PublishCheckpointHead != rewritten {
		t.Fatalf("setup did not record a checkpoint for the rewritten head: present=%v err=%v cp=%q", present, err, rec.PublishCheckpointHead)
	}
	return f, gate, deps, rewritten
}

// tamperCheckpoint rewrites the on-disk receipt with one checkpoint field
// mutated — a still-VALID receipt whose recorded identity no longer matches
// current reality — so a resume must invalidate it and re-run the gate.
func tamperCheckpoint(t *testing.T, f *rebaseFixture, mut func(*workspace.RebaseReceipt)) {
	t.Helper()
	rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present {
		t.Fatalf("reading receipt to tamper: present=%v err=%v", present, err)
	}
	mut(&rec)
	if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, rec); err != nil {
		t.Fatalf("writing tampered receipt: %v", err)
	}
}

// TestIntegrationFinalizeRebaseRecoveryCheckpointReuse proves the marquee behavior: a
// resume after a denied publish (completed rewrite, checkpoint recorded, remote
// and PR untouched) reuses the recorded evidence and publishes-readies WITHOUT
// invoking the suite — the gate report is skipped, carries the recorded
// evidence verifying the rewritten head, and the gate seam is never called a
// second time.
func TestIntegrationFinalizeRebaseRecoveryCheckpointReuse(t *testing.T) {
	requireRealGit(t)
	f, gate, deps, rewritten := setupPassedRebaseCheckpoint(t)

	res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})

	if res.Result != ResultApplied || res.Disposition != RebaseDispRebased {
		t.Fatalf("resume = %q disp %q (reason %q msg %q), want applied/rebased", res.Result, res.Disposition, res.Reason, res.Message)
	}
	if gate.calls != 1 {
		t.Fatalf("gate calls = %d, want 1 — a valid checkpoint must never re-run the suite", gate.calls)
	}
	if res.Gate == nil || res.Gate.Compose != gateComposeSkipped {
		t.Fatalf("gate report = %+v, want compose skipped", res.Gate)
	}
	if res.Gate.Permit != rewritten {
		t.Errorf("skip permit = %q, want the rewritten head %q", res.Gate.Permit, rewritten)
	}
	if v := evidence.Verify([]byte(res.Gate.Evidence), rewritten); v != evidence.VerdictVerified {
		t.Errorf("reused evidence verdict = %q, want verified for the rewritten head", v)
	}
	if res.Head != rewritten || res.Attempt == "" {
		t.Errorf("resume head/attempt = %q/%q, want the rewritten head and the owned attempt", res.Head, res.Attempt)
	}
}

// TestIntegrationFinalizeRebaseRecoveryCheckpointInvalidation proves every recorded
// identity is load-bearing: a moved local head, a changed recorded command, a
// changed gate policy, a different PR, and evidence for the wrong head each
// invalidate the checkpoint — the gate re-runs and the receipt's checkpoint is
// rewritten by the new terminal, never reused stale. A forward-moved BASE
// forward-refreshes onto the advanced base and always retests (change 0438),
// never reusing the superseded checkpoint.
func TestIntegrationFinalizeRebaseRecoveryCheckpointInvalidation(t *testing.T) {
	requireRealGit(t)

	t.Run("moved-local-head-reruns", func(t *testing.T) {
		f, gate, deps, _ := setupPassedRebaseCheckpoint(t)
		// The head moves past the checkpoint (still descending the base).
		writeRepoFile(t, f.wp, "more.txt", "more work\n")
		runGit(t, f.wp, "add", "-A")
		runGit(t, f.wp, "commit", "-q", "-m", "extra work after the pass")
		moved := f.localHead()
		res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispRebased || gate.calls != 2 {
			t.Fatalf("resume = disp %q gate calls %d, want rebased with the gate re-run", res.Disposition, gate.calls)
		}
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if rec.PublishCheckpointHead != moved {
			t.Errorf("checkpoint after re-run = %q, want re-recorded for the moved head %q", rec.PublishCheckpointHead, moved)
		}
	})

	tamperCases := map[string]func(*workspace.RebaseReceipt){
		"changed-command": func(r *workspace.RebaseReceipt) { r.PublishCheckpointCommand = "make other-suite" },
		"changed-gate":    func(r *workspace.RebaseReceipt) { r.PublishCheckpointGate = "off" },
		"different-pr":    func(r *workspace.RebaseReceipt) { r.PublishCheckpointPRNumber = "99" },
		"wrong-head-evidence": func(r *workspace.RebaseReceipt) {
			r.PublishCheckpointEvidence = strings.ReplaceAll(
				r.PublishCheckpointEvidence, r.PublishCheckpointHead, r.OrigHead)
		},
	}
	for name, mut := range tamperCases {
		mut := mut
		t.Run(name+"-reruns", func(t *testing.T) {
			f, gate, deps, rewritten := setupPassedRebaseCheckpoint(t)
			tamperCheckpoint(t, f, mut)
			res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
				FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
			if res.Disposition != RebaseDispRebased || gate.calls != 2 {
				t.Fatalf("resume = disp %q gate calls %d (reason %q), want rebased with the gate re-run", res.Disposition, gate.calls, res.Reason)
			}
			rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
			if rec.PublishCheckpointHead != rewritten || rec.PublishCheckpointCommand != "go test ./..." {
				t.Errorf("stale checkpoint not replaced by the re-run terminal: %+v", rec)
			}
		})
	}

	t.Run("moved-base-forward-refreshes-and-retests", func(t *testing.T) {
		// Change 0438 (spec §4): a forward base advance under a completed, quiescent,
		// unpublished rewrite no longer blocks — it forward-rebases onto the advanced
		// base and ALWAYS re-runs the suite; the checkpoint recorded against the
		// superseded base is never reused.
		f, gate, deps, _ := setupPassedRebaseCheckpoint(t)
		f.advanceBase(t)
		res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultApplied || res.Disposition != RebaseDispRebased {
			t.Fatalf("moved-base refresh = %q/%q (reason %q), want applied/rebased", res.Result, res.Disposition, res.Reason)
		}
		if res.Gate == nil || res.Gate.Compose != gateComposeRan {
			t.Fatalf("gate = %+v, want compose ran (a base refresh always retests)", res.Gate)
		}
		if gate.calls != 2 {
			t.Errorf("gate calls = %d, want 2 — the refresh re-runs the suite on the new base", gate.calls)
		}
	})
}

// --- carried-descendant preservation gate (Task 6) ------------------------

// fakeRebaseCarryGitHub serves the two GitHub reads a carry-gated rebase makes:
// the parent's open PR (FindOpenPullRequestsByHead, for probeRebasePR) and each
// stacked descendant's merged reprobe (ProbeMerged, for probeDescendantFacts).
// Every other finalize-half method panics so an accidental call is loud. It
// composes the two seams a plain fakeRebaseGitHub and fakeCloseoutGitHub each
// cover alone, so one fixture exercises the pre-rewrite proof end to end.
type fakeRebaseCarryGitHub struct {
	repo   githubcli.Repository
	prs    []githubcli.PullRequest
	merged map[int]closeoutProbe
	probes int
}

func (f *fakeRebaseCarryGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	return f.repo, nil
}
func (f *fakeRebaseCarryGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, headBranch string) ([]githubcli.PullRequest, error) {
	var out []githubcli.PullRequest
	for _, pr := range f.prs {
		if pr.HeadBranch == headBranch {
			out = append(out, pr)
		}
	}
	return out, nil
}
func (f *fakeRebaseCarryGitHub) ProbeMerged(_ context.Context, _ githubcli.Repository, number int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	f.probes++
	p, ok := f.merged[number]
	if !ok {
		return githubcli.MergeNotMergeable, githubcli.MergedFacts{}, nil
	}
	return p.outcome, p.facts, nil
}
func (f *fakeRebaseCarryGitHub) ViewPullRequest(context.Context, githubcli.Repository, int) (githubcli.PullRequest, error) {
	panic("ViewPullRequest: carry-gated rebase must not call this")
}
func (f *fakeRebaseCarryGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: carry-gated rebase must not call this")
}
func (f *fakeRebaseCarryGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: carry-gated rebase must not call this")
}
func (f *fakeRebaseCarryGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: carry-gated rebase must not call this")
}
func (f *fakeRebaseCarryGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeResult, error) {
	panic("MergePullRequest: carry-gated rebase must not call this")
}

// seedRebaseCarryChild seeds a stacked-merged descendant (id 6, gadget, PR #8)
// stacked on the rebase fixture's parent (id 5) onto the metadata branch, so the
// snapshot the rebase reads promises to carry the child's merged work.
func seedRebaseCarryChild(t *testing.T, f *rebaseFixture) {
	t.Helper()
	recPath := groomPath(6, "gadget")
	desc := closeoutRecord(6, "gadget", "stacked-merged", "github.com/acme/widget#8", "",
		"docs/superpowers/plans/2026-08-16-gadget-plan.md", "")
	desc = strings.Replace(desc, "stacked_on:\n", "stacked_on: 5\n", 1)
	f.repo.writerAdvance(t, f.branch, map[string]string{recPath: desc})
}

// carryDroppedCommit builds a real commit on a branch off main in the writer,
// pushes it to origin, fetches it into the invocation store, and returns its
// OID — a real object that EXISTS yet is NOT carried on feat/widget, so a
// preservation proof against the feature head observes a DROP, distinct from a
// missing object (green-suite-untested-branch: the discriminating input is a
// real dropped object).
func carryDroppedCommit(t *testing.T, f *rebaseFixture, files map[string]string) string {
	t.Helper()
	w := f.repo.writer
	runGit(t, w, "checkout", "-q", "-b", "carry-dropped", "main")
	for rel, content := range files {
		writeRepoFile(t, w, rel, content)
	}
	runGit(t, w, "add", "-A")
	runGit(t, w, "commit", "-q", "-m", "child merge dropped from parent")
	runGit(t, w, "push", "-q", "origin", "carry-dropped")
	oid := runGit(t, w, "rev-parse", "HEAD")
	runGit(t, f.repo.invocation, "fetch", "-q", "origin", "+refs/heads/*:refs/remotes/origin/*")
	return oid
}

// TestIntegrationFinalizeRebaseRecoveryCarryPreservation proves the two enforcement
// points FinalizeRebase adds: the fresh-path PRE-rewrite gate (refuse before any
// receipt/rewrite when a carried descendant's merged work is not preserved at the
// agreed head) and the POST-rewrite chokepoint at composeLocalGate (refuse a
// completed rewrite that dropped a carried child, retaining the owned receipt for
// the abort/repair flow). Every refusal fixture builds local/remote/PR heads that
// AGREE by construction, so a refusal exercises the NEW carry proof, not the
// existing head-mismatch guard, and a green suite never substitutes for it.
func TestIntegrationFinalizeRebaseRecoveryCarryPreservation(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]
	ctx := context.Background()

	t.Run("pre-rewrite-refusal-leaves-receipt-workspace-remote-untouched", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		seedRebaseCarryChild(t, f)
		dropped := carryDroppedCommit(t, f, map[string]string{"catalog.yaml": "Y\n"})
		// The dropped merge object EXISTS (a drop, not a missing object): pins that
		// the refusal is an observed non-preservation, not an observation error.
		if _, err := tryGit(f.repo.invocation, "cat-file", "-e", dropped); err != nil {
			t.Fatalf("the child merge object must exist to pin a drop (not a missing object): %v", err)
		}
		gh := &fakeRebaseCarryGitHub{
			repo:   retargetRepo(),
			prs:    []githubcli.PullRequest{f.prForHead(f.head, "")},
			merged: map[int]closeoutProbe{8: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", dropped)}},
		}
		remoteBefore := originTip(t, f.repo.origin, "feat/"+f.slug)
		res := FinalizeRebase(ctx, f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})

		assertRebaseRefused(t, res, ResultBlocked, ReasonCarryUnproven)
		if len(res.Findings) == 0 {
			t.Errorf("a carry refusal carried no findings naming the unproven descendant")
		}
		// The missing effects: no owned receipt, workspace head unchanged, remote
		// feature ref untouched — a pre-rewrite refusal leaves Git untouched.
		f.receiptAbsent(t)
		if f.localHead() != f.head {
			t.Errorf("a pre-rewrite carry refusal moved the workspace head: %q -> %q", f.head, f.localHead())
		}
		if after := originTip(t, f.repo.origin, "feat/"+f.slug); after != remoteBefore {
			t.Errorf("a pre-rewrite carry refusal touched the remote feature ref: %q -> %q", remoteBefore, after)
		}
	})

	t.Run("pre-rewrite-pass-through-when-carry-preserved", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		seedRebaseCarryChild(t, f)
		// The child's merge-result is an ancestor of the feature head (a real object
		// preserved by ancestry), so the proof passes and the operation proceeds to a
		// normal rebase outcome — never a carry refusal.
		gh := &fakeRebaseCarryGitHub{
			repo:   retargetRepo(),
			prs:    []githubcli.PullRequest{f.prForHead(f.head, greenEvidenceFor(t, f.head))},
			merged: map[int]closeoutProbe{8: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", f.baseTip)}},
		}
		res := FinalizeRebase(ctx, f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Reason == ReasonCarryUnproven {
			t.Fatalf("a preserved carry was refused: reason %q msg %q", res.Reason, res.Message)
		}
		if res.Result != ResultNoOp && res.Result != ResultApplied {
			t.Fatalf("pass-through = %q disp %q (reason %q msg %q), want a normal rebase outcome", res.Result, res.Disposition, res.Reason, res.Message)
		}
	})

	t.Run("post-rewrite-refusal-retains-receipt-and-orig-head", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		seedRebaseCarryChild(t, f)
		dropped := carryDroppedCommit(t, f, map[string]string{"catalog.yaml": "Z\n"})
		baseHead := originTip(t, f.repo.origin, "main")
		// Pre-create an owned receipt for a COMPLETED no-op rebase so FinalizeRebase
		// takes the recovery path (which skips the pre-rewrite gate) and reaches
		// composeLocalGate — the single chokepoint all post-rewrite paths funnel
		// through. The rewrite dropped the carried child (its merge object exists but
		// is absent from the head), so gate (b) refuses.
		rec := workspace.RebaseReceipt{
			RepoIdentity:   f.gitrepo.CommonDir,
			ChangeID:       "5",
			OrigHead:       f.head,
			OrigRemoteHead: f.head,
			BaseRef:        string(f.target.BaseRef),
			BaseHead:       baseHead,
			Attempt:        "manual-carry-attempt",
			CreatedUTC:     "2026-09-07T00:00:00Z",
		}
		if err := f.svc.WriteRebaseReceipt(ctx, f.metaDir, rec); err != nil {
			t.Fatalf("write receipt: %v", err)
		}
		gate := &fakeGate{}
		gh := &fakeRebaseCarryGitHub{
			repo:   retargetRepo(),
			prs:    []githubcli.PullRequest{f.prForHead(f.head, "")},
			merged: map[int]closeoutProbe{8: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", dropped)}},
		}
		res := FinalizeRebase(ctx, f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})

		assertRebaseRefused(t, res, ResultBlocked, ReasonCarryUnproven)
		// The owned receipt and its orig head survive: the abort/repair flow stays
		// available (a post-rewrite refusal never clears the receipt).
		recAfter, present, err := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
		if err != nil || !present {
			t.Fatalf("the post-rewrite carry refusal cleared the receipt (present=%v err=%v)", present, err)
		}
		if recAfter.OrigHead != f.head {
			t.Errorf("the post-rewrite carry refusal lost the receipt orig head: %q, want %q", recAfter.OrigHead, f.head)
		}
		// Gate (b) refuses before the suite composes: the seam never ran.
		if gate.calls != 0 {
			t.Errorf("the gate seam ran %d time(s) on a carry refusal; want 0", gate.calls)
		}
	})

	t.Run("green-suite-does-not-substitute-for-the-carry-proof", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		seedRebaseCarryChild(t, f)
		dropped := carryDroppedCommit(t, f, map[string]string{"catalog.yaml": "Q\n"})
		baseHead := originTip(t, f.repo.origin, "main")
		rec := workspace.RebaseReceipt{
			RepoIdentity:   f.gitrepo.CommonDir,
			ChangeID:       "5",
			OrigHead:       f.head,
			OrigRemoteHead: f.head,
			BaseRef:        string(f.target.BaseRef),
			BaseHead:       baseHead,
			Attempt:        "manual-carry-attempt",
			CreatedUTC:     "2026-09-07T00:00:00Z",
		}
		if err := f.svc.WriteRebaseReceipt(ctx, f.metaDir, rec); err != nil {
			t.Fatalf("write receipt: %v", err)
		}
		// Exact-head GREEN evidence on the PR body would otherwise skip the local
		// suite; the carry proof runs before and independently of that decision, so
		// the refusal is the carry reason, never a skipped success.
		gate := &fakeGate{}
		gh := &fakeRebaseCarryGitHub{
			repo:   retargetRepo(),
			prs:    []githubcli.PullRequest{f.prForHead(f.head, greenEvidenceFor(t, f.head))},
			merged: map[int]closeoutProbe{8: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", dropped)}},
		}
		res := FinalizeRebase(ctx, f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonCarryUnproven)
		if res.Gate != nil && res.Gate.Compose == gateComposeSkipped {
			t.Errorf("green evidence substituted for the carry proof: the gate skipped instead of refusing")
		}
		if gate.calls != 0 {
			t.Errorf("the gate seam ran %d time(s) on a carry refusal; want 0", gate.calls)
		}
	})
}

// TestIntegrationFinalizeRebaseRecoveryPreStartResume covers the durable-before-
// effect window (spec §3): the receipt persisted but Git never started. Re-entry
// resumes BeginRebase against the receipt's recorded target instead of refusing
// "head does not descend the base".
func TestIntegrationFinalizeRebaseRecoveryPreStartResume(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]
	f := setupRebaseFixture(t, main)
	f.advanceBase(t)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	// Simulate the crash window: hand-write the receipt FinalizeRebase would have
	// written (OrigHead = current head, BaseHead = the advanced base's head), with
	// Git untouched.
	baseHead := originTip(t, f.repo.origin, "main")
	rec := workspace.RebaseReceipt{
		RepoIdentity: f.gitrepo.CommonDir, ChangeID: itoaTest(f.id),
		OrigHead: f.head, OrigRemoteHead: f.head,
		BaseRef: string(f.target.BaseRef), BaseHead: baseHead,
		Attempt: "20260920T000000Z-pre-start", CreatedUTC: "2026-09-20T00:00:00Z",
		ResolverBudgetVersion: "1", ResolverLimit: "10", ResolverUsed: "0",
	}
	if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, rec); err != nil {
		t.Fatal(err)
	}
	out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if out.Disposition != RebaseDispRebased {
		t.Fatalf("pre-start resume = %q (reason %q msg %q), want rebased", out.Disposition, out.Reason, out.Message)
	}
	if out.Attempt != rec.Attempt {
		t.Errorf("resume minted a new attempt %q; want the recorded %q", out.Attempt, rec.Attempt)
	}
	// The gate ran (no PR-evidence skip on recovery) and the head now descends the base.
	if out.Gate == nil || out.Gate.Compose != gateComposeRan {
		t.Fatalf("gate = %+v, want compose ran", out.Gate)
	}
}

// TestIntegrationFinalizeRebaseRecoveryNoEvidenceSkip proves receipt-based
// completion recovery with no valid checkpoint runs the gate (spec §4): a fresh
// invocation may skip on exact-head green PR evidence (existing behavior), but a
// REPLAY after response loss must run the gate because a receipt now exists and no
// checkpoint was recorded.
func TestIntegrationFinalizeRebaseRecoveryNoEvidenceSkip(t *testing.T) {
	requireRealGit(t)
	f := setupRebaseFixture(t, planRepoModes()[0])
	// Base NOT advanced: a fresh call is a mechanical no-op whose PR body carries
	// green evidence for the exact current head and command.
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, greenEvidenceFor(t, f.head))}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)

	first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if first.Gate == nil || first.Gate.Compose != gateComposeSkipped {
		t.Fatalf("first call = %+v, want a fresh skip on exact-head green PR evidence", first.Gate)
	}
	if gate.calls != 0 {
		t.Fatalf("the suite ran on the fresh no-op skip; want 0 calls, got %d", gate.calls)
	}

	// Replay after a lost response: a receipt now exists and no checkpoint was
	// recorded, so recovery must RUN the gate — PR-body evidence cannot bypass the
	// retest on recovery.
	second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if second.Gate == nil || second.Gate.Compose != gateComposeRan {
		t.Fatalf("replay recovery = %+v, want compose ran (no PR-evidence skip on recovery)", second.Gate)
	}
	if gate.calls != 1 {
		t.Errorf("recovery ran the gate %d time(s); want exactly 1", gate.calls)
	}
}

// TestIntegrationFinalizeRebaseRecoveryForwardRefresh covers acceptance 1+2 (spec
// §1–§2): a completed rebase carrying a resolver-authored resolution, interrupted
// before publication; the base advances again (non-conflicting); re-invoking
// finalize.rebase forward-rebases from the CURRENT head, preserving the
// resolution, running the suite on the new head, and refreshing OrigHead/BaseHead/
// Attempt while preserving OrigRemoteHead and the consumed resolver budget. The
// superseded attempt token can no longer continue the refreshed rewrite.
func TestIntegrationFinalizeRebaseRecoveryForwardRefresh(t *testing.T) {
	requireRealGit(t)
	f, deps, head, begin := beginSuccessiveConflicts(t, 0, nil,
		map[string]string{"feature.txt": "conflicting base content\n"})
	gate := deps.Gate.(*fakeGate)
	attempt := begin.Attempt
	_, cont := reserveResolveContinue(t, f, deps, attempt, begin.UnmergedPaths, 1)
	if cont.Disposition != RebaseDispRebased {
		t.Fatalf("continue = %q (reason %q), want rebased (completed rewrite)", cont.Disposition, cont.Reason)
	}
	rewritten := f.localHead()
	recBefore, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if gate.calls != 1 {
		t.Fatalf("gate calls after the completing continue = %d, want 1", gate.calls)
	}

	// The base advances AGAIN (non-conflicting this time) before publication.
	f.repo.writerAdvance(t, "main", map[string]string{"later.txt": "base moved again\n"})

	out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	if out.Result != ResultApplied || out.Disposition != RebaseDispRebased {
		t.Fatalf("forward refresh = %q/%q (reason %q msg %q), want applied/rebased", out.Result, out.Disposition, out.Reason, out.Message)
	}
	// A base refresh always retests: the old checkpoint is not reused and the fake
	// gate ran a second time for the refresh call (spec §4).
	if out.Gate == nil || out.Gate.Compose != gateComposeRan {
		t.Fatalf("gate = %+v, want compose ran (a base refresh always retests)", out.Gate)
	}
	if gate.calls != 2 {
		t.Errorf("gate calls after the refresh = %d, want 2 (the refresh re-ran the suite)", gate.calls)
	}
	// The resolution content survived and the branch was never reset.
	if got := readRepoFile(t, f.wp, "feature.txt"); got != "reconciled content for cycle 1\n" {
		t.Fatalf("resolution lost: feature.txt = %q", got)
	}
	rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if rec.OrigHead != rewritten {
		t.Errorf("refreshed OrigHead = %q, want the completed rewrite head %q (never the pre-first-rebase head)", rec.OrigHead, rewritten)
	}
	if rec.OrigRemoteHead != recBefore.OrigRemoteHead {
		t.Errorf("the publication lease moved: %q -> %q", recBefore.OrigRemoteHead, rec.OrigRemoteHead)
	}
	if rec.Attempt == attempt {
		t.Errorf("refresh kept the superseded attempt token %q", attempt)
	}
	if rec.BaseHead == recBefore.BaseHead {
		t.Errorf("refreshed BaseHead unchanged %q; want the advanced base", rec.BaseHead)
	}
	if rec.ResolverLimit != recBefore.ResolverLimit || rec.ResolverUsed != recBefore.ResolverUsed {
		t.Errorf("budget changed: %s/%s -> %s/%s (base movement never replenishes)",
			recBefore.ResolverUsed, recBefore.ResolverLimit, rec.ResolverUsed, rec.ResolverLimit)
	}
	// The superseded-base checkpoint was cleared before the refresh's own gate; any
	// checkpoint present now is the refresh's own, recorded against the NEW base.
	if rec.PublishCheckpointBaseHead == recBefore.BaseHead {
		t.Errorf("the superseded-base checkpoint survived the refresh: %q", rec.PublishCheckpointBaseHead)
	}
	// The old attempt token can no longer continue the refreshed rewrite.
	stale := FinalizeRebaseContinue(context.Background(), deps, f.repo.invocation, f.id, attempt,
		ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
			ConflictedPaths: []string{"feature.txt"}, ResolverReservation: "stale-token"})
	if stale.Reason != ReasonRebaseAttemptMismatch {
		t.Errorf("stale-token continue reason = %q, want attempt-token-mismatch", stale.Reason)
	}
}

// assertRefreshRetained proves a refused base refresh retained local work: the
// workspace head, the on-disk receipt, and the remote feature head are all
// byte-identical to the pre-invocation state captured by the caller.
func assertRefreshRetained(t *testing.T, f *rebaseFixture, head string, rec workspace.RebaseReceipt, remoteBefore string) {
	t.Helper()
	if got := f.localHead(); got != head {
		t.Errorf("a refused refresh moved the local head: %q -> %q", head, got)
	}
	after, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present {
		t.Fatalf("a refused refresh left no receipt (present=%v err=%v)", present, err)
	}
	if after != rec {
		t.Errorf("a refused refresh mutated the receipt:\n before %+v\n after  %+v", rec, after)
	}
	if got := originTip(t, f.repo.origin, "feat/"+f.slug); got != remoteBefore {
		t.Errorf("a refused refresh moved the remote feature head: %q -> %q", remoteBefore, got)
	}
}

// TestIntegrationFinalizeRebaseRecoveryForwardRefreshRefusals covers acceptance 4:
// each admission conjunct refuses with head, files, receipt, and remote unchanged.
// A moved lease, a divergent (rewritten) base, and a dirty tree each block; a
// still-conflicted attempt with an outstanding reservation settles against its
// RECORDED base rather than being refreshed over (spec §1, §3).
func TestIntegrationFinalizeRebaseRecoveryForwardRefreshRefusals(t *testing.T) {
	requireRealGit(t)

	// setup drives a conflict to a completed, quiescent, unpublished rewrite (no
	// base advance yet); each case then moves the base its own way.
	setup := func(t *testing.T) (*rebaseFixture, FinalizeDeps, string) {
		f, deps, head, begin := beginSuccessiveConflicts(t, 0, nil,
			map[string]string{"feature.txt": "conflicting base content\n"})
		_, cont := reserveResolveContinue(t, f, deps, begin.Attempt, begin.UnmergedPaths, 1)
		if cont.Disposition != RebaseDispRebased {
			t.Fatalf("setup continue = %q (reason %q), want rebased", cont.Disposition, cont.Reason)
		}
		return f, deps, head
	}

	t.Run("remote-lease-moved", func(t *testing.T) {
		f, deps, head := setup(t)
		f.repo.writerAdvance(t, "main", map[string]string{"later.txt": "base moved forward\n"})
		// The remote feature head moves off the recorded publication lease.
		before := f.localHead()
		runGit(t, f.wp, "commit", "-q", "--allow-empty", "-m", "remote lease moved")
		runGit(t, f.wp, "push", "-q", "-f", "origin", "HEAD:refs/heads/feat/"+f.slug)
		runGit(t, f.wp, "reset", "--hard", before)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		remoteBefore := originTip(t, f.repo.origin, "feat/"+f.slug)
		out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		assertRebaseRefused(t, out, ResultBlocked, ReasonRebaseRemoteHeadMismatch)
		assertRefreshRetained(t, f, before, rec, remoteBefore)
	})

	t.Run("divergent-base", func(t *testing.T) {
		f, deps, head := setup(t)
		// The base tip is amended and force-pushed: the new base does NOT descend the
		// recorded base, so the refresh refuses (only forward movement is in scope).
		writeRepoFile(t, f.repo.writer, "divergent.txt", "rewritten base\n")
		runGit(t, f.repo.writer, "add", "-A")
		runGit(t, f.repo.writer, "commit", "-q", "--amend", "--no-edit")
		runGit(t, f.repo.writer, "push", "-q", "-f", "origin", "main")
		headBefore := f.localHead()
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		remoteBefore := originTip(t, f.repo.origin, "feat/"+f.slug)
		out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		assertRebaseRefused(t, out, ResultBlocked, ReasonRebaseMovedBase)
		assertRefreshRetained(t, f, headBefore, rec, remoteBefore)
	})

	t.Run("dirty-workspace", func(t *testing.T) {
		f, deps, head := setup(t)
		f.repo.writerAdvance(t, "main", map[string]string{"later.txt": "base moved forward\n"})
		writeRepoFile(t, f.wp, "scratch.txt", "uncommitted\n")
		headBefore := f.localHead()
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		remoteBefore := originTip(t, f.repo.origin, "feat/"+f.slug)
		out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		assertRebaseRefused(t, out, ResultBlocked, ReasonRebaseWorkspaceDirty)
		assertRefreshRetained(t, f, headBefore, rec, remoteBefore)
	})

	t.Run("reservation-outstanding-settles-conflicted-against-recorded-base", func(t *testing.T) {
		// A still-conflicted owned attempt with an outstanding reservation settles
		// against its RECORDED base even after the base advances — it is never
		// refreshed over (spec §3: settle a persisted rewrite before a newer base).
		f, deps, head, begin := beginSuccessiveConflicts(t, 0, nil,
			map[string]string{"feature.txt": "conflicting base content\n"})
		attempt := begin.Attempt
		recorded, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		reserve := FinalizeResolverReserve(context.Background(), deps, f.repo.invocation, f.id, attempt)
		if reserve.Disposition != ReserveReserved {
			t.Fatalf("reserve = %q (reason %q), want reserved", reserve.Disposition, reserve.Reason)
		}
		// The base advances again under the conflicted attempt.
		f.repo.writerAdvance(t, "main", map[string]string{"later.txt": "base moved during conflict\n"})
		out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		if out.Disposition != RebaseDispConflicted {
			t.Fatalf("moved base under a conflicted attempt = %q (reason %q), want conflicted", out.Disposition, out.Reason)
		}
		if out.BaseHead != recorded.BaseHead {
			t.Errorf("conflicted recovery reported base %q, want the recorded base %q (never the advanced base)", out.BaseHead, recorded.BaseHead)
		}
	})
}

// TestIntegrationFinalizeRebaseRecoveryForwardRefreshInterruptions covers
// acceptance 3 (spec §3): a crash after the refreshed receipt persisted but before
// Git started resumes BeginRebase against the recorded NEW target keeping the
// recorded attempt; a lost response after a completed refresh does not rebase
// again; and a further base advance triggers a second refresh with a fresh token.
func TestIntegrationFinalizeRebaseRecoveryForwardRefreshInterruptions(t *testing.T) {
	requireRealGit(t)

	completedThenAdvance := func(t *testing.T) (*rebaseFixture, FinalizeDeps, string, string) {
		f, deps, head, begin := beginSuccessiveConflicts(t, 0, nil,
			map[string]string{"feature.txt": "conflicting base content\n"})
		_, cont := reserveResolveContinue(t, f, deps, begin.Attempt, begin.UnmergedPaths, 1)
		if cont.Disposition != RebaseDispRebased {
			t.Fatalf("setup continue = %q (reason %q), want rebased", cont.Disposition, cont.Reason)
		}
		return f, deps, head, f.localHead()
	}

	t.Run("receipt-persisted-git-not-started-resumes-new-target", func(t *testing.T) {
		f, deps, head, rewritten := completedThenAdvance(t)
		recBefore, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		newBaseHead := f.repo.writerAdvance(t, "main", map[string]string{"later.txt": "base moved\n"})
		// Hand-write the refreshed receipt the refresh WOULD write, with Git untouched.
		crashed := recBefore
		crashed.OrigHead = rewritten
		crashed.BaseHead = newBaseHead
		crashed.Attempt = "20260920T101010Z-refresh-crash"
		crashed.GateDriveID, crashed.GateOwnerGeneration = "", ""
		crashed.PublishCheckpointHead, crashed.PublishCheckpointBaseHead = "", ""
		crashed.PublishCheckpointCommand, crashed.PublishCheckpointGate = "", ""
		crashed.PublishCheckpointPRNumber, crashed.PublishCheckpointEvidence = "", ""
		if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, crashed); err != nil {
			t.Fatal(err)
		}
		out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		if out.Disposition != RebaseDispRebased {
			t.Fatalf("pre-start resume = %q (reason %q msg %q), want rebased", out.Disposition, out.Reason, out.Message)
		}
		if out.Attempt != crashed.Attempt {
			t.Errorf("resume minted a new attempt %q; want the recorded %q", out.Attempt, crashed.Attempt)
		}
		if out.Gate == nil || out.Gate.Compose != gateComposeRan {
			t.Fatalf("gate = %+v, want compose ran (no PR-evidence skip on recovery)", out.Gate)
		}
		if got := readRepoFile(t, f.wp, "feature.txt"); got != "reconciled content for cycle 1\n" {
			t.Fatalf("resolution lost on resume: feature.txt = %q", got)
		}
	})

	t.Run("response-loss-after-refresh-does-not-rebase-again", func(t *testing.T) {
		f, deps, head, _ := completedThenAdvance(t)
		f.repo.writerAdvance(t, "main", map[string]string{"later.txt": "base moved\n"})
		first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		if first.Disposition != RebaseDispRebased {
			t.Fatalf("first refresh = %q (reason %q), want rebased", first.Disposition, first.Reason)
		}
		refreshedHead := f.localHead()
		recAfterFirst, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		// A lost response: the identical request is replayed. It must not rebase again.
		second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		if second.Result != ResultApplied || second.Disposition != RebaseDispRebased {
			t.Fatalf("replay = %q/%q (reason %q), want applied/rebased", second.Result, second.Disposition, second.Reason)
		}
		if f.localHead() != refreshedHead {
			t.Fatalf("the replay re-rebased: %q -> %q", refreshedHead, f.localHead())
		}
		recAfterSecond, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if recAfterSecond.Attempt != recAfterFirst.Attempt {
			t.Errorf("replay minted a new attempt %q; want the refreshed %q", recAfterSecond.Attempt, recAfterFirst.Attempt)
		}
	})

	t.Run("another-advance-triggers-a-second-refresh", func(t *testing.T) {
		f, deps, head, _ := completedThenAdvance(t)
		gate := deps.Gate.(*fakeGate)
		f.repo.writerAdvance(t, "main", map[string]string{"later.txt": "base moved once\n"})
		first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		if first.Disposition != RebaseDispRebased {
			t.Fatalf("first refresh = %q (reason %q), want rebased", first.Disposition, first.Reason)
		}
		firstAttempt := first.Attempt
		callsAfterFirst := gate.calls
		f.repo.writerAdvance(t, "main", map[string]string{"later2.txt": "base moved twice\n"})
		second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
		if second.Result != ResultApplied || second.Disposition != RebaseDispRebased {
			t.Fatalf("second refresh = %q/%q (reason %q), want applied/rebased", second.Result, second.Disposition, second.Reason)
		}
		if second.Gate == nil || second.Gate.Compose != gateComposeRan {
			t.Fatalf("second refresh gate = %+v, want compose ran", second.Gate)
		}
		if second.Attempt == firstAttempt {
			t.Errorf("the second refresh reused the first refresh's attempt token %q", second.Attempt)
		}
		if gate.calls <= callsAfterFirst {
			t.Errorf("the second refresh did not re-run the suite: gate calls %d -> %d", callsAfterFirst, gate.calls)
		}
	})
}
