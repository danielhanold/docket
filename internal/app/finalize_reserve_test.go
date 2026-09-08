package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file drives finalize.resolver-reserve — the durable reserve-before-dispatch
// admission operation (change 0349) — over the same REAL feature workspace the
// rebase tests use (a real gitcli.Client, a real workspace.Service with its owned
// rebase receipt, and a real per-workspace operation lock), so the receipt
// reload-check-write critical section, the cross-goroutine serialization, and the
// no-permission-before-durability rule are exercised against real Git and a real
// flock rather than a stub.

// --- helpers --------------------------------------------------------------

// reloadReceipt reads the current on-disk owned receipt through the real service.
func reloadReceipt(t *testing.T, f *rebaseFixture) workspace.RebaseReceipt {
	t.Helper()
	rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present {
		t.Fatalf("reload receipt: present=%v err=%v", present, err)
	}
	return rec
}

// seedReserveReceipt reloads the on-disk owned receipt, applies mutate, and writes
// it back through the real service so it passes the receipt validator. It returns
// the re-read on-disk value for byte-comparison assertions.
func seedReserveReceipt(t *testing.T, f *rebaseFixture, mutate func(*workspace.RebaseReceipt)) workspace.RebaseReceipt {
	t.Helper()
	ctx := context.Background()
	rec := reloadReceipt(t, f)
	mutate(&rec)
	if err := f.svc.WriteRebaseReceipt(ctx, f.metaDir, rec); err != nil {
		t.Fatalf("seed receipt: %v", err)
	}
	return reloadReceipt(t, f)
}

// liveStoppedCommit is the full object id the live conflicted rebase is stopped on.
func liveStoppedCommit(t *testing.T, f *rebaseFixture) string {
	t.Helper()
	oid, err := f.deps.Client.StoppedRebaseCommit(context.Background(), f.wp)
	if err != nil {
		t.Fatalf("probe live stopped commit: %v", err)
	}
	return string(oid)
}

var errReserveProbe = errors.New("reserve probe boom")
var errReserveWrite = errors.New("reserve write boom")

// faultyReserveGit wraps the real git seam and faults exactly one probe so a
// reserve test can prove the operation refuses (blocked) and never increments the
// budget on an unprovable probe.
type faultyReserveGit struct {
	FinalizeReserveGit
	failStopped bool
	failState   bool
}

func (g *faultyReserveGit) RebaseState(ctx context.Context, dir string) (gitcli.RebaseStatus, error) {
	if g.failState {
		return gitcli.RebaseStatus{}, errReserveProbe
	}
	return g.FinalizeReserveGit.RebaseState(ctx, dir)
}

func (g *faultyReserveGit) StoppedRebaseCommit(ctx context.Context, dir string) (gitcli.ObjectID, error) {
	if g.failStopped {
		return "", errReserveProbe
	}
	return g.FinalizeReserveGit.StoppedRebaseCommit(ctx, dir)
}

// writeFailWorkspace wraps a FinalizeWorkspace and faults WriteRebaseReceipt so a
// reserve test can prove no `reserved` disposition is returned and the on-disk
// used count is unchanged when the durable write fails. Every other seam method
// (the operation lock, the receipt read) delegates to the embedded real service.
type writeFailWorkspace struct {
	FinalizeWorkspace
	failWrite bool
}

func (w *writeFailWorkspace) WriteRebaseReceipt(ctx context.Context, dir string, r workspace.RebaseReceipt) error {
	if w.failWrite {
		return errReserveWrite
	}
	return w.FinalizeWorkspace.WriteRebaseReceipt(ctx, dir, r)
}

// --- reserved -------------------------------------------------------------

// TestFinalizeResolverReserveReserved proves a first reservation over a budgeted
// receipt with capacity durably increments used, records the reservation token and
// the live stopped commit, and returns `reserved` with the post-increment counts —
// only after the receipt lands.
func TestFinalizeResolverReserveReserved(t *testing.T) {
	f, begin, deps := beginConflictedWithLimit(t, 2)
	ctx := context.Background()
	wantStopped := liveStoppedCommit(t, f)

	res := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, begin.Attempt)
	if res.Result != ResultApplied || res.Disposition != ReserveReserved {
		t.Fatalf("reserve = (%q, %q) reason %q msg %q, want applied/reserved", res.Result, res.Disposition, res.Reason, res.Message)
	}
	if res.Operation != OperationFinalizeResolverReserve {
		t.Errorf("operation = %q, want %q", res.Operation, OperationFinalizeResolverReserve)
	}
	if res.Reservation == "" {
		t.Fatalf("reserved carried no reservation token")
	}
	if res.StoppedCommit != wantStopped {
		t.Errorf("stopped commit = %q, want the live REBASE_HEAD %q", res.StoppedCommit, wantStopped)
	}
	if res.ResolverLimit != 2 || res.ResolverUsed != 1 || res.ResolverRemaining != 1 {
		t.Errorf("counts = %d/%d/%d, want 2/1/1", res.ResolverLimit, res.ResolverUsed, res.ResolverRemaining)
	}
	// The reservation is durable: the reloaded receipt carries used+1, the token,
	// and the stopped commit; no continuation is started yet.
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "1" || rec.ResolverReservationToken != res.Reservation || rec.ResolverReservationStopped != wantStopped {
		t.Fatalf("receipt after reserve = used %q tok %q stopped %q, want 1/%q/%q",
			rec.ResolverUsed, rec.ResolverReservationToken, rec.ResolverReservationStopped, res.Reservation, wantStopped)
	}
	if rec.ResolverContinuationStarted != "" {
		t.Errorf("reserve started a continuation: %q", rec.ResolverContinuationStarted)
	}
	// The token is minted like the attempt token: clock stamp + stopped-commit prefix.
	if !strings.HasSuffix(res.Reservation, wantStopped[:12]) {
		t.Errorf("token %q does not carry the stopped-commit prefix %q", res.Reservation, wantStopped[:12])
	}
	// The JSON document projects the reservation and counts.
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(buf), `"operation":"finalize.resolver-reserve"`) ||
		!strings.Contains(string(buf), `"reservation":`) ||
		!strings.Contains(string(buf), `"resolver_used":1`) {
		t.Errorf("JSON document missing reserve fields: %s", buf)
	}
}

// --- pending --------------------------------------------------------------

// TestFinalizeResolverReservePending proves that when a reservation is already
// outstanding, reserve echoes the same token as `pending` and admits nothing new
// (no double admission): the used count is unchanged.
func TestFinalizeResolverReservePending(t *testing.T) {
	f, begin, deps := beginConflictedWithLimit(t, 3)
	ctx := context.Background()
	stopped := liveStoppedCommit(t, f)
	before := seedReserveReceipt(t, f, func(r *workspace.RebaseReceipt) {
		r.ResolverUsed = "1"
		r.ResolverReservationToken = "tok-existing"
		r.ResolverReservationStopped = stopped
	})

	res := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, begin.Attempt)
	if res.Result != ResultNoOp || res.Disposition != ReservePending {
		t.Fatalf("reserve = (%q, %q) reason %q, want no-op/pending", res.Result, res.Disposition, res.Reason)
	}
	if res.Reservation != "tok-existing" {
		t.Errorf("pending token = %q, want the outstanding tok-existing echoed unchanged", res.Reservation)
	}
	if res.StoppedCommit != stopped {
		t.Errorf("pending stopped = %q, want %q", res.StoppedCommit, stopped)
	}
	if res.ResolverUsed != 1 || res.ResolverLimit != 3 {
		t.Errorf("pending counts = %d/%d, want 1/3", res.ResolverUsed, res.ResolverLimit)
	}
	after := reloadReceipt(t, f)
	if after != before {
		t.Fatalf("pending mutated the receipt:\n before %+v\n after  %+v", before, after)
	}
}

// --- exhausted ------------------------------------------------------------

// TestFinalizeResolverReserveExhausted proves that used == limit (no outstanding
// token) returns `exhausted` with reason resolver-budget-exhausted, the counts,
// and no Git or receipt change.
func TestFinalizeResolverReserveExhausted(t *testing.T) {
	f, begin, deps := beginConflictedWithLimit(t, 2)
	ctx := context.Background()
	before := seedReserveReceipt(t, f, func(r *workspace.RebaseReceipt) {
		r.ResolverUsed = "2" // used == limit
	})

	res := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, begin.Attempt)
	if res.Result != ResultBlocked || res.Disposition != ReserveExhausted || res.Reason != ReasonResolverBudgetExhausted {
		t.Fatalf("reserve = (%q, %q, %q), want blocked/exhausted/%q", res.Result, res.Disposition, res.Reason, ReasonResolverBudgetExhausted)
	}
	if res.ResolverLimit != 2 || res.ResolverUsed != 2 || res.ResolverRemaining != 0 {
		t.Errorf("counts = %d/%d/%d, want 2/2/0", res.ResolverLimit, res.ResolverUsed, res.ResolverRemaining)
	}
	if !strings.Contains(res.Message, "finalize.resolver_max_attempts") {
		t.Errorf("exhausted message %q does not name finalize.resolver_max_attempts", res.Message)
	}
	after := reloadReceipt(t, f)
	if after != before {
		t.Fatalf("exhausted mutated the receipt:\n before %+v\n after  %+v", before, after)
	}
}

// --- legacy ---------------------------------------------------------------

// TestFinalizeResolverReserveLegacy proves a legacy receipt (no budget group) is
// refused with blocked/resolver-budget-unavailable and left unchanged.
func TestFinalizeResolverReserveLegacy(t *testing.T) {
	f, begin, deps := beginConflictedWithLimit(t, 2)
	ctx := context.Background()
	before := seedReserveReceipt(t, f, func(r *workspace.RebaseReceipt) {
		r.ResolverBudgetVersion = ""
		r.ResolverLimit = ""
		r.ResolverUsed = ""
		r.ResolverReservationToken = ""
		r.ResolverReservationStopped = ""
		r.ResolverContinuationStarted = ""
	})

	res := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, begin.Attempt)
	if res.Result != ResultBlocked || res.Disposition != ReserveBlocked || res.Reason != ReasonResolverBudgetUnavailable {
		t.Fatalf("reserve = (%q, %q, %q), want blocked/blocked/%q", res.Result, res.Disposition, res.Reason, ReasonResolverBudgetUnavailable)
	}
	after := reloadReceipt(t, f)
	if after != before {
		t.Fatalf("legacy refusal mutated the receipt")
	}
}

// --- foreign attempt ------------------------------------------------------

// TestFinalizeResolverReserveForeignAttempt proves a wrong attempt token is
// refused by the shared owned-attempt gate (blocked/attempt-token-mismatch) and
// nothing is incremented.
func TestFinalizeResolverReserveForeignAttempt(t *testing.T) {
	f, _, deps := beginConflictedWithLimit(t, 2)
	ctx := context.Background()

	res := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, "not-the-owned-token")
	if res.Result != ResultBlocked || res.Disposition != ReserveBlocked || res.Reason != ReasonRebaseAttemptMismatch {
		t.Fatalf("reserve = (%q, %q, %q), want blocked/blocked/%q", res.Result, res.Disposition, res.Reason, ReasonRebaseAttemptMismatch)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "0" {
		t.Errorf("foreign attempt incremented used to %q", rec.ResolverUsed)
	}
}

// --- non-conflicted -------------------------------------------------------

// TestFinalizeResolverReserveNonConflicted proves a budgeted receipt with no live
// conflict (the rebase already completed) is refused (blocked/no-conflict) and
// left unchanged — there is nothing to reserve against.
func TestFinalizeResolverReserveNonConflicted(t *testing.T) {
	f, gh, real := completedBudgetedReceipt(t, func(r *workspace.RebaseReceipt) {
		// A budgeted receipt with capacity and NO outstanding reservation.
		r.ResolverUsed = "0"
		r.ResolverReservationToken = ""
		r.ResolverReservationStopped = ""
	})
	ctx := context.Background()
	deps := f.finalizeDeps(gh, &fakeGate{})

	res := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, real.Attempt)
	if res.Result != ResultBlocked || res.Disposition != ReserveBlocked || res.Reason != ReasonRebaseNoConflict {
		t.Fatalf("reserve = (%q, %q, %q), want blocked/blocked/%q", res.Result, res.Disposition, res.Reason, ReasonRebaseNoConflict)
	}
	after := reloadReceipt(t, f)
	if after != real {
		t.Fatalf("non-conflicted refusal mutated the receipt")
	}
}

// --- stopped-commit probe error -------------------------------------------

// TestFinalizeResolverReserveStoppedProbeError proves that if the stopped-commit
// probe errors (never a clean "not stopped"), reserve refuses (blocked) without
// incrementing the budget.
func TestFinalizeResolverReserveStoppedProbeError(t *testing.T) {
	f, begin, deps := beginConflictedWithLimit(t, 2)
	ctx := context.Background()
	deps.ReserveGit = &faultyReserveGit{FinalizeReserveGit: f.deps.Client, failStopped: true}
	before := reloadReceipt(t, f)

	res := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, begin.Attempt)
	if res.Disposition == ReserveReserved {
		t.Fatalf("a stopped-commit probe error returned reserved")
	}
	if res.Result != ResultExternalFailed || res.Disposition != ReserveBlocked {
		t.Fatalf("reserve = (%q, %q) reason %q, want external-failed/blocked", res.Result, res.Disposition, res.Reason)
	}
	after := reloadReceipt(t, f)
	if after != before {
		t.Fatalf("stopped-commit probe error changed the receipt (incremented the budget)")
	}
}

// --- write-failure injection ----------------------------------------------

// TestFinalizeResolverReserveWriteFailureNoAdmission proves the no-permission-
// before-durability rule: when the receipt write fails, no `reserved` disposition
// is returned and the on-disk used count is unchanged.
func TestFinalizeResolverReserveWriteFailureNoAdmission(t *testing.T) {
	f, begin, deps := beginConflictedWithLimit(t, 2)
	ctx := context.Background()
	before := reloadReceipt(t, f)
	deps.Workspace = &writeFailWorkspace{FinalizeWorkspace: f.svc, failWrite: true}

	res := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, begin.Attempt)
	if res.Disposition == ReserveReserved {
		t.Fatalf("a failed receipt write returned reserved; no permission before durability")
	}
	if res.Result != ResultExternalFailed || res.Reason != ReasonRebaseReceiptWrite {
		t.Fatalf("reserve = (%q, %q), want external-failed/%q", res.Result, res.Reason, ReasonRebaseReceiptWrite)
	}
	after := reloadReceipt(t, f)
	if after.ResolverUsed != before.ResolverUsed || after != before {
		t.Fatalf("write failure changed the on-disk used count: before %q after %q", before.ResolverUsed, after.ResolverUsed)
	}
}

// --- concurrency ----------------------------------------------------------

// TestFinalizeResolverReserveConcurrent proves the per-workspace operation lock
// serializes two concurrent reservations: exactly one is `reserved`, the other is
// `pending` (or contended), and the final used count is exactly 1 — no double
// admission.
func TestFinalizeResolverReserveConcurrent(t *testing.T) {
	f, begin, _ := beginConflictedWithLimit(t, 3)

	// Each racing reservation gets its OWN process-like deps (own reader, service,
	// and client) over the SAME on-disk repo, so the only thing they share is the
	// per-workspace flock on the metaDir — exactly how two concurrent `docket
	// finalize resolver-reserve` OS processes contend. Sharing one in-process deps
	// (and thus one gitStatusReader) would trip the race detector on a reader field
	// no real deployment shares, and would test shared Go memory rather than the
	// file lock this test is about. Build both deps here (on the test goroutine) so
	// no t.Fatalf runs off it.
	perGoroutineDeps := []FinalizeDeps{f.freshFinalizeDeps(t), f.freshFinalizeDeps(t)}

	var wg sync.WaitGroup
	results := make([]FinalizeReserveResult, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = FinalizeResolverReserve(context.Background(), perGoroutineDeps[i], f.repo.invocation, f.id, begin.Attempt)
		}(i)
	}
	wg.Wait()

	reserved, other := 0, 0
	for _, r := range results {
		switch r.Disposition {
		case ReserveReserved:
			reserved++
		case ReservePending, ReserveContended:
			other++
		default:
			t.Fatalf("unexpected disposition %q (reason %q): %+v", r.Disposition, r.Reason, results)
		}
	}
	if reserved != 1 || other != 1 {
		t.Fatalf("dispositions = %d reserved / %d pending-or-contended, want exactly 1 / 1: %+v", reserved, other, results)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "1" {
		t.Fatalf("final used = %q, want exactly 1 (no double admission)", rec.ResolverUsed)
	}
}

// compile-time assertion the result type is a renderable operation outcome.
var _ OperationResult = FinalizeReserveResult{}

// keep githubcli imported for the fake PR helper reuse across reserve tests.
var _ = githubcli.PullRequest{}
