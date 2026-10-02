// Fault injection and restart recovery for the worktree execution slot a drive
// start and its terminal release pass through (change 0405 Task 7, change 0375).
// Each test injects a fault at one durable transition point — the process launch
// or the slot release — and, where a restart is the
// recovery, RESTARTS (a fresh Driver over a fresh OpenStore of the same durable
// root) and proves the recovered state is coherent: never a duplicate launch and
// never a slot freed while a process may still be live.
//
// Faults are injected at the seams the driver already takes (a fake ProcessSeam's
// Launch/Stop) and at the filesystem (a read-only drive or slot directory makes
// the next atomic write fail); a mid-transition crash is modeled by hand-driving
// the durable first half of a two-phase transition and then STOPPING before the
// second, which is exactly what a fresh Store observes after a real crash.
package gatedrive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// reopenStore models a process restart: OpenStore roots a store at
// <gitCommonDir>/docket/gate-drives/v2, so recovering the common dir and opening a
// FRESH store over it exercises the same durable records a restarted process reads,
// with no carried-over in-memory state.
func reopenStore(s *Store) *Store {
	common := filepath.Dir(filepath.Dir(filepath.Dir(s.root)))
	return OpenStore(common)
}

// TestHaltedDriveDoesNotReleaseSlot proves a HALTED record does not by itself
// prove teardown. A deadline stop whose ownership cannot be established leaves
// the admission unresolved and blocks a second execution.
func TestHaltedDriveDoesNotReleaseSlot(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateRunning, runDir), nil
		},
		stop: func(string, string) (*process.StopOutcome, error) {
			return nil, fmt.Errorf("gatedrive-test: stop ownership unproven")
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()
	req.Budget = 0
	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("deadline stop must HALT, got %s (%s)", doc.Outcome, doc.Cause)
	}

	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionUnresolved {
		t.Fatalf("unproven HALTED teardown must leave an unresolved slot, got %q", slot.State)
	}
	if _, err := store.ReserveWorktreeExecution(sampleAdmission(req.Worktree)); !isOwnership(err, ErrLaunchUnconfirmed) {
		t.Fatalf("unproven HALTED teardown must block the next admission, got %v", err)
	}
}

// TestPassedDriveReleasesSlot proves a durable PASSED supervisor result releases
// the worktree execution slot for the next top-level drive.
func TestPassedDriveReleasesSlot(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, store := newTestDriver(t, clk, passObserveProc(), stableGit())
	req := sampleStart()
	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != PASSED {
		t.Fatalf("positive control must pass, got %s (%s)", doc.Outcome, doc.Cause)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionReleased {
		t.Fatalf("PASSED drive must release its slot, got %q", slot.State)
	}
}

// TestFaultCrashBetweenConfirmAndReleaseThenRestart models a process death after
// the terminal drive outcome and admission confirmation are durable but before
// the release write. A fresh driver must complete the idempotent release.
func TestFaultCrashBetweenConfirmAndReleaseThenRestart(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token, err := store.ReserveWorktreeExecution(sampleAdmission(wt))
	if err != nil {
		t.Fatalf("reserve admission: %v", err)
	}
	if err := store.ConfirmWorktreeExecution(wt, token, "run-confirmed", "/runs/confirmed"); err != nil {
		t.Fatalf("confirm admission: %v", err)
	}
	rec := seedRecord(t)
	rec.WorktreePath = wt
	rec.RawRunDir = "/runs/confirmed"
	rec.AdmissionToken = token
	rec.LastOutcome = PASSED
	id, owner := seedDrive(t, store, rec)

	rd := NewDriver(reopenStore(store), &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	if _, err := rd.Advance(id, owner); err != nil {
		t.Fatalf("restart Advance: %v", err)
	}
	slot, _, err := rd.store.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load recovered slot: %v", err)
	}
	if slot.State != admissionReleased {
		t.Fatalf("restart must release confirmed terminal slot, got %q", slot.State)
	}
}

// TestFaultReleaseInterruptedThenRestart proves a release write that was
// interrupted before its atomic rename leaves the durable terminal record able
// to recover and release on the next Advance after restart.
func TestFaultReleaseInterruptedThenRestart(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	token, err := store.ReserveWorktreeExecution(sampleAdmission(wt))
	if err != nil {
		t.Fatalf("reserve admission: %v", err)
	}
	if err := store.ConfirmWorktreeExecution(wt, token, "run-interrupted", "/runs/interrupted"); err != nil {
		t.Fatalf("confirm admission: %v", err)
	}
	rec := seedRecord(t)
	rec.WorktreePath = wt
	rec.RawRunDir = "/runs/interrupted"
	rec.AdmissionToken = token
	rec.LastOutcome = PASSED
	id, owner := seedDrive(t, store, rec)

	canonical, err := filepath.EvalSymlinks(wt)
	if err != nil {
		t.Fatalf("canonical worktree: %v", err)
	}
	slotDir := filepath.Join(store.admissionRoot, admissionKey(canonical))
	if err := os.Chmod(slotDir, 0o500); err != nil {
		t.Fatalf("make release write fail: %v", err)
	}
	d := NewDriver(store, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	if _, err := d.Advance(id, owner); err != nil {
		t.Fatalf("Advance during interrupted release: %v", err)
	}
	if err := os.Chmod(slotDir, 0o700); err != nil {
		t.Fatalf("restore admission permissions: %v", err)
	}

	rd := NewDriver(reopenStore(store), &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	if _, err := rd.Advance(id, owner); err != nil {
		t.Fatalf("restart Advance: %v", err)
	}
	slot, _, err := rd.store.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load recovered slot: %v", err)
	}
	if slot.State != admissionReleased {
		t.Fatalf("restart must complete interrupted release, got %q", slot.State)
	}
}

// TestFaultLaunchLostResponseLeavesUnresolved (change 0375 Task 3): a start's
// launch returns an error and ResolveReservation cannot prove the run never started
// (a lost launch response). The worktree slot must fail CLOSED to unresolved — a
// possibly-live process never frees the worktree — so a later start on that worktree
// is refused ErrLaunchUnconfirmed until recovery resolves it.
func TestFaultLaunchLostResponseLeavesUnresolved(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			return nil, fmt.Errorf("gatedrive-test: launch response lost")
		},
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "unresolved"}, nil
		},
	}
	d := storeTestDriver(store, clk, proc, stableGit())
	req := sampleStart()

	if _, err := d.Start(req); err == nil {
		t.Fatalf("a lost launch response must be a command failure (error)")
	}
	if proc.resolveN == 0 {
		t.Fatalf("the launch-failure leg must consult ResolveReservation")
	}
	// The worktree slot fails closed to unresolved.
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionUnresolved {
		t.Fatalf("a lost launch response must mark the worktree slot unresolved, got %q", slot.State)
	}
	// A follow-up start for a DIFFERENT change on the same worktree is refused
	// ErrLaunchUnconfirmed — the ambiguous slot blocks admission until recovery.
	req2 := sampleStart()
	req2.ChangeID = "0343"
	if _, err := d.Start(req2); !isOwnershipKind(err, ErrLaunchUnconfirmed) {
		t.Fatalf("a start over an unresolved worktree slot must fail ErrLaunchUnconfirmed, got %v", err)
	}
}

// --- change 0446 Task 9: terminal-result and cleanup audit -------------------

// slotDirFor returns the on-disk worktree slot directory the store keys for
// worktree, so a fault test can make the next slot write fail (read-only dir).
func slotDirFor(t *testing.T, store *Store, worktree string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatalf("canonical worktree: %v", err)
	}
	return filepath.Join(store.admissionRoot, admissionKey(canonical))
}

// freezeSlot makes the worktree slot directory read-only so the next release /
// stopping / unresolved write fails, and restores it at cleanup.
func freezeSlot(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("freeze slot dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}

// TestReleaseFailureSurfacesOnTerminalDoc (change 0446 spec §5 audit): a release
// write that fails at the terminal is never silently dropped. Both the persisted-
// transition site (a live drive reaching PASSED on a slice) and the idempotent
// terminal re-advance site carry a bounded ReleaseFinding, withhold the run root,
// and leave the slot NOT freed.
func TestReleaseFailureSurfacesOnTerminalDoc(t *testing.T) {
	t.Run("slice-persisted-terminal", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		running := true
		proc := &fakeProc{observe: func(runDir string) (*process.Observation, error) {
			if running {
				return obs(process.StateRunning, runDir), nil
			}
			return obs(process.StatePassed, runDir), nil
		}}
		d, store := newTestDriver(t, clk, proc, stableGit())
		req := sampleStart()
		doc, err := d.Start(req)
		if err != nil || doc.Outcome != WAITING {
			t.Fatalf("Start: %v %s", err, doc.Outcome)
		}
		freezeSlot(t, slotDirFor(t, store, req.Worktree))
		running = false
		doc, err = d.Advance(doc.DriveID, doc.Generation)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.Outcome != PASSED {
			t.Fatalf("outcome = %s, want PASSED", doc.Outcome)
		}
		if doc.ReleaseFinding == "" {
			t.Fatalf("a failed release at the terminal must surface a ReleaseFinding")
		}
		if doc.RunRoot != "" {
			t.Fatalf("a terminal whose release failed must withhold the run root, got %q", doc.RunRoot)
		}
		slot, _, err := store.LoadWorktreeExecution(req.Worktree)
		if err != nil {
			t.Fatalf("load slot: %v", err)
		}
		if slot.State == admissionReleased {
			t.Fatalf("the slot must NOT be freed when its release write failed")
		}
	})
	t.Run("terminal-readvance", func(t *testing.T) {
		store := OpenStore(testsupport.TempDir(t))
		wt := mkWorktree(t)
		token, err := store.ReserveWorktreeExecution(sampleAdmission(wt))
		if err != nil {
			t.Fatalf("reserve admission: %v", err)
		}
		if err := store.ConfirmWorktreeExecution(wt, token, "run-t9", "/runs/t9"); err != nil {
			t.Fatalf("confirm admission: %v", err)
		}
		rec := seedRecord(t)
		rec.WorktreePath = wt
		rec.RawRunDir = "/runs/t9"
		rec.AdmissionToken = token
		rec.LastOutcome = FAILED
		id, owner := seedDrive(t, store, rec)
		freezeSlot(t, slotDirFor(t, store, wt))

		d := NewDriver(store, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
		doc, err := d.Advance(id, owner)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.ReleaseFinding == "" {
			t.Fatalf("a failed release on terminal re-advance must surface a ReleaseFinding")
		}
		if strings.Contains(doc.ReleaseFinding, token) || strings.Contains(doc.ReleaseFinding, wt) {
			t.Fatalf("ReleaseFinding %q must be bounded and credential-free", doc.ReleaseFinding)
		}
		if doc.RunRoot != "" {
			t.Fatalf("a terminal whose release failed must withhold the run root, got %q", doc.RunRoot)
		}
		slot, _, err := store.LoadWorktreeExecution(wt)
		if err != nil {
			t.Fatalf("load slot: %v", err)
		}
		if slot.State != admissionExecuting {
			t.Fatalf("slot state = %q, want executing (not freed)", slot.State)
		}
	})
}

// TestHaltedDocWithholdsRunRootWhenSlotUnsettled (change 0446 spec §5 audit): a
// HALTED terminal advertises its run root only when the slot's release evidence
// is settled. A halt whose Stop could not prove teardown marks the slot stopping
// and withholds the root; a halt whose Stop proved teardown releases the slot and
// exposes it. PASSED/FAILED with a settled release are unchanged (exposed).
func TestHaltedDocWithholdsRunRootWhenSlotUnsettled(t *testing.T) {
	seedHalted := func(t *testing.T) (*Store, string, string, string) {
		store := OpenStore(testsupport.TempDir(t))
		wt := mkWorktree(t)
		token, err := store.ReserveWorktreeExecution(sampleAdmission(wt))
		if err != nil {
			t.Fatalf("reserve admission: %v", err)
		}
		if err := store.ConfirmWorktreeExecution(wt, token, "run-h", "/runs/h"); err != nil {
			t.Fatalf("confirm admission: %v", err)
		}
		rec := seedRecord(t)
		rec.WorktreePath = wt
		rec.RawRunDir = "/runs/h"
		rec.AdmissionToken = token
		rec.LastOutcome = HALTED
		rec.LastCause = "stopped-not-initiated"
		id, owner := seedDrive(t, store, rec)
		return store, wt, id, owner
	}
	t.Run("unproven-stop-withholds", func(t *testing.T) {
		store, wt, id, owner := seedHalted(t)
		proc := &fakeProc{stop: func(runDir, _ string) (*process.StopOutcome, error) {
			return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir}, nil
		}}
		doc, err := NewDriver(store, &fakeClock{now: startRun()}, proc, stableGit()).Advance(id, owner)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		slot, _, err := store.LoadWorktreeExecution(wt)
		if err != nil {
			t.Fatalf("load slot: %v", err)
		}
		if slot.State != admissionStopping {
			t.Fatalf("slot state = %q, want stopping", slot.State)
		}
		if doc.RunRoot != "" {
			t.Fatalf("HALTED with an unsettled slot must withhold RunRoot, got %q", doc.RunRoot)
		}
		if doc.ReleaseFinding != "" {
			t.Fatalf("a persisted stopping mark is not a release failure, got finding %q", doc.ReleaseFinding)
		}
	})
	t.Run("proven-stop-exposes", func(t *testing.T) {
		store, wt, id, owner := seedHalted(t)
		doc, err := NewDriver(store, &fakeClock{now: startRun()}, &fakeProc{}, stableGit()).Advance(id, owner)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		slot, _, err := store.LoadWorktreeExecution(wt)
		if err != nil {
			t.Fatalf("load slot: %v", err)
		}
		if slot.State != admissionReleased {
			t.Fatalf("slot state = %q, want released", slot.State)
		}
		if doc.RunRoot == "" {
			t.Fatalf("HALTED with a proven release must expose RunRoot")
		}
	})
	t.Run("tokenless-halted-withholds", func(t *testing.T) {
		store := OpenStore(testsupport.TempDir(t))
		rec := seedRecord(t)
		rec.LastOutcome = HALTED
		rec.LastCause = "stopped-not-initiated"
		id, owner := seedDrive(t, store, rec)
		doc, err := NewDriver(store, &fakeClock{now: startRun()}, &fakeProc{}, stableGit()).Advance(id, owner)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.RunRoot != "" {
			t.Fatalf("a tokenless (legacy) HALTED record proves no teardown; RunRoot must be withheld, got %q", doc.RunRoot)
		}
	})
	t.Run("passed-settled-exposes", func(t *testing.T) {
		store, _, id, owner := seedHalted(t)
		if err := store.ownerCAS(id, func(r *driveRecord) error { r.LastOutcome = PASSED; r.LastCause = ""; return nil }); err != nil {
			t.Fatalf("flip to PASSED: %v", err)
		}
		doc, err := NewDriver(store, &fakeClock{now: startRun()}, &fakeProc{}, stableGit()).Advance(id, owner)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.RunRoot == "" || doc.ReleaseFinding != "" {
			t.Fatalf("PASSED with a settled release must expose RunRoot and carry no finding, got root %q finding %q", doc.RunRoot, doc.ReleaseFinding)
		}
	})
}
