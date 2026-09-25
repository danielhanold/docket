package gatedrive

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// ---------------------------------------------------------------------------
// Successor execution-reservation rotation (change 0437 Task 4). A same-scope
// successor that continues a sequence over the SAME executing worktree slot no
// longer reuses the predecessor's reservation token: it ROTATES the slot to its
// OWN fresh reservation (a new ReservationToken, a bumped ExecutionGen, cleared
// raw-run identity), so a stale predecessor cleanup can never free or poison the
// successor's slot, and the successor owns its post-launch failure legs.
// ---------------------------------------------------------------------------

// driveSuccessorRotation drives a scoped first start to an executing worktree
// slot, settles its drive to a terminal PASSED WITHOUT releasing the slot (the
// terminal-before-release window in which the executing arm is reached), then
// drives the same-scope successor over the still-executing slot so it rotates.
// It returns the store, the shared request, and the predecessor's now-stale
// reservation token.
func driveSuccessorRotation(t *testing.T) (store *Store, req StartRequest, oldToken string) {
	t.Helper()
	clk := &fakeClock{now: startEpoch()}
	store = OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{} // launch + observe running: first start WAITs, slot executing
	d := scopedTestDriver(store, clk, proc, stableGit())
	_, req = prepareScopedStart(t, store)
	req.RunEpochID = "E-rot"

	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if first.Outcome != WAITING {
		t.Fatalf("first start over a live run must WAIT, got %s (%s)", first.Outcome, first.Cause)
	}
	predSlot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if predSlot.State != admissionExecuting {
		t.Fatalf("the predecessor slot must be executing, got %q", predSlot.State)
	}
	oldToken = predSlot.ReservationToken

	// Terminal-before-release window: the predecessor's suite reached PASSED but its
	// slot has not been released yet, so the successor reaches the executing arm.
	if err := store.ownerCAS(first.DriveID, func(r *driveRecord) error {
		r.LastOutcome = PASSED
		return nil
	}); err != nil {
		t.Fatalf("settle predecessor terminal: %v", err)
	}

	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	second, err := d.Start(succ)
	if err != nil {
		t.Fatalf("successor Start: %v", err)
	}
	if second.Outcome != WAITING {
		t.Fatalf("successor start over a live run must WAIT, got %s (%s)", second.Outcome, second.Cause)
	}
	if second.DriveID == first.DriveID {
		t.Fatalf("a successor must be a NEW drive, got the predecessor id")
	}
	return store, req, oldToken
}

// TestSuccessorRotatesExecutingSlot proves the executing arm rotates: the
// successor launches under a FRESH reservation token with a bumped ExecutionGen
// and cleared raw-run identity, the scope/epoch survive, and after launch-confirm
// the slot carries the SUCCESSOR's run identity.
func TestSuccessorRotatesExecutingSlot(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	var launchSnaps []admissionRecord
	worktree := sampleWorktree()
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			slot, _, _ := store.LoadWorktreeExecution(worktree)
			launchSnaps = append(launchSnaps, slot)
			id := fmt.Sprintf("run%d", len(launchSnaps))
			return &process.LaunchOutcome{RunID: id, RunDir: "/runs/" + id, State: process.StateRunning}, nil
		},
		// observe defaults to running so each start WAITs and holds the slot.
	}
	d := scopedTestDriver(store, clk, proc, stableGit())
	_, req := prepareScopedStart(t, store)
	req.RunEpochID = "E-rot"

	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if first.Outcome != WAITING {
		t.Fatalf("first start must WAIT, got %s (%s)", first.Outcome, first.Cause)
	}
	predSlot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	oldToken := predSlot.ReservationToken

	// Terminal-before-release window.
	if err := store.ownerCAS(first.DriveID, func(r *driveRecord) error {
		r.LastOutcome = PASSED
		return nil
	}); err != nil {
		t.Fatalf("settle predecessor terminal: %v", err)
	}

	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	second, err := d.Start(succ)
	if err != nil {
		t.Fatalf("successor Start: %v", err)
	}
	if second.Outcome != WAITING {
		t.Fatalf("successor start must WAIT, got %s (%s)", second.Outcome, second.Cause)
	}

	if len(launchSnaps) != 2 {
		t.Fatalf("want two launches (predecessor + successor), got %d", len(launchSnaps))
	}
	succSnap := launchSnaps[1]
	if succSnap.ReservationToken == oldToken {
		t.Fatalf("the successor must launch under a ROTATED token, still had the predecessor's")
	}
	if succSnap.State != admissionReserved {
		t.Fatalf("at the successor launch the rotated slot must be reserved, got %q", succSnap.State)
	}
	if succSnap.ExecutionGen != predSlot.ExecutionGen+1 {
		t.Fatalf("rotation must bump ExecutionGen, got %d want %d", succSnap.ExecutionGen, predSlot.ExecutionGen+1)
	}
	if succSnap.RawRunID != "" || succSnap.RawRunDir != "" {
		t.Fatalf("rotation must clear the raw-run identity before launch, got (%q,%q)", succSnap.RawRunID, succSnap.RawRunDir)
	}
	if succSnap.RunEpochID != "E-rot" {
		t.Fatalf("rotation must preserve the run epoch, got %q", succSnap.RunEpochID)
	}
	if succSnap.ScopeID != req.ScopeID {
		t.Fatalf("rotation must preserve the scope, got %q want %q", succSnap.ScopeID, req.ScopeID)
	}

	final, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after successor: %v", err)
	}
	if final.State != admissionExecuting {
		t.Fatalf("after the successor launch the slot must be executing, got %q", final.State)
	}
	if final.ReservationToken == oldToken {
		t.Fatalf("the successor slot must carry the rotated token, still had the predecessor's")
	}
	if final.RawRunID != "run2" {
		t.Fatalf("after launch-confirm the slot must carry the successor's run identity, got %q", final.RawRunID)
	}
	if final.RunEpochID != "E-rot" || final.ScopeID != req.ScopeID {
		t.Fatalf("the scope/epoch fields must survive rotation, got epoch=%q scope=%q", final.RunEpochID, final.ScopeID)
	}
}

// TestLatePredecessorReleaseCannotFreeSuccessor proves the predecessor's stale
// token has NO authority after rotation: its late release/unresolve/stopping calls
// all fail ErrNotOwner and leave the successor's slot byte-stable.
func TestLatePredecessorReleaseCannotFreeSuccessor(t *testing.T) {
	store, req, oldToken := driveSuccessorRotation(t)

	after, genAfter, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	newToken := after.ReservationToken
	if newToken == oldToken {
		t.Fatalf("the successor slot must carry a rotated token")
	}

	stale := map[string]func() error{
		"release":    func() error { return store.ReleaseWorktreeExecution(req.Worktree, oldToken) },
		"unresolved": func() error { return store.MarkWorktreeExecutionUnresolved(req.Worktree, oldToken) },
		"stopping":   func() error { return store.MarkWorktreeExecutionStopping(req.Worktree, oldToken) },
	}
	for name, call := range stale {
		if err := call(); !isOwnership(err, ErrNotOwner) {
			t.Fatalf("%s with the stale predecessor token must be ErrNotOwner, got %v", name, err)
		}
	}

	check, genCheck, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after stale calls: %v", err)
	}
	if genCheck != genAfter {
		t.Fatalf("a stale predecessor call must not rewrite the successor slot (generation changed)")
	}
	if check.ReservationToken != newToken || check.State != admissionExecuting {
		t.Fatalf("the successor slot must survive stale predecessor calls, got token=%q state=%q", check.ReservationToken, check.State)
	}
}

// TestSuccessorAdmissionFailureLegsReleaseRotatedSlot proves a post-rotation
// admission-half failure releases the rotated reservation (never leaks it, never
// leaves it blocking), and that a stale receipt never reaches that leg at all.
//
// A stale receipt — naming a reusable PASSED drive that is NOT the scope's current
// drive — passes the unlocked precheck, but admitScopedWorktree applies
// reserveScopeDrive's staleness predicate BEFORE the rotation (change 0453): it is
// refused ErrStalePredecessor with the executing slot unrotated.
//
// A FRESH receipt whose scope closes between the unlocked precheck and admission
// (modelled through the epoch-gate seam, which runs after precheck and wraps the
// admission body) rotates the slot and is then refused ErrScopeClosed by the
// reserveScopeDrive authority: the rotated slot must be released, and the scope
// record stays byte-unchanged by the failed reservation.
func TestSuccessorAdmissionFailureLegsReleaseRotatedSlot(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{} // WAIT: executing slot
	d := scopedTestDriver(store, clk, proc, stableGit())
	_, req := prepareScopedStart(t, store)
	req.RunEpochID = "E-fail"

	cur, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if cur.Outcome != WAITING {
		t.Fatalf("first start must WAIT, got %s (%s)", cur.Outcome, cur.Cause)
	}
	predSlot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	// Terminal-before-release window for the current drive (its process ended).
	if err := store.ownerCAS(cur.DriveID, func(r *driveRecord) error {
		r.LastOutcome = PASSED
		return nil
	}); err != nil {
		t.Fatalf("settle current terminal: %v", err)
	}

	// A stale receipt: a reusable PASSED drive that is NOT the scope's current drive.
	oldRec := seedRecord(t)
	oldRec.LastOutcome = PASSED
	oldRec.OwnerGeneration = "stale-owner-generation"
	staleID, staleGen := seedDrive(t, store, oldRec)

	scopeBefore := readScopeBytes(t, store, req.ScopeID)
	launchesBefore := proc.launchN

	succ := req
	succ.PredecessorDriveID = staleID
	succ.PredecessorOwnerGen = staleGen
	if _, serr := d.Start(succ); !isOwnership(serr, ErrStalePredecessor) {
		t.Fatalf("a stale-receipt successor must be refused ErrStalePredecessor, got %v", serr)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a refused successor must never launch, launched %d->%d", launchesBefore, proc.launchN)
	}

	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after stale refusal: %v", err)
	}
	if slot.State != admissionExecuting || slot.ReservationToken != predSlot.ReservationToken || slot.ExecutionGen != predSlot.ExecutionGen {
		t.Fatalf("a stale receipt must be refused BEFORE rotation: state %q, token changed=%v, gen %d->%d",
			slot.State, slot.ReservationToken != predSlot.ReservationToken, predSlot.ExecutionGen, slot.ExecutionGen)
	}
	if !bytes.Equal(scopeBefore, readScopeBytes(t, store, req.ScopeID)) {
		t.Fatalf("a refused stale successor must leave the scope record byte-unchanged")
	}

	// Post-rotation failure leg: a fresh receipt whose scope closes after the
	// unlocked precheck. The gate closes the scope, then runs the admission body.
	var scopeClosed []byte
	d.SetEpochLaunchGate(func(_, _ string, reserve func() error) error {
		if err := store.scopeCAS(req.ScopeID, func(rec *scopeRecord) error {
			rec.Closed = true
			return nil
		}); err != nil {
			t.Fatalf("close scope mid-admission: %v", err)
		}
		scopeClosed = readScopeBytes(t, store, req.ScopeID)
		return reserve()
	})
	fresh := req
	fresh.PredecessorDriveID = cur.DriveID
	fresh.PredecessorOwnerGen = cur.Generation
	if _, serr := d.Start(fresh); !isOwnership(serr, ErrScopeClosed) {
		t.Fatalf("a successor whose scope closed mid-admission must be refused ErrScopeClosed, got %v", serr)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a refused successor must never launch, launched %d->%d", launchesBefore, proc.launchN)
	}
	slot, _, err = store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after post-rotation refusal: %v", err)
	}
	if slot.State != admissionReleased {
		t.Fatalf("a post-rotation failure must RELEASE the rotated reservation, got %q", slot.State)
	}
	if slot.ReservationToken == predSlot.ReservationToken {
		t.Fatalf("the rotation must have minted a fresh token before the failing leg")
	}
	if scopeClosed == nil || !bytes.Equal(scopeClosed, readScopeBytes(t, store, req.ScopeID)) {
		t.Fatalf("a failed successor reservation must leave the scope record byte-unchanged")
	}
}

// TestFirstStartPeerSharingUnchanged pins the same-scope RESERVED arm: a start
// that finds a concurrent first-start peer's provisional reservation ADOPTS its
// token (reservedFresh=false, ownsSlot=true, rotated=false) rather than rotating.
// The peer-sharing + isSameScopeRaceLoss adoption rule stays exactly as today.
func TestFirstStartPeerSharingUnchanged(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())
	_, req := prepareScopedStart(t, store)

	// A concurrent same-scope first-start peer has provisionally RESERVED the slot
	// (reserved, not executing).
	peerToken, err := store.ReserveWorktreeExecution(admissionRecord{
		RepoIdentity: req.RepoDir,
		WorktreeRoot: req.Worktree,
		ScopeID:      req.ScopeID,
		RunEpochID:   req.RunEpochID,
		Kind:         "scoped",
	})
	if err != nil {
		t.Fatalf("peer reserve: %v", err)
	}
	_, genBefore, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("load before adopt: %v", err)
	}

	token, reservedFresh, ownsSlot, rotated, legacy, aerr := d.admitScopedWorktree(req)
	if aerr != nil {
		t.Fatalf("admitScopedWorktree over a same-scope reserved peer must adopt, got %v", aerr)
	}
	if token != peerToken {
		t.Fatalf("the reserved arm must ADOPT the peer's token, got %q want %q", token, peerToken)
	}
	if reservedFresh {
		t.Fatalf("adopting a peer's reservation is not a fresh reservation")
	}
	if !ownsSlot {
		t.Fatalf("the adopter still confirms the reserved slot")
	}
	if rotated {
		t.Fatalf("adopting a reserved peer must NOT rotate the slot")
	}
	if legacy != nil {
		t.Fatalf("a reused slot carries no legacy summary")
	}

	after, genAfter, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("load after adopt: %v", err)
	}
	if genAfter != genBefore {
		t.Fatalf("adopting a reserved peer must not rewrite the slot record")
	}
	if after.ReservationToken != peerToken || after.State != admissionReserved {
		t.Fatalf("the peer's reservation must be untouched, got token=%q state=%q", after.ReservationToken, after.State)
	}
}

// TestOrdinaryReleasePreservesRunEpoch pins that an ordinary release of a slot's
// own token preserves RunEpochID on the released record (the between-drives fence
// depends on it; change 0437 must not weaken it).
func TestOrdinaryReleasePreservesRunEpoch(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	rec := sampleAdmission(wt)
	rec.RunEpochID = "E-keep"
	token, err := s.ReserveWorktreeExecution(rec)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := s.ConfirmWorktreeExecution(wt, token, "run-x", "/runs/x"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := s.ReleaseWorktreeExecution(wt, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	got, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.State != admissionReleased {
		t.Fatalf("state after release = %q, want %q", got.State, admissionReleased)
	}
	if got.RunEpochID != "E-keep" {
		t.Fatalf("ordinary release must preserve RunEpochID, got %q", got.RunEpochID)
	}
}

// TestPendingPredecessorTransitionRefusesSuccessor proves a successor over an
// ambiguous predecessor transition (a dangling PendingAckDriveID) is refused
// ErrUnresolvedLaunchTransition BEFORE any rotation runs: the executing slot keeps
// the predecessor's token and executing state, unrotated.
func TestPendingPredecessorTransitionRefusesSuccessor(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{} // WAIT: executing slot
	d := scopedTestDriver(store, clk, proc, stableGit())
	_, req := prepareScopedStart(t, store)
	req.RunEpochID = "E-pend"

	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	predSlot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	oldToken := predSlot.ReservationToken
	if err := store.ownerCAS(first.DriveID, func(r *driveRecord) error {
		r.LastOutcome = PASSED
		return nil
	}); err != nil {
		t.Fatalf("settle predecessor terminal: %v", err)
	}
	// A dangling pending-ack journal: an unresolved predecessor transition.
	if err := store.scopeCAS(req.ScopeID, func(rec *scopeRecord) error {
		rec.PendingAckDriveID = "pending-x"
		return nil
	}); err != nil {
		t.Fatalf("set pending ack: %v", err)
	}

	launchesBefore := proc.launchN
	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	if _, serr := d.Start(succ); !isOwnership(serr, ErrUnresolvedLaunchTransition) {
		t.Fatalf("a successor over a pending predecessor transition must be refused ErrUnresolvedLaunchTransition, got %v", serr)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a refused successor must never launch, launched %d->%d", launchesBefore, proc.launchN)
	}

	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after refusal: %v", err)
	}
	if slot.ReservationToken != oldToken {
		t.Fatalf("rotation must NOT run under a pending predecessor transition; token changed")
	}
	if slot.State != admissionExecuting {
		t.Fatalf("the slot must remain the predecessor's executing reservation, got %q", slot.State)
	}
}

// TestSuccessorRotationStoreContract unit-tests rotateWorktreeExecutionForSuccessor
// directly: it requires the executing state, verifies oldToken, mints a fresh
// token, bumps ExecutionGen, clears the raw-run identity, preserves the identity
// fields, and lands in reserved — and refuses every other state, a token mismatch,
// or an unreadable record typed, writing nothing.
func TestSuccessorRotationStoreContract(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	rec := sampleAdmission(wt)
	rec.RunEpochID = "E7"
	rec.ScopeID = "scope-succ"
	rec.Kind = "scoped"
	oldToken, err := s.ReserveWorktreeExecution(rec)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	// A reserved (non-executing) slot refuses rotation typed and writes nothing.
	reservedBefore, genReserved, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load reserved: %v", err)
	}
	if _, err := s.rotateWorktreeExecutionForSuccessor(wt, oldToken); !isOwnership(err, ErrUnresolvedLaunchTransition) {
		t.Fatalf("rotation of a non-executing slot must refuse ErrUnresolvedLaunchTransition, got %v", err)
	}
	if _, gen, _ := s.LoadWorktreeExecution(wt); gen != genReserved {
		t.Fatalf("a refused rotation must write nothing")
	}

	// Confirm to executing.
	if err := s.ConfirmWorktreeExecution(wt, oldToken, "run-pred", "/runs/pred"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	before, genExec, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load executing: %v", err)
	}

	// A wrong token refuses ErrNotOwner and writes nothing.
	if _, err := s.rotateWorktreeExecutionForSuccessor(wt, "not-the-token"); !isOwnership(err, ErrNotOwner) {
		t.Fatalf("rotation with a wrong token must refuse ErrNotOwner, got %v", err)
	}
	if _, gen, _ := s.LoadWorktreeExecution(wt); gen != genExec {
		t.Fatalf("a wrong-token rotation must write nothing")
	}

	// A correct rotation mints a fresh reservation.
	newToken, err := s.rotateWorktreeExecutionForSuccessor(wt, oldToken)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if newToken == "" || newToken == oldToken {
		t.Fatalf("rotation must mint a fresh non-empty token distinct from the old one")
	}
	after, _, err := s.LoadWorktreeExecution(wt)
	if err != nil {
		t.Fatalf("load after rotate: %v", err)
	}
	if after.State != admissionReserved {
		t.Fatalf("a rotated slot must land reserved, got %q", after.State)
	}
	if after.ReservationToken != newToken {
		t.Fatalf("a rotated slot must carry the new token")
	}
	if after.ExecutionGen != before.ExecutionGen+1 {
		t.Fatalf("rotation must bump ExecutionGen, got %d want %d", after.ExecutionGen, before.ExecutionGen+1)
	}
	if after.RawRunID != "" || after.RawRunDir != "" {
		t.Fatalf("rotation must clear the raw-run identity, got (%q,%q)", after.RawRunID, after.RawRunDir)
	}
	if after.RunEpochID != "E7" || after.ScopeID != "scope-succ" || after.Kind != before.Kind ||
		after.RepoIdentity != before.RepoIdentity || after.WorktreeRoot != before.WorktreeRoot {
		t.Fatalf("rotation must preserve the identity fields")
	}
	_ = reservedBefore

	// The predecessor's old token now has NO authority.
	if err := s.ReleaseWorktreeExecution(wt, oldToken); !isOwnership(err, ErrNotOwner) {
		t.Fatalf("the old token must lose authority after rotation, release got %v", err)
	}
}
