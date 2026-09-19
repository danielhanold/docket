package gatedrive

import (
	"errors"
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// ---------------------------------------------------------------------------
// EpochLaunchGate seam: Admit fences its durable reservation behind the
// app-injected epoch liveness read (change 0437 Task 1). The gate is faked as a
// closure recording its calls; the reservation body (reserve) runs only when the
// gate lets it, so a refusal reserves nothing.
// ---------------------------------------------------------------------------

// recordingGate is a fake EpochLaunchGate: it records each call's epoch id and
// worktree, then defers to an injected behavior. A nil behavior runs reserve
// directly (a permissive gate).
type recordingGate struct {
	calls     int
	epochIDs  []string
	worktrees []string
	behave    func(epochID, worktree string, reserve func() error) error
}

func (g *recordingGate) gate() EpochLaunchGate {
	return func(epochID, worktree string, reserve func() error) error {
		g.calls++
		g.epochIDs = append(g.epochIDs, epochID)
		g.worktrees = append(g.worktrees, worktree)
		if g.behave == nil {
			return reserve()
		}
		return g.behave(epochID, worktree, reserve)
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

// TestAdmitConsultsEpochGateWithIDAndWorktree proves a scoped and a scopeless
// Admit carrying RunEpochID each consult the gate exactly once with the epoch id
// and the request's worktree, and admission succeeds under a permissive gate.
func TestAdmitConsultsEpochGateWithIDAndWorktree(t *testing.T) {
	t.Run("scopeless", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, _ := newTestDriver(t, clk, proc, stableGit())
		rg := &recordingGate{}
		d.SetEpochLaunchGate(rg.gate())

		req := sampleStart()
		req.RunEpochID = "e1"
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
		if rg.epochIDs[0] != "e1" || rg.worktrees[0] != req.Worktree {
			t.Fatalf("gate called with (%q,%q), want (%q,%q)", rg.epochIDs[0], rg.worktrees[0], "e1", req.Worktree)
		}
	})

	t.Run("scoped", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		store := OpenStore(testsupport.TempDir(t))
		grant, req := prepareScopedStart(t, store)
		req.RunEpochID = "e1"
		d := scopedTestDriver(store, clk, proc, stableGit())
		rg := &recordingGate{}
		d.SetEpochLaunchGate(rg.gate())

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
		if rg.epochIDs[0] != "e1" || rg.worktrees[0] != req.Worktree {
			t.Fatalf("gate called with (%q,%q), want (%q,%q)", rg.epochIDs[0], rg.worktrees[0], "e1", req.Worktree)
		}
		// The scope slot was durably reserved inside the gate.
		if _, err := store.LoadScope(grant.ScopeID); err != nil {
			t.Fatalf("LoadScope: %v", err)
		}
	})
}

// TestAdmitEpochGateRefusalReservesNothing proves a gate refusal reserves
// nothing: reserve is never called, Admit returns the gate's exact error, no
// worktree slot exists, no reserved drive record was minted, and (scoped) the
// scope slot is untouched.
func TestAdmitEpochGateRefusalReservesNothing(t *testing.T) {
	sentinel := errors.New("gatedrive-test: epoch fence refusal")

	t.Run("scopeless", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		rg := &recordingGate{behave: func(_, _ string, _ func() error) error {
			return sentinel // reserve is never invoked
		}}
		d.SetEpochLaunchGate(rg.gate())

		req := sampleStart()
		req.RunEpochID = "e1"
		ticket, err := d.Admit(req)
		if !errors.Is(err, sentinel) {
			t.Fatalf("Admit error = %v, want the gate's sentinel", err)
		}
		if ticket != nil {
			t.Fatalf("a refused admission must return no ticket, got %+v", ticket)
		}
		if _, _, lerr := store.LoadWorktreeExecution(req.Worktree); !storeErrIs(lerr, ErrNotFound) {
			t.Fatalf("no worktree slot must exist after refusal, LoadWorktreeExecution err = %v", lerr)
		}
		if n := driveRecordCount(t, store); n != 0 {
			t.Fatalf("a refused admission must mint no reserved drive record, got %d", n)
		}
	})

	t.Run("scoped", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		store := OpenStore(testsupport.TempDir(t))
		grant, req := prepareScopedStart(t, store)
		req.RunEpochID = "e1"
		d := scopedTestDriver(store, clk, proc, stableGit())
		rg := &recordingGate{behave: func(_, _ string, _ func() error) error {
			return sentinel // reserve is never invoked
		}}
		d.SetEpochLaunchGate(rg.gate())

		ticket, err := d.Admit(req)
		if !errors.Is(err, sentinel) {
			t.Fatalf("Admit error = %v, want the gate's sentinel", err)
		}
		if ticket != nil {
			t.Fatalf("a refused admission must return no ticket, got %+v", ticket)
		}
		if _, _, lerr := store.LoadWorktreeExecution(req.Worktree); !storeErrIs(lerr, ErrNotFound) {
			t.Fatalf("no worktree slot must exist after refusal, LoadWorktreeExecution err = %v", lerr)
		}
		if n := driveRecordCount(t, store); n != 0 {
			t.Fatalf("a refused admission must mint no reserved drive record, got %d", n)
		}
		// The scope slot is untouched: no drive reserved into it.
		scope, err := store.LoadScope(grant.ScopeID)
		if err != nil {
			t.Fatalf("LoadScope: %v", err)
		}
		if scope.CurrentDriveID != "" {
			t.Fatalf("a refused admission must not touch the scope slot, got current drive %q", scope.CurrentDriveID)
		}
	})
}

// TestAdmitReservationRunsInsideGate proves the durable decision happens while
// the gate is held: the fake gate observes the worktree slot absent before
// reserve and reserved after.
func TestAdmitReservationRunsInsideGate(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())

	sawBeforeAbsent := false
	sawAfterReserved := false
	rg := &recordingGate{behave: func(_, worktree string, reserve func() error) error {
		if _, _, err := store.LoadWorktreeExecution(worktree); storeErrIs(err, ErrNotFound) {
			sawBeforeAbsent = true
		} else {
			t.Errorf("worktree slot must be absent before reserve, LoadWorktreeExecution err = %v", err)
		}
		if err := reserve(); err != nil {
			return err
		}
		slot, _, err := store.LoadWorktreeExecution(worktree)
		if err != nil {
			t.Errorf("worktree slot must exist after reserve: %v", err)
		} else if slot.State == admissionReserved {
			sawAfterReserved = true
		} else {
			t.Errorf("slot state after reserve = %q, want %q", slot.State, admissionReserved)
		}
		return nil
	}}
	d.SetEpochLaunchGate(rg.gate())

	req := sampleStart()
	req.RunEpochID = "e1"
	if _, err := d.Admit(req); err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if !sawBeforeAbsent {
		t.Fatal("the gate must observe the worktree slot ABSENT before reserve")
	}
	if !sawAfterReserved {
		t.Fatal("the gate must observe the worktree slot RESERVED after reserve")
	}
}

// TestAdmitNilGateOrEmptyEpochUnchanged proves the epoch-less standalone path is
// preserved: a nil gate with an epoch id, and a set gate with an empty
// RunEpochID, both admit as today. The empty-epoch case never consults the gate.
func TestAdmitNilGateOrEmptyEpochUnchanged(t *testing.T) {
	t.Run("nil-gate-with-epoch-id", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		// No SetEpochLaunchGate: the gate is nil.

		req := sampleStart()
		req.RunEpochID = "e1"
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit with a nil gate must admit as today: %v", err)
		}
		if ticket == nil {
			t.Fatal("Admit must return a ticket")
		}
		if slot, _, lerr := store.LoadWorktreeExecution(req.Worktree); lerr != nil || slot.State != admissionReserved {
			t.Fatalf("a nil-gate admission must reserve the slot as today, state=%q err=%v", slot.State, lerr)
		}
	})

	t.Run("empty-epoch-does-not-consult-gate", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		rg := &recordingGate{}
		d.SetEpochLaunchGate(rg.gate())

		req := sampleStart() // RunEpochID == ""
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit with an empty epoch must admit as today: %v", err)
		}
		if ticket == nil {
			t.Fatal("Admit must return a ticket")
		}
		if rg.calls != 0 {
			t.Fatalf("an empty-epoch admission must NOT consult the gate, called %d times", rg.calls)
		}
		if slot, _, lerr := store.LoadWorktreeExecution(req.Worktree); lerr != nil || slot.State != admissionReserved {
			t.Fatalf("an empty-epoch admission must reserve the slot as today, state=%q err=%v", slot.State, lerr)
		}
	})
}
