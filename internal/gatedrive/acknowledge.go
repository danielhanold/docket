// Terminal acknowledgement: the scope's final "successor" that consumes the last
// drive result and closes the recovery scope (change 0405 Task 5, spec "Finishing
// the task").
//
// A recovery scope carries a SEQUENCE of task-owned drives through one slot —
// baseline, RED, GREEN, verification — each successor start acknowledging its
// predecessor's durable PASSED/FAILED result (driver.go startScoped). The LAST
// drive has no successor start to retire it, so Driver.Acknowledge is its
// acknowledger: it retires the final drive's recovery authority (the same
// predecessorReusableError authority a successor start uses) and closes the scope
// with FinalAcked, so a normal completion leaves no stale recovery candidate for
// the outer takeover/enumeration path (FindScopeDriveIDs excludes an owner-cleared
// terminal record).
//
// Acknowledge fails closed exactly like the rest of the driver: wrong credentials,
// a different drive, a live (WAITING) or HALTED outcome, an outstanding handoff, a
// superseded owner, a mid-transition (reserved or pending-ack) slot, or a scope
// already closed by a claim or takeover are typed rejections that WRITE NOTHING. A
// byte-identical repeat after a successful acknowledgement is an idempotent no-op
// returning the recorded document, so a retried terminal call never double-writes.
package gatedrive

// Acknowledge consumes the scope's final drive result and closes the scope. It
// verifies the child capability, that driveID is the scope's current LAUNCHED
// drive, a durable PASSED/FAILED outcome, no outstanding handoff, and current
// ownership (ownerGen); then it clears the drive's recovery authority
// (retirePredecessor) and closes the scope with FinalAcked (closeScopeFinal,
// which revalidates the slot under the scope lock). A byte-identical repeat after
// success is an idempotent no-op returning the recorded document. Wrong
// credentials, a different drive, live/HALTED outcomes, or pending transitions are
// typed rejections that write nothing. A scope record that cannot be read (unknown
// schema, corrupt, missing) surfaces the typed store error unchanged.
func (d *Driver) Acknowledge(scopeID, childCapability, driveID, ownerGen string) (DriveDoc, error) {
	scope, err := d.store.LoadScope(scopeID)
	if err != nil {
		return DriveDoc{}, err
	}

	// The child capability authorizes the acknowledgement (the same authority a
	// successor start presents). An empty or wrong token confers nothing — checked
	// first so a rejected credential never reveals slot state.
	if childCapability == "" || scope.ChildCapHash != capHash(childCapability) {
		return DriveDoc{}, ownershipErr(ErrScopeCapabilityMismatch, "acknowledge")
	}

	// A closed scope is terminal. The one exception is a byte-identical repeat of a
	// successful acknowledgement: a scope that is Closed AND FinalAcked, still naming
	// this drive, whose drive record is already owner-cleared with a durable
	// PASSED/FAILED verdict, is the recorded terminal — return its document with no
	// write. A scope closed by a claim or takeover (FinalAcked false), or a
	// non-matching drive, is a fail-closed ErrScopeClosed.
	if scope.Closed {
		if scope.FinalAcked && scope.CurrentDriveID == driveID {
			rec, lerr := d.store.Load(driveID)
			if lerr != nil {
				return DriveDoc{}, lerr
			}
			if rec.OwnerGeneration == "" && (rec.LastOutcome == PASSED || rec.LastOutcome == FAILED) {
				return d.recordedDoc(driveID, ownerGen, rec), nil
			}
		}
		return DriveDoc{}, ownershipErr(ErrScopeClosed, "acknowledge")
	}

	// Slot verification (unlocked fast-fail; retirePredecessor and closeScopeFinal
	// under their locks are the authority that arbitrate a concurrent transition).
	// driveID must be the scope's CURRENT drive: this guard is load-bearing — it
	// keeps retirePredecessor from ever clearing a drive that is not this scope's
	// slot occupant (a write to the wrong drive).
	if scope.CurrentDriveID != driveID {
		return DriveDoc{}, ownershipErr(ErrStalePredecessor, "acknowledge")
	}
	// A mid-transition slot is an unresolved launch transition, never a quiescent
	// result: a journaled pending ack, or a reservation not yet launch-confirmed.
	if scope.PendingAckDriveID != "" {
		return DriveDoc{}, ownershipErr(ErrUnresolvedLaunchTransition, "acknowledge")
	}
	if scope.CurrentDriveState != scopeStateLaunched {
		return DriveDoc{}, ownershipErr(ErrUnresolvedLaunchTransition, "acknowledge")
	}

	// Resumable-half recovery: a crash between retirePredecessor and closeScopeFinal
	// leaves the scope's current LAUNCHED drive already owner-cleared with a durable
	// terminal verdict while the scope is still OPEN — retirePredecessor ran, its
	// close did not. That is the resumable second half of THIS same acknowledgement,
	// not a fresh one: re-running retirePredecessor would misreport the cleared owner
	// as ErrStalePredecessor and strand the scope open forever. Detect it and skip
	// straight to the close (the resumption completing the transition). The
	// discriminator is exact — a retired predecessor becomes PriorDriveID, never the
	// CurrentDriveID this branch already required, so only an interrupted final ack
	// reaches an owner-cleared current drive. An outstanding handoff excludes the
	// state (a handed-off drive is also owner-cleared but never a terminal result).
	rec, lerr := d.store.Load(driveID)
	if lerr != nil {
		return DriveDoc{}, lerr
	}
	resumable := rec.HandoffGeneration == "" && rec.OwnerGeneration == "" &&
		(rec.LastOutcome == PASSED || rec.LastOutcome == FAILED)

	// Retire the final drive's recovery authority: the shared predecessorReusableError
	// authority verifies (in order) no outstanding handoff (ErrHandoffOutstanding),
	// the presented owner is still current (ErrStalePredecessor), and a durable
	// PASSED/FAILED outcome (ErrPredecessorNotReusable) — then clears the owner
	// generation so the record survives only as consumed history. On any rejection
	// nothing is written. The resumable-half case skips it: its authority is already
	// retired, so the only work left is the close.
	if !resumable {
		if rerr := d.store.retirePredecessor(driveID, ownerGen); rerr != nil {
			return DriveDoc{}, rerr
		}
	}

	// Close the scope as terminally acknowledged, revalidating the slot under the
	// scope lock (revalidate-after-authority). The owner is already retired, so a
	// concurrent transition that moved the slot fails this close closed rather than
	// closing over the wrong drive.
	if cerr := d.store.closeScopeFinal(scopeID, driveID); cerr != nil {
		return DriveDoc{}, cerr
	}

	// Build the document from the authoritative post-retirement record.
	rec, err = d.store.Load(driveID)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.recordedDoc(driveID, ownerGen, rec), nil
}

// closeScopeFinal atomically closes a scope as terminally acknowledged, marking
// FinalAcked, but ONLY when driveID is still the scope's current drive — the
// revalidation-after-authority the lock order requires (a concurrent transition
// that moved the slot between the caller's read and this close is caught here). A
// mismatched drive id is a fail-closed ErrStalePredecessor; an already-closed
// scope is ErrScopeClosed. On any rejection the persisted record is untouched.
func (s *Store) closeScopeFinal(scopeID, driveID string) error {
	return s.scopeCAS(scopeID, func(rec *scopeRecord) error {
		if rec.Closed {
			return ownershipErr(ErrScopeClosed, "acknowledge-close")
		}
		if rec.CurrentDriveID != driveID {
			return ownershipErr(ErrStalePredecessor, "acknowledge-close")
		}
		rec.Closed = true
		rec.FinalAcked = true
		return nil
	})
}
