// Driver-level Handoff/Claim tests (Task 9 wiring): the state-machine methods
// that recompute the current fingerprint through the injected git seam and
// compose the Task-5 ownership primitives (writeHandoffReceipt /
// consumeHandoffCAS). ownership_test.go and handoff_test.go prove the underlying
// CAS primitives directly; this file proves the Driver methods that the app
// service seam (internal/app) drives: a live-drive transfer round-trip, the
// old-owner invalidation a handoff causes, and the fail-closed HALTs for a wrong
// owner, an unoffered claim, and a claim over a drifted worktree.
package gatedrive

import (
	"strings"
	"testing"
)

// TestDriverHandoffThenClaimTransfersOwnership proves the full transfer chain
// through the Driver methods: a WAITING drive hands off (returning the single-use
// handoff token in Generation), the old owner can no longer advance, a fresh
// claimant consumes the receipt (returning a distinct new owner generation), and
// only the fresh owner can advance the same live run.
func TestDriverHandoffThenClaimTransfersOwnership(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{} // stays running across slices
	d, _ := newTestDriver(t, clk, proc, stableGit())

	started, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s", started.Outcome)
	}

	handoff, err := d.Handoff(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	if handoff.Outcome == HALTED {
		t.Fatalf("a clean handoff of a live drive must not HALT: %s", handoff.Cause)
	}
	if handoff.Generation == "" || handoff.Generation == started.Generation {
		t.Fatalf("Handoff must return a fresh single-use handoff token distinct from the owner, got %q", handoff.Generation)
	}
	if handoff.RawRunDir != "" {
		t.Fatalf("a non-PASSED transfer must not expose a raw run dir")
	}

	// The old owner is invalidated: it cannot advance after handing off.
	stale, err := d.Advance(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("stale Advance: %v", err)
	}
	if stale.Outcome != HALTED {
		t.Fatalf("an old owner advancing after handoff must HALT, got %s", stale.Outcome)
	}

	// A fresh claimant consumes the receipt and receives a new owner generation.
	claim, err := d.Claim(started.DriveID, handoff.Generation)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if claim.Outcome == HALTED {
		t.Fatalf("a matching claim must not HALT: %s", claim.Cause)
	}
	newOwner := claim.Generation
	if newOwner == "" || newOwner == started.Generation || newOwner == handoff.Generation {
		t.Fatalf("Claim must mint a fresh owner generation distinct from the chain, got %q", newOwner)
	}

	// Only the fresh owner can now advance the same live run.
	resumed, err := d.Advance(started.DriveID, newOwner)
	if err != nil {
		t.Fatalf("fresh-owner Advance: %v", err)
	}
	if resumed.Outcome != WAITING {
		t.Fatalf("the fresh owner must drive the same live run, got %s (%s)", resumed.Outcome, resumed.Cause)
	}
	if proc.launchN != 1 {
		t.Fatalf("a transfer must not relaunch the suite: launched %d", proc.launchN)
	}
}

// TestDriverHandoffWrongOwnerHalts proves a Handoff presenting a stale/wrong
// owner generation fails closed to HALTED (never a silent transfer) and writes no
// receipt, so the drive stays owned by its real owner.
func TestDriverHandoffWrongOwnerHalts(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())

	started, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	got, err := d.Handoff(started.DriveID, "not-the-owner")
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	if got.Outcome != HALTED {
		t.Fatalf("a wrong-owner handoff must HALT, got %s", got.Outcome)
	}
	if !strings.Contains(got.Cause, string(ErrNotOwner)) {
		t.Fatalf("cause must name the ownership rejection, got %q", got.Cause)
	}
	// No receipt was written: the drive is still owned, not offered.
	rec, err := store.Load(started.DriveID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.HandoffGeneration != "" {
		t.Fatalf("a rejected handoff must write no receipt, got %q", rec.HandoffGeneration)
	}
	if rec.OwnerGeneration != started.Generation {
		t.Fatalf("a rejected handoff must leave the real owner intact")
	}
}

// TestDriverClaimNoHandoffHalts proves a Claim on a plain WAITING drive with no
// outstanding handoff fails closed to HALTED — seeing a running suite is not
// authority to take it over.
func TestDriverClaimNoHandoffHalts(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, _ := newTestDriver(t, clk, proc, stableGit())

	started, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	got, err := d.Claim(started.DriveID, "some-token")
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if got.Outcome != HALTED {
		t.Fatalf("a claim with no outstanding handoff must HALT, got %s", got.Outcome)
	}
	if !strings.Contains(got.Cause, string(ErrNoHandoffOffered)) {
		t.Fatalf("cause must name the no-handoff rejection, got %q", got.Cause)
	}
}

// TestDriverClaimFingerprintMismatchHalts proves a claim whose worktree drifted
// since the handoff fails closed to HALTED and consumes no receipt, so the
// single-use offer survives for a correct claimant.
func TestDriverClaimFingerprintMismatchHalts(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	git := stableGit()
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, git)

	started, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	handoff, err := d.Handoff(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}

	// The worktree drifts between handoff and claim.
	git.status = "DRIFTED"
	got, err := d.Claim(started.DriveID, handoff.Generation)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if got.Outcome != HALTED {
		t.Fatalf("a drifted claim must HALT, got %s", got.Outcome)
	}
	if !strings.Contains(got.Cause, string(ErrFingerprintMismatch)) {
		t.Fatalf("cause must name the fingerprint mismatch, got %q", got.Cause)
	}
	// The receipt survives a rejected claim.
	rec, err := store.Load(started.DriveID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.HandoffGeneration != handoff.Generation {
		t.Fatalf("a rejected claim must preserve the receipt, got %q", rec.HandoffGeneration)
	}
}

// TestClaimClosesScope proves a normal Handoff+Claim of a scope-bound drive
// closes the recovery scope: the nearest-owner chain moves up on the cooperative
// transfer too, so a parent never later takes over a drive that was already
// claimed.
func TestClaimClosesScope(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{} // stays running across slices
	d, store := newTestDriver(t, clk, proc, stableGit())

	grant, started := bindWaiting(t, d, store)

	handoff, err := d.Handoff(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	claim, err := d.Claim(started.DriveID, handoff.Generation)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if claim.Outcome == HALTED {
		t.Fatalf("a clean claim must not HALT: %s", claim.Cause)
	}

	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if !scope.Closed {
		t.Fatalf("a cooperative claim of a scope-bound drive must close the scope")
	}
}
