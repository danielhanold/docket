// Event-authorized parent takeover: the exceptional ownership transfer for a
// direct child that returned WITHOUT handing off.
//
// The cooperative transfer (ownership.go / driver.go Handoff+Claim) is the
// PREFERRED path and the only one a healthy child uses. Takeover exists for the
// one case cooperation cannot cover: a parent's direct child was dispatched under
// a recovery scope (scope.go), then returned without offering a handoff — because
// it crashed, was killed, or a harness dropped its return. The parent recovers
// the still-live (or terminal-but-unconsumed) drive so a progressing run is
// continued rather than restarted.
//
// The AUTHORIZATION is a workflow fact, never a timer: the CALLER asserts "my
// direct child returned without handing off" simply by calling Takeover at all
// (spec Constraint 2: no timers, heartbeats, log-activity checks, claim-age, or
// process-name liveness guesses anywhere). Takeover proves EVERYTHING ELSE before
// it transfers a single generation:
//
//   - the exact PARENT capability (distinct from the child's own capability);
//   - the scope is open (a closed scope was already claimed or taken over);
//   - exactly ONE candidate drive resolves (ambiguity or none fails closed);
//   - no outstanding unclaimed handoff (a valid handoff means Claim, not takeover);
//   - the drive/scope IDENTITY agrees and the worktree FINGERPRINT still matches;
//   - a live drive's DEADLINE has not expired.
//
// Only then does it CLAIM THE SCOPE — a single-use open→closed transition that is
// the atomic winner-selection point under a race — and, under that gate, swap the
// superseded child owner generation for a fresh parent-minted one. It never
// launches, stops, or duplicates a process; every uncertainty returns a HALTED
// document (never a red suite result), exactly like the rest of the driver.
package gatedrive

// Takeover performs the event-authorized exceptional transfer of a scope-bound
// drive to a fresh owner the parent mints. driveID may be "" — then the scope's
// BoundDriveID (a task scope bound at Start) or the unique gate-context match (an
// outer scope resolved by FindScopeDriveIDs) resolves it. On success the returned
// document carries, in Generation, the fresh owner generation the parent advances
// with, and the scope is closed. Any capability failure, ambiguity, identity
// drift, outstanding handoff, expired deadline, or lost race returns a HALTED
// document — never a launch, a stop, or a duplicated process.
func (d *Driver) Takeover(scopeID, parentCapability, driveID string) (DriveDoc, error) {
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

	resolvedID, cause, rerr := d.resolveTakeoverDrive(scope, driveID)
	if rerr != nil {
		return DriveDoc{}, rerr
	}
	if cause != "" {
		return d.haltDoc(driveID, "", driveRecord{}, cause), nil
	}

	rec, err := d.store.Load(resolvedID)
	if err != nil {
		if se, ok := AsStoreError(err); ok {
			switch se.Kind {
			case ErrUnknownSchema, ErrCorruptRecord:
				return d.haltDoc(resolvedID, "", driveRecord{}, CauseSchemaMismatch), nil
			}
		}
		return DriveDoc{}, err
	}

	// The resolved drive must be the scope's own work: its identity (repo, branch,
	// worktree, change, task, phase — for each field the scope actually pins) must
	// agree with the scope. A drift is fail-closed, never a transfer.
	if !scopeIdentityMatch(scope, rec.RepoIdentity, rec.Branch, rec.WorktreePath, rec.ChangeID, rec.TaskID, rec.Phase) {
		return d.haltDoc(resolvedID, "", rec, "identity-mismatch"), nil
	}
	// A drive that already carries an unclaimed handoff must be CLAIMED, not taken
	// over — the child cooperated after all.
	if rec.HandoffGeneration != "" {
		return d.haltDoc(resolvedID, "", rec, string(ErrHandoffOutstanding)), nil
	}
	// The worktree must still match the drive-start execution identity, so a
	// continuation certifies the original bytes. This is the SOLE fingerprint check
	// on the takeover path (a post-takeover pass re-validates it again in
	// driveSlice), so its removal is directly mutation-observable.
	current, ferr := ComputeFingerprint(rec.WorktreePath, d.git)
	if ferr != nil {
		return d.haltDoc(resolvedID, "", rec, "fingerprint-error"), nil
	}
	if !rec.Fingerprint.Equal(current) {
		return d.haltDoc(resolvedID, "", rec, string(ErrFingerprintMismatch)), nil
	}
	// A live (nonterminal) drive whose fixed deadline has passed earns no
	// continuation. A terminal-unconsumed drive is past its run, so its deadline is
	// immaterial — only the recorded verdict is consumed.
	if !isTerminalOutcome(rec.LastOutcome) {
		if expired, _ := rec.deadlineState(d.clock.Now()); expired {
			return d.haltDoc(resolvedID, "", rec, CauseDeadlineExpired), nil
		}
	}
	// The child owner generation this takeover supersedes. It must be present (a
	// fully consumed drive has no owner to supersede).
	supersededOwner := rec.OwnerGeneration
	if supersededOwner == "" {
		return d.haltDoc(resolvedID, "", rec, string(ErrNotOwner)), nil
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
			return d.haltDoc(resolvedID, "", rec, string(oe.Kind)), nil
		}
		return DriveDoc{}, cerr
	}

	// Under the drive's ownership CAS, atomically invalidate the child owner
	// generation and install the fresh parent-minted one. The supersededOwner guard
	// fails closed if a concurrent cooperative transfer moved the owner between our
	// read and this write.
	cerr := d.store.ownerCAS(resolvedID, func(r *driveRecord) error {
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
			return d.haltDoc(resolvedID, "", rec, string(oe.Kind)), nil
		}
		return DriveDoc{}, cerr
	}

	cur, err := d.store.Load(resolvedID)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.transferDoc(resolvedID, freshOwner, cur), nil
}

// resolveTakeoverDrive resolves the single drive a takeover targets. An explicit
// driveID is used as given. Otherwise a task scope resolves to its BoundDriveID,
// and an outer scope (no bound drive) resolves to the UNIQUE gate-context match:
// its nested drives carry GateContextHash == the outer scope's child capability
// hash (the dispatch context is the outer scope's child capability). Zero matches
// or more than one fail closed with a distinct cause; a real scan fault is a
// command error.
func (d *Driver) resolveTakeoverDrive(scope scopeRecord, driveID string) (string, string, error) {
	if driveID != "" {
		return driveID, "", nil
	}
	if scope.BoundDriveID != "" {
		return scope.BoundDriveID, "", nil
	}
	ids, err := d.store.FindScopeDriveIDs(scope.ChangeID, scope.ChildCapHash)
	if err != nil {
		return "", "", err
	}
	switch len(ids) {
	case 0:
		return "", CauseTakeoverNoCandidate, nil
	case 1:
		return ids[0], "", nil
	default:
		return "", CauseTakeoverAmbiguous, nil
	}
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
func (s *Store) claimScopeForTakeover(scopeID string) error {
	return s.scopeCAS(scopeID, func(rec *scopeRecord) error {
		if rec.Closed {
			return ownershipErr(ErrScopeClosed, "takeover-close")
		}
		rec.Closed = true
		return nil
	})
}
