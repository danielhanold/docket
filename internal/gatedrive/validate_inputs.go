package gatedrive

import "fmt"

// ValidateChildInputs performs the same read-only scope and predecessor checks
// as Start, without reserving a slot, minting a drive, or launching a process.
// Start remains the atomic authority and repeats these checks under its locks.
func (d *Driver) ValidateChildInputs(req StartRequest) error {
	if req.ScopeID == "" {
		return fmt.Errorf("gatedrive: child inputs require a scope")
	}
	return d.precheckScopedStart(req)
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
	if rec.OwnerGeneration != in.OwnerGeneration || !isTerminalOutcome(rec.LastOutcome) {
		return ownershipErr(ErrStalePredecessor, "validate-recovered")
	}
	if rec.ChangeID != in.ChangeID || rec.TaskID != in.TaskID || rec.Phase != in.Phase || scope.RunEpochID != in.RunEpochID {
		return ownershipErr(ErrScopeIdentityMismatch, "validate-recovered")
	}
	if rec.GateContextHash != "" && rec.GateContextHash != capHash(in.GateContext) {
		return ownershipErr(ErrScopeIdentityMismatch, "validate-recovered")
	}
	return nil
}
