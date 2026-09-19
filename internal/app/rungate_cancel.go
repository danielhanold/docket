// The `docket run cancel` operation (change 0375 Task 10): explicit, human-driven
// cancellation of one workflow implementation run. A child failure or an ordinary
// dispatch return NEVER invokes this — only an explicit human cancellation (or, in
// Task 13, a registered lifecycle event) does. Cancellation is the coordinator's
// authoritative Stop: it durably FENCES the run epoch (rungate_epoch.go) before any
// teardown, so once fenced no new participant, start, relaunch, successor, resume
// claim, or mutation admission can attach to the epoch, and it reports `cancelled`
// ONLY after full accounting of registered tasks, processes and admitted mutations.
//
// AUTHORITY. The gate key LOCATES the run (the durable gate record + the epoch that
// lives beside it); it does not authorize. Authorization is the conjunction the spec
// pins: the record's repository must be the current repository (LoadGateRecord fails
// closed on wrong-repo), the presented epoch id must equal the record's public
// EpochID, the record must carry a parent-held authority (a non-empty ParentCap),
// and a CONFIRMED claim binding for the epoch's change must exist (LoadGateClaimBinding).
// Any missing/mismatched conjunct is a `refused` disposition with a bounded finding —
// never a fence, never a stop.
//
// ORDER (spec "Flow (exact order)"). validate key + load record + epoch; validate
// authority; CAS active→cancelling (the durable fence); cancel registered native
// tasks (Task 13's adapter hook — an absent adapter produces a FINDING, not silence);
// for each registered raw-run/gate participant mark the worktree slot stopping and
// process.Stop it; RE-ENUMERATE the epoch's participants and the worktree admission
// record after stopping (a launch admitted before the fence won and can register
// after the initial snapshot); reconcile AdmittedMutations (any admitted-not-completed
// entry keeps it pending); all accounted → CAS cancelling→cancelled and release
// proven slots (`cancelled`), else `cancellation-pending`. A repeat against a
// cancelling epoch RESUMES cleanup without restoring authority; against a
// cancelled/superseded epoch it is `already-cancelled`. Completed work is never
// rolled back.
//
// ACCOUNTING (spec). Cancellation charges NO full-suite attempt and resets NO
// deadline/relaunch/budget/retry state: RunCancel touches only the epoch record and
// the worktree admission slot — never the change-owned suite budget or the gate
// retry markers. The WAITING/PASSED/FAILED/HALTED outcome vocabulary is not widened;
// cancellation is never a test failure or a retry permission.
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
	// CancelDispositionCancelled: the epoch was fenced and cancellation completed
	// with full accounting — every process torn down (proven), the worktree slot
	// released, and no admitted-not-completed mutation.
	CancelDispositionCancelled = "cancelled"
	// CancelDispositionAlreadyCancelled: the epoch was already cancelled (or
	// superseded) — idempotent, nothing to do.
	CancelDispositionAlreadyCancelled = "already-cancelled"
	// CancelDispositionPending: the epoch is fenced (durably cancelling) but full
	// accounting is not yet reached — an unproven stop, an unaccounted racing
	// participant, or an admitted-not-completed mutation. Repeatable: a later
	// run.cancel resumes cleanup without restoring authority.
	CancelDispositionPending = "cancellation-pending"
	// CancelDispositionRefused: authority could not be validated — a wrong repo,
	// a mismatched epoch, an unconfirmed claim, or a missing parent authority. No
	// fence, no stop.
	CancelDispositionRefused = "refused"
)

// The participant kinds RunCancel dispatches on, matching rungate_epoch.go's
// EpochParticipant.Kind vocabulary: coordinator/task are NATIVE tasks (a Codex
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
// and the authoritative teardown of that task's process still flows through the
// worktree slot / execution-participant stop path.
type nativeTaskCanceller interface {
	cancelNativeTask(handle string) error
}

// epochLaunchReconciler accounts, for an already-fenced epoch, the pending and
// replacement LAUNCH obligations the durable drive records name — a reserved-but-
// unlaunched drive, a busy launch claim, or a relaunch replacement the worktree slot
// still records the predecessor for — that the participant/slot teardown above cannot
// see (change 0437 Task 6). It wraps gatedrive.Driver.ReconcileEpochLaunches. A nil
// reconciler is NOT silence: reconcileEpochTeardown records a finding and fails
// closed (accounted=false), mirroring the nil-stopper rule.
type epochLaunchReconciler interface {
	reconcile(worktree, epochID string) (gatedrive.EpochLaunchReport, error)
}

// cancelSeams bundles the injectable cancellation seams. Production composes them
// over the gatedrive admission store, the app gate seam (process.Stop), the launch
// reconciler (a gatedrive driver over the same store), and — from Task 13 — the
// native adapter; unit tests fake each one. A nil store means no worktree slot to
// reconcile (a keyless/standalone run); a nil native canceller is the honest "no
// adapter" state that yields findings; a nil launch reconciler fails closed.
type cancelSeams struct {
	store    *gatedrive.Store
	stopper  cancelStopper
	native   nativeTaskCanceller
	launches epochLaunchReconciler
}

// RunCancel is the public `run cancel` entry. deps and wdeps are accepted for the
// stable operation signature and are reserved for the mutation-boundary
// reconciliation later tasks wire (Task 11); Task 10 reconciles from the epoch's own
// durable journal and needs neither. It composes the production cancellation seams
// from repoDir and delegates to runCancel, which owns the whole flow.
func RunCancel(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir, key, expectEpoch, reason string) RunCancelResult {
	return runCancel(productionCancelSeams(repoDir), repoDir, key, expectEpoch, reason)
}

// productionCancelSeams composes the real cancellation seams for repoDir: the
// gatedrive admission store at the repository's Git common dir, the app-gate-seam
// process stopper, and (until Task 13) a nil native adapter — so a native
// participant yields an honest finding rather than a fabricated cancellation. When
// the common dir cannot be resolved the store is nil; runCancel never reaches the
// slot path in that case because LoadGateRecord has already failed closed for the
// same reason.
func productionCancelSeams(repoDir string) cancelSeams {
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		return cancelSeams{stopper: appGateStopper{}}
	}
	store := gatedrive.OpenStore(common)
	return cancelSeams{
		store:    store,
		stopper:  appGateStopper{},
		native:   nil, // Task 13 wires the native adapter hook.
		launches: appLaunchReconciler{store: store},
	}
}

// appLaunchReconciler is the production epochLaunchReconciler: it composes a gatedrive
// driver over the cancellation store and the app gate seam's process service, then
// reconciles one epoch's launch obligations through ReconcileEpochLaunches. The
// composed driver needs no epoch launch gate (reconcile is teardown, not admission,
// and takes no epoch lock). A nil store or an unresolvable process service proves
// nothing (fail closed): reconcile returns an error the caller turns into a finding +
// accounted=false, mirroring the nil-stopper rule. It resolves the process service
// per call, exactly as appGateStopper does.
type appLaunchReconciler struct {
	store *gatedrive.Store
}

func (r appLaunchReconciler) reconcile(worktree, epochID string) (gatedrive.EpochLaunchReport, error) {
	if r.store == nil {
		return gatedrive.EpochLaunchReport{}, fmt.Errorf("gate store unavailable")
	}
	svc, _, reason := gateService()
	if svc == nil {
		return gatedrive.EpochLaunchReport{}, fmt.Errorf("gate service unavailable: %s", reason)
	}
	return gatedrive.NewSystemDriver(r.store, svc).ReconcileEpochLaunches(worktree, epochID)
}

// appGateStopper is the production cancelStopper: it drives the ownership-gated
// process.Stop through the app gate seam and reports PROVEN teardown, reusing the
// same proof rule the raw-launch confirmation path uses (a performed stop, or a run
// state that proves the group is gone). It carries no credential into the stop
// reason.
type appGateStopper struct{}

func (appGateStopper) stopProcess(runDir string) (bool, error) {
	svc, _, reason := gateService()
	if svc == nil {
		return false, fmt.Errorf("gate service unavailable: %s", reason)
	}
	out, err := svc.Stop(runDir, "run.cancel: stopping a cancelled run's process")
	if err != nil {
		return false, err
	}
	return out.Performed || rawTeardownProven(out.State), nil
}

// runCancel drives the whole cancellation flow in the spec's exact order over the
// injected seams. See the file header for authority, order, and accounting.
func runCancel(seams cancelSeams, repoDir, key, expectEpoch, reason string) RunCancelResult {
	// A human reason is required (the CLI enforces it too); an empty reason is a
	// usage refusal, never a silent cancellation.
	if strings.TrimSpace(reason) == "" {
		return cancelRefused("reason-required")
	}

	// (1) Validate key shape + load the gate record. The load carries the
	// REPOSITORY authority: a record whose Repo does not name this repository's
	// canonical common dir (wrong-repo), a malformed key, or an absent record all
	// fail closed here — the key locates nothing to cancel.
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		return cancelRefused(gateStoreReason(err))
	}

	// (1b) Load the epoch that lives beside the gate record. A missing or corrupt
	// epoch is refused — there is no run fence to drive.
	ep, _, err := LoadEpochRecord(repoDir, key)
	if err != nil {
		return cancelRefused(cancelEpochReason(err))
	}

	// (2) Validate the remaining authority conjuncts: the presented epoch id must be
	// the record's public EpochID (a stale locator confers nothing); the record must
	// carry a parent-held authority; a CONFIRMED claim binding for the epoch's change
	// must exist.
	if expectEpoch == "" || ep.EpochID != expectEpoch {
		return cancelRefused("epoch-mismatch")
	}
	if rec.ParentCap == "" {
		return cancelRefused("authority-unavailable")
	}
	binding, ok, berr := LoadGateClaimBinding(repoDir, key)
	if berr != nil {
		return cancelRefused("claim-unreadable")
	}
	if !ok || !binding.Confirmed {
		return cancelRefused("claim-unconfirmed")
	}
	if ep.ChangeID != "" && strconv.Itoa(binding.ChangeID) != ep.ChangeID {
		return cancelRefused("claim-mismatch")
	}

	// (3) State gate + the durable fence. A terminal epoch is already-cancelled; an
	// active epoch is fenced active→cancelling here; a cancelling epoch is a repeat
	// that resumes cleanup WITHOUT restoring authority (no re-fence, no state
	// restore).
	switch ep.State {
	case EpochCancelled, EpochSuperseded:
		return cancelResult(CancelDispositionAlreadyCancelled, nil)
	case EpochActive:
		if ferr := epochCAS(repoDir, key, func(r *EpochRecord) error {
			if r.State == EpochActive {
				r.State = EpochCancelling
			}
			// A concurrent cancel that already fenced it leaves it cancelling: not an
			// error — cleanup simply resumes.
			return nil
		}); ferr != nil {
			return cancelRefused("fence-failed")
		}
	case EpochCancelling:
		// Repeat: resume cleanup on the already-fenced epoch.
	default:
		return cancelRefused("epoch-state-unknown")
	}

	// Reload after the fence so the cleanup below reads the fenced record.
	ep, _, err = LoadEpochRecord(repoDir, key)
	if err != nil {
		return cancelRefused(cancelEpochReason(err))
	}

	// (4)–(7) Teardown accounting over the fenced epoch: cancel native tasks, stop
	// execution participants and the worktree slot, re-enumerate to catch a launch
	// admitted before the fence, and reconcile the mutation journal.
	accounted, findings, terr := reconcileEpochTeardown(seams, repoDir, key, ep)
	if terr != nil {
		return cancelRefused(cancelEpochReason(terr))
	}

	// (8) Verdict. Not fully accounted → cancellation-pending (durable fence held,
	// repeatable). Fully accounted → CAS cancelling→cancelled (proven slots already
	// released above) → cancelled.
	if !accounted {
		return cancelResult(CancelDispositionPending, findings)
	}
	if ferr := epochCAS(repoDir, key, func(r *EpochRecord) error {
		if r.State == EpochCancelling {
			r.State = EpochCancelled
		}
		return nil
	}); ferr != nil {
		return cancelRefused("finalize-failed")
	}
	return cancelResult(CancelDispositionCancelled, findings)
}

// reconcileEpochTeardown performs the cancellation teardown accounting for an
// already-FENCED epoch — the spec's flow steps (4)–(7): cancel registered native
// tasks through the adapter hook (an absent adapter is a bounded FINDING, not
// silence), stop each registered execution participant and the worktree admission
// slot on proven teardown, RE-ENUMERATE the participants after stopping (a launch
// admitted before the fence won and can register after the first snapshot), and
// reconcile the admitted-mutation journal (an admitted-not-completed entry keeps
// the run pending). It returns whether the run is fully accounted, the bounded
// credential-free findings, and a non-nil err only for an epoch re-read fault.
//
// It NEVER validates authority and NEVER transitions the epoch: the caller fences
// first — run.cancel under the authority conjunction, or the detached death
// guardian on abrupt owner death — and finalizes cancelling→cancelled after. Both
// callers share this one accounting so the two fencing authorities reconcile a run
// identically.
func reconcileEpochTeardown(seams cancelSeams, repoDir, gateKey string, ep EpochRecord) (accounted bool, findings []string, err error) {
	accounted = true

	// (4) Cancel registered native tasks through the adapter hook. An absent adapter
	// is a FINDING, not silence — the task's process teardown is still accounted by
	// the execution-participant / worktree-slot stop below.
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

	// (5) Stop each registered raw-run/gate participant: mark the worktree slot
	// stopping and process.Stop the participant's run. Track which handles proved
	// teardown so re-enumeration can tell an accounted stop from a racing arrival.
	proven := map[string]bool{}
	for _, p := range ep.Participants {
		if !isExecutionParticipant(p.Kind) {
			continue
		}
		markWorktreeSlotStopping(seams, ep.Worktree)
		if stopParticipantProcess(seams, p.NativeHandle) {
			proven[p.NativeHandle] = true
		} else {
			findings = append(findings, "stop-unproven:"+p.NativeHandle)
			accounted = false
		}
	}

	// (5b) Reconcile the worktree admission slot itself — the top-level execution the
	// epoch owns. Marking stopping, stopping its process, and releasing on proven
	// teardown is the authoritative slot teardown; an unproven or unreadable slot
	// keeps cancellation pending (fail closed).
	slotAccounted, slotFinding := reconcileWorktreeSlot(seams, ep.Worktree)
	if slotFinding != "" {
		findings = append(findings, slotFinding)
	}
	if !slotAccounted {
		accounted = false
	}

	// (5c) Reconcile the epoch's pending and replacement LAUNCH obligations the durable
	// drive records name — a reserved-but-unlaunched drive, a busy launch claim, or a
	// relaunch replacement the worktree slot still records the predecessor for — that
	// the participant/slot teardown above cannot see (change 0437 Task 6). A nil or
	// unavailable reconciler is a FINDING and fails closed (accounted=false), mirroring
	// the nil-stopper rule; a reconciler that reports unsettled launches keeps the
	// cancellation pending so a completed replacement can never first appear afterward.
	if seams.launches == nil {
		findings = append(findings, "launch-reconciler-unavailable")
		accounted = false
	} else if report, rcerr := seams.launches.reconcile(ep.Worktree, ep.EpochID); rcerr != nil {
		findings = append(findings, "launch-reconcile-failed")
		accounted = false
	} else {
		findings = append(findings, report.Findings...)
		if !report.Accounted {
			accounted = false
		}
	}

	// (6) RE-ENUMERATE after stopping: a launch admitted before the fence won and can
	// register a participant after the snapshot in (5). Any execution participant not
	// proven-stopped in this pass is unaccounted — a repeat resumes its cleanup.
	reEp, _, rerr := LoadEpochRecord(repoDir, gateKey)
	if rerr != nil {
		return false, findings, rerr
	}
	for _, p := range reEp.Participants {
		if isExecutionParticipant(p.Kind) && !proven[p.NativeHandle] {
			findings = append(findings, "unaccounted-participant:"+p.NativeHandle)
			accounted = false
		}
	}

	// (7) Reconcile admitted mutations from the re-enumerated journal: any
	// admitted-not-completed entry (in-flight or uncertain) keeps cancellation
	// pending so a premature `cancelled` never claims a mutation is done.
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

// markWorktreeSlotStopping best-effort marks the epoch's worktree execution slot
// stopping before its process is stopped, using the slot's own persisted
// reservation token (RunCancel holds no token of its own). A missing store, empty
// worktree, unreadable slot, or already-terminal slot is left untouched — the
// authoritative release decision is reconcileWorktreeSlot's.
func markWorktreeSlotStopping(seams cancelSeams, worktree string) {
	if seams.store == nil || worktree == "" {
		return
	}
	slot, _, err := seams.store.LoadWorktreeExecution(worktree)
	if err != nil {
		return
	}
	switch string(slot.State) {
	case "reserved", "executing":
		_ = seams.store.MarkWorktreeExecutionStopping(worktree, slot.ReservationToken)
	}
}

// reconcileWorktreeSlot stops the epoch's worktree execution slot and releases it on
// PROVEN teardown. It returns whether the slot is accounted (released, absent, or
// already released) and a bounded finding when it is not. A nil store or empty
// worktree is vacuously accounted (a keyless/standalone run owns no slot); an
// unreadable-but-present slot fails closed to pending.
func reconcileWorktreeSlot(seams cancelSeams, worktree string) (accounted bool, finding string) {
	if seams.store == nil || worktree == "" {
		return true, ""
	}
	slot, _, err := seams.store.LoadWorktreeExecution(worktree)
	if err != nil {
		if se, ok := gatedrive.AsStoreError(err); ok && se.Kind == gatedrive.ErrNotFound {
			return true, "" // no slot to reconcile
		}
		return false, "slot-unreadable"
	}
	switch string(slot.State) {
	case "released":
		return true, ""
	case "reserved", "executing", "stopping":
		_ = seams.store.MarkWorktreeExecutionStopping(worktree, slot.ReservationToken)
		if slot.RawRunDir == "" {
			// A bare reservation with no launched process is not proven torn down
			// here; a later recovery resolves it. Fail closed to pending.
			return false, "slot-stop-unproven"
		}
		if stopParticipantProcess(seams, slot.RawRunDir) {
			_ = seams.store.ReleaseWorktreeExecution(worktree, slot.ReservationToken)
			return true, ""
		}
		return false, "slot-stop-unproven"
	default:
		return false, "slot-state-unknown"
	}
}

// cancelEpochReason maps an epoch-store load error to a bounded refusal reason
// token — the EpochError kind when one is present, else a generic fallback.
func cancelEpochReason(err error) string {
	if ee, ok := AsEpochError(err); ok {
		return string(ee.Kind)
	}
	return "epoch-unreadable"
}

// Compile-time seam assertions.
var (
	_ cancelStopper         = appGateStopper{}
	_ epochLaunchReconciler = appLaunchReconciler{}
	_ OperationResult       = RunCancelResult{}
)
