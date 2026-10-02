// Event-authorized parent takeover: the exceptional ownership transfer for an
// implement-next run that returned WITHOUT handing off its live suite gate.
//
// The cooperative transfer (ownership.go / driver.go Handoff+Claim) is the
// PREFERRED path and the only one a healthy child uses. Takeover exists for the
// one case cooperation cannot cover, and only at the coordinator → implement-next
// boundary: run.start prepares one outer recovery scope per run (scope.go) whose
// child capability is the run context the run's suite gates carry, and when
// implement-next dies, is killed, or has its return dropped while a gate is still
// running, run.verdict recovers that drive so the progressing run is continued
// rather than restarted. run.verdict resolves the one live candidate itself
// (FindScopeDriveIDs, which lists only nonterminal drives) and calls Takeover with
// that drive's id; Takeover never scans for a candidate.
//
// The AUTHORIZATION is a workflow fact, never a timer: the CALLER asserts "my
// direct child returned without handing off" simply by calling Takeover at all
// (spec Constraint 2: no timers, heartbeats, log-activity checks, claim-age, or
// process-name liveness guesses anywhere). Takeover proves EVERYTHING ELSE before
// it transfers a single generation:
//
//   - the scope is open (a closed scope was already claimed or taken over);
//   - the exact PARENT capability (distinct from the child's own capability);
//   - the drive/scope IDENTITY agrees (repo, branch, worktree, change);
//   - no outstanding unclaimed handoff (a valid handoff means Claim, not takeover);
//   - the worktree FINGERPRINT still matches the drive start;
//   - a live drive's DEADLINE has not expired.
//
// Only then does it CLAIM THE SCOPE — a single-use open→closed transition that is
// the atomic winner-selection point under a race — and, under that gate, swap the
// superseded child owner generation for a fresh parent-minted one. A drive that
// finished between run.verdict's scan and the takeover is still accepted: its
// recorded verdict is handed over unchanged, so a suite that completed in that
// window is not lost. It never launches, stops, or duplicates a process; every
// uncertainty returns a HALTED document (never a red suite result), exactly like
// the rest of the driver.
package gatedrive

import "fmt"

// BindScopeChange binds a fresh outer scope's change id exactly once, exposing the
// store's bind-once transition (scope.go bindScopeChange) across the package
// boundary so the application-layer verdict path can pin the attributed change id
// when attribution first resolves a claim (spec §3 defense-in-depth). Bind-once
// semantics carry through: an already-bound matching id is an idempotent no-op, a
// different id fails closed with ErrScopeIdentityMismatch, and a closed scope
// refuses the bind. It is a thin delegation so a single composed *Driver
// satisfies the application-layer engine seam. (change 0359)
func (d *Driver) BindScopeChange(scopeID, changeID string) error {
	return d.store.bindScopeChange(scopeID, changeID)
}

// Takeover performs the event-authorized exceptional transfer of the outer-scope
// drive driveID to a fresh owner the parent mints. driveID is required: its only
// caller (run.verdict's outer continuation) passes the single live candidate its
// own FindScopeDriveIDs scan resolved, and an empty id is a command error that
// touches neither record. A drive that finished after that scan is still accepted
// and its recorded verdict handed over. On success the returned document carries,
// in Generation, the fresh owner generation the parent advances with, and the
// scope is closed. Any capability failure, identity drift, outstanding handoff,
// changed worktree, expired deadline, or lost race returns a HALTED document —
// never a launch, a stop, or a duplicated process.
func (d *Driver) Takeover(scopeID, parentCapability, driveID string) (DriveDoc, error) {
	if driveID == "" {
		return DriveDoc{}, fmt.Errorf("gatedrive: takeover requires an explicit drive id")
	}
	scope, err := d.store.LoadScope(scopeID)
	if err != nil {
		// A recognized-but-unusable scope (unknown schema, corrupt) fails closed to
		// HALTED; a missing/malformed scope id is a command failure.
		if se, ok := AsStoreError(err); ok {
			switch se.Kind {
			case ErrUnknownSchema, ErrCorruptRecord:
				return d.haltDoc(driveID, "", driveRecord{}, CauseSchemaMismatch), nil
			}
		}
		return DriveDoc{}, err
	}
	if scope.Closed {
		return d.haltDoc(driveID, "", driveRecord{}, string(ErrScopeClosed)), nil
	}
	// The PARENT capability authorizes a takeover. A wrong token, an empty token,
	// or the child's own capability presented as the parent's all fail here.
	if parentCapability == "" || scope.ParentCapHash != capHash(parentCapability) {
		return d.haltDoc(driveID, "", driveRecord{}, string(ErrScopeCapabilityMismatch)), nil
	}

	rec, err := d.store.Load(driveID)
	if err != nil {
		if se, ok := AsStoreError(err); ok {
			switch se.Kind {
			case ErrUnknownSchema, ErrCorruptRecord:
				return d.haltDoc(driveID, "", driveRecord{}, CauseSchemaMismatch), nil
			}
		}
		return DriveDoc{}, err
	}

	// The drive must be the scope's own work: its identity (repo, branch,
	// worktree, change — for each field the scope actually pins) must agree with
	// the scope. A drift is fail-closed, never a transfer.
	if !scopeIdentityMatch(scope, rec.RepoIdentity, rec.Branch, rec.WorktreePath, rec.ChangeID, rec.TaskID, rec.Phase) {
		return d.haltDoc(driveID, "", rec, string(ErrScopeIdentityMismatch)), nil
	}
	// A drive that already carries an unclaimed handoff must be CLAIMED, not taken
	// over — the child cooperated after all.
	if rec.HandoffGeneration != "" {
		return d.haltDoc(driveID, "", rec, string(ErrHandoffOutstanding)), nil
	}
	// The worktree must still match the drive-start execution identity, so a
	// continuation certifies the original bytes. This is the SOLE fingerprint check
	// on the takeover path (a post-takeover pass re-validates it again in
	// driveSlice), so its removal is directly mutation-observable.
	current, ferr := ComputeFingerprint(rec.WorktreePath, d.git)
	if ferr != nil {
		return d.haltDoc(driveID, "", rec, "fingerprint-error"), nil
	}
	if !rec.Fingerprint.Equal(current) {
		return d.haltDoc(driveID, "", rec, string(ErrFingerprintMismatch)), nil
	}
	// A live (nonterminal) drive whose fixed deadline has passed earns no
	// continuation. A drive that finished after run.verdict's scan is past its run,
	// so its deadline is immaterial — only the recorded verdict is handed over.
	if !isTerminalOutcome(rec.LastOutcome) {
		if expired, _ := rec.deadlineState(d.clock.Now()); expired {
			return d.haltDoc(driveID, "", rec, CauseDeadlineExpired), nil
		}
	}
	// The child owner generation this takeover supersedes. It must be present (a
	// fully consumed drive has no owner to supersede).
	supersededOwner := rec.OwnerGeneration
	if supersededOwner == "" {
		return d.haltDoc(driveID, "", rec, string(ErrNotOwner)), nil
	}

	freshOwner, err := randomToken(genNBytes)
	if err != nil {
		return DriveDoc{}, storeErr(ErrIO, "takeover", err)
	}

	// Claim the scope: a single-use open→closed transition that serializes racing
	// takeovers so EXACTLY ONE proceeds. Every fail-closed check above ran first,
	// so a rejected takeover never spends the scope; only a fully validated one
	// reaches this gate. Losing the race (the scope is now closed) is a HALT.
	if cerr := d.store.claimScopeForTakeover(scopeID); cerr != nil {
		if oe, ok := AsOwnershipError(cerr); ok {
			return d.haltDoc(driveID, "", rec, string(oe.Kind)), nil
		}
		return DriveDoc{}, cerr
	}

	// Under the drive's ownership CAS, atomically invalidate the child owner
	// generation and install the fresh parent-minted one. The supersededOwner guard
	// fails closed if a concurrent cooperative transfer moved the owner between our
	// read and this write.
	cerr := d.store.ownerCAS(driveID, func(r *driveRecord) error {
		if r.OwnerGeneration == "" || r.OwnerGeneration != supersededOwner {
			return ownershipErr(ErrNotOwner, "takeover")
		}
		if r.HandoffGeneration != "" {
			return ownershipErr(ErrHandoffOutstanding, "takeover")
		}
		r.OwnerGeneration = freshOwner
		return nil
	})
	if cerr != nil {
		if oe, ok := AsOwnershipError(cerr); ok {
			return d.haltDoc(driveID, "", rec, string(oe.Kind)), nil
		}
		return DriveDoc{}, cerr
	}

	cur, err := d.store.Load(driveID)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.transferDoc(driveID, freshOwner, cur), nil
}

// scopeIdentityMatch reports whether a scope's identity agrees with a candidate
// drive's (or a start request's) identity. It compares only the fields the scope
// actually PINS: an empty scope field matches anything, so an outer scope that
// does not fix a task or phase still binds by repo/branch/worktree/change, while a
// task scope that fixes every field is checked in full. A non-empty scope field
// that disagrees is a fail-closed mismatch.
func scopeIdentityMatch(scope scopeRecord, repo, branch, worktree, change, task, phase string) bool {
	return (scope.RepoIdentity == "" || scope.RepoIdentity == repo) &&
		(scope.Branch == "" || scope.Branch == branch) &&
		(scope.Worktree == "" || scope.Worktree == worktree) &&
		(scope.ChangeID == "" || changeIDsEqual(scope.ChangeID, change)) &&
		(scope.TaskID == "" || scope.TaskID == task) &&
		(scope.Phase == "" || scope.Phase == phase)
}

// claimScopeForTakeover atomically transitions an open scope to closed, failing
// closed with ErrScopeClosed if it is already closed. Unlike closeScope (which is
// idempotent, used by the cooperative claim path where a redundant close is fine),
// this is the SINGLE-USE takeover gate: under a race exactly one caller wins the
// open→closed transition, so exactly one takeover mints a fresh owner.
//
// Single-use is per-scope, and a scope is minted once per run START, so the
// outer recovery scope grants at most ONE automatic outer takeover per start:
// the first accepted takeover closes it, and a second detached-crash takeover
// under the same run key then finds scope.Closed and HALTs scope-closed
// (runTrackerOuterContinuation maps that to a terminal run-stop run-tracker-unavailable, no
// retry spent). This is intentional fail-closed behavior — a human recovers by
// restarting a fresh scope via `run start --resume` — not a bug; see the spec's
// §5 continuation clause ("remains active until implement-next reaches a true
// terminal disposition") for the documented bound.
func (s *Store) claimScopeForTakeover(scopeID string) error {
	return s.scopeCAS(scopeID, func(rec *scopeRecord) error {
		if rec.Closed {
			return ownershipErr(ErrScopeClosed, "takeover-close")
		}
		rec.Closed = true
		return nil
	})
}
