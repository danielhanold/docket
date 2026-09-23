// Finished-incumbent reconciliation at the normal admission boundary (change 0446
// spec §3).
//
// A worktree execution slot can still say reserved/executing/stopping/unresolved
// after its execution provably finished, because the process ended before its owner
// persisted the release (an interrupted Advance, a crashed CLI, a raw launch whose
// slot is otherwise released only by an explicit stop). Before a fresh start is
// refused over such a slot, the admission paths call reconcileFinishedIncumbent: a
// bounded, synchronous inspection of the ONE exact incumbent that settles only the
// facts the existing machinery can prove, then lets that same admission continue.
// It is not a retry controller, a sweeper, or a new liveness implementation.
//
// Decision table (the proof each incumbent shape needs before its slot is released):
//
//   - A driven (scoped/scopeless) slot whose CURRENT-TOKEN drive — the one drive
//     whose AdmissionToken equals the slot's ReservationToken, the only drive↔slot
//     link the records carry — is PASSED/FAILED: the supervisor-committed drive
//     record is itself the completion evidence. A terminal drive can never launch
//     again (revalidateAdmittedLaunch and reserveRelaunch both refuse a terminal
//     record), so no launch claim is needed.
//   - That drive HALTED: the label proves nothing (spec §4 "no blanket trust in
//     HALTED"). Its nonblocking launch claim must be free, and every recorded run
//     (current and prior) must be proven torn down by the process-recovery
//     predicate; a HALTED drive that never launched needs ResolveReservation's
//     never-launched verdict for its exact admission token. The claim is held across
//     the release CAS so no launcher can act in between.
//   - A raw slot with a confirmed run dir: the same process-recovery proof on that
//     run (this deliberately narrows ADR-0118's explicit-stop-only raw release; it
//     grants no signalling authority).
//   - Everything else refuses with a bounded finding: a nonterminal drive (a live
//     owner may still advance or relaunch it), an unattached relaunch reservation, a
//     delayed ticket (a reserved slot whose drive never launched is indistinguishable
//     from an admitter between Admit and StartAdmitted, so it is never settled here),
//     a busy claim, an unconfirmed raw reservation, an unprovable process, an
//     unfindable or ambiguous current-token drive, and a slot another run epoch owns.
//
// Proof is gathered OUTSIDE the slot lock; the release is then applied under the
// slot's CAS with the EXACT expected reservation token and state
// (releaseProvenIncumbent). If a successor won the slot in between, the CAS
// refuses, and the actual incumbent is re-evaluated from scratch once — its own
// proof, never the old proof under the newly observed token. A failed release write
// is never reported as settled. Admission never stops a process to make room.
package gatedrive

import (
	"os"
	"sort"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// incumbentProofSeam is the process predicate set finished-incumbent
// reconciliation consults: the process-recovery classifier (the single
// liveness/teardown predicate, also used by the first-admission legacy inventory)
// and reservation resolution for a launch that may never have happened. Both
// *process.Service and the driver's ProcessSeam satisfy it. A nil seam proves
// nothing: only a PASSED/FAILED drive incumbent can then be settled.
type incumbentProofSeam interface {
	ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error)
	ResolveReservation(root, token string) (*process.ReservationResolution, error)
}

// incumbentApplyHook is a package-private test seam fired once per evaluation,
// between the out-of-lock proof and the release CAS. Production leaves it nil; a
// test sets it to land a successor reservation or fault the slot directory at
// exactly that instant.
var incumbentApplyHook func()

// maxIncumbentEvaluations bounds reconciliation to the original incumbent plus ONE
// re-evaluation of a successor that raced the release CAS (spec §3 "report/
// re-evaluate that actual incumbent once").
const maxIncumbentEvaluations = 2

// Reconciliation findings — bounded, credential-free tokens carried on
// OwnershipError.Reconciliation when a refusal stays final.
const (
	findingIncumbentSettled    = "incumbent-settled"
	findingIncumbentAdmissible = "incumbent-admissible"
	findingSlotUnreadable      = "incumbent-slot-unreadable"
	findingEpochFenced         = "incumbent-epoch-fenced"
	findingKindUnknown         = "incumbent-kind-unknown"
	findingReservationPending  = "incumbent-reservation-pending"
	findingRunUnproven         = "incumbent-run-unproven"
	findingDriveUnresolved     = "incumbent-drive-unresolved"
	findingDriveAmbiguous      = "incumbent-drive-ambiguous"
	findingNonterminal         = "incumbent-nonterminal"
	findingRelaunchPending     = "incumbent-relaunch-pending"
	findingClaimBusy           = "incumbent-claim-busy"
	findingClaimUnresolved     = "incumbent-claim-unresolved"
	findingHaltedUnproven      = "incumbent-halted-unproven"
	findingIncumbentRaced      = "incumbent-raced"
	findingReleaseWriteFailed  = "release-write-failed"
)

// ReconcileFinishedIncumbent is the exported entry for callers outside this package
// that hold a store and a process service but no Driver — the app layer's raw
// launch path. See reconcileFinishedIncumbent.
func (s *Store) ReconcileFinishedIncumbent(worktreeRoot, runEpochID string, proc incumbentProofSeam) (bool, string, error) {
	return s.reconcileFinishedIncumbent(worktreeRoot, runEpochID, proc)
}

// ReconcileFinishedIncumbent settles a proven-finished incumbent on worktreeRoot's
// slot with this driver's own process seam (reconcileFinishedIncumbent). It is the
// seam the application layer's advisory pre-admission check consults before a busy
// refusal becomes final.
func (d *Driver) ReconcileFinishedIncumbent(worktreeRoot, runEpochID string) (bool, string, error) {
	return d.reconcileFinishedIncumbent(worktreeRoot, runEpochID)
}

// reconcileFinishedIncumbent is the driver's form of the store reconciliation,
// supplying the driver's ProcessSeam as the proof seam.
func (d *Driver) reconcileFinishedIncumbent(worktreeRoot, runEpochID string) (bool, string, error) {
	var proc incumbentProofSeam
	if d.proc != nil {
		proc = d.proc
	}
	return d.store.reconcileFinishedIncumbent(worktreeRoot, runEpochID, proc)
}

// reconcileFinishedIncumbent inspects worktreeRoot's exact incumbent and, only on
// positive proof per the file-level decision table, releases it under the slot CAS.
// settled is true when the release write succeeded or the slot became admissible
// concurrently — the caller then retries its reservation ONCE (the reserve remains
// the admission authority). finding is a bounded token naming the outcome. err is
// non-nil only for a failed release write (settled is then false). runEpochID is the
// requesting admission's epoch: a slot another epoch owns is never touched, because
// the reserve's run-epoch fence refuses that admission regardless of the
// incumbent's state.
func (s *Store) reconcileFinishedIncumbent(worktreeRoot, runEpochID string, proc incumbentProofSeam) (settled bool, finding string, err error) {
	for pass := 0; pass < maxIncumbentEvaluations; pass++ {
		slot, _, lerr := s.LoadWorktreeExecution(worktreeRoot)
		if lerr != nil {
			if storeErrIs(lerr, ErrNotFound) {
				return true, findingIncumbentAdmissible, nil // no incumbent: the reserve decides
			}
			return false, findingSlotUnreadable, nil
		}
		if slot.State == admissionReleased {
			return true, findingIncumbentAdmissible, nil
		}
		if slot.RunEpochID != "" && slot.RunEpochID != runEpochID {
			return false, findingEpochFenced, nil
		}
		proven, pfinding, release := s.proveIncumbentFinished(slot, proc)
		if !proven {
			return false, pfinding, nil
		}
		if incumbentApplyHook != nil {
			incumbentApplyHook()
		}
		werr := s.releaseProvenIncumbent(worktreeRoot, slot.ReservationToken, slot.State)
		release()
		if werr == nil {
			return true, findingIncumbentSettled, nil
		}
		if oe, ok := AsOwnershipError(werr); ok && oe.Op == opReleaseProvenIncumbent {
			// The exact incumbent the proof covered is gone (a successor's token or a
			// changed state): re-evaluate the ACTUAL incumbent once, from scratch.
			continue
		}
		return false, findingReleaseWriteFailed, werr
	}
	return false, findingIncumbentRaced, nil
}

// proveIncumbentFinished applies the decision table to one slot snapshot. On proof
// it returns proven=true and a release func the caller runs AFTER the release CAS
// (it drops a held launch claim; a no-op otherwise). On refusal it returns the
// bounded finding and has already dropped anything it acquired.
func (s *Store) proveIncumbentFinished(slot admissionRecord, proc incumbentProofSeam) (proven bool, finding string, release func()) {
	noop := func() {}
	switch slot.Kind {
	case "raw":
		if slot.RawRunDir == "" {
			// An unconfirmed raw reservation: its launcher may be between reserve and
			// launch. Never settled here.
			return false, findingReservationPending, noop
		}
		if !runTornDown(proc, slot.RawRunDir) {
			return false, findingRunUnproven, noop
		}
		return true, "", noop
	case "scoped", "scopeless":
		id, rec, f := s.findIncumbentDrive(slot.ReservationToken)
		if f != "" {
			return false, f, noop
		}
		return s.proveDriveFinished(id, rec, slot.ReservationToken, proc)
	default:
		return false, findingKindUnknown, noop
	}
}

// findIncumbentDrive returns the ONE readable drive whose AdmissionToken equals the
// slot's current reservation token. The slot records no drive id, so the token is
// the only link; a registry that cannot be listed, no match, or more than one match
// is a bounded refusal finding (never a guess). An unreadable record is not a match:
// resolving the incumbent never depends on reading a record nothing names.
func (s *Store) findIncumbentDrive(token string) (string, driveRecord, string) {
	if token == "" {
		return "", driveRecord{}, findingDriveUnresolved
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return "", driveRecord{}, findingDriveUnresolved
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var (
		matchID  string
		matchRec driveRecord
		matches  int
	)
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || validateID(id) != nil {
			continue
		}
		rec, lerr := s.Load(id)
		if lerr != nil || rec.AdmissionToken != token {
			continue
		}
		matches++
		matchID, matchRec = id, rec
	}
	switch matches {
	case 0:
		return "", driveRecord{}, findingDriveUnresolved
	case 1:
		return matchID, matchRec, ""
	default:
		return "", driveRecord{}, findingDriveAmbiguous
	}
}

// proveDriveFinished applies the driven rows of the decision table to the
// current-token drive.
func (s *Store) proveDriveFinished(id string, rec driveRecord, token string, proc incumbentProofSeam) (bool, string, func()) {
	noop := func() {}
	if rec.RelaunchReserved {
		// A reserved-but-unattached replacement may still launch or be live.
		return false, findingRelaunchPending, noop
	}
	switch rec.LastOutcome {
	case PASSED, FAILED:
		return true, "", noop
	case HALTED:
		// handled below: a HALTED label is never proof on its own.
	default:
		return false, findingNonterminal, noop
	}

	claim, busy, cerr := s.tryRelaunchClaim(id)
	if cerr != nil {
		return false, findingClaimUnresolved, noop
	}
	if busy {
		return false, findingClaimBusy, noop
	}
	// Re-read under the held claim: the durable record may have moved between the
	// registry walk and the claim acquisition.
	cur, lerr := s.Load(id)
	if lerr != nil || cur.AdmissionToken != token || cur.LastOutcome != HALTED || cur.RelaunchReserved {
		claim.close()
		return false, findingIncumbentRaced, noop
	}
	if !haltedTornDown(cur, token, proc) {
		claim.close()
		return false, findingHaltedUnproven, noop
	}
	return true, "", claim.close
}

// haltedTornDown reports positive teardown proof for a HALTED drive: every recorded
// run (current and prior attempt) torn down, or — for a drive that never attached a
// run — its exact admission reservation provably never launched (or launched a run
// that is itself proven torn down).
func haltedTornDown(rec driveRecord, token string, proc incumbentProofSeam) bool {
	if proc == nil {
		return false
	}
	if rec.RawRunDir == "" {
		res, err := proc.ResolveReservation(rec.RunRoot, token)
		if err != nil || res == nil {
			return false
		}
		switch res.Disposition {
		case "never-launched":
			return true
		case "identified":
			return res.RunDir != "" && runTornDown(proc, res.RunDir)
		default:
			return false
		}
	}
	for _, runDir := range []string{rec.RawRunDir, rec.PriorRawRunDir} {
		if runDir != "" && !runTornDown(proc, runDir) {
			return false
		}
	}
	return true
}

// runTornDown reports whether the process-recovery predicate proves one run's
// group gone: a durable terminal record or completed-stop marker with the
// supervisor gone, an abandoned marker already present, or provable group absence
// (recorded as an abandoned marker, exactly as the first-admission legacy inventory
// records it). A live, unprovable, foreign, invalid, or erroring assessment proves
// nothing. A free lock alone ("vanished") is never sufficient.
func runTornDown(proc incumbentProofSeam, runDir string) bool {
	if proc == nil || runDir == "" {
		return false
	}
	entry, err := proc.ClassifyRun(runDir, true)
	if err != nil {
		return false
	}
	switch entry.Disposition {
	case "terminal", "stopped", "already-abandoned", "abandoned-marked":
		return true
	default:
		return false
	}
}

// opReleaseProvenIncumbent is the op every logical rejection of the release CAS
// carries, so reconciliation can tell "the exact incumbent is gone" (re-evaluate)
// from an IO fault (a failed release write).
const opReleaseProvenIncumbent = "release-proven-incumbent"

// releaseProvenIncumbent vacates the slot to released under the slot CAS ONLY when
// it still carries the exact reservation token AND state the proof was gathered
// against. Unlike ReleaseWorktreeExecution (whose owner holds the token and needs
// no state check), this caller is not the owner: a state change under the same
// token means the incumbent moved, so its proof is stale. The released record keeps
// every other field, RunEpochID included — settling a finished execution never
// detaches a run epoch's between-drive ownership.
func (s *Store) releaseProvenIncumbent(worktreeRoot, expectToken string, expectState admissionState) error {
	return s.admissionCAS(worktreeRoot, func(rec *admissionRecord) error {
		if err := verifyAdmissionToken(rec, expectToken, opReleaseProvenIncumbent); err != nil {
			return err
		}
		if rec.State != expectState {
			return ownershipErr(ErrUnresolvedLaunchTransition, opReleaseProvenIncumbent)
		}
		rec.State = admissionReleased
		rec.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// isIncumbentRefusal reports whether err is a worktree-admission refusal decided on
// an occupying incumbent (worktree-busy or unresolved-execution carrying the
// incumbent snapshot) — the only refusals finished-incumbent reconciliation can
// change. A stale-run-epoch fence, a legacy-inventory refusal (no incumbent: the slot
// is absent), and every store fault are not.
func isIncumbentRefusal(err error) (*OwnershipError, bool) {
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Incumbent == nil {
		return nil, false
	}
	if oe.Kind != ErrWorktreeBusy && oe.Kind != ErrUnresolvedExecution {
		return nil, false
	}
	return oe, true
}
