// Cancellation-specific epoch retirement (change 0435). Ordinary execution release
// (ReleaseWorktreeExecution) deliberately RETAINS RunEpochID so a live epoch owns
// its worktree between drives; a COMPLETED cancellation must be able to detach that
// ownership so a replacement build gate or an epoch-less finalize gate can admit
// again. RetireWorktreeExecutionEpoch is that one store operation: an admissionCAS
// mutate closure that clears ONLY RunEpochID on a RELEASED slot the expected epoch
// owns, preserving every historical field (DriveID/RawRunID/RawRunDir/ExecutionGen/
// ScopeID/Kind/ReservationToken and the legacy-inventory facts). Authorized
// cancellation completion — and the matching bounded terminal-repair / resume
// quiescence check — invokes it after complete launch/participant/mutation
// accounting. Admission invokes it too, but only for a RELEASED slot whose leftover
// RunEpochID the app-injected EpochSettledFunc proves completed or
// confirmed-cancelled (settleStaleReleasedEpoch, change 0446), so a successfully
// completed run never has to be cancelled to free its worktree. The death guardian
// never does (it fences and reaps but leaves the epoch cancelling — see the app
// layer's guardianFenceAndReap contract).
package gatedrive

import (
	"errors"
	"time"
)

// EpochSettledFunc reports whether the run epoch named by epochID is durably
// SETTLED: its successful closeout completed, or its cancellation completed with
// confirmed accounting (the epoch's terminal state in its readable record). An
// active, cancelling, or completing epoch is not settled — it may still own its
// worktree between drives. A clean "no such epoch" is (false, ErrEpochUnresolved):
// a slot-named epoch that resolves to no readable record is an unresolved owner,
// never proof of settlement. An enumeration/IO fault is a non-nil error; the admission fence
// treats both as unsettled and keeps refusing (fail closed). It never returns a
// credential.
type EpochSettledFunc func(epochID string) (settled bool, err error)

// ErrEpochUnresolved is the sentinel an EpochSettledFunc wraps when NO readable
// run-epoch record carries the named epoch. It is unsettled (the fence keeps
// refusing), and the refusal's incumbent snapshot is marked EpochUnresolved so its
// remedy never points at a cancellation that cannot resolve that epoch.
var ErrEpochUnresolved = errors.New("run epoch record unresolved")

// SetEpochSettledResolver injects the optional run-epoch settlement seam the
// worktree admission fence consults (change 0446 spec §§2, 5). The application
// layer wires the production resolver over its run-epoch registry at composition,
// before any concurrent reservation, so it needs no lock; gatedrive tests inject a
// fake. Passing nil clears it: a released slot naming another epoch then refuses
// ErrStaleRunEpoch exactly as before.
func (s *Store) SetEpochSettledResolver(fn EpochSettledFunc) { s.epochSettled = fn }

// EpochSettledResolverWired reports whether a settlement seam has been injected. It
// is a read-only composition probe the app-layer wiring test keys on — never
// consulted by an admission.
func (s *Store) EpochSettledResolverWired() bool { return s.epochSettled != nil }

// SetEpochSettledResolver injects the settlement seam into the driver's store (see
// Store.SetEpochSettledResolver), so the driver's admissions settle a released
// slot's leftover epoch through the same fence. Composition-time only.
func (d *Driver) SetEpochSettledResolver(fn EpochSettledFunc) { d.store.SetEpochSettledResolver(fn) }

// EpochSettledResolverWired reports whether the driver's store carries the
// settlement seam (composition probe; see Store.EpochSettledResolverWired).
func (d *Driver) EpochSettledResolverWired() bool { return d.store.EpochSettledResolverWired() }

// staleReleasedEpoch identifies the exact released slot whose leftover RunEpochID
// refused a reservation: the canonical worktree, the epoch the slot names, and the
// slot's reservation token read under the slot lock — the exact identity
// RetireWorktreeExecutionEpoch checks before detaching.
type staleReleasedEpoch struct {
	worktree string
	epochID  string
	token    string
}

// settleStaleReleasedEpoch decides whether a released slot's leftover RunEpochID is
// settled and, only then, retires it through RetireWorktreeExecutionEpoch with the
// exact epoch and token read under the lock (change 0446 spec §2: "settled through
// the existing exact-token retirement … not refused with ErrStaleRunEpoch"). It
// runs OUTSIDE the slot lock. retry=false keeps the original refusal: no seam, a
// seam error, or an unsettled epoch (active/cancelling/completing, or unreadable).
// A retirement the slot refused logically (the token, epoch, or state moved — a
// successor raced it) still returns retry=true: the caller's single retry re-reads
// the ACTUAL slot under the lock and applies the fence to it without settling
// again. A retirement store fault is returned as err (never reported as admission).
//
// unresolved reports that the seam found NO readable record for the epoch
// (ErrEpochUnresolved): the refusal stands, and the caller marks its incumbent
// snapshot so the remedy never suggests cancelling an epoch that cannot resolve.
func (s *Store) settleStaleReleasedEpoch(st staleReleasedEpoch) (retry, unresolved bool, err error) {
	if s.epochSettled == nil {
		return false, false, nil
	}
	settled, serr := s.epochSettled(st.epochID)
	if errors.Is(serr, ErrEpochUnresolved) {
		return false, true, nil
	}
	if serr != nil || !settled {
		return false, false, nil
	}
	if rerr := s.RetireWorktreeExecutionEpoch(st.worktree, st.epochID, st.token); rerr != nil {
		if _, ok := AsOwnershipError(rerr); ok {
			return true, false, nil
		}
		return false, false, rerr
	}
	return true, false, nil
}

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
