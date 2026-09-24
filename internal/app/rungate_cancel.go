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
// entry keeps it pending); all accounted → retire the epoch's released-slot
// ownership (RetireWorktreeExecutionEpoch), then CAS cancelling→cancelled and release
// proven slots (`cancelled`), else `cancellation-pending`. A repeat against a
// cancelling epoch RESUMES cleanup without restoring authority; against a
// cancelled/superseded epoch it performs the bounded historical repair
// (repairTerminalEpoch): quiescence-checked retirement of a stale released slot, an
// idempotent `already-cancelled` when there is nothing to repair, and a `refused`
// finding for an unsafe history (never `cancellation-pending` over durable terminal
// state). Completed work is never rolled back.
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
	// observer and launchObserver are the OBSERVATION-ONLY seams the successful-run
	// closeout (completeSuccessfulRun, change 0441) consumes: one shared seam bundle,
	// two flows — cancellation STOPS (stopper/native/launches), completion only
	// OBSERVES (observer/launchObserver). observer observes whether an execution's
	// process is proven-terminal without stopping it; launchObserver walks an epoch's
	// launch obligations without settling any. A nil observer/launchObserver proves
	// nothing (fail closed), mirroring the nil-stopper/nil-reconciler rule.
	observer       processObserver
	launchObserver epochLaunchObserver
	// retire overrides the slot epoch-retirement write (unit tests inject faults and
	// successor races); nil delegates to store.RetireWorktreeExecutionEpoch. A nil
	// store with a nil retire proves nothing (retireSlot fails closed).
	retire func(worktree, epoch, token string) error
}

// retireSlot performs the ownership-checked epoch retirement write through the
// seam, defaulting to the production store operation
// (RetireWorktreeExecutionEpoch). A nil store and a nil retire seam cannot prove a
// detachment, so it fails closed rather than reporting a false success.
func (s cancelSeams) retireSlot(worktree, epoch, token string) error {
	if s.retire != nil {
		return s.retire(worktree, epoch, token)
	}
	if s.store == nil {
		return fmt.Errorf("gate store unavailable")
	}
	return s.store.RetireWorktreeExecutionEpoch(worktree, epoch, token)
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
		// The observation-only seams the successful-run closeout consumes (change 0441):
		// a nil pairing would fail closed, so both are wired for the completion path.
		observer:       appGateObserver{},
		launchObserver: appLaunchObserver{store: store},
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
		// A terminal epoch by itself is not proof of quiescence: a pre-0435 cancel
		// released its slot but never retired the stale RunEpochID. Run the bounded
		// historical repair over the records already present. Authority was validated
		// above; repairTerminalEpoch never revives the epoch, replays a mutation, resets
		// a budget, regresses terminal state, or stops a replacement's process.
		return repairTerminalEpoch(seams, repoDir, ep)
	case EpochActive, EpochCompleting:
		// An explicit human cancellation WINS even from a completing (successful,
		// mid-closeout) epoch (change 0441): fence active/completing→cancelling and run
		// the existing teardown/accounting unchanged. Completion then loses without
		// reporting success — its completing→completed CAS refuses once this fence
		// lands. A concurrent cancel that already fenced it leaves it cancelling: not an
		// error — cleanup simply resumes.
		if ferr := epochCAS(repoDir, key, func(r *EpochRecord) error {
			if r.State == EpochActive || r.State == EpochCompleting {
				r.State = EpochCancelling
			}
			return nil
		}); ferr != nil {
			return cancelRefused("fence-failed")
		}
	case EpochCompleted:
		// A completed (successfully closed-out) run cannot be cancelled (change 0441):
		// a no-op refusal with the completed-run explanation, never a state regression
		// and never cancellation-pending over durable terminal state.
		return cancelRefused("run-completed")
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
	// repeatable). Fully accounted → retire the epoch's released-slot ownership UNDER
	// THE ADMISSION LOCK (retireWorktreeSlotOwnership → RetireWorktreeExecutionEpoch),
	// and only then CAS cancelling→cancelled. These are two existing records, not a
	// transaction: a failure BEFORE retirement leaves ownership intact and
	// cancellation pending; a failure AFTER safe detachment (all launch, execution,
	// and mutation obligations settled first) leaves the epoch fenced cancelling —
	// also pending, never refused and never a rollback — and a retry revalidates the
	// proof, accepts the already-detached slot, and finishes the transition.
	if !accounted {
		return cancelResult(CancelDispositionPending, findings)
	}
	retired, rfinding := retireWorktreeSlotOwnership(seams, ep)
	if rfinding != "" {
		findings = append(findings, rfinding)
	}
	if !retired {
		return cancelResult(CancelDispositionPending, findings)
	}
	if ferr := epochCAS(repoDir, key, func(r *EpochRecord) error {
		if r.State == EpochCancelling {
			r.State = EpochCancelled
		}
		return nil
	}); ferr != nil {
		findings = append(findings, "finalize-unpersisted")
		return cancelResult(CancelDispositionPending, findings)
	}
	return cancelResult(CancelDispositionCancelled, findings)
}

// slotRetirement is the one outcome shape of the shared slot retirement
// (retireSlotOwnership). detached reports that the epoch's ownership of the slot is
// accounted detached — retired now, already detached, absent, epoch-less, or held by
// a successor; retired reports that THIS call wrote the retirement (terminal repair
// distinguishes an applied repair from an idempotent no-op on it); finding is the
// bounded, credential-free finding, when any.
type slotRetirement struct {
	detached bool
	retired  bool
	finding  string
}

// retireWorktreeSlotOwnership retires the epoch's ownership of its RELEASED worktree
// slot and reports (detached, finding) — the cancellation-specific detachment
// ordinary execution release never performs (ordinary ReleaseWorktreeExecution
// retains RunEpochID for between-drives ownership). It is the single retirement
// implementation (change 0446 spec §4): runCancel's completion, successful-run
// closeout, repairTerminalEpoch, and validateResumeQuiescence all retire through
// retireSlotOwnership, so a successor holding the slot yields one defined outcome at
// every site. It runs ONLY after complete accounting, inside the authorized
// completion decision.
func retireWorktreeSlotOwnership(seams cancelSeams, ep EpochRecord) (bool, string) {
	r := retireSlotOwnership(seams, ep)
	return r.detached, r.finding
}

// retireSlotOwnership is the shared body behind every retirement site. It touches
// ONLY a slot this epoch owns (classifySlotOwnership: a different nonempty RunEpochID
// is a foreign owner, never cleared, never retried with its token), and only once
// that slot is released. Outcomes:
//   - a nil store or an empty ep.Worktree (a keyless/standalone run owns no slot), an
//     absent slot, or an epoch-less slot (linked or not: it carries no RunEpochID
//     ownership field to retire) → detached;
//   - a slot a different nonempty RunEpochID holds — whether the successor already
//     held it or won the retirement CAS race (the re-read below) — → detached with
//     the one defined successor finding "slot-replaced-by-successor", the successor
//     untouched;
//   - an owned slot that is not released → not detached, "slot-not-released";
//   - an unreadable slot → not detached, "slot-unreadable";
//   - a refused retirement CAS is re-read ONCE: an already-cleared field (a
//     concurrent replay's retirement) is detached, a successor is the successor
//     outcome, anything else is not detached, "slot-retire-failed".
//
// A superseded epoch's cleared Worktree is resolved to its replacement's worktree by
// the caller (resolveTerminalEpochSlot) before this runs, so the predecessor's stale
// ownership of that slot is retired here too.
func retireSlotOwnership(seams cancelSeams, ep EpochRecord) slotRetirement {
	if seams.store == nil || ep.Worktree == "" {
		return slotRetirement{detached: true} // keyless/standalone: no slot ownership to retire
	}
	slot, _, err := seams.store.LoadWorktreeExecution(ep.Worktree)
	if err != nil {
		if se, ok := gatedrive.AsStoreError(err); ok && se.Kind == gatedrive.ErrNotFound {
			return slotRetirement{detached: true} // absent after full accounting: idempotently detached
		}
		return slotRetirement{finding: "slot-unreadable"}
	}
	switch classifySlotOwnership(slot.RunEpochID, slot.RawRunDir, ep) {
	case slotForeign:
		return slotRetirement{detached: true, finding: "slot-replaced-by-successor"}
	case slotUnowned, slotLinkedLegacy:
		return slotRetirement{detached: true}
	}
	// slotOwned: only a released owned slot may be detached.
	if string(slot.State) != "released" {
		return slotRetirement{finding: "slot-not-released"}
	}
	if rerr := seams.retireSlot(ep.Worktree, ep.EpochID, slot.ReservationToken); rerr != nil {
		cur, _, lerr := seams.store.LoadWorktreeExecution(ep.Worktree)
		if lerr == nil && cur.RunEpochID == "" {
			return slotRetirement{detached: true}
		}
		if lerr == nil && cur.RunEpochID != ep.EpochID {
			return slotRetirement{detached: true, finding: "slot-replaced-by-successor"}
		}
		return slotRetirement{finding: "slot-retire-failed"}
	}
	return slotRetirement{detached: true, retired: true}
}

// resolveTerminalEpochSlot returns ep with the worktree whose slot a TERMINAL epoch's
// quiescence checks address, plus a bounded finding when that worktree cannot be
// resolved. An epoch that still records its Worktree (or any non-superseded epoch)
// is returned unchanged. A SUPERSEDED epoch's Worktree was cleared by
// SupersedeCancelledEpoch, but an empty worktree is not proof of quiescence (change
// 0446 spec §4: "For a superseded epoch, that check uses the replacement's worktree
// slot to confirm the predecessor's token and epoch no longer hold it"). The
// replacement is followed through ReplacementReserved — the replacement's GATE KEY,
// so its epoch is read by LoadEpochRecord — and, when that replacement was itself
// superseded, onward along the chain until an epoch that binds a worktree.
//
// A TORN resume ends the chain at a replacement that binds no worktree:
// armResumeReplacement supersedes, then mints the replacement epoch, then binds its
// Worktree, so a failure between leaves the replacement epoch never minted
// (ErrEpochNotFound) or minted unbound. That is a known, recoverable shape — not
// corruption — so it resolves to an EXISTING stored identity rather than dead-ending
// every later resume and cancel (change 0446 spec: "Repeated cancellation,
// completion, and admission after safe reconciliation converge using existing
// operations"): requestWorktree when the caller holds one (a resume's verified
// feature worktree — exactly what armResumeReplacement binds), else the replacement
// gate record's prepared scope worktree (armResumeReplacement prepares that scope
// before the supersede), else the predecessor gate record's scope worktree
// (storedScopeWorktree). Only when none exists is it refused
// replacement-worktree-unresolved:<gate key>.
//
// A genuinely bad chain still fails closed with its exact locator, never inferred
// safe: a replacement epoch that cannot be read (corrupt, I/O) is
// replacement-epoch-unreadable:<gate key>, a superseded link that records no
// replacement is replacement-worktree-unresolved, and a loop is
// replacement-chain-cycle:<gate key>.
func resolveTerminalEpochSlot(seams cancelSeams, repoDir string, ep EpochRecord, requestWorktree string) (EpochRecord, string) {
	if ep.Worktree != "" || ep.State != EpochSuperseded {
		return ep, ""
	}
	seen := map[string]bool{}
	if ep.GateKey != "" {
		seen[ep.GateKey] = true
	}
	bind := func(worktree string) (EpochRecord, string) {
		out := ep
		out.Worktree = worktree
		return out, ""
	}
	// torn resolves an unbound chain tail (tailKey) to the first existing stored
	// identity, in the order documented above.
	torn := func(tailKey string) (EpochRecord, string) {
		w := requestWorktree
		if w == "" {
			w = storedScopeWorktree(seams, repoDir, tailKey)
		}
		if w == "" {
			w = storedScopeWorktree(seams, repoDir, ep.GateKey)
		}
		if w == "" {
			return ep, "replacement-worktree-unresolved:" + tailKey
		}
		return bind(w)
	}
	key := ep.ReplacementReserved
	for {
		if key == "" {
			return ep, "replacement-worktree-unresolved"
		}
		if seen[key] {
			return ep, "replacement-chain-cycle:" + key
		}
		seen[key] = true
		rec, _, err := LoadEpochRecord(repoDir, key)
		if err != nil {
			if ee, ok := AsEpochError(err); ok && ee.Kind == ErrEpochNotFound {
				return torn(key) // the replacement epoch was never minted
			}
			return ep, "replacement-epoch-unreadable:" + key
		}
		if rec.Worktree != "" {
			return bind(rec.Worktree)
		}
		if rec.State != EpochSuperseded {
			return torn(key) // minted but never bound to its worktree
		}
		key = rec.ReplacementReserved
	}
}

// storedScopeWorktree returns the feature worktree recorded on the outer recovery
// scope the gate record under gateKey names, or "" when there is none to read — no
// store, no such gate record, no scope id, or an unreadable scope. It is an identity
// lookup only: "" never means safe, it means this source has no identity to offer.
func storedScopeWorktree(seams cancelSeams, repoDir, gateKey string) string {
	if seams.store == nil || gateKey == "" {
		return ""
	}
	rec, err := LoadGateRecord(repoDir, gateKey)
	if err != nil || rec.ScopeID == "" {
		return ""
	}
	scope, err := seams.store.LoadScope(rec.ScopeID)
	if err != nil {
		return ""
	}
	return scope.Worktree
}

// verifyTerminalEpochQuiescence revalidates a terminal (cancelled/superseded)
// epoch's EXISTING launch and mutation evidence using the same bounded accounting
// cancellation uses — change 0437's launch reconciler plus the admitted-mutation
// journal (reconcileEpochTeardown's steps (5c) and (7)). It first resolves the slot
// worktree (resolveTerminalEpochSlot) and returns that resolved record, which the
// caller hands to the shared retirement: for a superseded epoch the launch census
// runs with the PREDECESSOR's epoch id against the REPLACEMENT's worktree, so both
// the scope-linked drives (enumerable by RunEpochID) and the replacement slot's
// references are accounted. It fails closed: an unresolvable replacement worktree,
// an absent or erroring reconciler, an unaccounted launch obligation (a busy claim,
// an unresolved relaunch), or an admitted-not-completed mutation is non-quiescence
// with a bounded finding. It performs no epoch or slot write, and it does not
// re-prove participants, which terminal repair deliberately does not re-enumerate.
// requestWorktree is the caller's own verified worktree identity, if any (resume's
// feature worktree; "" for run.cancel), used only to resolve a torn replacement
// chain (resolveTerminalEpochSlot).
func verifyTerminalEpochQuiescence(seams cancelSeams, repoDir string, ep EpochRecord, requestWorktree string) (EpochRecord, bool, []string) {
	var findings []string
	quiescent := true
	slotEp, rfinding := resolveTerminalEpochSlot(seams, repoDir, ep, requestWorktree)
	if rfinding != "" {
		findings = append(findings, rfinding)
		quiescent = false
	}
	if seams.launches == nil {
		findings = append(findings, "launch-reconciler-unavailable")
		quiescent = false
	} else if report, err := seams.launches.reconcile(slotEp.Worktree, ep.EpochID); err != nil {
		findings = append(findings, "launch-reconcile-failed")
		quiescent = false
	} else {
		findings = append(findings, report.Findings...)
		if !report.Accounted {
			quiescent = false
		}
	}
	for _, m := range ep.AdmittedMutations {
		if m.Status != mutationStatusCompleted {
			findings = append(findings, "mutation-pending:"+m.OpKey)
			quiescent = false
		}
	}
	return slotEp, quiescent, findings
}

// repairTerminalEpoch is the bounded repair a repeat run.cancel performs against a
// DURABLY terminal (cancelled/superseded) epoch: after re-proving quiescence
// (verifyTerminalEpochQuiescence) it retires, through the shared retirement
// (retireSlotOwnership), a released slot that still carries this epoch's RunEpochID —
// the historical stale-ownership incident change 0435 closed — and otherwise no-ops
// idempotently. A retirement it wrote is cancelled (applied); an already-detached,
// absent, epoch-less, or successor-held slot is already-cancelled (the successor
// finding surfaced, the successor untouched); an unsafe or unverifiable history is
// refused with its specific finding — never cancellation-pending over durable
// terminal state, never a regression to cancelling, never a revived epoch. It is not
// a general recovery engine: it uses only the records already present, and missing
// or contradictory evidence fails closed to refused. It never writes the epoch record.
func repairTerminalEpoch(seams cancelSeams, repoDir string, ep EpochRecord) RunCancelResult {
	slotEp, quiescent, findings := verifyTerminalEpochQuiescence(seams, repoDir, ep, "")
	if !quiescent {
		return cancelResult(CancelDispositionRefused, findings)
	}
	r := retireSlotOwnership(seams, slotEp)
	if r.finding != "" {
		findings = append(findings, r.finding)
	}
	switch {
	case !r.detached:
		return cancelResult(CancelDispositionRefused, findings)
	case r.retired:
		return cancelResult(CancelDispositionCancelled, findings)
	default:
		return cancelResult(CancelDispositionAlreadyCancelled, findings)
	}
}

// reconcileEpochTeardown performs the cancellation teardown accounting for an
// already-FENCED epoch — the spec's flow steps (4)–(7): cancel registered native
// tasks through the adapter hook (an absent adapter is a bounded FINDING, not
// silence), stop each registered execution participant and the worktree admission
// slot on proven teardown, RE-ENUMERATE the participants after stopping (a launch
// admitted before the fence won and can register after the first snapshot), settle
// uncertain publications a later completed identical retry proves (change 0444), and
// reconcile the admitted-mutation journal (an admitted-not-completed entry keeps
// the run pending). It returns whether the run is fully accounted, the bounded
// credential-free findings, and a non-nil err only for an epoch re-read fault.
//
// It NEVER validates authority and NEVER transitions the epoch (its only epoch write
// is settleUncertainPublications' uncertain→completed flip of retry-proven journal
// entries, which leaves the epoch state untouched): the caller fences
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
		markWorktreeSlotStopping(seams, ep)
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
	slotAccounted, slotFinding := reconcileWorktreeSlot(seams, ep)
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

	// (5d) Settle uncertain publications proven by a later completed identical
	// retry (change 0444) — a durable, journal-derived repair with NO Git or
	// GitHub call. Runs before the re-enumeration reload so steps (6)-(7)
	// evaluate the settled record. Shared by both fencing authorities
	// (run.cancel and the death guardian): settlement is observation of durable
	// journal fact, like the completion callback. A failed settlement write is
	// a bounded finding; the entry stays uncertain and step (7) keeps the
	// cancellation pending, so exclusion is retained until a repeat converges.
	settledTokens, sfindings := settleUncertainPublications(repoDir, gateKey)
	findings = append(findings, settledTokens...)
	findings = append(findings, sfindings...)

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

	// (7) Reconcile admitted mutations from the re-enumerated (post-settlement)
	// journal: any admitted-not-completed entry (in-flight, or uncertain with no
	// completed identical retry) keeps cancellation pending so a premature
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

// slotOwnershipClass classifies a loaded worktree slot against the fenced epoch —
// the ownership predicate every slot-touching cancel path shares. Ownership is
// established BEFORE marking, stopping, releasing, or retiring: the slot's current
// reservation token alone is not proof of epoch ownership (spec). A different
// nonempty RunEpochID is a foreign owner (or a successor) and is never touched;
// Tasks 4/5/7 reuse this classifier.
type slotOwnershipClass int

const (
	// slotOwned: the slot records this epoch's id — the epoch's own top-level execution.
	slotOwned slotOwnershipClass = iota
	// slotLinkedLegacy: an epoch-less slot whose exact execution (RawRunDir) is
	// independently linked to one of this epoch's REGISTERED execution participants.
	slotLinkedLegacy
	// slotForeign: a different nonempty RunEpochID — a foreign owner or a successor.
	// Never marked, stopped, released, or cleared.
	slotForeign
	// slotUnowned: epoch-less with no independent linkage — not provably this
	// epoch's; left untouched with an unresolved-ownership finding.
	slotUnowned
)

// classifySlotOwnership decides whether the fenced epoch owns the loaded slot. A
// nonempty RunEpochID is authoritative: equal to this epoch it is slotOwned, a
// different nonempty id is a foreign owner (slotForeign) — never touched. An
// epoch-less slot (the legacy shape) is ours only when its exact execution
// (RawRunDir) matches one of this epoch's REGISTERED execution participants
// (slotLinkedLegacy); otherwise it is not provably ours (slotUnowned).
func classifySlotOwnership(slotEpochID, slotRawRunDir string, ep EpochRecord) slotOwnershipClass {
	if slotEpochID != "" {
		if slotEpochID == ep.EpochID {
			return slotOwned
		}
		return slotForeign
	}
	if slotRawRunDir != "" {
		for _, p := range ep.Participants {
			if isExecutionParticipant(p.Kind) && p.NativeHandle == slotRawRunDir {
				return slotLinkedLegacy
			}
		}
	}
	return slotUnowned
}

// markWorktreeSlotStopping best-effort marks the epoch's worktree execution slot
// stopping before its process is stopped, using the slot's own persisted
// reservation token (RunCancel holds no token of its own). It marks ONLY a slot
// this epoch owns — slotOwned or slotLinkedLegacy per classifySlotOwnership: a
// different nonempty RunEpochID is a foreign owner and is left untouched. A missing
// store, empty worktree, unreadable slot, or already-terminal slot is also left
// untouched — the authoritative release decision is reconcileWorktreeSlot's.
func markWorktreeSlotStopping(seams cancelSeams, ep EpochRecord) {
	if seams.store == nil || ep.Worktree == "" {
		return
	}
	slot, _, err := seams.store.LoadWorktreeExecution(ep.Worktree)
	if err != nil {
		return
	}
	switch classifySlotOwnership(slot.RunEpochID, slot.RawRunDir, ep) {
	case slotOwned, slotLinkedLegacy:
		// proceed
	default:
		return // foreign or unowned: never marked
	}
	switch string(slot.State) {
	case "reserved", "executing":
		_ = seams.store.MarkWorktreeExecutionStopping(ep.Worktree, slot.ReservationToken)
	}
}

// reconcileWorktreeSlot stops the epoch's worktree execution slot and releases it on
// PROVEN teardown. It touches ONLY a slot this epoch owns (classifySlotOwnership): a
// foreign owner (a different nonempty RunEpochID) is never marked, stopped,
// released, or cleared, and an epoch-less slot with no independent participant
// linkage is left untouched with its ownership surfaced — both are accounted (they
// are not this epoch's obligation; launch obligations independently linked to this
// epoch are still accounted by the launch reconciler). For an owned slot it returns
// whether the slot is accounted (released, absent, or already released) and a
// bounded finding when it is not. It CHECKS the release write: a proven process stop
// does not prove the release was durably recorded, so a failed release fails closed
// to pending (slot-release-failed). A nil store or empty worktree is vacuously
// accounted (a keyless/standalone run owns no slot); an unreadable-but-present slot
// fails closed to pending.
func reconcileWorktreeSlot(seams cancelSeams, ep EpochRecord) (accounted bool, finding string) {
	if seams.store == nil || ep.Worktree == "" {
		return true, ""
	}
	slot, _, err := seams.store.LoadWorktreeExecution(ep.Worktree)
	if err != nil {
		if se, ok := gatedrive.AsStoreError(err); ok && se.Kind == gatedrive.ErrNotFound {
			return true, "" // no slot to reconcile
		}
		return false, "slot-unreadable"
	}
	switch classifySlotOwnership(slot.RunEpochID, slot.RawRunDir, ep) {
	case slotForeign:
		// A different nonempty RunEpochID is a foreign owner: not this epoch's
		// obligation, never marked/stopped/released/cleared.
		return true, "slot-foreign-owner"
	case slotUnowned:
		// Epoch-less with no independent participant linkage: not provably ours —
		// left untouched, ownership surfaced.
		return true, "slot-ownership-unresolved"
	}
	switch string(slot.State) {
	case "released":
		return true, ""
	case "reserved", "executing", "stopping":
		_ = seams.store.MarkWorktreeExecutionStopping(ep.Worktree, slot.ReservationToken)
		if slot.RawRunDir == "" {
			// A bare reservation with no launched process is not proven torn down
			// here; a later recovery resolves it. Fail closed to pending.
			return false, "slot-stop-unproven"
		}
		if stopParticipantProcess(seams, slot.RawRunDir) {
			if rerr := seams.store.ReleaseWorktreeExecution(ep.Worktree, slot.ReservationToken); rerr != nil {
				// A stopped process with an unrecorded release is NOT accounted: the
				// durable slot still claims a live execution (fail closed).
				return false, "slot-release-failed"
			}
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
	_ processObserver       = appGateObserver{}
	_ epochLaunchObserver   = appLaunchObserver{}
	_ OperationResult       = RunCancelResult{}
)
