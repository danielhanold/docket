// Fault injection and restart recovery for the sequential-scope transitions
// (change 0405 Task 7, spec verification 8). Each test injects a fault at one of
// the durable transition points a scoped start or terminal acknowledgement passes
// through — the reservation write, the process launch, the launch-handle
// persistence, the predecessor retirement, and the closing acknowledgement — then
// RESTARTS (a fresh Driver over a fresh OpenStore of the same durable root) and
// proves the recovered state is coherent: never a duplicate launch, never a false
// "no work" (a start/ack/takeover/enumeration that wrongly reports the scope empty
// or quiescent), and — for the acknowledgement interrupted between the retire and
// the close — an idempotent repeat that COMPLETES the close is the recovery.
//
// Faults are injected at the seams the driver already takes (a fake ProcessSeam's
// Launch/Stop) and at the filesystem (a read-only scope or drive directory makes
// the next atomic write fail); a mid-transition crash is modeled by hand-driving
// the durable first half of a two-phase transition and then STOPPING before the
// second, which is exactly what a fresh Store observes after a real crash.
package gatedrive

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// reopenStore models a process restart: OpenStore roots a store at
// <gitCommonDir>/docket/gate-drives/v1, so recovering the common dir and opening a
// FRESH store over it exercises the same durable records a restarted process reads,
// with no carried-over in-memory state.
func reopenStore(s *Store) *Store {
	common := filepath.Dir(filepath.Dir(filepath.Dir(s.root)))
	return OpenStore(common)
}

// faultFirstPassed prepares a task scope and drives a first PASSED drive over its
// slot, returning the store, driver, the driver's process seam (for launch
// counts), the grant, the base request, and the first drive doc. The current slot
// is that first drive: launched, PASSED, owner set — a durable reusable
// predecessor a successor start (or the terminal acknowledgement) can consume.
func faultFirstPassed(t *testing.T) (*Store, *Driver, *fakeProc, ScopeGrant, StartRequest, DriveDoc) {
	t.Helper()
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := passObserveProc()
	d := scopedTestDriver(store, clk, proc, stableGit())
	grant, req := prepareScopedStart(t, store)
	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if first.Outcome != PASSED {
		t.Fatalf("first start must PASS (positive control), got %s (%s)", first.Outcome, first.Cause)
	}
	return store, d, proc, grant, req, first
}

// successorReq builds a successor StartRequest that presents pred as its
// predecessor receipt.
func successorReq(base StartRequest, pred DriveDoc) StartRequest {
	r := base
	r.PredecessorDriveID = pred.DriveID
	r.PredecessorOwnerGen = pred.Generation
	return r
}

// TestFaultAdmissionThenScopeReservationLostReleasesSlot (change 0375 Task 3): a
// first start freshly reserves the worktree execution slot, then reserveScopeDrive's
// write fails GENUINELY (the scope directory is read-only) — no same-scope peer
// adopted the reservation. The fresh worktree slot must be RELEASED rather than
// leaked, so the worktree is free for a later execution; nothing launches.
func TestFaultAdmissionThenScopeReservationLostReleasesSlot(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := passObserveProc()
	d := scopedTestDriver(store, clk, proc, stableGit())
	grant, req := prepareScopedStart(t, store)

	// Make the scope dir read-only so reserveScopeDrive's slot write fails AFTER the
	// worktree slot is freshly reserved (the admission root stays writable).
	scopeDir := filepath.Join(store.scopeRoot, grant.ScopeID)
	if err := os.Chmod(scopeDir, 0o500); err != nil {
		t.Fatalf("chmod scope dir read-only: %v", err)
	}
	_, serr := d.Start(req)
	if cerr := os.Chmod(scopeDir, 0o700); cerr != nil {
		t.Fatalf("restore scope dir perms: %v", cerr)
	}
	if serr == nil {
		t.Fatalf("a genuine scope-reservation write failure must fail the start")
	}
	if proc.launchN != 0 {
		t.Fatalf("a reservation failure must never launch, got %d", proc.launchN)
	}

	// The fresh worktree slot is released — a genuine loss with no adopter never leaks it.
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionReleased {
		t.Fatalf("a genuine scope-reservation loss must release the fresh worktree slot, got %q", slot.State)
	}
	// The released slot genuinely re-admits a new execution.
	if _, rerr := store.ReserveWorktreeExecution(admissionRecord{
		RepoIdentity: req.RepoDir, WorktreeRoot: req.Worktree, ScopeID: "other-scope", Kind: "scoped",
	}); rerr != nil {
		t.Fatalf("a released slot must re-admit a new execution, got %v", rerr)
	}
}

// TestHaltedDriveDoesNotReleaseSlot proves a HALTED record does not by itself
// prove teardown. A deadline stop whose ownership cannot be established leaves
// the admission unresolved and blocks a second execution.
func TestHaltedDriveDoesNotReleaseSlot(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
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
	if _, err := store.ReserveWorktreeExecution(sampleAdmission(req.Worktree)); !isOwnership(err, ErrUnresolvedExecution) {
		t.Fatalf("unproven HALTED teardown must block the next admission, got %v", err)
	}
}

// TestPassedDriveReleasesSlot proves a durable PASSED supervisor result releases
// the worktree execution slot for the next top-level drive.
func TestPassedDriveReleasesSlot(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
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

	rd := NewDriver(reopenStore(store), &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit())
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
	d := NewDriver(store, &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit())
	if _, err := d.Advance(id, owner); err != nil {
		t.Fatalf("Advance during interrupted release: %v", err)
	}
	if err := os.Chmod(slotDir, 0o700); err != nil {
		t.Fatalf("restore admission permissions: %v", err)
	}

	rd := NewDriver(reopenStore(store), &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit())
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

// TestFaultLaunchLostResponseLeavesUnresolved (change 0375 Task 3): a scoped start's
// launch returns an error and ResolveReservation cannot prove the run never started
// (a lost launch response). The worktree slot must fail CLOSED to unresolved — a
// possibly-live process never frees the worktree — so a later start on that worktree
// is refused ErrUnresolvedExecution until recovery resolves it.
func TestFaultLaunchLostResponseLeavesUnresolved(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			return nil, fmt.Errorf("gatedrive-test: launch response lost")
		},
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "unresolved"}, nil
		},
	}
	d := scopedTestDriver(store, clk, proc, stableGit())
	_, req := prepareScopedStart(t, store)

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
	// A follow-up start from a DIFFERENT scope on the same worktree is refused
	// ErrUnresolvedExecution — the ambiguous slot blocks admission until recovery.
	_, req2 := prepareScopedStartAt(t, store, req.Worktree, "0343")
	if _, err := d.Start(req2); !isOwnershipKind(err, ErrUnresolvedExecution) {
		t.Fatalf("a start over an unresolved worktree slot must fail ErrUnresolvedExecution, got %v", err)
	}
}

// TestFaultSuccessorReservationWriteFailsThenRestart (case a): a successor's
// reservation write fails (the scope directory is read-only around
// reserveScopeDrive). A pre-reservation failure must leave the scope AND the
// predecessor byte-unchanged (the predecessor keeps its recovery authority —
// retirePredecessor is never reached), launch nothing, and — after a restart — a
// correct successor retry succeeds and launches exactly once.
func TestFaultSuccessorReservationWriteFailsThenRestart(t *testing.T) {
	store, d, proc, grant, req, first := faultFirstPassed(t)
	succ := successorReq(req, first)

	scopeBefore := readScopeBytes(t, store, grant.ScopeID)
	predBefore := readDriveBytes(t, store, first.DriveID)
	launchesBefore := proc.launchN

	scopeDir := filepath.Join(store.scopeRoot, grant.ScopeID)
	if err := os.Chmod(scopeDir, 0o500); err != nil {
		t.Fatalf("chmod scope dir read-only: %v", err)
	}
	_, serr := d.Start(succ)
	// Restore perms before any read so the restart and cleanup work.
	if cerr := os.Chmod(scopeDir, 0o700); cerr != nil {
		t.Fatalf("restore scope dir perms: %v", cerr)
	}
	if serr == nil {
		t.Fatalf("a successor reservation write failure must fail the start")
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a reservation failure must never launch, launched %d->%d", launchesBefore, proc.launchN)
	}
	// Pre-reservation failure: scope and predecessor are byte-unchanged.
	if !bytes.Equal(scopeBefore, readScopeBytes(t, store, grant.ScopeID)) {
		t.Fatalf("a failed reservation must leave the scope record byte-unchanged")
	}
	if !bytes.Equal(predBefore, readDriveBytes(t, store, first.DriveID)) {
		t.Fatalf("a failed reservation must leave the predecessor byte-unchanged (recovery authority intact)")
	}

	// Restart: a correct successor retry over the same durable state succeeds and
	// launches exactly once — no duplicate from the failed attempt.
	rstore := reopenStore(store)
	rproc := passObserveProc()
	rd := scopedTestDriver(rstore, &fakeClock{now: startEpoch()}, rproc, stableGit())
	second, err := rd.Start(succ)
	if err != nil {
		t.Fatalf("a successor retry after a reservation failure must succeed: %v", err)
	}
	if second.Outcome != PASSED {
		t.Fatalf("the successor retry must PASS, got %s (%s)", second.Outcome, second.Cause)
	}
	if second.DriveID == first.DriveID {
		t.Fatalf("a successor must be a NEW drive, never a relaunch of the predecessor")
	}
	if rproc.launchN != 1 {
		t.Fatalf("the successor retry must launch exactly once, got %d", rproc.launchN)
	}
	predRec, err := rstore.Load(first.DriveID)
	if err != nil {
		t.Fatalf("Load predecessor after retry: %v", err)
	}
	if predRec.OwnerGeneration != "" {
		t.Fatalf("after the successful retry the predecessor must be retired (owner cleared)")
	}
	scope, err := rstore.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope after retry: %v", err)
	}
	if scope.CurrentDriveID != second.DriveID || scope.CurrentDriveState != scopeStateLaunched {
		t.Fatalf("after the retry the slot must be the launched successor, got id=%q state=%q", scope.CurrentDriveID, scope.CurrentDriveState)
	}
}

// TestFaultLaunchFailsThenRestart (case b): the process launch errors. The slot
// stays durably reserved (a launch-failed HALTED record with its owner retained —
// a terminal-unconsumed record outer recovery can see), never empty. After a
// restart no path launches a duplicate: a fresh start is refused ErrScopeBusy, a
// takeover of the reserved slot HALTs unresolved-launch-transition, and
// enumeration does NOT report the scope empty.
func TestFaultLaunchFailsThenRestart(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			return nil, fmt.Errorf("gatedrive-test: launch failed")
		},
	}
	d := scopedTestDriver(store, clk, proc, stableGit())
	grant, req := prepareScopedStart(t, store)

	if _, err := d.Start(req); err == nil {
		t.Fatalf("a launch failure must be a command failure (error)")
	}

	// Restart over the same durable state.
	rstore := reopenStore(store)
	scope, err := rstore.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope after restart: %v", err)
	}
	if scope.CurrentDriveID == "" || scope.CurrentDriveState != scopeStateReserved {
		t.Fatalf("after a launch failure the slot must stay reserved, got id=%q state=%q", scope.CurrentDriveID, scope.CurrentDriveState)
	}
	// No false no-work: the launch-failed drive is a terminal-unconsumed candidate.
	ids, err := rstore.FindScopeDriveIDs(req.ChangeID, "")
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) == 0 {
		t.Fatalf("a launch-failed reserved slot must not report the scope empty")
	}

	// No duplicate launch: a fresh start over the reserved slot is refused, proven by
	// a fresh (never-launching) proc.
	rproc := passObserveProc()
	rd := scopedTestDriver(rstore, &fakeClock{now: startEpoch()}, rproc, stableGit())
	if _, err := rd.Start(req); !isOwnershipKind(err, ErrScopeBusy) {
		t.Fatalf("a start over a launch-failed reserved slot must fail ErrScopeBusy, got %v", err)
	}
	if rproc.launchN != 0 {
		t.Fatalf("a refused start must never launch, got %d", rproc.launchN)
	}
	// A takeover of the reserved (unresolved) slot fails closed — a reservation is not
	// a quiescent result — without launching.
	tdoc, err := rd.Takeover(grant.ScopeID, grant.ParentCapability, "")
	if err != nil {
		t.Fatalf("Takeover: %v", err)
	}
	if tdoc.Outcome != HALTED || tdoc.Cause != string(ErrUnresolvedLaunchTransition) {
		t.Fatalf("takeover of a reserved slot must HALT unresolved-launch-transition, got %s/%q", tdoc.Outcome, tdoc.Cause)
	}
	if rproc.launchN != 0 {
		t.Fatalf("recovery must never launch, got %d", rproc.launchN)
	}
}

// TestFaultAttachLaunchFailsThenRestart (case c): the launch-handle persistence
// (attachLaunch) fails after a run was launched. The orphaned run is stopped, the
// slot stays reserved (never empty), and after a restart no path launches a
// duplicate and enumeration does not report the scope empty.
func TestFaultAttachLaunchFailsThenRestart(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	grant, req := prepareScopedStart(t, store)

	const runDir = "/runs/run1"
	var stopped []string
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			// Sabotage the reserved drive's record dir so the attachLaunch write fails.
			sc, err := store.LoadScope(grant.ScopeID)
			if err != nil {
				return nil, err
			}
			if sc.CurrentDriveID != "" {
				if err := os.Chmod(filepath.Join(store.root, sc.CurrentDriveID), 0o500); err != nil {
					return nil, err
				}
			}
			return &process.LaunchOutcome{RunID: "run1", RunDir: runDir, State: process.StateRunning}, nil
		},
		stop: func(rd, reason string) (*process.StopOutcome, error) {
			stopped = append(stopped, rd)
			return &process.StopOutcome{State: process.StateStopped, RunDir: rd, Performed: true}, nil
		},
	}
	d := scopedTestDriver(store, clk, proc, stableGit())

	if _, err := d.Start(req); err == nil {
		t.Fatalf("an attachLaunch persist failure must be a command failure (error)")
	}
	// Restore perms so the restart's reads and cleanup work.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.CurrentDriveID != "" {
		if err := os.Chmod(filepath.Join(store.root, scope.CurrentDriveID), 0o700); err != nil {
			t.Fatalf("restore drive dir perms: %v", err)
		}
	}
	// The orphaned run was stopped (orphan control).
	foundStop := false
	for _, rd := range stopped {
		if rd == runDir {
			foundStop = true
		}
	}
	if !foundStop {
		t.Fatalf("a persist failure must stop the orphaned run %q, stops=%v", runDir, stopped)
	}

	// Restart: the slot stays reserved (never treated as empty).
	rstore := reopenStore(store)
	rscope, err := rstore.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope after restart: %v", err)
	}
	if rscope.CurrentDriveID == "" || rscope.CurrentDriveState != scopeStateReserved {
		t.Fatalf("after a persist failure the slot must stay reserved, got id=%q state=%q", rscope.CurrentDriveID, rscope.CurrentDriveState)
	}
	ids, err := rstore.FindScopeDriveIDs(req.ChangeID, "")
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) == 0 {
		t.Fatalf("a persist-failed reserved slot must not report the scope empty")
	}
	// No duplicate launch: a fresh start is refused ErrScopeBusy without launching.
	rproc := passObserveProc()
	rd := scopedTestDriver(rstore, &fakeClock{now: startEpoch()}, rproc, stableGit())
	if _, err := rd.Start(req); !isOwnershipKind(err, ErrScopeBusy) {
		t.Fatalf("a start over a persist-failed reserved slot must fail ErrScopeBusy, got %v", err)
	}
	if rproc.launchN != 0 {
		t.Fatalf("a refused start must never launch, got %d", rproc.launchN)
	}
}

// TestFaultPredecessorRetirementInterruptedThenRestart (case d): a crash between a
// successor's won reservation and its predecessor retirement. The pending-ack
// journal is the recoverable record of the transition's first half; after a
// restart it is readable, and every start/ack/takeover fails typed with ZERO
// launches (never a duplicate), while the predecessor keeps its authority and
// enumeration does not report the scope empty (never a false no-work).
func TestFaultPredecessorRetirementInterruptedThenRestart(t *testing.T) {
	store, _, _, grant, req, first := faultFirstPassed(t)

	// Hand-drive ONLY the successor's first half — mint a reserved drive and win the
	// slot reservation (which journals the pending ack) — then STOP before
	// retirePredecessor and clearPendingAck, modeling a crash mid-transition.
	const succOwner = "succ-owner-gen"
	succRec := seedRecord(t)
	succRec.ScopeID = grant.ScopeID
	succRec.OwnerGeneration = succOwner
	newID, _, err := store.NewReservedDrive(succRec)
	if err != nil {
		t.Fatalf("NewReservedDrive: %v", err)
	}
	receipt := predecessorReceipt{DriveID: first.DriveID, OwnerGen: first.Generation}
	if err := store.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, newID, receipt); err != nil {
		t.Fatalf("reserveScopeDrive (first half): %v", err)
	}
	// (crash here — no retirePredecessor, no clearPendingAck, no launch)

	// Restart: the pending-ack journal survives as the recoverable second-phase record.
	rstore := reopenStore(store)
	scope, err := rstore.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope after restart: %v", err)
	}
	if scope.PendingAckDriveID != first.DriveID || scope.PendingAckOwnerGen != first.Generation {
		t.Fatalf("the pending-ack journal must name the predecessor, got id=%q gen=%q", scope.PendingAckDriveID, scope.PendingAckOwnerGen)
	}
	if scope.CurrentDriveID != newID || scope.CurrentDriveState != scopeStateReserved {
		t.Fatalf("the interrupted transition must leave the successor reserved, got id=%q state=%q", scope.CurrentDriveID, scope.CurrentDriveState)
	}
	if scope.PriorDriveID != first.DriveID {
		t.Fatalf("the reservation must record the predecessor as PriorDriveID, got %q", scope.PriorDriveID)
	}

	rproc := passObserveProc()
	rd := scopedTestDriver(rstore, &fakeClock{now: startEpoch()}, rproc, stableGit())

	// Every recovery path is a typed fail-closed with no launch. A successor start:
	// the slot is reserved (mid-transition) → ErrScopeBusy.
	if _, err := rd.Start(successorReq(req, first)); !isOwnershipKind(err, ErrScopeBusy) {
		t.Fatalf("a start over a reserved+pending slot must fail ErrScopeBusy, got %v", err)
	}
	// Acknowledging the current (reserved, pending-ack) drive is an unresolved
	// transition, never a quiescent result.
	if _, err := rd.Acknowledge(grant.ScopeID, grant.ChildCapability, newID, succOwner); !isOwnershipKind(err, ErrUnresolvedLaunchTransition) {
		t.Fatalf("ack of a pending-ack slot must fail ErrUnresolvedLaunchTransition, got %v", err)
	}
	// Takeover of the mid-transition slot HALTs unresolved-launch-transition.
	tdoc, err := rd.Takeover(grant.ScopeID, grant.ParentCapability, "")
	if err != nil {
		t.Fatalf("Takeover: %v", err)
	}
	if tdoc.Outcome != HALTED || tdoc.Cause != string(ErrUnresolvedLaunchTransition) {
		t.Fatalf("takeover of a pending-ack slot must HALT unresolved-launch-transition, got %s/%q", tdoc.Outcome, tdoc.Cause)
	}
	if rproc.launchN != 0 {
		t.Fatalf("no recovery path over an interrupted retirement may launch, got %d", rproc.launchN)
	}

	// No false no-work: the predecessor (still owned, PASSED) and the reserved
	// successor are both discoverable, and the predecessor keeps its authority.
	ids, err := rstore.FindScopeDriveIDs(req.ChangeID, "")
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) == 0 {
		t.Fatalf("an interrupted retirement must not report the scope empty")
	}
	predRec, err := rstore.Load(first.DriveID)
	if err != nil {
		t.Fatalf("Load predecessor: %v", err)
	}
	if predRec.OwnerGeneration == "" {
		t.Fatalf("an interrupted retirement must leave the predecessor's recovery authority intact")
	}
}

// TestFaultFinalAckInterruptedThenRestart (case e): a crash between the terminal
// acknowledgement's retirePredecessor (which clears the final drive's owner) and
// its closeScopeFinal (which closes the still-open scope). This is the recovery
// hole: a repeat Acknowledge with the same arguments must COMPLETE the close — the
// resumable second half of its own transition — not fail ErrStalePredecessor
// because the owner is already cleared. Recovery never launches and leaves zero
// stale recovery candidates.
func TestFaultFinalAckInterruptedThenRestart(t *testing.T) {
	_, store, grant, req, _, second := ackTwoDriveSequence(t)

	// Hand-drive the FIRST half of the terminal acknowledgement, then STOP before
	// closeScopeFinal: the drive's owner is cleared while the scope stays OPEN with
	// the final drive still current.
	if err := store.retirePredecessor(second.DriveID, second.Generation); err != nil {
		t.Fatalf("retirePredecessor (first half of the terminal ack): %v", err)
	}
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.Closed {
		t.Fatalf("the interrupted ack must leave the scope OPEN")
	}
	if scope.CurrentDriveID != second.DriveID {
		t.Fatalf("the final drive must still be the scope's current drive, got %q", scope.CurrentDriveID)
	}
	rec, err := store.Load(second.DriveID)
	if err != nil {
		t.Fatalf("Load final drive: %v", err)
	}
	if rec.OwnerGeneration != "" {
		t.Fatalf("the first half of the ack must clear the final drive's owner")
	}
	if rec.LastOutcome != PASSED {
		t.Fatalf("the final drive must retain its terminal verdict, got %s", rec.LastOutcome)
	}

	// Restart: a fresh driver over the same durable state. A repeat Acknowledge with
	// the SAME arguments must complete the close.
	rstore := reopenStore(store)
	rproc := passObserveProc()
	rd := scopedTestDriver(rstore, &fakeClock{now: startEpoch()}, rproc, stableGit())

	doc, err := rd.Acknowledge(grant.ScopeID, grant.ChildCapability, second.DriveID, second.Generation)
	if err != nil {
		t.Fatalf("a repeat Acknowledge after an interrupted close must complete it, got %v", err)
	}
	if doc.Outcome != PASSED {
		t.Fatalf("the recovered ack must report the recorded PASSED verdict, got %s (%s)", doc.Outcome, doc.Cause)
	}
	if doc.DriveID != second.DriveID {
		t.Fatalf("the recovered ack must name the final drive, got %q", doc.DriveID)
	}

	// The close is now complete.
	rscope, err := rstore.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope after recovery: %v", err)
	}
	if !rscope.Closed || !rscope.FinalAcked {
		t.Fatalf("the recovered ack must close the scope with FinalAcked, got Closed=%v FinalAcked=%v", rscope.Closed, rscope.FinalAcked)
	}

	// Recovery never launches, and leaves zero stale recovery candidates.
	if rproc.launchN != 0 {
		t.Fatalf("acknowledgement recovery must never launch, got %d", rproc.launchN)
	}
	ids, err := rstore.FindScopeDriveIDs(req.ChangeID, "")
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("after a completed acknowledgement there must be zero recovery candidates, got %v", ids)
	}

	// A further repeat is the idempotent byte-identical no-op (the closed-scope
	// recorded-terminal fast path).
	doc2, err := rd.Acknowledge(grant.ScopeID, grant.ChildCapability, second.DriveID, second.Generation)
	if err != nil {
		t.Fatalf("a repeat after recovery must be an idempotent no-op, got %v", err)
	}
	if doc2.Outcome != PASSED {
		t.Fatalf("the idempotent repeat must return the recorded PASSED, got %s", doc2.Outcome)
	}

	// A successor presenting the acknowledged final drive is refused: the scope is
	// closed, never a false no-work that admits a new launch.
	if _, err := rd.Start(successorReq(req, second)); !isOwnershipKind(err, ErrScopeClosed) {
		t.Fatalf("a successor after the recovered terminal ack must be refused ErrScopeClosed, got %v", err)
	}
	if rproc.launchN != 0 {
		t.Fatalf("a refused post-recovery successor must never launch, got %d", rproc.launchN)
	}
}

// --- change 0446 Task 9: terminal-result and cleanup audit -------------------

// slotDirFor returns the on-disk admission slot directory the store keys for
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
		clk := &fakeClock{now: startEpoch()}
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

		d := NewDriver(store, &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit())
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
		doc, err := NewDriver(store, &fakeClock{now: startEpoch()}, proc, stableGit()).Advance(id, owner)
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
		doc, err := NewDriver(store, &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit()).Advance(id, owner)
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
		doc, err := NewDriver(store, &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit()).Advance(id, owner)
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
		doc, err := NewDriver(store, &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit()).Advance(id, owner)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if doc.RunRoot == "" || doc.ReleaseFinding != "" {
			t.Fatalf("PASSED with a settled release must expose RunRoot and carry no finding, got root %q finding %q", doc.RunRoot, doc.ReleaseFinding)
		}
	})
}
