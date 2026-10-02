// Cancellation-specific run retirement (change 0435). Ordinary execution release
// (ReleaseWorktreeExecution) deliberately RETAINS RunID so a live run owns
// its worktree between drives; a COMPLETED cancellation must be able to detach that
// ownership so a replacement build gate or a no-run-record finalize gate can admit
// again. RetireWorktreeExecutionRun is that one store operation: an admissionCAS
// mutate closure that clears ONLY RunID on a RELEASED slot the expected run
// owns, preserving every historical field (DriveID/RawRunID/RawRunDir/ExecutionGen/
// Kind/ReservationToken and the legacy-inventory facts). Authorized
// cancellation completion — and the matching bounded terminal-repair / resume
// quiescence check — invokes it after complete launch/participant/mutation
// accounting. Admission invokes it too, but only for a RELEASED slot whose leftover
// RunID the app-injected RunSettledFunc proves completed or
// confirmed-cancelled (settleStaleReleasedRun, change 0446), so a successfully
// completed run never has to be cancelled to free its worktree. The death guardian
// never does (it fences and reaps but leaves the run cancelling — see the app
// layer's guardianFenceAndReap contract).
package gatedrive

import (
	"errors"
	"time"
)

// RunSettledFunc reports whether the run named by runID is durably
// SETTLED: its successful closeout completed, or its cancellation completed with
// confirmed accounting (the run's terminal state in its readable record). An
// active, cancelling, or completing run is not settled — it may still own its
// worktree between drives. A clean "no such run" is (false, ErrRunRecordUnresolved):
// a slot-named run that resolves to no readable record is an unresolved owner,
// never proof of settlement. An enumeration/IO fault is a non-nil error; the admission fence
// treats both as unsettled and keeps refusing (fail closed). It never returns a
// credential.
type RunSettledFunc func(runID string) (settled bool, err error)

// ErrRunRecordUnresolved is the sentinel a RunSettledFunc wraps when NO readable
// run record carries the named run. It is unsettled (the fence keeps
// refusing), and the refusal's incumbent snapshot is marked RunUnresolved so its
// remedy never points at a cancellation that cannot resolve that run.
var ErrRunRecordUnresolved = errors.New("run record unresolved")

// SetRunSettledResolver injects the optional run settlement seam the
// worktree admission fence consults (change 0446 spec §§2, 5). The application
// layer wires the production resolver over its run registry at composition,
// before any concurrent reservation, so it needs no lock; gatedrive tests inject a
// fake. Passing nil clears it: a released slot naming another run then refuses
// ErrStaleRunID exactly as before.
func (s *Store) SetRunSettledResolver(fn RunSettledFunc) { s.runSettled = fn }

// RunSettledResolverWired reports whether a settlement seam has been injected. It
// is a read-only composition probe the app-layer wiring test keys on — never
// consulted by an admission.
func (s *Store) RunSettledResolverWired() bool { return s.runSettled != nil }

// SetRunSettledResolver injects the settlement seam into the driver's store (see
// Store.SetRunSettledResolver), so the driver's admissions settle a released
// slot's leftover run through the same fence. Composition-time only.
func (d *Driver) SetRunSettledResolver(fn RunSettledFunc) { d.store.SetRunSettledResolver(fn) }

// RunSettledResolverWired reports whether the driver's store carries the
// settlement seam (composition probe; see Store.RunSettledResolverWired).
func (d *Driver) RunSettledResolverWired() bool { return d.store.RunSettledResolverWired() }

// staleReleasedRun identifies the exact released slot whose leftover RunID
// refused a reservation: the canonical worktree, the run the slot names, and the
// slot's reservation token read under the slot lock — the exact identity
// RetireWorktreeExecutionRun checks before detaching.
type staleReleasedRun struct {
	worktree string
	runID    string
	token    string
}

// settleStaleReleasedRun decides whether a released slot's leftover RunID is
// settled and, only then, retires it through RetireWorktreeExecutionRun with the
// exact run and token read under the lock (change 0446 spec §2: "settled through
// the existing exact-token retirement … not refused with ErrStaleRunID"). It
// runs OUTSIDE the slot lock. retry=false keeps the original refusal: no seam, a
// seam error, or an unsettled run (active/cancelling/completing, or unreadable).
// A retirement the slot refused logically (the token, run, or state moved — a
// successor raced it) still returns retry=true: the caller's single retry re-reads
// the ACTUAL slot under the lock and applies the fence to it without settling
// again. A retirement store fault is returned as err (never reported as admission).
//
// unresolved reports that the seam found NO readable record for the run
// (ErrRunRecordUnresolved): the refusal stands, and the caller marks its incumbent
// snapshot so the remedy never suggests cancelling a run that cannot resolve.
func (s *Store) settleStaleReleasedRun(st staleReleasedRun) (retry, unresolved bool, err error) {
	if s.runSettled == nil {
		return false, false, nil
	}
	settled, serr := s.runSettled(st.runID)
	if errors.Is(serr, ErrRunRecordUnresolved) {
		return false, true, nil
	}
	if serr != nil || !settled {
		return false, false, nil
	}
	if rerr := s.RetireWorktreeExecutionRun(st.worktree, st.runID, st.token); rerr != nil {
		if _, ok := AsOwnershipError(rerr); ok {
			return true, false, nil
		}
		return false, false, rerr
	}
	return true, false, nil
}

// errRunAlreadyDetached aborts the CAS with no write when the slot carries no
// RunID: retirement is idempotent, so an already-detached slot is success,
// not a refusal. Internal to RetireWorktreeExecutionRun.
var errRunAlreadyDetached = errors.New("worktree slot run already detached")

// RetireWorktreeExecutionRun clears the run ownership of a RELEASED
// worktree slot, atomically checking expected run, reservation token, and state
// under the slot's flock + physical generation (admissionCAS):
//
//   - RunID already ""            → idempotent success, no write.
//   - RunID != expectRunID        → ErrStaleRunID (a foreign owner or a
//     successor: never touched — a different nonempty RunID is a foreign
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
// expectRunID must be non-empty: an empty run is not ownership proof
// (ErrInvalidID). On success only RunID and UpdatedAt change.
func (s *Store) RetireWorktreeExecutionRun(worktreeRoot, expectRunID, expectToken string) error {
	const op = "retire-worktree-execution-run"
	if expectRunID == "" {
		return storeErr(ErrInvalidID, op, nil)
	}
	err := s.admissionCAS(worktreeRoot, func(rec *admissionRecord) error {
		if rec.RunID == "" {
			return errRunAlreadyDetached
		}
		if rec.RunID != expectRunID {
			return ownershipErr(ErrStaleRunID, op)
		}
		if verr := verifyAdmissionToken(rec, expectToken, op); verr != nil {
			return verr
		}
		if rec.State != admissionReleased {
			return ownershipErr(ErrWorktreeBusy, op)
		}
		rec.RunID = ""
		rec.UpdatedAt = time.Now().UTC()
		return nil
	})
	if errors.Is(err, errRunAlreadyDetached) {
		return nil
	}
	return err
}
