// The successful-run ownership closeout engine (change 0441). completeSuccessfulRun
// is the counterpart to run.cancel's runCancel: where cancellation is the
// coordinator's authoritative Stop, this is the authoritative "the run finished
// successfully — release its run ownership so later workflow mutations on its
// worktree are no longer fenced to it". It is driven ONLY from the attributed, keyed RunVerdict
// path on a verified run-complete (Task 8 wires the caller); RunVerify stays
// read-only and unattributed observe verdicts never reach it.
//
// OBSERVATION ONLY. Closeout stops nothing and signals nothing: it never invokes
// native cancellation, never process.Stop, and never settles a never-launched
// reservation terminal. Its one journal repair is settleUncertainPublications (change
// 0444) — an uncertain→completed flip of publication entries a later verified
// identical retry proves, derived from the durable journal alone with no Git or
// GitHub call. It reuses cancellation's accounting SHAPES through the two
// observation seams (processObserver / runLaunchObserver), but every
// per-participant, per-process, and per-launch decision is a pure observation. The
// stop-capable cancellation seams (stopper / native / launches) are never touched
// on this path. Closeout writes no worktree record: a gate's worktree lock is held
// by its supervisor and frees itself when that supervisor exits (change 0490).
//
// STEPS. (1) the success fence active→completing; (1b) settle retry-proven
// uncertain publications; (2) reload; (3) observation-only accounting — native and
// execution participants, the observe-only launch census for the run's context
// hash, and the mutation journal; (4) re-enumerate participants and mutations;
// (5) any unsettled obligation blocks (completion-unaccounted); (6) CAS
// completing→completed.
//
// FAIL CLOSED. A live, busy, pending, uncertain, or unreadable obligation blocks
// completion: missing terminal evidence is UNPROVEN, never implicitly complete. A
// blocked closeout leaves the run durably completing (the success fence holds) and
// returns completion-unaccounted with the bounded findings that name what to settle;
// the remedy is to settle the named evidence and repeat the same keyed verdict, or to
// cancel explicitly. No retry, budget, or attempt is consumed by a blocked closeout.
//
// NEVER RELABEL. completing/completed are new states; a cancelling/cancelled/
// superseded/mismatched run is never relabelled successful (run-cancelled /
// stale-run-id), and an explicit human cancellation may win from completing —
// completion then loses without reporting success (CompleteRun's completing→
// completed CAS refuses once a cancel fence lands).
//
// LOCK ORDERING. The run writes (FenceRunCompleting, settleUncertainPublications,
// CompleteRun) each run under their own runRecordCAS; ALL proof — participant
// observation, process observation, and the launch walk — runs OUTSIDE any run
// lock, never holding a lock across a process observation or a per-drive claim
// probe.
package app

import (
	"fmt"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// processObserver observes whether one execution's OS process is provably TERMINAL,
// by its run directory — the observation-only counterpart of cancelStopper (which
// stops). Production appGateObserver observes through the app gate seam
// (process.Observe) and reuses the same proven-terminal rule the stop path uses
// (supervisorGone); a nil observer proves nothing (fail closed).
type processObserver interface {
	observeProcessTerminal(runDir string) (bool, error)
}

// runLaunchObserver accounts, for a completing run, every gate drive started inside
// the run (attributed by contextHash, the run-tracker record's child_context_hash)
// — the observation-only counterpart of runLaunchReconciler (which stops running
// supervisors and settles never-launched launches terminal). Production
// appLaunchObserver wraps gatedrive.Driver.ObserveRunLaunches; a nil observer
// proves nothing (fail closed).
type runLaunchObserver interface {
	observe(contextHash string) (gatedrive.RunLaunchReport, error)
}

// appGateObserver is the production processObserver: it observes a run through the
// app gate seam (process.Observe) and reports PROVEN teardown using supervisorGone
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
	return supervisorGone(obs.State), nil
}

// appLaunchObserver is the production runLaunchObserver: it composes a gatedrive
// driver over the completion store and the app gate seam's process service, then
// observes one run's drives through ObserveRunLaunches (observation
// only — it stops nothing and settles nothing). It resolves the process service per
// call, exactly as appLaunchReconciler.reconcile does. A nil store or an unresolvable
// process service proves nothing (fail closed).
type appLaunchObserver struct {
	store *gatedrive.Store
}

func (o appLaunchObserver) observe(contextHash string) (gatedrive.RunLaunchReport, error) {
	if o.store == nil {
		return gatedrive.RunLaunchReport{}, fmt.Errorf("gate store unavailable")
	}
	svc, _, reason := gateService()
	if svc == nil {
		return gatedrive.RunLaunchReport{}, fmt.Errorf("gate service unavailable: %s", reason)
	}
	return gatedrive.NewSystemDriver(o.store, svc).ObserveRunLaunches(contextHash)
}

// completeSuccessfulRun drives the whole successful-run ownership closeout over the
// injected seams, returning ok, a bounded reason token for the run-tracker-unavailable
// channel when ok is false (one of run-cancelled, stale-run-id,
// completion-unaccounted, completion-unpersisted, run-record-unreadable), and the bounded
// credential-free findings that name every unsettled obligation; findings may also
// carry informational mutation-settled:<op> tokens, even on a successful closeout.
// The caller (Task 8)
// has already resolved the confirmed claim binding and the run-complete verdict; this
// function owns only the run's closeout. See the file header for the
// observation-only, fail-closed, never-relabel, and lock-ordering contracts.
func completeSuccessfulRun(seams cancelSeams, repoDir, runKey string) (ok bool, reason string, findings []string) {
	// (1) Durable success fence: CAS active→completing. The observed state under the
	// lock decides a rejection's bounded reason — a cancelling/cancelled run is never
	// relabelled successful, a superseded run is a stale run, and a store fault is
	// unreadable. An already-completed run is an idempotent completed-receipt replay
	// (safe after scratch cleanup): success with no further work.
	observed, ferr := FenceRunCompleting(repoDir, runKey, "")
	if ferr != nil {
		if ee, ok := AsRunError(ferr); ok && ee.Kind == ErrRunNotActive {
			switch observed {
			case RunCancelling, RunCancelled:
				return false, "run-cancelled", nil
			case RunSuperseded:
				return false, "stale-run-id", nil
			default:
				return false, "run-record-unreadable", nil // an unknown/garbage state: fail closed
			}
		}
		return false, "run-record-unreadable", nil // IO/corrupt/not-found/mismatch: fail closed
	}
	if observed == RunCompleted {
		return true, "", nil // idempotent completed-receipt replay
	}

	// (1b) Settle uncertain publications proven by a later completed identical
	// retry (change 0444) — journal-derived evidence only, persisted through the
	// ordinary run CAS. The attributed keyed closeout is a WRITE path (unlike
	// RunVerify and unattributed verdicts, which stay read-only), but it still
	// stops no task, launches no mutation, and settles no never-launched
	// reservation: the only write is uncertain→completed on matched journal
	// entries. It runs before the step (2) reload so both the step (3) accounting
	// read and the step (4) re-enumeration read see the settled journal. A failed
	// settlement is a bounded finding; the entry stays uncertain and the
	// accounting below blocks fail-closed as before.
	settledTokens, sfindings := settleUncertainPublications(repoDir, runKey)
	findings = appendFindings(settledTokens, sfindings)

	// (2) Reload the fenced record. All remaining proof runs OUTSIDE the run lock.
	ep, _, lerr := LoadRunRecord(repoDir, runKey)
	if lerr != nil {
		return false, "run-record-unreadable", findings
	}

	// (3) Observation-only accounting over the fenced record. Any blocking obligation
	// (an unobserved native task, a live/unproven execution process, an unaccounted
	// launch, an uncompleted mutation) fails closed; informational findings (a
	// settled drive's run-terminal) are accounted. The step (1b) settlement tokens
	// stay ahead of the accounting findings.
	blocked, afindings := accountCompletionObligations(seams, repoDir, runKey, ep)
	findings = appendFindings(findings, afindings)

	// (4) RE-ENUMERATE before completing: an operation admitted pre-fence may have
	// appended a participant or a mutation between the fence and step (3)'s read (the
	// completing fence rejects only NEW registration). The reload also catches a
	// cancellation that won meanwhile — a state no longer completing loses to it.
	reEp, _, rlerr := LoadRunRecord(repoDir, runKey)
	if rlerr != nil {
		return false, "run-record-unreadable", findings
	}
	switch reEp.State {
	case RunCompleting:
		// still ours to close out
	case RunCompleted:
		return true, "", findings // a concurrent replay finished the closeout
	default:
		return false, "run-cancelled", findings // a cancellation won from completing
	}
	pblocked, pf := accountCompletionParticipants(seams, reEp)
	mblocked, mf := accountCompletionMutations(reEp)
	if pblocked || mblocked {
		blocked = true
	}
	// Step (3) already accounted the participant/mutation findings off the fenced
	// record; step (4) re-enumerates the reloaded record to catch a late pre-fence
	// append. A persistently-unsettled participant or mutation is therefore named by
	// both reads, so dedupe (order-preserving) before returning — the operator-facing
	// findings must not list the same token twice. This is cosmetic only: blocked is
	// computed independently, so fail-closed behavior is unchanged (any finding still
	// blocks).
	findings = dedupeFindings(appendFindings(findings, pf, mf))

	// (5) Blocked ⇒ the run stays durably completing; the remedy is to settle the
	// named evidence and repeat the same keyed verdict. No retry/budget/attempt moves.
	if blocked {
		return false, "completion-unaccounted", findings
	}

	// (6) Persist the terminal transition: CAS completing→completed. A failure because
	// a cancellation won is run-cancelled (completion loses without reporting success);
	// any other persistence failure is completion-unpersisted (fail closed, reportable).
	if cerr := CompleteRun(repoDir, runKey); cerr != nil {
		if cur, _, e := LoadRunRecord(repoDir, runKey); e == nil {
			switch cur.State {
			case RunCancelling, RunCancelled, RunSuperseded:
				return false, "run-cancelled", findings
			}
		}
		return false, "completion-unpersisted", findings
	}
	return true, "", findings
}

// accountCompletionObligations is step (3)'s full observation-only accounting over
// the fenced record: native participants, execution participants, the run's drives
// (attributed by the run context runKey's run-tracker record carries), and the
// mutation journal. It returns whether the run is blocked (any obligation unproven)
// and the accumulated bounded findings. It reads the run snapshot in memory and
// probes every process/launch OUTSIDE any lock.
func accountCompletionObligations(seams cancelSeams, repoDir, runKey string, ep RunRecord) (bool, []string) {
	blocked := false
	var findings []string

	pblocked, pf := accountCompletionParticipants(seams, ep)
	lblocked, lf := accountCompletionLaunches(seams, repoDir, runKey)
	mblocked, mf := accountCompletionMutations(ep)

	if pblocked || lblocked || mblocked {
		blocked = true
	}
	findings = appendFindings(findings, pf, lf, mf)
	return blocked, findings
}

// accountCompletionParticipants proves every registered participant terminal: a
// native task (coordinator/task) must carry recorded terminal evidence
// (TerminalStatus != "" — a terminal FAILURE still counts as observed; RunVerify
// independently decided implementation success), and an execution participant
// (gate-scope/raw-run) must observe a proven-terminal process or, when that
// observation fails, carry an exact durable execution proof
// (executionParticipantProof). Absent evidence is UNPROVEN and blocks
// (participant-unobserved / process-*).
func accountCompletionParticipants(seams cancelSeams, ep RunRecord) (bool, []string) {
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
			if proven, reason := executionParticipantProof(seams, p.NativeHandle); !proven {
				findings = append(findings, reason)
				blocked = true
			}
		}
	}
	return blocked, findings
}

// executionParticipantProof proves one execution participant's run terminal (change
// 0446 spec §5) through the observation-only seam — it never stops. A non-terminal
// (live/signalled) run is process-live:<handle>. Direct observation is tried first,
// so a run observed LIVE is never overridden by a record. When the observation
// itself FAILS — typically because the
// run's optional scratch directory was cleaned up after it finished — the durable
// fact already recorded is the sufficient proof: the one persisted PASSED/FAILED
// drive record naming the run (durableExecutionProof). Nothing is synthesized from
// the missing files: with no such record the participant stays process-unobserved
// and blocks. A nil observer is not missing evidence and still blocks
// (process-observer-unavailable).
func executionParticipantProof(seams cancelSeams, handle string) (bool, string) {
	if seams.observer == nil {
		return false, "process-observer-unavailable"
	}
	proven, err := seams.observer.observeProcessTerminal(handle)
	if err == nil {
		if !proven {
			return false, "process-live:" + handle
		}
		return true, ""
	}
	if durableExecutionProof(seams, handle) {
		return true, ""
	}
	return false, "process-unobserved:" + handle
}

// durableExecutionProof reports whether an existing durable record proves the
// execution at handle finished: the one readable drive whose current run is handle,
// with a persisted PASSED or FAILED outcome — the supervisor-committed completion
// evidence, keyed on the EXACT run. HALTED is never accepted (spec §4: it is a
// fail-closed label, not proof of teardown), nor is a nonterminal, missing,
// unreadable, or ambiguous drive.
func durableExecutionProof(seams cancelSeams, handle string) bool {
	if handle == "" || seams.store == nil {
		return false
	}
	outcome, found := seams.store.TerminalDriveOutcomeForRunDir(handle)
	return found && (outcome == gatedrive.PASSED || outcome == gatedrive.FAILED)
}

// accountCompletionMutations blocks on any admitted-not-completed mutation
// (mutation-pending:<op>), mirroring cancellation's mutation reconciliation.
func accountCompletionMutations(ep RunRecord) (bool, []string) {
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

// accountCompletionLaunches observes the run's drives through the observation-only
// launch seam, attributed by the run context hash runKey's run-tracker record
// carries (runContextHash). A nil observer proves nothing (fail closed,
// launch-observer-unavailable); an unreadable run context blocks
// (run-context-unreadable); an observation error blocks (launch-observe-failed); an
// unaccounted report blocks and surfaces its findings. An accounted report's
// informational findings are surfaced without blocking.
func accountCompletionLaunches(seams cancelSeams, repoDir, runKey string) (bool, []string) {
	if seams.launchObserver == nil {
		return true, []string{"launch-observer-unavailable"}
	}
	contextHash, cerr := runContextHash(repoDir, runKey)
	if cerr != nil {
		return true, []string{"run-context-unreadable"}
	}
	report, err := seams.launchObserver.observe(contextHash)
	if err != nil {
		return true, []string{"launch-observe-failed"}
	}
	if !report.Accounted {
		return true, report.Findings
	}
	return false, report.Findings
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

// dedupeFindings returns findings with duplicate tokens removed, preserving each
// token's first-occurrence order. The two-read completion accounting can name the
// same persistently-unsettled obligation twice (step (3) off the fenced record and
// step (4) off the reload); this keeps the operator-facing diagnostic free of that
// cosmetic noise without altering the fail-closed blocked decision.
func dedupeFindings(findings []string) []string {
	if len(findings) < 2 {
		return findings
	}
	seen := make(map[string]struct{}, len(findings))
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}
