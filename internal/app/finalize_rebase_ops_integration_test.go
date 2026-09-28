//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_finalizerebaseops.sh (prefix ^TestIntegrationFinalizeRebaseOps).

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseResolverBudgetSnapshot proves a fresh owned rebase snapshots
// the RESOLVED finalize.resolver_max_attempts into the receipt as a versioned
// budget group (version "1", the resolved non-default limit, used 0, no
// outstanding reservation), and that the conflicted result surfaces the counts in
// both its JSON document and its human text.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseResolverBudgetSnapshot(t *testing.T) {
	f, res, _ := beginConflictedWithLimit(t, 2)

	// The receipt carries the versioned budget group at the resolved non-default.
	rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present {
		t.Fatalf("receipt after fresh rebase: present=%v err=%v", present, err)
	}
	if rec.ResolverBudgetVersion != "1" || rec.ResolverLimit != "2" || rec.ResolverUsed != "0" {
		t.Fatalf("receipt budget = ver %q limit %q used %q, want 1/2/0 (the resolved non-default 2, not the built-in 3)",
			rec.ResolverBudgetVersion, rec.ResolverLimit, rec.ResolverUsed)
	}
	if rec.ResolverReservationToken != "" || rec.ResolverReservationStopped != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("fresh receipt carried a reservation: token %q stopped %q cont %q",
			rec.ResolverReservationToken, rec.ResolverReservationStopped, rec.ResolverContinuationStarted)
	}

	// The conflicted result carries the counts (2/0/2).
	if res.ResolverLimit != 2 || res.ResolverUsed != 0 || res.ResolverRemaining != 2 {
		t.Fatalf("result counts = %d/%d/%d, want 2/0/2", res.ResolverLimit, res.ResolverUsed, res.ResolverRemaining)
	}
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		ResolverLimit     int `json:"resolver_limit"`
		ResolverRemaining int `json:"resolver_remaining"`
	}
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.ResolverLimit != 2 || doc.ResolverRemaining != 2 {
		t.Errorf("JSON counts = limit %d remaining %d, want 2/2 (raw %s)", doc.ResolverLimit, doc.ResolverRemaining, buf)
	}
	if !strings.Contains(string(buf), `"resolver_limit":2`) || !strings.Contains(string(buf), `"resolver_remaining":2`) {
		t.Errorf("JSON document missing resolver counts: %s", buf)
	}
	if h := res.HumanText(); !strings.Contains(h, "0/2") || !strings.Contains(h, "2 remaining") {
		t.Errorf("HumanText does not mention the resolver counts: %q", h)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseResolverBudgetRecoveryNoResnapshot proves recoverFromReceipt
// adopts the receipt's stored budget and NEVER re-snapshots the current config: a
// mid-attempt config change (2 -> 5) does not apply to an owned attempt.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseResolverBudgetRecoveryNoResnapshot(t *testing.T) {
	f, first, deps := beginConflictedWithLimit(t, 2)
	if first.ResolverLimit != 2 {
		t.Fatalf("fresh conflicted limit = %d, want 2", first.ResolverLimit)
	}
	// The operator raises the cap mid-attempt; the owned attempt must ignore it.
	setResolverConfig(t, f, 5)
	second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if second.Disposition != RebaseDispConflicted {
		t.Fatalf("recovery = disp %q (reason %q), want conflicted", second.Disposition, second.Reason)
	}
	if second.ResolverLimit != 2 {
		t.Fatalf("recovery reported limit %d; the mid-attempt config change to 5 must NOT apply — want the receipt's 2", second.ResolverLimit)
	}
	rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if rec.ResolverLimit != "2" || rec.ResolverUsed != "0" {
		t.Errorf("recovery re-snapshotted the receipt budget to limit %q used %q; want the owned 2/0", rec.ResolverLimit, rec.ResolverUsed)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseResolverBudgetWaitingReloadsForward proves the WAITING
// gate-continuation write copies the resolver-budget group forward from the
// freshly reloaded on-disk receipt, not from a stale in-memory copy.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseResolverBudgetWaitingReloadsForward(t *testing.T) {
	f, gh, real := completedBudgetedReceipt(t, func(*workspace.RebaseReceipt) {}) // no gate pair
	ctx := context.Background()

	// Stale = the budget rolled back to its pre-reserve state; the gate pair is
	// empty so the recovery composes the gate afresh.
	stale := real
	stale.ResolverUsed = "0"
	stale.ResolverReservationToken = ""
	stale.ResolverReservationStopped = ""
	wrap := &staleFirstReadWorkspace{FinalizeWorkspace: f.svc, stale: stale}
	deps := FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: wrap,
		Gate: &fakeGate{result: LocalGateResult{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "drive-1", Generation: "gen-1"}}}}

	res := FinalizeRebase(ctx, deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Disposition != RebaseDispWaiting {
		t.Fatalf("waiting slice = %q (reason %q msg %q), want waiting", res.Disposition, res.Reason, res.Message)
	}
	after, present, err := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
	if err != nil || !present {
		t.Fatalf("receipt after WAITING: present=%v err=%v", present, err)
	}
	if after.GateDriveID != "drive-1" || after.GateOwnerGeneration != "gen-1" {
		t.Fatalf("WAITING did not set the gate pair: %q/%q", after.GateDriveID, after.GateOwnerGeneration)
	}
	assertResolverFieldsEqual(t, "after WAITING set", real, after)
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseResolverBudgetClearReloadsForward proves the terminal
// gate-continuation clear copies the resolver-budget group forward from the
// freshly reloaded on-disk receipt, not from a stale in-memory copy.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseResolverBudgetClearReloadsForward(t *testing.T) {
	f, gh, real := completedBudgetedReceipt(t, func(r *workspace.RebaseReceipt) {
		// A recorded WAITING drive so the recovery advances and then CLEARS it.
		r.PublishCheckpointHead, r.PublishCheckpointBaseHead = "", ""
		r.PublishCheckpointCommand, r.PublishCheckpointGate = "", ""
		r.PublishCheckpointPRNumber, r.PublishCheckpointEvidence = "", ""
		r.GateDriveID = "drive-9"
		r.GateOwnerGeneration = "gen-9"
	})
	ctx := context.Background()

	// Stale = the budget rolled back to its pre-reserve state; the gate pair is
	// retained so composeLocalGate advances the recorded drive to its terminal.
	stale := real
	stale.ResolverUsed = "0"
	stale.ResolverReservationToken = ""
	stale.ResolverReservationStopped = ""
	wrap := &staleFirstReadWorkspace{FinalizeWorkspace: f.svc, stale: stale}
	deps := FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: wrap,
		Gate: &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}}

	res := FinalizeRebase(ctx, deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Result != ResultApplied || res.Gate == nil || res.Gate.Evidence == "" {
		t.Fatalf("passed slice = %q gate %+v (reason %q), want applied with evidence", res.Result, res.Gate, res.Reason)
	}
	after, present, err := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
	if err != nil || !present {
		t.Fatalf("receipt after terminal: present=%v err=%v", present, err)
	}
	if after.GateDriveID != "" || after.GateOwnerGeneration != "" {
		t.Fatalf("terminal did not clear the gate pair: %q/%q", after.GateDriveID, after.GateOwnerGeneration)
	}
	assertResolverFieldsEqual(t, "after terminal clear", real, after)
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateOffCreatesNoReceipt pins that finalize.gate: off skips
// the rebase entirely — no receipt, hence no resolver budget, is created.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateOffCreatesNoReceipt(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	writeRepoFile(t, f.repo.invocation, ".docket.local.yml", "finalize:\n  gate: \"off\"\n")
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Result != ResultNoOp || res.Reason != ReasonRebaseGateOff {
		t.Fatalf("gate off = %q reason %q, want no-op/gate-off", res.Result, res.Reason)
	}
	f.receiptAbsent(t)
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReservationMissing proves a report that carries no
// resolver_reservation is refused BEFORE staging (reservation-missing), spends
// nothing, and leaves the outstanding reservation and the receipt untouched.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReservationMissing(t *testing.T) {
	f, deps, attempt, _, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam
	before := reloadReceipt(t, f)

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved, ConflictedPaths: []string{"feature.txt"}}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultBlocked || res.Reason != ReasonRebaseReservationMissing {
		t.Fatalf("continue = (%q, %q), want blocked/%q", res.Result, res.Reason, ReasonRebaseReservationMissing)
	}
	if seam.calls != 0 {
		t.Errorf("a reservation-missing refusal staged %d time(s); want 0", seam.calls)
	}
	if after := reloadReceipt(t, f); after != before {
		t.Fatalf("a reservation-missing refusal mutated the receipt:\n before %+v\n after  %+v", before, after)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReservationStaleToken proves a report echoing a foreign
// token is refused (reservation-stale) before staging. (Mutation cell a: dropping
// the token comparison lets this continue reach staging and reddens here.)
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReservationStaleToken(t *testing.T) {
	f, deps, attempt, _, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam
	// Resolve the file so that, if the guard were dropped, the continue would
	// complete (applied) rather than merely erroring — a cleaner mutation signal.
	writeRepoFile(t, f.wp, "feature.txt", "reconciled content\n")

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: "not-the-reserved-token"}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultBlocked || res.Reason != ReasonRebaseReservationStale {
		t.Fatalf("continue = (%q, %q), want blocked/%q", res.Result, res.Reason, ReasonRebaseReservationStale)
	}
	if seam.calls != 0 {
		t.Errorf("a foreign-token refusal staged %d time(s); want 0", seam.calls)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReservationStaleCommit proves a correct token bound to
// a DIFFERENT stopped commit than the live rebase is refused (reservation-stale):
// identical conflicted paths must NOT rescue a stale reservation. (Mutation cell b:
// dropping the stopped-commit comparison lets this continue proceed and reddens.)
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReservationStaleCommit(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	// Rebind the reservation to a different (but valid) stopped commit; the live
	// rebase remains stopped on the real feature commit, so the identities diverge
	// while the conflicted path (feature.txt) is byte-identical.
	seedReserveReceipt(t, f, func(r *workspace.RebaseReceipt) {
		r.ResolverReservationStopped = strings.Repeat("b", 40)
	})
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam
	writeRepoFile(t, f.wp, "feature.txt", "reconciled content\n")

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultBlocked || res.Reason != ReasonRebaseReservationStale {
		t.Fatalf("continue = (%q, %q), want blocked/%q (identical paths must not rescue a stale reservation)", res.Result, res.Reason, ReasonRebaseReservationStale)
	}
	if seam.calls != 0 {
		t.Errorf("a stale-commit refusal staged %d time(s); want 0", seam.calls)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueMarksStartedBeforeStaging proves the continuation-started
// marker is durably written BEFORE StageAndContinueRebase runs, and that a completed
// continue clears the reservation while preserving used. (Mutation cell c: skipping
// the continuation-started write reddens the contAtCall assertion.)
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueMarksStartedBeforeStaging(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam
	writeRepoFile(t, f.wp, "feature.txt", "reconciled content\n")

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultApplied || res.Disposition != RebaseDispRebased {
		t.Fatalf("continue = (%q, %q) reason %q msg %q, want applied/rebased", res.Result, res.Disposition, res.Reason, res.Message)
	}
	if seam.calls != 1 {
		t.Fatalf("staging calls = %d, want exactly 1", seam.calls)
	}
	if seam.contAtCall != "1" {
		t.Fatalf("continuation-started marker at staging time = %q, want %q (the mark must be durable before the Git mutation)", seam.contAtCall, "1")
	}
	// The completed continue composed the gate and cleared the reservation, keeping used.
	if res.Gate == nil || res.Gate.Evidence == "" {
		t.Errorf("a completed continue did not compose the gate: %+v", res.Gate)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverReservationToken != "" || rec.ResolverReservationStopped != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("a completed continue left the reservation outstanding: token %q stopped %q cont %q",
			rec.ResolverReservationToken, rec.ResolverReservationStopped, rec.ResolverContinuationStarted)
	}
	if rec.ResolverUsed != "1" {
		t.Errorf("used = %q after a completed continue, want 1 preserved", rec.ResolverUsed)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueNextConflictUnderBudget proves a continuation that
// surfaces the NEXT conflict with used < limit returns a conflicted result carrying
// the counts, with the (now-spent) reservation reconciled/cleared.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueNextConflictUnderBudget(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2) // used becomes 1
	ctx := context.Background()
	// Script the staged continue to surface another conflict without a real
	// multi-commit fixture (the real e2e is Task 8).
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f,
		script: &gitcli.RebaseStatus{Disposition: gitcli.RebaseConflicted, HeadOID: gitcli.ObjectID(strings.Repeat("c", 40)), UnmergedPaths: []string{"feature.txt"}}}
	deps.ContinueGit = seam

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultApplied || res.Disposition != RebaseDispConflicted || res.Reason != ReasonRebaseConflicted {
		t.Fatalf("continue = (%q, %q, %q), want applied/conflicted/%q", res.Result, res.Disposition, res.Reason, ReasonRebaseConflicted)
	}
	if res.ResolverLimit != 2 || res.ResolverUsed != 1 || res.ResolverRemaining != 1 {
		t.Errorf("counts = %d/%d/%d, want 2/1/1", res.ResolverLimit, res.ResolverUsed, res.ResolverRemaining)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("the next-conflict outcome left the reservation outstanding: token %q cont %q", rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
	if rec.ResolverUsed != "1" {
		t.Errorf("used = %q, want 1 preserved", rec.ResolverUsed)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueNextConflictExhausted proves the last permitted
// continuation (used == limit) that surfaces another conflict routes to the
// existing rebase disposition `blocked` with reason resolver-budget-exhausted and
// carries the counts; the rebase disposition vocabulary does not grow. The
// exhaustion HumanText names finalize.resolver_max_attempts, the used/limit, and
// the next explicit finalize attempt.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueNextConflictExhausted(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 1) // used becomes 1 == limit
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f,
		script: &gitcli.RebaseStatus{Disposition: gitcli.RebaseConflicted, HeadOID: gitcli.ObjectID(strings.Repeat("c", 40)), UnmergedPaths: []string{"feature.txt"}}}
	deps.ContinueGit = seam

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultBlocked || res.Disposition != RebaseDispBlocked || res.Reason != ReasonResolverBudgetExhausted {
		t.Fatalf("continue = (%q, %q, %q), want blocked/blocked/%q", res.Result, res.Disposition, res.Reason, ReasonResolverBudgetExhausted)
	}
	if res.ResolverLimit != 1 || res.ResolverUsed != 1 || res.ResolverRemaining != 0 {
		t.Errorf("counts = %d/%d/%d, want 1/1/0", res.ResolverLimit, res.ResolverUsed, res.ResolverRemaining)
	}
	h := res.HumanText()
	if !strings.Contains(h, "finalize.resolver_max_attempts") || !strings.Contains(h, "next explicit finalize attempt") || !strings.Contains(h, "1/1") {
		t.Errorf("exhaustion HumanText %q does not name finalize.resolver_max_attempts + used/limit + next explicit finalize attempt", h)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueLegacyRefuses proves a legacy receipt (no budget group)
// refuses any continue with resolver-budget-unavailable, while FinalizeRebaseAbort
// on the SAME legacy receipt still succeeds (abort is reservation-agnostic).
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueLegacyRefuses(t *testing.T) {
	f, _, deps := beginConflictedWithLimit(t, 2)
	ctx := context.Background()
	attempt := reloadReceipt(t, f).Attempt
	// Strip the whole budget group -> a legacy receipt (still valid: all six empty).
	seedReserveReceipt(t, f, func(r *workspace.RebaseReceipt) {
		r.ResolverBudgetVersion = ""
		r.ResolverLimit = ""
		r.ResolverUsed = ""
		r.ResolverReservationToken = ""
		r.ResolverReservationStopped = ""
		r.ResolverContinuationStarted = ""
	})

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved, ConflictedPaths: []string{"feature.txt"}}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultBlocked || res.Reason != ReasonResolverBudgetUnavailable {
		t.Fatalf("legacy continue = (%q, %q), want blocked/%q", res.Result, res.Reason, ReasonResolverBudgetUnavailable)
	}
	// Abort on the same legacy receipt still succeeds.
	abort := FinalizeRebaseAbort(ctx, deps, f.repo.invocation, f.id, attempt,
		ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverStuck})
	if abort.Result != ResultApplied || abort.Disposition != RebaseDispBlocked {
		t.Fatalf("legacy abort = (%q, %q), want applied/blocked", abort.Result, abort.Disposition)
	}
	f.receiptAbsent(t)
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueRepeatedConsumedReservation proves a repeated continue
// with an already-consumed reservation cannot advance a later commit: the second
// call refuses (reservation-missing, the reservation was cleared) and stages nothing.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueRepeatedConsumedReservation(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam
	writeRepoFile(t, f.wp, "feature.txt", "reconciled content\n")

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	first := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if first.Result != ResultApplied {
		t.Fatalf("first continue = %q (reason %q), want applied", first.Result, first.Reason)
	}
	if seam.calls != 1 {
		t.Fatalf("first continue staged %d time(s), want 1", seam.calls)
	}
	// Replaying the same (now consumed) reservation must not stage again.
	second := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if second.Result != ResultBlocked || second.Reason != ReasonRebaseReservationMissing {
		t.Fatalf("repeated continue = (%q, %q), want blocked/%q", second.Result, second.Reason, ReasonRebaseReservationMissing)
	}
	if seam.calls != 1 {
		t.Fatalf("repeated continue staged again (calls = %d); a consumed reservation cannot advance a later commit", seam.calls)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueStartedAmbiguousRetains proves a response-lost
// continuation (continuation-started marked, live rebase still stopped on the SAME
// commit) is retained and blocked with no second continue — never blindly replayed.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueStartedAmbiguousRetains(t *testing.T) {
	f, deps, attempt, token, stopped := reserveOnConflict(t, 2)
	ctx := context.Background()
	// The prior continuation marked started but its response was lost; the live
	// rebase is still stopped on the reserved commit (stopped == X).
	before := seedReserveReceipt(t, f, func(r *workspace.RebaseReceipt) {
		r.ResolverReservationStopped = stopped
		r.ResolverContinuationStarted = "1"
	})
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultBlocked || res.Reason != ReasonRebaseContinuationAmbiguous {
		t.Fatalf("ambiguous recovery = (%q, %q), want blocked/%q", res.Result, res.Reason, ReasonRebaseContinuationAmbiguous)
	}
	if seam.calls != 0 {
		t.Errorf("an ambiguous recovery staged %d time(s); want 0 (no second continue)", seam.calls)
	}
	if after := reloadReceipt(t, f); after != before {
		t.Fatalf("an ambiguous recovery mutated the receipt:\n before %+v\n after  %+v", before, after)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueStartedCompletedRecovers proves a response-lost
// continuation whose rebase provably completed (RebaseState clean, head descends
// the base) is recovered WITHOUT another charge: the reservation is reconciled, the
// gate composes, and used is preserved — no second StageAndContinueRebase.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueStartedCompletedRecovers(t *testing.T) {
	f, gh, real := completedBudgetedReceipt(t, func(r *workspace.RebaseReceipt) {
		r.ResolverContinuationStarted = "1" // a started continuation whose response was lost
	})
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps := f.finalizeDeps(gh, &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}})
	deps.ContinueGit = seam

	report := ResolverReport{ChangeID: f.id, Attempt: real.Attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: real.ResolverReservationToken}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, real.Attempt, report)
	if res.Result != ResultApplied || res.Gate == nil || res.Gate.Evidence == "" {
		t.Fatalf("completed recovery = %q gate %+v (reason %q), want applied with gate evidence", res.Result, res.Gate, res.Reason)
	}
	if seam.calls != 0 {
		t.Errorf("a completed recovery staged %d time(s); want 0 (recover without another continue)", seam.calls)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("a completed recovery left the reservation outstanding: token %q cont %q", rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
	if rec.ResolverUsed != real.ResolverUsed {
		t.Errorf("used = %q after recovery, want %q preserved (no new charge)", rec.ResolverUsed, real.ResolverUsed)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReconcileWriteFailurePreserves (change 0411, AC1)
// proves a receipt-write failure injected ONLY at post-continue reservation
// reconciliation (the started marker landed durably first) keeps the unchanged
// error result/disposition/reason, emits a message naming the same-attempt
// finalize.rebase-continue remedy WITHOUT claiming the whole rebase finished,
// preserves the outstanding reservation + started marker + used count, and never
// runs the gate before reconciliation succeeds.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReconcileWriteFailurePreserves(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2) // used == 1
	ctx := context.Background()
	writeRepoFile(t, f.wp, "feature.txt", "reconciled content\n")                 // resolve so the continue completes
	ws := &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 1, fail: true} // marker write passes, reconcile write faults
	deps.Workspace = ws
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed}}
	deps.Gate = gate

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultExternalFailed || res.Disposition != RebaseDispBlocked || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("continue = (%q, %q, %q), want external-failed/blocked/%q unchanged", res.Result, res.Disposition, res.Reason, ReasonRebaseReceiptWrite)
	}
	if !strings.Contains(res.Message, "finalize.rebase-continue") ||
		!strings.Contains(res.Message, "same change id, owned attempt, and original resolved report") ||
		!strings.Contains(res.Message, errReconcileWrite.Error()) {
		t.Errorf("message %q must name the same-attempt finalize.rebase-continue remedy and keep the write error", res.Message)
	}
	if strings.Contains(res.Message, "rebase completed") {
		t.Errorf("message %q may not assert the whole rebase finished at the post-continue site", res.Message)
	}
	if gate.calls != 0 {
		t.Errorf("gate ran %d time(s) before reconciliation succeeded; want 0", gate.calls)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverReservationToken != token || rec.ResolverContinuationStarted != "1" || rec.ResolverUsed != "1" {
		t.Errorf("receipt after failed reconcile: token %q cont %q used %q, want reservation + started marker + used preserved", rec.ResolverReservationToken, rec.ResolverContinuationStarted, rec.ResolverUsed)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReconcileWriteFailureNextConflict (change 0411, AC3)
// proves the post-continue reconcile-write failure message stays honest when the
// continue surfaced ANOTHER conflict: same remedy, no completion claim, receipt
// retained with the reservation outstanding.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueReconcileWriteFailureNextConflict(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f,
		script: &gitcli.RebaseStatus{Disposition: gitcli.RebaseConflicted, HeadOID: gitcli.ObjectID(strings.Repeat("c", 40)), UnmergedPaths: []string{"feature.txt"}}}
	deps.ContinueGit = seam
	ws := &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 1, fail: true}
	deps.Workspace = ws

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultExternalFailed || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("continue = (%q, %q), want external-failed/%q", res.Result, res.Reason, ReasonRebaseReceiptWrite)
	}
	if !strings.Contains(res.Message, "finalize.rebase-continue") || strings.Contains(res.Message, "rebase completed") {
		t.Errorf("message %q must name the remedy and never claim completion while a conflict remains", res.Message)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverReservationToken != token || rec.ResolverContinuationStarted != "1" {
		t.Errorf("receipt lost the outstanding reservation: token %q cont %q", rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueStartedCompletedRecoveryWriteFails (change 0411, AC2)
// proves the completed-rebase recovery branch's failed reconciliation write emits
// the completed-specific remedy message, repeats no staging, preserves the receipt
// — and that restoring writes and retrying the SAME report recovers: reservation
// cleared only by the existing recovery, used preserved, gate composed.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueStartedCompletedRecoveryWriteFails(t *testing.T) {
	f, gh, real := completedBudgetedReceipt(t, func(r *workspace.RebaseReceipt) {
		r.ResolverContinuationStarted = "1"
	})
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps := f.finalizeDeps(gh, &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}})
	deps.ContinueGit = seam
	ws := &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 0, fail: true} // the recovery's one write faults
	deps.Workspace = ws

	report := ResolverReport{ChangeID: f.id, Attempt: real.Attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: real.ResolverReservationToken}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, real.Attempt, report)
	if res.Result != ResultExternalFailed || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("recovery write-fail = (%q, %q), want external-failed/%q", res.Result, res.Reason, ReasonRebaseReceiptWrite)
	}
	if !strings.Contains(res.Message, "owned rebase completed") || !strings.Contains(res.Message, "finalize.rebase-continue") ||
		!strings.Contains(res.Message, errReconcileWrite.Error()) {
		t.Errorf("message %q must say the owned rebase completed, name the remedy, and keep the write error", res.Message)
	}
	if seam.calls != 0 {
		t.Errorf("recovery staged %d time(s); want 0", seam.calls)
	}
	if rec := reloadReceipt(t, f); rec.ResolverReservationToken == "" || rec.ResolverContinuationStarted != "1" || rec.ResolverUsed != real.ResolverUsed {
		t.Errorf("failed recovery mutated the receipt: %+v", rec)
	}

	ws.fail = false // durable writes restored — retry the SAME report
	res2 := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, real.Attempt, report)
	if res2.Result != ResultApplied || res2.Gate == nil {
		t.Fatalf("retry = %q gate %+v (reason %q), want applied with gate", res2.Result, res2.Gate, res2.Reason)
	}
	if seam.calls != 0 {
		t.Errorf("retry staged %d time(s); want 0 (no repeated Git continuation)", seam.calls)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" || rec.ResolverUsed != real.ResolverUsed {
		t.Errorf("retry left receipt %+v; want reservation cleared, used %q preserved (no charge, no refund)", rec, real.ResolverUsed)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueStartedAdvancedRecoveryWriteFails (change 0411, AC3)
// proves the advanced-conflict recovery branch's failed reconciliation write says
// the continuation advanced — never that the rebase completed — and a retry after
// restoring writes surfaces the next conflict without replaying the continuation.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueStartedAdvancedRecoveryWriteFails(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	// The receipt records a DIFFERENT stopped commit than the live rebase, so the
	// started continuation provably advanced.
	seedReserveReceipt(t, f, func(r *workspace.RebaseReceipt) {
		r.ResolverReservationStopped = strings.Repeat("d", 40)
		r.ResolverContinuationStarted = "1"
	})
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam
	ws := &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 0, fail: true}
	deps.Workspace = ws

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultExternalFailed || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("advanced recovery write-fail = (%q, %q), want external-failed/%q", res.Result, res.Reason, ReasonRebaseReceiptWrite)
	}
	if !strings.Contains(res.Message, "advanced to another conflict") || strings.Contains(res.Message, "rebase completed") ||
		!strings.Contains(res.Message, "finalize.rebase-continue") {
		t.Errorf("message %q must say advanced-to-another-conflict, name the remedy, and never claim completion", res.Message)
	}
	if seam.calls != 0 {
		t.Errorf("recovery staged %d time(s); want 0", seam.calls)
	}

	ws.fail = false
	res2 := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res2.Result != ResultApplied || res2.Disposition != RebaseDispConflicted {
		t.Fatalf("retry = (%q, %q, reason %q), want applied/conflicted surfacing the live conflict", res2.Result, res2.Disposition, res2.Reason)
	}
	if seam.calls != 0 {
		t.Errorf("retry replayed the continuation %d time(s); want 0", seam.calls)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueMarkerWriteFailureNoRecoveryClaim (change 0411, AC4)
// proves a write failure BEFORE Git ran (the continuation-started marker) does not
// acquire the post-completion recovery remedy: same reason, no remedy phrase.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseContinueMarkerWriteFailureNoRecoveryClaim(t *testing.T) {
	f, deps, attempt, token, _ := reserveOnConflict(t, 2)
	ctx := context.Background()
	seam := &stageSeam{FinalizeContinueGit: f.deps.Client, f: f}
	deps.ContinueGit = seam
	deps.Workspace = &reconcileFailWorkspace{FinalizeWorkspace: f.svc, allow: 0, fail: true} // the FIRST write (marker) faults

	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"feature.txt"}, ResolverReservation: token}
	res := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if res.Result != ResultExternalFailed || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("marker write-fail = (%q, %q), want external-failed/%q", res.Result, res.Reason, ReasonRebaseReceiptWrite)
	}
	if strings.Contains(res.Message, "finalize.rebase-continue") {
		t.Errorf("pre-continue marker write failure %q must not carry the post-completion recovery remedy", res.Message)
	}
	if seam.calls != 0 {
		t.Errorf("a failed marker write staged %d time(s); want 0", seam.calls)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateHaltCarriesAdmissionRefusal proves the composition
// carries the halt detail into GateReport (JSON) and the human line, keeping the
// blocked disposition, rebase-gate-halted reason, and unavailable halt cause.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateHaltCarriesAdmissionRefusal(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{
		Outcome: FinalizeGateHalted, HaltCause: GateHaltUnavailable,
		HaltReason:  "worktree-busy",
		HaltMessage: "a raw gate run occupies this worktree's execution slot; settle it with docket gate stop '/runs/x' --reason <why>",
		HaltStage:   stageWorktreeAdmission,
		HaltLocator: "incumbent-run:0123456789abcdef0123456789abcdef",
	}}
	res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if gate.calls != 1 {
		t.Fatalf("gate ran %d time(s); want exactly 1 (the halt must come from a run)", gate.calls)
	}
	if res.Result != ResultBlocked || res.Reason != ReasonRebaseGateHalted {
		t.Fatalf("result/reason = %q/%q, want blocked/%q", res.Result, res.Reason, ReasonRebaseGateHalted)
	}
	if res.Gate == nil {
		t.Fatal("no gate report on a halted composition")
	}
	if res.Gate.HaltCause != GateHaltUnavailable {
		t.Fatalf("halt cause = %q, want %q", res.Gate.HaltCause, GateHaltUnavailable)
	}
	if res.Gate.Reason != "worktree-busy" || res.Gate.Stage != stageWorktreeAdmission ||
		res.Gate.Locator != "incumbent-run:0123456789abcdef0123456789abcdef" {
		t.Fatalf("gate detail dropped: reason=%q stage=%q locator=%q", res.Gate.Reason, res.Gate.Stage, res.Gate.Locator)
	}
	if !strings.Contains(res.Gate.Message, "gate stop") {
		t.Fatalf("gate message %q lacks the remedy", res.Gate.Message)
	}
	if res.Gate.RunDir != "" {
		t.Fatalf("run_dir carries incumbent facts: %q", res.Gate.RunDir)
	}
	human := res.HumanText()
	if !strings.Contains(human, "worktree-busy") ||
		!strings.Contains(human, "incumbent-run:0123456789abcdef0123456789abcdef") ||
		!strings.Contains(human, "gate stop") {
		t.Fatalf("human line %q lacks reason + locator + remedy", human)
	}
}

// TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateHaltGenericUnchanged proves a detail-less halt keeps
// today's generic output exactly: Gate.Reason/Message/Stage/Locator all empty,
// the generic result message, and a HumanText without any locator fragment.
func TestIntegrationFinalizeRebaseOpsFinalizeRebaseGateHaltGenericUnchanged(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltUnavailable}}
	res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Result != ResultBlocked || res.Reason != ReasonRebaseGateHalted {
		t.Fatalf("result/reason = %q/%q, want blocked/%q", res.Result, res.Reason, ReasonRebaseGateHalted)
	}
	if res.Gate == nil {
		t.Fatal("no gate report on a halted composition")
	}
	if res.Gate.Reason != "" || res.Gate.Message != "" || res.Gate.Stage != "" || res.Gate.Locator != "" {
		t.Fatalf("detail-less halt grew detail: %+v", res.Gate)
	}
	if res.Message != "the local gate did not reach a decidable pass/fail; retained, no red fabricated" {
		t.Fatalf("generic halt message changed: %q", res.Message)
	}
	human := res.HumanText()
	if strings.Contains(human, "[gate:") || strings.Contains(human, "incumbent-") {
		t.Fatalf("generic human line carries a locator fragment: %q", human)
	}
}

// TestIntegrationFinalizeRebaseOpsMutateReceiptForAttemptSkipsSuperseded covers the attempt-identity guard
// (change 0438): a receipt writer observing attempt "A" must not modify a
// receipt that now records attempt "B" (a refresh superseded the rewrite
// mid-flight); the same helper writes when the observed attempt still matches.
func TestIntegrationFinalizeRebaseOpsMutateReceiptForAttemptSkipsSuperseded(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	ctx := context.Background()
	deps := f.finalizeDeps(nil, nil)
	rc := &rebaseContext{metaDir: f.metaDir}

	recB := workspace.RebaseReceipt{
		RepoIdentity:        f.gitrepo.CommonDir,
		ChangeID:            fmt.Sprintf("%d", f.id),
		OrigHead:            strings.Repeat("a", 40),
		OrigRemoteHead:      strings.Repeat("a", 40),
		BaseRef:             "refs/heads/main",
		BaseHead:            strings.Repeat("b", 40),
		Attempt:             "B",
		GateDriveID:         "drive-1",
		GateOwnerGeneration: "gen-1",
		CreatedUTC:          "2026-09-20T00:00:00Z",
	}
	if err := f.svc.WriteRebaseReceipt(ctx, f.metaDir, recB); err != nil {
		t.Fatalf("seed receipt B: %v", err)
	}

	mutate := func(r *workspace.RebaseReceipt) { r.GateDriveID, r.GateOwnerGeneration = "", "" }

	// Observing attempt "A" over an on-disk attempt-"B" receipt: superseded, skip.
	written, err := mutateReceiptForAttempt(ctx, deps, rc, "A", mutate)
	if err != nil {
		t.Fatalf("mutate for A: unexpected err %v", err)
	}
	if written {
		t.Errorf("mutate for A reported written; the superseded write must not land")
	}
	after, present, err := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
	if err != nil || !present {
		t.Fatalf("read after A: present=%v err=%v", present, err)
	}
	if after != recB {
		t.Errorf("the superseded write mutated the receipt:\n got %+v\nwant byte-identical %+v", after, recB)
	}

	// Observing attempt "B" (the current on-disk token): the write lands, and
	// only the mutated fields change.
	written, err = mutateReceiptForAttempt(ctx, deps, rc, "B", mutate)
	if err != nil {
		t.Fatalf("mutate for B: unexpected err %v", err)
	}
	if !written {
		t.Errorf("mutate for B did not report written")
	}
	got, present, err := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
	if err != nil || !present {
		t.Fatalf("read after B: present=%v err=%v", present, err)
	}
	want := recB
	want.GateDriveID, want.GateOwnerGeneration = "", ""
	if got != want {
		t.Errorf("mutate for B changed more than the gate pair:\n got %+v\nwant %+v", got, want)
	}
}
