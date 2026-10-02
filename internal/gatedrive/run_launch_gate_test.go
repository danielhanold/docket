package gatedrive

import (
	"errors"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// RunLaunchGate seam: Admit fences its admission behind the app-injected run
// liveness read (change 0437 Task 1; kept for change 0491). The gate is faked as
// a closure recording its calls; the admission body (reserve) — taking the
// worktree lock and minting the reserved drive (change 0490) — runs only when
// the gate lets it, so a refusal takes and creates nothing.
// ---------------------------------------------------------------------------

// recordingGate is a fake RunLaunchGate: it records each call's run id and
// worktree, then defers to an injected behavior. A nil behavior runs reserve
// directly (a permissive gate).
type recordingGate struct {
	calls     int
	runIDs    []string
	worktrees []string
	behave    func(runID, worktree string, reserve func() error) error
}

func (g *recordingGate) gate() RunLaunchGate {
	return func(runID, worktree string, reserve func() error) error {
		g.calls++
		g.runIDs = append(g.runIDs, runID)
		g.worktrees = append(g.worktrees, worktree)
		if g.behave == nil {
			return reserve()
		}
		return g.behave(runID, worktree, reserve)
	}
}

// driveRecordCount reports how many drive records the store holds. The store
// mints nothing until the first NewDrive/NewReservedDrive, so an absent root is
// zero — a gate refusal that reserves nothing leaves the root absent.
func driveRecordCount(t *testing.T, store *Store) int {
	t.Helper()
	entries, err := os.ReadDir(store.root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("ReadDir drive root: %v", err)
	}
	return len(entries)
}

// TestAdmitConsultsRunLaunchGateWithIDAndWorktree proves an Admit carrying RunID
// consults the gate exactly once with the run id and the request's worktree, and
// admission succeeds under a permissive gate.
func TestAdmitConsultsRunLaunchGateWithIDAndWorktree(t *testing.T) {
	t.Run("scopeless", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, _ := newTestDriver(t, clk, proc, stableGit())
		rg := &recordingGate{}
		d.SetRunLaunchGate(rg.gate())

		req := sampleStart()
		req.RunID = "e1"
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit: %v", err)
		}
		if ticket == nil {
			t.Fatal("Admit must return a ticket on success")
		}
		if rg.calls != 1 {
			t.Fatalf("gate consulted %d times, want exactly 1", rg.calls)
		}
		if rg.runIDs[0] != "e1" || rg.worktrees[0] != req.Worktree {
			t.Fatalf("gate called with (%q,%q), want (%q,%q)", rg.runIDs[0], rg.worktrees[0], "e1", req.Worktree)
		}
	})
}

// TestAdmitRunLaunchGateRefusalReservesNothing proves a gate refusal reserves
// nothing: reserve is never called, Admit returns the gate's exact error, the
// worktree lock stays free, and no reserved drive record was minted.
func TestAdmitRunLaunchGateRefusalReservesNothing(t *testing.T) {
	sentinel := errors.New("gatedrive-test: run fence refusal")

	t.Run("scopeless", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		rg := &recordingGate{behave: func(_, _ string, _ func() error) error {
			return sentinel // reserve is never invoked
		}}
		d.SetRunLaunchGate(rg.gate())

		req := sampleStart()
		req.RunID = "e1"
		ticket, err := d.Admit(req)
		if !errors.Is(err, sentinel) {
			t.Fatalf("Admit error = %v, want the gate's sentinel", err)
		}
		if ticket != nil {
			t.Fatalf("a refused admission must return no ticket, got %+v", ticket)
		}
		if !worktreeFree(t, store, req.Cwd) {
			t.Fatalf("a refused admission must leave the worktree lock free")
		}
		if n := driveRecordCount(t, store); n != 0 {
			t.Fatalf("a refused admission must mint no reserved drive record, got %d", n)
		}
	})
}

// TestAdmitGateErrorAfterReserveLeavesNothing proves an admission the caller
// never sees leaks nothing: a gate that runs reserve and then still fails makes
// Admit return the gate's error with the worktree lock closed and the reserved
// drive removed.
func TestAdmitGateErrorAfterReserveLeavesNothing(t *testing.T) {
	sentinel := errors.New("gatedrive-test: gate failed after reserve")
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	rg := &recordingGate{behave: func(_, _ string, reserve func() error) error {
		if err := reserve(); err != nil {
			return err
		}
		return sentinel
	}}
	d.SetRunLaunchGate(rg.gate())

	req := sampleStart()
	req.RunID = "e1"
	ticket, err := d.Admit(req)
	if !errors.Is(err, sentinel) || ticket != nil {
		t.Fatalf("Admit = (%v, %v), want the gate's error and no ticket", ticket, err)
	}
	if !worktreeFree(t, store, req.Cwd) {
		t.Fatalf("an admission the caller never sees must not keep the worktree lock")
	}
	if n := driveRecordCount(t, store); n != 0 {
		t.Fatalf("an admission the caller never sees must leave no reserved drive, got %d", n)
	}
}

// TestAdmitReservationRunsInsideGate proves the admission happens while the gate
// is held: the fake gate observes the worktree lock free and no drive before
// reserve, and the lock held with one reserved drive after.
func TestAdmitReservationRunsInsideGate(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()
	req.RunID = "e1"

	sawBeforeFree := false
	sawAfterHeld := false
	rg := &recordingGate{behave: func(_, _ string, reserve func() error) error {
		if worktreeFree(t, store, req.Cwd) && driveRecordCount(t, store) == 0 {
			sawBeforeFree = true
		} else {
			t.Errorf("the worktree must be free with no drive before reserve")
		}
		if err := reserve(); err != nil {
			return err
		}
		if !worktreeFree(t, store, req.Cwd) && driveRecordCount(t, store) == 1 {
			sawAfterHeld = true
		} else {
			t.Errorf("reserve must take the worktree lock and mint one reserved drive")
		}
		return nil
	}}
	d.SetRunLaunchGate(rg.gate())

	if _, err := d.Admit(req); err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if !sawBeforeFree {
		t.Fatal("the gate must observe the worktree FREE before reserve")
	}
	if !sawAfterHeld {
		t.Fatal("the gate must observe the worktree HELD after reserve")
	}
}

// TestAdmitNilGateOrEmptyRunUnchanged proves the no-run-record standalone path is
// preserved: a nil gate with a run id, and a set gate with an empty
// RunID, both admit as today. The empty-run case never consults the gate.
func TestAdmitNilGateOrEmptyRunUnchanged(t *testing.T) {
	t.Run("nil-gate-with-run-id", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		// No SetRunLaunchGate: the gate is nil.

		req := sampleStart()
		req.RunID = "e1"
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit with a nil gate must admit as today: %v", err)
		}
		if ticket == nil {
			t.Fatal("Admit must return a ticket")
		}
		if worktreeFree(t, store, req.Cwd) {
			t.Fatalf("a nil-gate admission must take the worktree lock")
		}
	})

	t.Run("empty-run-does-not-consult-gate", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		rg := &recordingGate{}
		d.SetRunLaunchGate(rg.gate())

		req := sampleStart() // RunID == ""
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit with an empty run must admit as today: %v", err)
		}
		if ticket == nil {
			t.Fatal("Admit must return a ticket")
		}
		if rg.calls != 0 {
			t.Fatalf("an empty-run admission must NOT consult the gate, called %d times", rg.calls)
		}
		if worktreeFree(t, store, req.Cwd) {
			t.Fatalf("an empty-run admission must take the worktree lock")
		}
	})
}
