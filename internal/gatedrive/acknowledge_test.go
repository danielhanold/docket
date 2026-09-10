// Terminal-acknowledgement tests (change 0405 Task 5). Driver.Acknowledge is the
// scope's final "successor": it consumes the current LAUNCHED drive's durable
// PASSED/FAILED result, retires the drive's recovery authority, and closes the
// scope with FinalAcked. These exercise the happy path (evidence retention, zero
// stale recovery candidates), the idempotent repeat, the typed refusals that write
// nothing, the post-ack successor rejection, and the closeScopeFinal
// revalidate-after-authority store transition directly.
package gatedrive

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// failObserveProc returns a fakeProc whose every run reports FAILED on the first
// observation, so a scoped Start returns a durable FAILED in one call.
func failObserveProc() *fakeProc {
	return &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateFailed, runDir), nil
		},
	}
}

// startedScope prepares a fresh no-context task scope over a new store and Starts
// its first drive with proc, returning the driver, store, grant, the started doc,
// and the base request. The caller chooses proc to fix the first drive's outcome
// (PASSED, FAILED, or a still-running WAITING).
func startedScope(t *testing.T, proc *fakeProc) (*Driver, *Store, ScopeGrant, DriveDoc, StartRequest) {
	t.Helper()
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	d := scopedTestDriver(store, clk, proc, stableGit())
	grant, req := prepareScopedStart(t, store)
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return d, store, grant, started, req
}

// readDriveBytes returns the raw record.json bytes for a drive, so a rejection
// test can prove the persisted drive record is untouched with a before/after
// compare.
func readDriveBytes(t *testing.T, s *Store, id string) []byte {
	t.Helper()
	return mustReadBytes(t, filepath.Join(s.root, id, recordFileName))
}

// ackTwoDriveSequence prepares a scope, drives a first PASSED drive, then a
// successor PASSED drive over the same slot, and returns the driver, store, grant,
// the base request, and the two drive docs. The current slot occupant is the
// second (PASSED, launched, owner set), ready to be acknowledged.
func ackTwoDriveSequence(t *testing.T) (*Driver, *Store, ScopeGrant, StartRequest, DriveDoc, DriveDoc) {
	t.Helper()
	d, store, grant, first, req := startedScope(t, passObserveProc())
	if first.Outcome != PASSED {
		t.Fatalf("first start must PASS (positive control), got %s (%s)", first.Outcome, first.Cause)
	}
	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	second, err := d.Start(succ)
	if err != nil {
		t.Fatalf("successor Start: %v", err)
	}
	if second.Outcome != PASSED {
		t.Fatalf("successor start must PASS, got %s (%s)", second.Outcome, second.Cause)
	}
	return d, store, grant, req, first, second
}

// TestAcknowledgeHappyPathClosesScope reproduces spec verification 9: a two-drive
// sequence whose final drive is durably PASSED is acknowledged; the scope becomes
// Closed && FinalAcked with its CurrentDriveID retained for history; the drive
// record retains its id, verdict, fingerprint, and RawRunDir with the owner
// generation cleared; the returned document carries the recorded verdict; and
// FindScopeDriveIDs returns ZERO candidates (no stale recovery candidates after a
// normal completion).
func TestAcknowledgeHappyPathClosesScope(t *testing.T) {
	d, store, grant, req, first, second := ackTwoDriveSequence(t)

	beforeFP, err := store.Load(second.DriveID)
	if err != nil {
		t.Fatalf("Load before ack: %v", err)
	}

	doc, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, second.DriveID, second.Generation)
	if err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}

	// The returned document carries the recorded PASSED verdict and its evidence.
	if doc.Outcome != PASSED {
		t.Fatalf("ack of a PASSED final drive must report PASSED, got %s (%s)", doc.Outcome, doc.Cause)
	}
	if doc.DriveID != second.DriveID {
		t.Fatalf("ack document must name the final drive, got %q want %q", doc.DriveID, second.DriveID)
	}

	// The scope is closed AND final-acked; CurrentDriveID is retained for history.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if !scope.Closed || !scope.FinalAcked {
		t.Fatalf("ack must close the scope with FinalAcked, got Closed=%v FinalAcked=%v", scope.Closed, scope.FinalAcked)
	}
	if scope.CurrentDriveID != second.DriveID {
		t.Fatalf("ack must retain CurrentDriveID for history, got %q want %q", scope.CurrentDriveID, second.DriveID)
	}

	// Evidence retention: the drive record keeps its id, verdict, fingerprint, and
	// RawRunDir; only the recovery authority (owner generation) is cleared.
	rec, err := store.Load(second.DriveID)
	if err != nil {
		t.Fatalf("Load after ack: %v", err)
	}
	if rec.LastOutcome != PASSED {
		t.Fatalf("ack must retain the drive verdict, got %s", rec.LastOutcome)
	}
	if rec.OwnerGeneration != "" {
		t.Fatalf("ack must clear the drive's owner generation, got %q", rec.OwnerGeneration)
	}
	if rec.RawRunDir == "" || rec.RawRunDir != beforeFP.RawRunDir {
		t.Fatalf("ack must retain the drive's RawRunDir evidence, got %q want %q", rec.RawRunDir, beforeFP.RawRunDir)
	}
	if !rec.Fingerprint.Equal(beforeFP.Fingerprint) {
		t.Fatalf("ack must retain the drive's fingerprint evidence")
	}
	if doc.RawRunDir != rec.RawRunDir {
		t.Fatalf("ack document must expose the PASSED drive's RawRunDir, got %q want %q", doc.RawRunDir, rec.RawRunDir)
	}

	// Zero stale recovery candidates: both the acknowledged predecessor (retired at
	// the successor start) and the final drive (retired here) have their owner
	// cleared, so the outer enumeration finds nothing to recover.
	ids, err := store.FindScopeDriveIDs(req.ChangeID, "")
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("after a normal completion there must be zero recovery candidates, got %v", ids)
	}
	// Sanity: the predecessor is still readable with its verdict intact.
	firstRec, err := store.Load(first.DriveID)
	if err != nil {
		t.Fatalf("Load predecessor: %v", err)
	}
	if firstRec.LastOutcome != PASSED || firstRec.OwnerGeneration != "" {
		t.Fatalf("predecessor must survive as consumed history, got %s owner=%q", firstRec.LastOutcome, firstRec.OwnerGeneration)
	}
}

// TestAcknowledgeFailedFinalResult proves a FAILED final result is a valid
// acknowledgement (task success is the caller's concern, not the driver's): the
// scope closes with FinalAcked and the returned document reports FAILED.
func TestAcknowledgeFailedFinalResult(t *testing.T) {
	d, store, grant, started, _ := startedScope(t, failObserveProc())
	if started.Outcome != FAILED {
		t.Fatalf("want a FAILED final drive, got %s (%s)", started.Outcome, started.Cause)
	}

	doc, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Acknowledge FAILED final: %v", err)
	}
	if doc.Outcome != FAILED {
		t.Fatalf("ack of a FAILED final drive must report FAILED, got %s", doc.Outcome)
	}
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if !scope.Closed || !scope.FinalAcked {
		t.Fatalf("a FAILED ack must still close the scope with FinalAcked, got Closed=%v FinalAcked=%v", scope.Closed, scope.FinalAcked)
	}
	rec, err := store.Load(started.DriveID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.OwnerGeneration != "" {
		t.Fatalf("a FAILED ack must clear the drive's owner generation, got %q", rec.OwnerGeneration)
	}
}

// TestAcknowledgeIdempotentRepeat proves a byte-identical repeat after a successful
// acknowledgement is a no-op: the same four arguments return a nil error and the
// same recorded document, and the persisted scope and drive bytes are unchanged.
func TestAcknowledgeIdempotentRepeat(t *testing.T) {
	d, store, grant, _, _, second := ackTwoDriveSequence(t)

	firstDoc, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, second.DriveID, second.Generation)
	if err != nil {
		t.Fatalf("first Acknowledge: %v", err)
	}
	scopeBytes := readScopeBytes(t, store, grant.ScopeID)
	driveBytes := readDriveBytes(t, store, second.DriveID)

	repeatDoc, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, second.DriveID, second.Generation)
	if err != nil {
		t.Fatalf("repeat Acknowledge must be a nil-error no-op, got %v", err)
	}
	if !reflect.DeepEqual(firstDoc, repeatDoc) {
		t.Fatalf("repeat Acknowledge must return the same document, got %+v want %+v", repeatDoc, firstDoc)
	}
	if !bytes.Equal(scopeBytes, readScopeBytes(t, store, grant.ScopeID)) {
		t.Fatalf("repeat Acknowledge must not rewrite the scope record")
	}
	if !bytes.Equal(driveBytes, readDriveBytes(t, store, second.DriveID)) {
		t.Fatalf("repeat Acknowledge must not rewrite the drive record")
	}
}

// TestAcknowledgeRefusals reproduces the acknowledgement half of spec verification
// 4: every wrong-credential, wrong-drive, non-terminal, or mid-transition
// acknowledgement is a typed rejection that writes NOTHING (asserted by a
// before/after byte compare of the scope and, where present, the drive record).
func TestAcknowledgeRefusals(t *testing.T) {
	t.Run("waiting drive not reusable", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, &fakeProc{})
		if started.Outcome != WAITING {
			t.Fatalf("want a WAITING drive, got %s", started.Outcome)
		}
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		driveBytes := readDriveBytes(t, store, started.DriveID)
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation); !isOwnershipKind(err, ErrPredecessorNotReusable) {
			t.Fatalf("a WAITING drive must reject ack ErrPredecessorNotReusable, got %v", err)
		}
		assertUnchanged(t, store, grant.ScopeID, scopeBytes, started.DriveID, driveBytes)
	})

	t.Run("halted drive not reusable", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, &fakeProc{})
		rec, err := store.Load(started.DriveID)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		rec.LastOutcome = HALTED
		rec.LastCause = "some-halt"
		overwriteDriveRecord(t, store, started.DriveID, rec)
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		driveBytes := readDriveBytes(t, store, started.DriveID)
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation); !isOwnershipKind(err, ErrPredecessorNotReusable) {
			t.Fatalf("a HALTED drive must reject ack ErrPredecessorNotReusable, got %v", err)
		}
		assertUnchanged(t, store, grant.ScopeID, scopeBytes, started.DriveID, driveBytes)
	})

	t.Run("unrelated drive id", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, passObserveProc())
		if started.Outcome != PASSED {
			t.Fatalf("want a PASSED current drive, got %s", started.Outcome)
		}
		// A SEPARATE durable PASSED, owned drive: if the up-front CurrentDriveID
		// slot check were removed, retirePredecessor would clear THIS drive's owner —
		// a write to the wrong drive the byte compare would catch.
		other := seedRecord(t)
		other.LastOutcome = PASSED
		otherID, otherGen := seedDrive(t, store, other)

		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		otherBytes := readDriveBytes(t, store, otherID)
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, otherID, otherGen); !isOwnershipKind(err, ErrStalePredecessor) {
			t.Fatalf("an unrelated drive id must reject ack ErrStalePredecessor, got %v", err)
		}
		if !bytes.Equal(scopeBytes, readScopeBytes(t, store, grant.ScopeID)) {
			t.Fatalf("a rejected ack must not rewrite the scope record")
		}
		if !bytes.Equal(otherBytes, readDriveBytes(t, store, otherID)) {
			t.Fatalf("a rejected ack must not touch an unrelated drive record")
		}
	})

	t.Run("superseded generation", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, passObserveProc())
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		driveBytes := readDriveBytes(t, store, started.DriveID)
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, "not-the-owner"); !isOwnershipKind(err, ErrStalePredecessor) {
			t.Fatalf("a superseded owner generation must reject ack ErrStalePredecessor, got %v", err)
		}
		assertUnchanged(t, store, grant.ScopeID, scopeBytes, started.DriveID, driveBytes)
	})

	t.Run("outstanding handoff", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, &fakeProc{})
		if _, err := d.Handoff(started.DriveID, started.Generation); err != nil {
			t.Fatalf("Handoff: %v", err)
		}
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		driveBytes := readDriveBytes(t, store, started.DriveID)
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation); !isOwnershipKind(err, ErrHandoffOutstanding) {
			t.Fatalf("an outstanding handoff must reject ack ErrHandoffOutstanding, got %v", err)
		}
		assertUnchanged(t, store, grant.ScopeID, scopeBytes, started.DriveID, driveBytes)
	})

	t.Run("wrong capability", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, passObserveProc())
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		driveBytes := readDriveBytes(t, store, started.DriveID)
		if _, err := d.Acknowledge(grant.ScopeID, "wrong-capability", started.DriveID, started.Generation); !isOwnershipKind(err, ErrScopeCapabilityMismatch) {
			t.Fatalf("a wrong child capability must reject ack ErrScopeCapabilityMismatch, got %v", err)
		}
		assertUnchanged(t, store, grant.ScopeID, scopeBytes, started.DriveID, driveBytes)
	})

	t.Run("scope closed by claim or takeover", func(t *testing.T) {
		d, store, grant, started, _ := startedScope(t, passObserveProc())
		if err := store.closeScope(grant.ScopeID); err != nil {
			t.Fatalf("closeScope: %v", err)
		}
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		driveBytes := readDriveBytes(t, store, started.DriveID)
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation); !isOwnershipKind(err, ErrScopeClosed) {
			t.Fatalf("a scope closed (not final-acked) must reject ack ErrScopeClosed, got %v", err)
		}
		assertUnchanged(t, store, grant.ScopeID, scopeBytes, started.DriveID, driveBytes)
	})

	t.Run("reserved slot", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		store := OpenStore(testsupport.TempDir(t))
		d := scopedTestDriver(store, clk, &fakeProc{}, stableGit())
		grant, err := store.PrepareScope(sampleScopeReq())
		if err != nil {
			t.Fatalf("PrepareScope: %v", err)
		}
		if err := store.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveA, predecessorReceipt{}); err != nil {
			t.Fatalf("reserveScopeDrive: %v", err)
		}
		scopeBytes := readScopeBytes(t, store, grant.ScopeID)
		if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, scopeDriveA, "any-owner-gen"); !isOwnershipKind(err, ErrUnresolvedLaunchTransition) {
			t.Fatalf("a reserved (unlaunched) slot must reject ack ErrUnresolvedLaunchTransition, got %v", err)
		}
		if !bytes.Equal(scopeBytes, readScopeBytes(t, store, grant.ScopeID)) {
			t.Fatalf("a rejected ack must not rewrite the scope record")
		}
	})
}

// assertUnchanged fails if either the scope record or the drive record changed
// since the captured before-bytes.
func assertUnchanged(t *testing.T, store *Store, scopeID string, scopeBytes []byte, driveID string, driveBytes []byte) {
	t.Helper()
	if !bytes.Equal(scopeBytes, readScopeBytes(t, store, scopeID)) {
		t.Fatalf("a rejected ack must not rewrite the scope record")
	}
	if !bytes.Equal(driveBytes, readDriveBytes(t, store, driveID)) {
		t.Fatalf("a rejected ack must not rewrite the drive record")
	}
}

// TestAcknowledgePostAckSuccessorRefused proves a Start presenting the
// acknowledged final drive as its predecessor is refused ErrScopeClosed — the
// terminal acknowledgement closed the scope, so no further successor can open.
func TestAcknowledgePostAckSuccessorRefused(t *testing.T) {
	d, _, grant, req, _, second := ackTwoDriveSequence(t)
	if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, second.DriveID, second.Generation); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	succ := req
	succ.PredecessorDriveID = second.DriveID
	succ.PredecessorOwnerGen = second.Generation
	if _, err := d.Start(succ); !isOwnershipKind(err, ErrScopeClosed) {
		t.Fatalf("a successor after the terminal ack must be refused ErrScopeClosed, got %v", err)
	}
}

// TestCloseScopeFinalRevalidates unit-tests the closeScopeFinal store transition
// directly: it closes a scope as FinalAcked ONLY when driveID is still the scope's
// current drive (the revalidation-after-authority the lock order requires); a
// mismatched drive id is ErrStalePredecessor with no write, and an already-closed
// scope is ErrScopeClosed.
func TestCloseScopeFinalRevalidates(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	grant, err := store.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	launchOne(t, store, grant, scopeDriveA)

	// A drive id that is not the scope's current drive is refused with no write.
	before := readScopeBytes(t, store, grant.ScopeID)
	if err := store.closeScopeFinal(grant.ScopeID, scopeDriveB); !isOwnershipKind(err, ErrStalePredecessor) {
		t.Fatalf("closeScopeFinal on a non-current drive must fail ErrStalePredecessor, got %v", err)
	}
	if !bytes.Equal(before, readScopeBytes(t, store, grant.ScopeID)) {
		t.Fatalf("a refused closeScopeFinal must not rewrite the scope record")
	}

	// The current drive id closes the scope as final-acked.
	if err := store.closeScopeFinal(grant.ScopeID, scopeDriveA); err != nil {
		t.Fatalf("closeScopeFinal on the current drive: %v", err)
	}
	rec, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if !rec.Closed || !rec.FinalAcked {
		t.Fatalf("closeScopeFinal must set Closed && FinalAcked, got Closed=%v FinalAcked=%v", rec.Closed, rec.FinalAcked)
	}
	if rec.CurrentDriveID != scopeDriveA {
		t.Fatalf("closeScopeFinal must retain CurrentDriveID, got %q", rec.CurrentDriveID)
	}

	// An already-closed scope refuses a second close.
	if err := store.closeScopeFinal(grant.ScopeID, scopeDriveA); !isOwnershipKind(err, ErrScopeClosed) {
		t.Fatalf("closeScopeFinal on a closed scope must fail ErrScopeClosed, got %v", err)
	}
}
