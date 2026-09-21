// The successful-run ownership closeout engine (change 0441). completeSuccessfulRun
// is the counterpart to run.cancel's runCancel: where cancellation is the
// coordinator's authoritative Stop, this is the authoritative "the run finished
// successfully — release its epoch ownership so a standalone finalize gate can admit
// on the same worktree". It is driven ONLY from the attributed, keyed RunGateVerdict
// path on a verified run-complete (Task 8 wires the caller); RunVerify stays
// read-only and unattributed observe verdicts never reach it.
//
// OBSERVATION ONLY. Closeout stops nothing, signals nothing, and settles nothing: it
// never invokes native cancellation, never process.Stop, and never settles a
// never-launched reservation terminal. It reuses cancellation's accounting SHAPES
// through the two observation seams (processObserver / epochLaunchObserver) and the
// shared classifySlotOwnership / retireWorktreeSlotOwnership helpers, but every
// per-participant, per-process, per-slot, and per-launch decision is a pure
// observation. The stop-capable cancellation seams (stopper / native / launches) are
// never touched on this path.
//
// FAIL CLOSED. A live, busy, pending, uncertain, or unreadable obligation blocks
// completion: missing terminal evidence is UNPROVEN, never implicitly complete. A
// blocked closeout leaves the epoch durably completing (the success fence holds) and
// returns completion-unaccounted with the bounded findings that name what to settle;
// the remedy is to settle the named evidence and repeat the same keyed verdict, or to
// cancel explicitly. No retry, budget, or attempt is consumed by a blocked closeout.
//
// NEVER RELABEL. completing/completed are new states; a cancelling/cancelled/
// superseded/mismatched run is never relabelled successful (run-cancelled /
// stale-run-epoch), and an explicit human cancellation may win from completing —
// completion then loses without reporting success (CompleteEpoch's completing→
// completed CAS refuses once a cancel fence lands).
//
// LOCK ORDERING. The two epoch writes (FenceEpochCompleting, CompleteEpoch) run under
// epochCAS; ALL proof — participant observation, process observation, the worktree
// slot load, and the launch walk — runs OUTSIDE any epoch or admission lock, never
// holding a lock across a process observation or a per-drive claim probe.
package app

import (
	"fmt"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// processObserver observes whether one execution's OS process is provably TERMINAL,
// by its run directory — the observation-only counterpart of cancelStopper (which
// stops). Production appGateObserver observes through the app gate seam
// (process.Observe) and reuses the same proven-terminal rule the stop path uses
// (rawTeardownProven); a nil observer proves nothing (fail closed).
type processObserver interface {
	observeProcessTerminal(runDir string) (bool, error)
}

// epochLaunchObserver accounts, for a completing epoch, the pending and replacement
// LAUNCH obligations the durable drive records name — the observation-only
// counterpart of epochLaunchReconciler (which stops identified runs and settles
// never-launched reservations terminal). Production appLaunchObserver wraps
// gatedrive.Driver.ObserveEpochLaunches; a nil observer proves nothing (fail closed).
type epochLaunchObserver interface {
	observe(worktree, epochID string) (gatedrive.EpochLaunchReport, error)
}

// appGateObserver is the production processObserver: it observes a run through the
// app gate seam (process.Observe) and reports PROVEN teardown using rawTeardownProven
// — the exact rule appGateStopper uses to prove a stop settled — without ever
// stopping the process. It resolves the process service per call, exactly as
// appGateStopper does. An unresolvable service or an observation error proves nothing
// (fail closed): the caller turns it into a bounded finding and blocks.
type appGateObserver struct{}

func (appGateObserver) observeProcessTerminal(runDir string) (bool, error) {
	svc, _, reason := gateService()
	if svc == nil {
		return false, fmt.Errorf("gate service unavailable: %s", reason)
	}
	obs, err := svc.Observe(runDir)
	if err != nil {
		return false, err
	}
	return rawTeardownProven(obs.State), nil
}

// appLaunchObserver is the production epochLaunchObserver: it composes a gatedrive
// driver over the completion store and the app gate seam's process service, then
// observes one epoch's launch obligations through ObserveEpochLaunches (observation
// only — it stops nothing and settles nothing). It resolves the process service per
// call, exactly as appLaunchReconciler.reconcile does. A nil store or an unresolvable
// process service proves nothing (fail closed).
type appLaunchObserver struct {
	store *gatedrive.Store
}

func (o appLaunchObserver) observe(worktree, epochID string) (gatedrive.EpochLaunchReport, error) {
	if o.store == nil {
		return gatedrive.EpochLaunchReport{}, fmt.Errorf("gate store unavailable")
	}
	svc, _, reason := gateService()
	if svc == nil {
		return gatedrive.EpochLaunchReport{}, fmt.Errorf("gate service unavailable: %s", reason)
	}
	return gatedrive.NewSystemDriver(o.store, svc).ObserveEpochLaunches(worktree, epochID)
}

// completeSuccessfulRun drives the whole successful-run ownership closeout over the
// injected seams, returning ok, a bounded reason token for the gate-unavailable
// channel when ok is false (one of run-cancelled, stale-run-epoch,
// completion-unaccounted, completion-unpersisted, epoch-unreadable), and the bounded
// credential-free findings that name every unsettled obligation. The caller (Task 8)
// has already resolved the confirmed claim binding and the run-complete verdict; this
// function owns only the ownership retirement. See the file header for the
// observation-only, fail-closed, never-relabel, and lock-ordering contracts.
func completeSuccessfulRun(seams cancelSeams, repoDir, gateKey string) (ok bool, reason string, findings []string) {
	// (1) Durable success fence: CAS active→completing. The observed state under the
	// lock decides a rejection's bounded reason — a cancelling/cancelled run is never
	// relabelled successful, a superseded run is a stale epoch, and a store fault is
	// unreadable. An already-completed epoch is an idempotent completed-receipt replay
	// (safe after scratch cleanup): success with no further work.
	observed, ferr := FenceEpochCompleting(repoDir, gateKey, "")
	if ferr != nil {
		if ee, ok := AsEpochError(ferr); ok && ee.Kind == ErrEpochNotActive {
			switch observed {
			case EpochCancelling, EpochCancelled:
				return false, "run-cancelled", nil
			case EpochSuperseded:
				return false, "stale-run-epoch", nil
			default:
				return false, "epoch-unreadable", nil // an unknown/garbage state: fail closed
			}
		}
		return false, "epoch-unreadable", nil // IO/corrupt/not-found/mismatch: fail closed
	}
	if observed == EpochCompleted {
		return true, "", nil // idempotent completed-receipt replay
	}

	// (2) Reload the fenced record. All remaining proof runs OUTSIDE the epoch lock.
	ep, _, lerr := LoadEpochRecord(repoDir, gateKey)
	if lerr != nil {
		return false, "epoch-unreadable", nil
	}

	// (3) Observation-only accounting over the fenced record. Any blocking obligation
	// (an unobserved native task, a live/unproven execution process, an unreleased or
	// unprovable owned slot, an unaccounted launch, an uncompleted mutation) fails
	// closed; informational findings (a successor slot) are accounted.
	blocked, findings := accountCompletionObligations(seams, ep)

	// (4) RE-ENUMERATE before retirement: an operation admitted pre-fence may have
	// appended a participant or a mutation between the fence and step (3)'s read (the
	// completing fence rejects only NEW registration). The reload also catches a
	// cancellation that won meanwhile — a state no longer completing loses to it.
	reEp, _, rlerr := LoadEpochRecord(repoDir, gateKey)
	if rlerr != nil {
		return false, "epoch-unreadable", findings
	}
	switch reEp.State {
	case EpochCompleting:
		// still ours to close out
	case EpochCompleted:
		return true, "", findings // a concurrent replay finished the closeout
	default:
		return false, "run-cancelled", findings // a cancellation won from completing
	}
	pblocked, pf := accountCompletionParticipants(seams, reEp)
	mblocked, mf := accountCompletionMutations(reEp)
	if pblocked || mblocked {
		blocked = true
	}
	findings = appendFindings(findings, pf, mf)

	// (5) Blocked ⇒ the epoch stays durably completing; the remedy is to settle the
	// named evidence and repeat the same keyed verdict. No retry/budget/attempt moves.
	if blocked {
		return false, "completion-unaccounted", findings
	}

	// (6) Retire the released-slot ownership — reused verbatim from cancellation
	// (ownership-checked, expected-token/expected-epoch, successor-safe, idempotent on
	// absent/detached). A failed retirement keeps the epoch completing (repeatable).
	retired, rfinding := retireWorktreeSlotOwnership(seams, ep)
	if rfinding != "" {
		findings = append(findings, rfinding)
	}
	if !retired {
		return false, "completion-unaccounted", findings
	}

	// (7) Persist the terminal transition: CAS completing→completed. A failure because
	// a cancellation won is run-cancelled (completion loses without reporting success);
	// any other persistence failure is completion-unpersisted (fail closed, reportable).
	if cerr := CompleteEpoch(repoDir, gateKey); cerr != nil {
		if cur, _, e := LoadEpochRecord(repoDir, gateKey); e == nil {
			switch cur.State {
			case EpochCancelling, EpochCancelled, EpochSuperseded:
				return false, "run-cancelled", findings
			}
		}
		return false, "completion-unpersisted", findings
	}
	return true, "", findings
}

// accountCompletionObligations is step (3)'s full observation-only accounting over
// the fenced record: native participants, execution participants, the worktree slot,
// the launch obligations, and the mutation journal. It returns whether the run is
// blocked (any obligation unproven) and the accumulated bounded findings. It reads
// the epoch snapshot in memory and probes every process/slot/launch OUTSIDE any lock.
func accountCompletionObligations(seams cancelSeams, ep EpochRecord) (bool, []string) {
	blocked := false
	var findings []string

	pblocked, pf := accountCompletionParticipants(seams, ep)
	sblocked, sf := accountCompletionSlot(seams, ep)
	lblocked, lf := accountCompletionLaunches(seams, ep)
	mblocked, mf := accountCompletionMutations(ep)

	if pblocked || sblocked || lblocked || mblocked {
		blocked = true
	}
	findings = appendFindings(findings, pf, sf, lf, mf)
	return blocked, findings
}

// accountCompletionParticipants proves every registered participant terminal: a
// native task (coordinator/task) must carry recorded terminal evidence
// (TerminalStatus != "" — a terminal FAILURE still counts as observed; RunVerify
// independently decided implementation success), and an execution participant
// (gate-scope/raw-run) must observe a proven-terminal process. Absent evidence is
// UNPROVEN and blocks (participant-unobserved / process-*).
func accountCompletionParticipants(seams cancelSeams, ep EpochRecord) (bool, []string) {
	blocked := false
	var findings []string
	for _, p := range ep.Participants {
		switch {
		case isNativeParticipant(p.Kind):
			if p.TerminalStatus == "" {
				findings = append(findings, "participant-unobserved:"+p.Kind)
				blocked = true
			}
		case isExecutionParticipant(p.Kind):
			if proven, reason := observeTerminalProof(seams, p.NativeHandle); !proven {
				findings = append(findings, reason)
				blocked = true
			}
		}
	}
	return blocked, findings
}

// accountCompletionMutations blocks on any admitted-not-completed mutation
// (mutation-pending:<op>), mirroring cancellation's mutation reconciliation.
func accountCompletionMutations(ep EpochRecord) (bool, []string) {
	blocked := false
	var findings []string
	for _, m := range ep.AdmittedMutations {
		if m.Status != mutationStatusCompleted {
			findings = append(findings, "mutation-pending:"+m.OpKey)
			blocked = true
		}
	}
	return blocked, findings
}

// accountCompletionLaunches observes the epoch's launch obligations through the
// observation-only launch seam. A nil observer proves nothing (fail closed,
// launch-observer-unavailable); an observation error blocks (launch-observe-failed);
// an unaccounted report blocks and surfaces its findings. An accounted report's
// informational findings are surfaced without blocking.
func accountCompletionLaunches(seams cancelSeams, ep EpochRecord) (bool, []string) {
	if seams.launchObserver == nil {
		return true, []string{"launch-observer-unavailable"}
	}
	report, err := seams.launchObserver.observe(ep.Worktree, ep.EpochID)
	if err != nil {
		return true, []string{"launch-observe-failed"}
	}
	if !report.Accounted {
		return true, report.Findings
	}
	return false, report.Findings
}

// accountCompletionSlot observes the epoch's worktree slot ownership. It touches
// nothing — a pure observation of whether the slot poses an obligation success cannot
// prove settled:
//   - a keyless/standalone run (nil store or empty worktree) owns no slot;
//   - an absent slot is safely detached;
//   - slotForeign (a successor reserved after safe detachment) is informational,
//     accounted, left untouched;
//   - slotLinkedLegacy is this epoch's own execution, already covered by the
//     execution-participant pass;
//   - slotUnowned that is RELEASED is torn down (our own prior detachment, or a
//     released remnant) — nothing live to prove; an unreleased unowned slot is a live
//     slot success cannot prove it owns and blocks (slot-ownership-unresolved);
//   - slotOwned must be released (else slot-not-released) and, when it names a run,
//     that run must observe proven-terminal (else process-*).
func accountCompletionSlot(seams cancelSeams, ep EpochRecord) (bool, []string) {
	if seams.store == nil || ep.Worktree == "" {
		return false, nil
	}
	slot, _, err := seams.store.LoadWorktreeExecution(ep.Worktree)
	if err != nil {
		if se, ok := gatedrive.AsStoreError(err); ok && se.Kind == gatedrive.ErrNotFound {
			return false, nil // absent: safely detached, nothing to prove
		}
		return true, []string{"slot-unreadable"}
	}
	switch classifySlotOwnership(slot.RunEpochID, slot.RawRunDir, ep) {
	case slotForeign:
		return false, []string{"slot-replaced-by-successor"} // successor, informational
	case slotLinkedLegacy:
		return false, nil // covered by the execution-participant pass
	case slotUnowned:
		if string(slot.State) == "released" {
			return false, nil // torn down: our prior detachment or a released remnant
		}
		return true, []string{"slot-ownership-unresolved"} // a live slot we cannot prove ours
	}
	// slotOwned: only a released owned slot whose run is proven-terminal is settled.
	if string(slot.State) != "released" {
		return true, []string{"slot-not-released"}
	}
	if slot.RawRunDir != "" {
		if proven, reason := observeTerminalProof(seams, slot.RawRunDir); !proven {
			return true, []string{reason}
		}
	}
	return false, nil
}

// observeTerminalProof observes one execution's process through the observation-only
// seam and reports whether teardown is PROVEN. A nil observer proves nothing
// (process-observer-unavailable); an observation error is process-unobserved:<handle>;
// a non-terminal (live/signalled) run is process-live:<handle>. It never stops.
func observeTerminalProof(seams cancelSeams, handle string) (bool, string) {
	if seams.observer == nil {
		return false, "process-observer-unavailable"
	}
	proven, err := seams.observer.observeProcessTerminal(handle)
	if err != nil {
		return false, "process-unobserved:" + handle
	}
	if !proven {
		return false, "process-live:" + handle
	}
	return true, ""
}

// appendFindings concatenates finding slices into a fresh slice, so a caller can
// combine phase findings without aliasing any input's backing array.
func appendFindings(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
