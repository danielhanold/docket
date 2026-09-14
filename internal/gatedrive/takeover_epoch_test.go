package gatedrive

import (
	"errors"
	"testing"
)

// errFakeEpochProbe is a canned EpochRevokedFunc fault so a takeover test can drive
// the fail-closed leg.
var errFakeEpochProbe = errors.New("epoch probe failed")

// Change 0375 Task 12: a parent takeover cannot revive a cancelled or superseded
// run epoch. The epoch state lives in the app-owned registry, reached through the
// injected EpochRevokedFunc; these tests inject a fake resolver so the gatedrive
// layer's refusal is exercised in isolation.

// bindWaitingWithEpoch prepares a task scope carrying runEpoch, Starts a
// scope-bound drive under it, asserts the first slice WAITs, and returns the grant
// and WAITING doc — the same shape as bindWaiting but with a run epoch on the scope.
func bindWaitingWithEpoch(t *testing.T, d *Driver, store *Store, runEpoch string) (ScopeGrant, DriveDoc) {
	t.Helper()
	req := sampleStart()
	sreq := scopeReqFor(req, "")
	sreq.RunEpochID = runEpoch
	grant, err := store.PrepareScope(sreq)
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	req.RunEpochID = runEpoch
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("scope-bound first slice must WAIT, got %s (%s)", started.Outcome, started.Cause)
	}
	return grant, started
}

// TestTakeoverCannotReviveCancelledEpoch proves the epoch gate: with the scope's
// run epoch reported revoked (cancelled/superseded), a takeover HALTs with an
// ErrNotOwner-shaped cause and mints no owner; with the SAME setup but the epoch
// reported live, the identical takeover succeeds — so the refusal keys on the epoch
// state, not on any other guard. A resolver fault fails closed to a HALT.
func TestTakeoverCannotReviveCancelledEpoch(t *testing.T) {
	const epochID = "epoch-under-test"

	t.Run("revoked epoch refuses takeover", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		d.SetEpochRevokedResolver(func(id string) (bool, error) {
			if id != epochID {
				t.Fatalf("resolver queried for %q, want the scope's epoch %q", id, epochID)
			}
			return true, nil // cancelled or superseded
		})

		grant, started := bindWaitingWithEpoch(t, d, store, epochID)
		launchesBefore, stopsBefore := proc.launchN, proc.stopN

		took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
		if err != nil {
			t.Fatalf("Takeover: %v", err)
		}
		if took.Outcome != HALTED {
			t.Fatalf("a takeover of a revoked epoch must HALT, got %s", took.Outcome)
		}
		if took.Cause != string(ErrNotOwner) {
			t.Fatalf("HALT cause = %q, want %q (ErrNotOwner-shaped)", took.Cause, string(ErrNotOwner))
		}
		if took.Generation != "" {
			t.Fatalf("a refused takeover must mint no owner generation, got %q", took.Generation)
		}
		if proc.launchN != launchesBefore || proc.stopN != stopsBefore {
			t.Fatalf("a refused takeover must not launch or stop any process")
		}
		// The refusal leaves the scope OPEN: it never spent the single-use claim on a
		// run it could not revive.
		scope, err := store.LoadScope(grant.ScopeID)
		if err != nil {
			t.Fatalf("LoadScope: %v", err)
		}
		if scope.Closed {
			t.Fatalf("a refused takeover must not close the scope")
		}
	})

	t.Run("live epoch permits takeover", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		d.SetEpochRevokedResolver(func(id string) (bool, error) { return false, nil })

		grant, started := bindWaitingWithEpoch(t, d, store, epochID)
		took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
		if err != nil {
			t.Fatalf("Takeover: %v", err)
		}
		if took.Outcome == HALTED {
			t.Fatalf("a takeover of a LIVE epoch must not HALT, got cause %q", took.Cause)
		}
		if took.Generation == "" || took.Generation == started.Generation {
			t.Fatalf("a valid takeover must mint a fresh owner generation")
		}
	})

	t.Run("resolver fault fails closed", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		d.SetEpochRevokedResolver(func(id string) (bool, error) {
			return false, errFakeEpochProbe
		})

		grant, started := bindWaitingWithEpoch(t, d, store, epochID)
		took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
		if err != nil {
			t.Fatalf("Takeover: %v", err)
		}
		if took.Outcome != HALTED || took.Cause != CauseEpochUnreadable {
			t.Fatalf("a resolver fault must HALT epoch-unreadable, got %s/%q", took.Outcome, took.Cause)
		}
	})
}
