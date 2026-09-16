package gatedrive

import "fmt"

// ValidateActiveChildInputs rechecks dispatch ownership without admitting a new
// launch. The immutable payload's predecessor may have been consumed by this
// worker's own drive sequence; it is launch authority, not active identity.
// Acknowledgement and takeover close the scope and invalidate this check.
func (d *Driver) ValidateActiveChildInputs(req StartRequest) error {
	scope, err := d.store.LoadScope(req.ScopeID)
	if err != nil {
		return err
	}
	if scope.Closed {
		return ownershipErr(ErrScopeClosed, "validate-active-child")
	}
	if req.ChildCapability == "" || scope.ChildCapHash != capHash(req.ChildCapability) {
		return ownershipErr(ErrScopeCapabilityMismatch, "validate-active-child")
	}
	if !scopedIdentityMatch(scope, req) {
		return ownershipErr(ErrScopeIdentityMismatch, "validate-active-child")
	}
	return d.validateScopeEpoch(scope, req.RunEpochID, "validate-active-child")
}

// ValidateChildInputs performs the same read-only scope and predecessor checks
// as Start, without reserving a slot, minting a drive, or launching a process.
// Start remains the atomic authority and repeats these checks under its locks.
func (d *Driver) ValidateChildInputs(req StartRequest) error {
	if req.ScopeID == "" {
		return fmt.Errorf("gatedrive: child inputs require a scope")
	}
	if err := d.precheckScopedStart(req); err != nil {
		return err
	}
	scope, err := d.store.LoadScope(req.ScopeID)
	if err != nil {
		return err
	}
	if err := d.validateScopeEpoch(scope, req.RunEpochID, "validate-child"); err != nil {
		return err
	}
	if req.PredecessorDriveID != "" && scope.CurrentDriveID != req.PredecessorDriveID {
		return ownershipErr(ErrStalePredecessor, "validate-child")
	}
	return nil
}

type RecoveredInputs struct {
	ScopeID         string
	DriveID         string
	OwnerGeneration string
	ChangeID        string
	TaskID          string
	Phase           string
	GateContext     string
	RunEpochID      string
	RepoDir         string
	Worktree        string
}

// ValidateRecoveredInputs verifies a closed dispatch boundary's terminal drive
// for a continuation. It returns no capability and performs no mutation.
func (d *Driver) ValidateRecoveredInputs(in RecoveredInputs) error {
	scope, err := d.store.LoadScope(in.ScopeID)
	if err != nil {
		return err
	}
	if !scope.Closed || scope.CurrentDriveID != in.DriveID {
		return ownershipErr(ErrScopeClosed, "validate-recovered")
	}
	rec, err := d.store.Load(in.DriveID)
	if err != nil {
		return err
	}
	if rec.OwnerGeneration != in.OwnerGeneration || rec.LastOutcome != PASSED {
		return ownershipErr(ErrStalePredecessor, "validate-recovered")
	}
	if rec.ScopeID != in.ScopeID || rec.ChangeID != in.ChangeID || rec.TaskID != in.TaskID || rec.Phase != in.Phase || rec.RepoIdentity != in.RepoDir || rec.WorktreePath != in.Worktree || scope.RepoIdentity != in.RepoDir || scope.Worktree != in.Worktree {
		return ownershipErr(ErrScopeIdentityMismatch, "validate-recovered")
	}
	if rec.GateContextHash != "" && rec.GateContextHash != capHash(in.GateContext) {
		return ownershipErr(ErrScopeIdentityMismatch, "validate-recovered")
	}
	if err := d.validateScopeEpoch(scope, in.RunEpochID, "validate-recovered"); err != nil {
		return err
	}
	current, err := ComputeFingerprint(rec.WorktreePath, d.git)
	if err != nil || !current.Equal(rec.Fingerprint) {
		return ownershipErr(ErrStalePredecessor, "validate-recovered")
	}
	return nil
}

func (d *Driver) validateScopeEpoch(scope scopeRecord, runEpochID, op string) error {
	if scope.RunEpochID != runEpochID {
		return ownershipErr(ErrStaleRunEpoch, op)
	}
	if scope.RunEpochID == "" || d.epochRevoked == nil {
		return nil
	}
	revoked, err := d.epochRevoked(scope.RunEpochID)
	if err != nil {
		return fmt.Errorf("gatedrive: %s epoch: %w", op, err)
	}
	if revoked {
		return ownershipErr(ErrStaleRunEpoch, op)
	}
	return nil
}
