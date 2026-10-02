// The `docket run cancel` operation (change 0375 Task 10): explicit, human-driven
// cancellation of one workflow implementation run. A child failure or an ordinary
// dispatch return NEVER invokes this — only an explicit human cancellation (or, in
// Task 13, a registered lifecycle event) does. Cancellation is the coordinator's
// authoritative Stop: it durably FENCES the run (runtracker_run_record.go) before any
// teardown, so once fenced no new participant, successor, resume claim, or
// mutation admission can attach to the run (gate starts are not fenced since
// change 0491; the census still accounts every drive the run's context names),
// and it reports `cancelled` ONLY after full accounting of registered tasks,
// processes and admitted mutations.
//
// AUTHORITY. The run key LOCATES the run (the durable gate record + the run that
// lives beside it); it does not authorize. Authorization is the set of conditions the spec
// pins: the record's repository must be the current repository (LoadRunTrackerRecord fails
// closed on wrong-repo), the presented run id must equal the record's public
// RunID, the record must carry a parent-held authority (a non-empty ParentCap),
// and a CONFIRMED claim binding for the run's change must exist (LoadRunTrackerClaimBinding)
// — or, for a record started by `run start --resume`, the resume-verified attribution
// (AttributedID set, no claim binding at all), the shape resolveRunTrackerOwnership accepts.
// Any missing/mismatched condition is a `refused` disposition with a bounded finding —
// never a fence, never a stop.
//
// ORDER (spec "Flow (exact order)"). validate key + load record + run; validate
// authority; CAS active→cancelling (the durable fence); cancel registered native
// tasks (Task 13's adapter hook — an absent adapter produces a FINDING, not silence);
// process.Stop each registered raw-run/gate participant; run the launch census over
// every gate drive started inside the run — attributed by the run context hash its
// run-tracker record carries (change 0490), with teardown proof "the supervisor is
// gone"; RE-ENUMERATE the run's participants after stopping (a launch admitted before
// the fence won and can register after the initial snapshot); reconcile
// AdmittedMutations (any admitted-not-completed entry keeps it pending); all
// accounted → CAS cancelling→cancelled (`cancelled`), else `cancellation-pending`.
// No worktree record is written: a gate's worktree lock is held by its supervisor
// and frees itself when that supervisor exits, so stopping the run's gates is all
// the teardown the worktree needs. A repeat against a cancelling run RESUMES cleanup
// without restoring authority; against a cancelled/superseded run it re-runs the
// census for that run's own context hash plus the journal check
// (repairTerminalRun): `already-cancelled` when the run is quiescent, and a
// `refused` finding when it is not (never `cancellation-pending` over durable
// terminal state). Completed work is never rolled back.
//
// ACCOUNTING (spec). Cancellation charges NO full-suite attempt and resets NO
// deadline/relaunch/budget/retry state: RunCancel writes only the run record and
// the run's own drive records (a never-launched launch settled HALTED) — never the
// change-owned suite budget or the run-tracker retry markers. The
// WAITING/PASSED/FAILED/HALTED outcome vocabulary is not widened; cancellation is
// never a test failure or a retry permission.
package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// OperationRunCancel is the operation key `run cancel` records in its envelope.
const OperationRunCancel = "run.cancel"

// The cancellation dispositions. Each is a produced report line, never a re-spelling
// of a test-outcome word.
const (
	// CancelDispositionCancelled: the run was fenced and cancellation completed
	// with full accounting — every process torn down (proven), every gate drive of
	// the run settled with its supervisor gone, and no admitted-not-completed
	// mutation.
	CancelDispositionCancelled = "cancelled"
	// CancelDispositionAlreadyCancelled: the run was already cancelled (or
	// superseded) — idempotent, nothing to do.
	CancelDispositionAlreadyCancelled = "already-cancelled"
	// CancelDispositionPending: the run is fenced (durably cancelling) but full
	// accounting is not yet reached — an unproven stop, an unaccounted racing
	// participant, or an admitted-not-completed mutation. Repeatable: a later
	// run.cancel resumes cleanup without restoring authority.
	CancelDispositionPending = "cancellation-pending"
	// CancelDispositionRefused: authority could not be validated — a wrong repo,
	// a mismatched run, an unconfirmed claim, or a missing parent authority. No
	// fence, no stop.
	CancelDispositionRefused = "refused"
)

// The participant kinds RunCancel dispatches on, matching runtracker_run_record.go's
// RunParticipant.Kind vocabulary: coordinator/task are NATIVE tasks (a Codex
// thread/turn) cancelled through the adapter hook; gate-scope/raw-run are EXECUTION
// participants whose OS process is stopped through the app gate seam.
const (
	participantKindCoordinator = "coordinator"
	participantKindTask        = "task"
	participantKindGateScope   = "gate-scope"
	participantKindRawRun      = "raw-run"
)

// mutationStatusCompleted is the only AdmittedMutation status that accounts as done.
// Any other status (admitted, uncertain) is an admitted-not-completed entry that
// keeps a cancellation pending (spec "reconcile AdmittedMutations").
const mutationStatusCompleted = "completed"

// RunCancelResult is the protocol-v1 document `run cancel` returns. It renders one
// report line and always exits 0 for a produced disposition; a refusal is a blocked
// result. Findings are bounded, credential-free strings (unproven participant or
// transition names) — never argv, environment, child output, a reservation token, or
// a child capability.
type RunCancelResult struct {
	Envelope
	Disposition string   `json:"disposition,omitempty" docket:"enum=cancel_dispositions"`
	Findings    []string `json:"findings,omitempty"`
}

// HumanText renders the disposition line followed by one line per finding. It never
// emits a credential — Findings carry only bounded safe locators.
func (r RunCancelResult) HumanText() string {
	lines := []string{"run-cancel " + r.Disposition}
	for _, f := range r.Findings {
		lines = append(lines, "finding: "+f)
	}
	return strings.Join(lines, "\n")
}

// cancelResult stamps the envelope for a produced disposition, mapping the
// disposition to the v1 result taxonomy: cancelled/pending are applied (exit 0),
// already-cancelled is a no-op, refused is blocked.
func cancelResult(disposition string, findings []string) RunCancelResult {
	var result Result
	switch disposition {
	case CancelDispositionCancelled, CancelDispositionPending:
		result = ResultApplied
	case CancelDispositionAlreadyCancelled:
		result = ResultNoOp
	default: // refused
		result = ResultBlocked
	}
	r := RunCancelResult{Disposition: disposition, Findings: findings}
	r.Envelope = NewEnvelope(OperationRunCancel, result)
	return r
}

// cancelRefused builds a refused disposition carrying a single bounded reason
// finding. It writes nothing durable — a refusal never fences and never stops.
func cancelRefused(reason string) RunCancelResult {
	return cancelResult(CancelDispositionRefused, []string{reason})
}

// cancelStopper stops one execution's OS process by its run directory and reports
// whether teardown is PROVEN — a stop this process performed, or a run already
// terminal-by-teardown. Production wraps the app gate seam (process.Stop); a nil
// stopper proves no teardown (fail closed).
type cancelStopper interface {
	stopProcess(runDir string) (proven bool, err error)
}

// nativeTaskCanceller cancels one registered native task (a Codex thread/turn) by
// its opaque handle — Task 13's adapter hook. A nil canceller is NOT silence:
// RunCancel records a bounded finding for every native participant it cannot cancel,
// and the authoritative teardown of that task's gates still flows through the launch
// census / execution-participant stop path.
type nativeTaskCanceller interface {
	cancelNativeTask(handle string) error
}

// runLaunchReconciler accounts, for an already-fenced run, every gate drive started
// inside the run — the drives whose run context hash equals contextHash, the run-
// tracker record's child_context_hash (runContextHash) — stopping a running
// supervisor and settling a never-launched launch (change 0437 Task 6; attribution
// by run context, change 0490). It wraps gatedrive.Driver.ReconcileRunLaunches. A
// nil reconciler is NOT silence: reconcileRunTeardown records a finding and fails
// closed (accounted=false), mirroring the nil-stopper rule.
type runLaunchReconciler interface {
	reconcile(contextHash string) (gatedrive.RunLaunchReport, error)
}

// runContextHash returns the run-tracker record's child_context_hash for runKey —
// the hash every drive started inside that run stores as run_context_hash, and so
// the census's attribution key. An empty hash (a record minted without a run
// context) names no drive.
func runContextHash(repoDir, runKey string) (string, error) {
	rec, err := LoadRunTrackerRecord(repoDir, runKey)
	if err != nil {
		return "", err
	}
	return rec.ChildContextHash, nil
}

// reconcileRunLaunchesFor runs the stop-capable launch census for the run under
// runKey and reports whether it is accounted plus its findings. A nil reconciler
// (launch-reconciler-unavailable), an unreadable run context
// (run-context-unreadable), and a census error (launch-reconcile-failed) each fail
// closed.
func reconcileRunLaunchesFor(seams cancelSeams, repoDir, runKey string) (bool, []string) {
	if seams.launches == nil {
		return false, []string{"launch-reconciler-unavailable"}
	}
	contextHash, err := runContextHash(repoDir, runKey)
	if err != nil {
		return false, []string{"run-context-unreadable"}
	}
	report, err := seams.launches.reconcile(contextHash)
	if err != nil {
		return false, []string{"launch-reconcile-failed"}
	}
	return report.Accounted, report.Findings
}

// cancelSeams bundles the injectable cancellation seams. Production composes them
// over the gatedrive store (the drive registry the census walks and the drive
// records the closeout's durable execution proof reads), the app gate seam
// (process.Stop), the launch reconciler (a gatedrive driver over the same store),
// and — from Task 13 — the native adapter; unit tests fake each one. A nil store
// proves no durable execution fact; a nil native canceller is the honest "no
// adapter" state that yields findings; a nil launch reconciler fails closed.
type cancelSeams struct {
	store    *gatedrive.Store
	stopper  cancelStopper
	native   nativeTaskCanceller
	launches runLaunchReconciler
	// observer and launchObserver are the STOP-FREE seams the successful-run
	// closeout (completeSuccessfulRun, change 0441) consumes: one shared seam bundle,
	// two flows — cancellation STOPS (stopper/native/launches), completion never
	// stops (observer/launchObserver). observer observes whether an execution's
	// process is proven-terminal without stopping it; launchObserver walks a run's
	// launch obligations in verdict mode: it stops nothing and settles only a proven
	// never-launched first launch (HALTED launch-abandoned, change 0491). A nil
	// observer/launchObserver proves nothing (fail closed), mirroring the
	// nil-stopper/nil-reconciler rule.
	observer       processObserver
	launchObserver runLaunchObserver
}

// RunCancel is the public `run cancel` entry. deps and wdeps are accepted for the
// stable operation signature and are reserved for the mutation-boundary
// reconciliation later tasks wire (Task 11); Task 10 reconciles from the run's own
// durable journal and needs neither. It composes the production cancellation seams
// from repoDir and delegates to runCancel, which owns the whole flow.
func RunCancel(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir, key, expectRunID, reason string) RunCancelResult {
	return runCancel(productionCancelSeams(repoDir), repoDir, key, expectRunID, reason)
}

// productionCancelSeams composes the real cancellation seams for repoDir: the
// gatedrive store at the repository's Git common dir, the app-gate-seam process
// stopper, and (until Task 13) a nil native adapter — so a native participant
// yields an honest finding rather than a fabricated cancellation. When the common
// dir cannot be resolved the store is nil; runCancel never reaches the census in
// that case because LoadRunTrackerRecord has already failed closed for the same
// reason.
func productionCancelSeams(repoDir string) cancelSeams {
	common, err := runTrackerGitCommonDir(repoDir)
	if err != nil {
		return cancelSeams{stopper: appGateStopper{}}
	}
	store := gatedrive.OpenStore(common)
	return cancelSeams{
		store:    store,
		stopper:  appGateStopper{},
		native:   nil, // Task 13 wires the native adapter hook.
		launches: appLaunchReconciler{store: store},
		// The observation-only seams the successful-run closeout consumes (change 0441):
		// a nil pairing would fail closed, so both are wired for the completion path.
		observer:       appGateObserver{},
		launchObserver: appLaunchObserver{store: store},
	}
}

// appLaunchReconciler is the production runLaunchReconciler: it composes a gatedrive
// driver over the cancellation store and the app gate seam's process service, then
// reconciles one run's drives through ReconcileRunLaunches. Reconcile is
// teardown, not admission, and takes no run lock. A nil store or an unresolvable
// process service proves nothing (fail closed): reconcile returns an error the caller turns into a finding +
// accounted=false, mirroring the nil-stopper rule. It resolves the process service
// per call, exactly as appGateStopper does.
type appLaunchReconciler struct {
	store *gatedrive.Store
}

func (r appLaunchReconciler) reconcile(contextHash string) (gatedrive.RunLaunchReport, error) {
	if r.store == nil {
		return gatedrive.RunLaunchReport{}, fmt.Errorf("gate store unavailable")
	}
	svc, _, reason := gateService()
	if svc == nil {
		return gatedrive.RunLaunchReport{}, fmt.Errorf("gate service unavailable: %s", reason)
	}
	return gatedrive.NewSystemDriver(r.store, svc).ReconcileRunLaunches(contextHash)
}

// appGateStopper is the production cancelStopper: it reports PROVEN teardown under
// the lock model's proof, "the supervisor is gone" (supervisorGone: any state but
// running), exactly as the launch census proves a drive's run dirs. A run already
// observed gone needs no stop — the ownership-gated process.Stop refuses to signal a
// group whose supervisor it can no longer prove alive, so stopping a vanished run
// would only fail. Otherwise it drives process.Stop through the app gate seam and
// accepts a performed stop or a resulting state that proves the supervisor gone; an
// observe that fails falls through to the stop, which fails closed. It carries no
// credential into the stop reason.
type appGateStopper struct{}

func (appGateStopper) stopProcess(runDir string) (bool, error) {
	svc, _, reason := gateService()
	if svc == nil {
		return false, fmt.Errorf("gate service unavailable: %s", reason)
	}
	if obs, oerr := svc.Observe(runDir); oerr == nil && obs != nil && obs.State.SupervisorExited() {
		return true, nil
	}
	out, err := svc.Stop(runDir, "run.cancel: stopping a cancelled run's process")
	if err != nil {
		return false, err
	}
	return out.Performed || out.State.SupervisorExited(), nil
}

// runCancel drives the whole cancellation flow in the spec's exact order over the
// injected seams. See the file header for authority, order, and accounting.
func runCancel(seams cancelSeams, repoDir, key, expectRunID, reason string) RunCancelResult {
	// A human reason is required (the CLI enforces it too); an empty reason is a
	// usage refusal, never a silent cancellation.
	if strings.TrimSpace(reason) == "" {
		return cancelRefused("reason-required")
	}

	// (1) Validate key shape + load the run-tracker record. The load carries the
	// REPOSITORY authority: a record whose Repo does not name this repository's
	// canonical common dir (wrong-repo), a malformed key, or an absent record all
	// fail closed here — the key locates nothing to cancel.
	rec, err := LoadRunTrackerRecord(repoDir, key)
	if err != nil {
		return cancelRefused(runTrackerStoreReason(err))
	}

	// (1b) Load the run that lives beside the run-tracker record. A missing or corrupt
	// run is refused — there is no run fence to drive.
	ep, _, err := LoadRunRecord(repoDir, key)
	if err != nil {
		return cancelRefused(cancelRunReason(err))
	}

	// (2) Validate the remaining authority conditions: the presented run id must be
	// the record's public RunID (a stale locator confers nothing); the record must
	// carry a parent-held authority; a CONFIRMED claim binding for the run's change
	// must exist.
	if expectRunID == "" || ep.RunID != expectRunID {
		return cancelRefused("run-id-mismatch")
	}
	if rec.ParentCap == "" {
		return cancelRefused("authority-unavailable")
	}
	binding, ok, berr := LoadRunTrackerClaimBinding(repoDir, key)
	if berr != nil {
		return cancelRefused("claim-unreadable")
	}
	ownerID := 0
	switch {
	case ok && binding.Confirmed:
		ownerID = binding.ChangeID
	case !ok && rec.resumeAttributed():
		// Resume-verified authority (change 0463): `run start --resume` pre-binds
		// AttributedID through WorkspaceInspect identity and never gets a claim binding
		// (change.claim requires a proposed change). It is the same shape
		// resolveRunTrackerOwnership accepts as ownership. Without it, the run a resume start
		// mints could never be cancelled, and the next resume would refuse
		// resume-active-run with a remedy that always refuses. Only a record with NO
		// binding file qualifies: a reservation that exists but is unconfirmed still
		// refuses below.
		ownerID = rec.AttributedID
	default:
		return cancelRefused("claim-unconfirmed")
	}
	if ep.ChangeID != "" && strconv.Itoa(ownerID) != ep.ChangeID {
		return cancelRefused("claim-mismatch")
	}

	// (3) State gate + the durable fence. A terminal run is already-cancelled; an
	// active run is fenced active→cancelling here; a cancelling run is a repeat
	// that resumes cleanup WITHOUT restoring authority (no re-fence, no state
	// restore).
	switch ep.State {
	case RunCancelled, RunSuperseded:
		// A terminal run by itself is not proof of quiescence: re-run the census for
		// the run's own context hash and the journal check over the records already
		// present. Authority was validated above; repairTerminalRun never revives the
		// run, replays a mutation, resets a budget, regresses terminal state, or stops
		// a replacement's process (a replacement run carries its own context hash).
		return repairTerminalRun(seams, repoDir, ep)
	case RunActive, RunCompleting:
		// An explicit human cancellation WINS even from a completing (successful,
		// mid-closeout) run (change 0441): fence active/completing→cancelling and run
		// the existing teardown/accounting unchanged. Completion then loses without
		// reporting success — its completing→completed CAS refuses once this fence
		// lands. A concurrent cancel that already fenced it leaves it cancelling: not an
		// error — cleanup simply resumes.
		if ferr := runRecordCAS(repoDir, key, func(r *RunRecord) error {
			if r.State == RunActive || r.State == RunCompleting {
				r.State = RunCancelling
			}
			return nil
		}); ferr != nil {
			return cancelRefused("fence-failed")
		}
	case RunCompleted:
		// A completed (successfully closed-out) run cannot be cancelled (change 0441):
		// a no-op refusal with the completed-run explanation, never a state regression
		// and never cancellation-pending over durable terminal state.
		return cancelRefused("run-completed")
	case RunCancelling:
		// Repeat: resume cleanup on the already-fenced run.
	default:
		return cancelRefused("run-state-unknown")
	}

	// Reload after the fence so the cleanup below reads the fenced record.
	ep, _, err = LoadRunRecord(repoDir, key)
	if err != nil {
		return cancelRefused(cancelRunReason(err))
	}

	// (4)–(7) Teardown accounting over the fenced run: cancel native tasks, stop
	// execution participants, stop the run's gate drives through the launch census,
	// re-enumerate to catch a launch admitted before the fence, and reconcile the
	// mutation journal.
	accounted, findings, terr := reconcileRunTeardown(seams, repoDir, key, ep)
	if terr != nil {
		return cancelRefused(cancelRunReason(terr))
	}

	// (8) Verdict. Not fully accounted → cancellation-pending (durable fence held,
	// repeatable). Fully accounted → CAS cancelling→cancelled. A failed final write
	// leaves the run fenced cancelling — pending, never refused and never a rollback —
	// and a retry revalidates the proof and finishes the transition.
	if !accounted {
		return cancelResult(CancelDispositionPending, findings)
	}
	if ferr := runRecordCAS(repoDir, key, func(r *RunRecord) error {
		if r.State == RunCancelling {
			r.State = RunCancelled
		}
		return nil
	}); ferr != nil {
		findings = append(findings, "finalize-unpersisted")
		return cancelResult(CancelDispositionPending, findings)
	}
	return cancelResult(CancelDispositionCancelled, findings)
}

// verifyTerminalRunQuiescence revalidates a terminal (cancelled/superseded)
// run's EXISTING launch and mutation evidence using the same bounded accounting
// cancellation uses — the launch census plus the admitted-mutation journal
// (reconcileRunTeardown's steps (5c) and (7)). The census is attributed by the
// run's OWN context hash (its run key's run-tracker record), so a superseded run's
// drives are found without the worktree supersession cleared, and a replacement
// run's drives — which carry the replacement's own context hash — are never
// touched. It fails closed: an absent or erroring reconciler, an unreadable run
// context, an unaccounted launch obligation (a busy claim, an unresolved launch, a
// supervisor still running after the stop), or an admitted-not-completed mutation
// is non-quiescence with a bounded finding. It writes no run record, and it does
// not re-prove participants, which terminal repair deliberately does not
// re-enumerate.
func verifyTerminalRunQuiescence(seams cancelSeams, repoDir string, ep RunRecord) (bool, []string) {
	quiescent, findings := reconcileRunLaunchesFor(seams, repoDir, ep.RunKey)
	for _, m := range ep.AdmittedMutations {
		if m.Status != mutationStatusCompleted {
			findings = append(findings, "mutation-pending:"+m.OpKey)
			quiescent = false
		}
	}
	return quiescent, findings
}

// repairTerminalRun is what a repeat run.cancel performs against a DURABLY
// terminal (cancelled/superseded) run: it re-proves quiescence
// (verifyTerminalRunQuiescence — the census for the run's own context hash, which
// stops any of the run's gates still running, plus the journal check). A quiescent
// run is already-cancelled (its informational findings surfaced); an unsafe or
// unverifiable history is refused with its specific findings — never
// cancellation-pending over durable terminal state, never a regression to
// cancelling, never a revived run. It never writes the run record.
func repairTerminalRun(seams cancelSeams, repoDir string, ep RunRecord) RunCancelResult {
	quiescent, findings := verifyTerminalRunQuiescence(seams, repoDir, ep)
	if !quiescent {
		return cancelResult(CancelDispositionRefused, findings)
	}
	return cancelResult(CancelDispositionAlreadyCancelled, findings)
}

// reconcileRunTeardown performs the cancellation teardown accounting for an
// already-FENCED run — the spec's flow steps (4)–(7): cancel registered native
// tasks through the adapter hook (an absent adapter is a bounded FINDING, not
// silence), stop each registered execution participant, stop the run's gate drives
// through the launch census, RE-ENUMERATE the participants after stopping (a launch
// admitted before the fence won and can register after the first snapshot), settle
// uncertain publications a later verified identical retry proves (change 0444), and
// reconcile the admitted-mutation journal (an admitted-not-completed entry keeps
// the run pending). It returns whether the run is fully accounted, the bounded
// credential-free findings, and a non-nil err only for a run re-read fault.
//
// It NEVER validates authority and NEVER transitions the run (its only run write
// is settleUncertainPublications' uncertain→completed flip of retry-proven journal
// entries, which leaves the run state untouched): the caller fences
// first — run.cancel under the authority conditions, or the detached death
// guardian on abrupt owner death — and finalizes cancelling→cancelled after. Both
// callers share this one accounting so the two fencing authorities reconcile a run
// identically.
func reconcileRunTeardown(seams cancelSeams, repoDir, runKey string, ep RunRecord) (accounted bool, findings []string, err error) {
	accounted = true

	// (4) Cancel registered native tasks through the adapter hook. An absent adapter
	// is a FINDING, not silence — the task's process teardown is still accounted by
	// the execution-participant stop and the launch census below.
	for _, p := range ep.Participants {
		if !isNativeParticipant(p.Kind) {
			continue
		}
		if seams.native == nil {
			findings = append(findings, "native-cancel-unavailable:"+p.Kind)
			continue
		}
		if cerr := seams.native.cancelNativeTask(p.NativeHandle); cerr != nil {
			findings = append(findings, "native-cancel-failed:"+p.Kind)
		}
	}

	// (5) Stop each registered raw-run/gate participant: process.Stop the
	// participant's run. Track which handles proved teardown so re-enumeration can
	// tell an accounted stop from a racing arrival.
	proven := map[string]bool{}
	for _, p := range ep.Participants {
		if !isExecutionParticipant(p.Kind) {
			continue
		}
		if stopParticipantProcess(seams, p.NativeHandle) {
			proven[p.NativeHandle] = true
		} else {
			findings = append(findings, "stop-unproven:"+p.NativeHandle)
			accounted = false
		}
	}

	// (5c) Reconcile every gate drive started inside the run — attributed by the run
	// context hash its run-tracker record carries (change 0490): stop a running
	// supervisor (teardown proof is "the supervisor is gone"), settle a launch that
	// provably never launched, and keep a busy claim or an unprovable probe pending. A
	// nil reconciler or an unreadable run context is a FINDING and fails closed
	// (accounted=false), mirroring the nil-stopper rule; a census that reports
	// unsettled drives keeps the cancellation pending so a completed replacement can
	// never first appear afterward.
	launchesAccounted, lfindings := reconcileRunLaunchesFor(seams, repoDir, runKey)
	findings = append(findings, lfindings...)
	if !launchesAccounted {
		accounted = false
	}

	// (5d) Settle uncertain publications proven by a later completed identical
	// retry (change 0444) — a durable, journal-derived repair with NO Git or
	// GitHub call. Runs before the re-enumeration reload so steps (6)-(7)
	// evaluate the settled record. Shared by both fencing authorities
	// (run.cancel and the death guardian): settlement is observation of durable
	// journal fact, like the completion callback. A failed settlement write is
	// a bounded finding; the entry stays uncertain and step (7) keeps the
	// cancellation pending, so exclusion is retained until a repeat converges.
	settledTokens, sfindings := settleUncertainPublications(repoDir, runKey)
	findings = append(findings, settledTokens...)
	findings = append(findings, sfindings...)

	// (6) RE-ENUMERATE after stopping: a launch admitted before the fence won and can
	// register a participant after the snapshot in (5). Any execution participant not
	// proven-stopped in this pass is unaccounted — a repeat resumes its cleanup.
	reEp, _, rerr := LoadRunRecord(repoDir, runKey)
	if rerr != nil {
		return false, findings, rerr
	}
	for _, p := range reEp.Participants {
		if isExecutionParticipant(p.Kind) && !proven[p.NativeHandle] {
			findings = append(findings, "unaccounted-participant:"+p.NativeHandle)
			accounted = false
		}
	}

	// (7) Reconcile admitted mutations from the re-enumerated (post-settlement)
	// journal: any admitted-not-completed entry (in-flight, or uncertain with no
	// verified identical retry) keeps cancellation pending so a premature
	// `cancelled` never claims a mutation is done.
	for _, m := range reEp.AdmittedMutations {
		if m.Status != mutationStatusCompleted {
			findings = append(findings, "mutation-pending:"+m.OpKey)
			accounted = false
		}
	}

	return accounted, findings, nil
}

// isNativeParticipant reports whether a participant kind names a NATIVE task (a
// coordinator or worker task whose cancellation flows through the adapter hook).
func isNativeParticipant(kind string) bool {
	return kind == participantKindCoordinator || kind == participantKindTask
}

// isExecutionParticipant reports whether a participant kind names an EXECUTION
// participant (a gate scope or raw run whose OS process is stopped through the app
// gate seam).
func isExecutionParticipant(kind string) bool {
	return kind == participantKindGateScope || kind == participantKindRawRun
}

// stopParticipantProcess stops one execution participant's run through the stopper
// seam and reports whether teardown is PROVEN. An empty handle or a nil stopper
// proves nothing (fail closed).
func stopParticipantProcess(seams cancelSeams, handle string) bool {
	if handle == "" || seams.stopper == nil {
		return false
	}
	proven, err := seams.stopper.stopProcess(handle)
	return err == nil && proven
}

// cancelRunReason maps a run-store load error to a bounded refusal reason
// token — the RunError kind when one is present, else a generic fallback.
func cancelRunReason(err error) string {
	if ee, ok := AsRunError(err); ok {
		return string(ee.Kind)
	}
	return "run-record-unreadable"
}

// Compile-time seam assertions.
var (
	_ cancelStopper       = appGateStopper{}
	_ runLaunchReconciler = appLaunchReconciler{}
	_ processObserver     = appGateObserver{}
	_ runLaunchObserver   = appLaunchObserver{}
	_ OperationResult     = RunCancelResult{}
)
