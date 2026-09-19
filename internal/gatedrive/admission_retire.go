// Cancellation-specific epoch retirement (change 0435). Ordinary execution release
// (ReleaseWorktreeExecution) deliberately RETAINS RunEpochID so a live epoch owns
// its worktree between drives; a COMPLETED cancellation must be able to detach that
// ownership so a replacement build gate or an epoch-less finalize gate can admit
// again. RetireWorktreeExecutionEpoch is that one store operation: an admissionCAS
// mutate closure that clears ONLY RunEpochID on a RELEASED slot the expected epoch
// owns, preserving every historical field (DriveID/RawRunID/RawRunDir/ExecutionGen/
// ScopeID/Kind/ReservationToken and the legacy-inventory facts). Only authorized
// cancellation completion — and the matching bounded terminal-repair / resume
// quiescence check — invokes it, after complete launch/participant/mutation
// accounting; the death guardian never does (it fences and reaps but leaves the
// epoch cancelling — see the app layer's guardianFenceAndReap contract).
package gatedrive

import (
	"errors"
	"time"
)

// errEpochAlreadyDetached aborts the CAS with no write when the slot carries no
// RunEpochID: retirement is idempotent, so an already-detached slot is success,
// not a refusal. Internal to RetireWorktreeExecutionEpoch.
var errEpochAlreadyDetached = errors.New("worktree slot epoch already detached")

// RetireWorktreeExecutionEpoch clears the run-epoch ownership of a RELEASED
// worktree slot, atomically checking expected epoch, reservation token, and state
// under the slot's flock + physical generation (admissionCAS):
//
//   - RunEpochID already ""            → idempotent success, no write.
//   - RunEpochID != expectEpoch        → ErrStaleRunEpoch (a foreign owner or a
//     successor: never touched — a different nonempty RunEpochID is a foreign
//     owner).
//   - reservation token mismatch       → ErrNotOwner (the reservation changed
//     under the caller: a raced replacement is never cleared blindly —
//     verifyAdmissionToken).
//   - State != admissionReleased       → ErrWorktreeBusy (retirement additionally
//     requires released state; a live or unresolved slot is never detached).
//   - absent slot                      → ErrNotFound; unreadable/corrupt/unknown
//     schema fail closed with their typed StoreError, exactly as every other
//     admission transition (admissionCAS).
//
// expectEpoch must be non-empty: an empty epoch is not ownership proof
// (ErrInvalidID). On success only RunEpochID and UpdatedAt change.
func (s *Store) RetireWorktreeExecutionEpoch(worktreeRoot, expectEpoch, expectToken string) error {
	const op = "retire-worktree-execution-epoch"
	if expectEpoch == "" {
		return storeErr(ErrInvalidID, op, nil)
	}
	err := s.admissionCAS(worktreeRoot, func(rec *admissionRecord) error {
		if rec.RunEpochID == "" {
			return errEpochAlreadyDetached
		}
		if rec.RunEpochID != expectEpoch {
			return ownershipErr(ErrStaleRunEpoch, op)
		}
		if verr := verifyAdmissionToken(rec, expectToken, op); verr != nil {
			return verr
		}
		if rec.State != admissionReleased {
			return ownershipErr(ErrWorktreeBusy, op)
		}
		rec.RunEpochID = ""
		rec.UpdatedAt = time.Now().UTC()
		return nil
	})
	if errors.Is(err, errEpochAlreadyDetached) {
		return nil
	}
	return err
}
