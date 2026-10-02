// The gate-driver state machine: Start and Advance advance one logical suite
// execution over the raw process supervisor in short, slice-bounded synchronous
// calls behind one persisted deadline and execution identity.
//
// The driver is deliberately thin over the composed seams and never
// re-implements process liveness (spec Constraints 1, 9): the injected
// ProcessSeam (a faithful facade over internal/process's Launch/Observe/Stop)
// is the sole authority for process identity and terminal status; the Clock
// governs the fixed-once deadline and per-slice bound; the GitSeam recomputes
// the execution fingerprint at every ownership boundary and before a pass; the
// Store persists an atomic record at each transition and the ownership layer
// arbitrates the single owner.
//
// Every successful Start/Advance returns exactly one typed Outcome document
// (WAITING/PASSED/FAILED/HALTED). The mapping from a raw observation to an
// outcome is the whole point of the file, and it fails closed: only an exact
// native running state is retryable; a death earns at most one relaunch under
// five conjoined conditions; every other uncertainty is HALTED, never coerced
// into a red suite result (spec "State transitions and recovery", "Typed
// outcomes").
package gatedrive

import (
	"errors"
	"fmt"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// ProcessSeam is the driver's faithful facade over the raw process supervisor.
// Its method set and types mirror internal/process.Service exactly, so the real
// service is a drop-in seam (Task 7 wires it directly) while tests inject a
// deterministic double. The driver depends on the native run-state vocabulary
// (process.State) unchanged — it never invents or reinterprets a state.
type ProcessSeam interface {
	// Launch starts one detached gate-supervisor run and returns its handle,
	// including the run's state at the moment Launch returned.
	Launch(process.LaunchRequest) (*process.LaunchOutcome, error)
	// Observe reads one read-only snapshot of a raw run's durable state without
	// waiting or changing policy.
	Observe(runDir string) (*process.Observation, error)
	// Stop performs an ownership-proven bounded termination, or reports the
	// already-terminal no-op that preserves the child's own verdict.
	Stop(runDir, reason string) (*process.StopOutcome, error)
	// ResolveReservation answers what became of a launch handed the caller
	// reservation token but whose response was lost — never-launched, identified,
	// or unresolved (Task 2). The start's launch-failure leg consults it to
	// decide whether to release the worktree execution slot (a proven
	// never-launched) or fail it closed to unresolved.
	ResolveReservation(root, token string) (*process.ReservationResolution, error)
	// ClassifyRun assesses one raw run dir's recovery disposition, marking it
	// abandoned when mark is true. It mirrors process.Service.ClassifyRun exactly
	// so the real service is a drop-in seam; the first-admission legacy inventory
	// consults it (through the recoverySeam view) to decide whether a HALTED
	// historical drive is provably torn down.
	ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error)
}

// productionSlice is the slice target: the maximum a single synchronous driver
// call observes a live run before returning WAITING. It is materially below the
// harness foreground-call ceiling and is plumbing, not a user knob (spec
// "Deadline semantics"). Tests shrink Driver.slice directly.
const productionSlice = 30 * time.Second

// productionPollInterval bounds how often a slice re-observes a live run.
const productionPollInterval = 250 * time.Millisecond

// StartRequest is the validated input to a new drive. It carries the repository
// and worktree identity, the change/task/phase the drive certifies, the
// authoritative resolved command + cwd + budget (never agent input), the launch
// environment/config provenance the record needs, and whether the gate is an
// idempotent suite gate eligible for the single relaunch. The application seam
// (Task 9) resolves these from authoritative config before calling Start.
type StartRequest struct {
	// Worktree is the working tree that is fingerprinted (Start computes the
	// fingerprint over it, and every later boundary re-fingerprints the recorded
	// WorktreePath), and is the canonical worktree path recorded on the drive.
	// RepoDir is the repository identity recorded on the drive (stored as
	// RepoIdentity); it does not drive the fingerprint. In a linked-worktree
	// layout they are the same directory.
	RepoDir  string
	Worktree string

	// Change/task/phase identity — the work the drive certifies.
	ChangeID string
	TaskID   string
	Phase    string

	// Branch/ref recorded alongside the fingerprint's HEAD object id.
	Branch string
	Ref    string

	// Resolved command + working directory for the raw launch (authoritative
	// config, never agent input).
	Command []string
	Cwd     string

	// Config provenance + observation budget (authoritative config).
	ConfigProvenance string
	Budget           time.Duration

	// EnvHash is a canonical hash of the launch environment; the values are
	// never persisted (spec "Persisted execution identity").
	EnvHash string

	// RunRoot is the native process-supervisor allocation root (LaunchRequest.Root).
	RunRoot string

	// IdempotentSuiteGate marks a gate the application contract designates
	// idempotent; ONLY such a gate may earn the single relaunch.
	IdempotentSuiteGate bool

	// RunContext is the RAW outer child-context token linking this drive to the
	// dispatched run whose outer recovery scope run.start prepared; it is stored
	// only as its sha256 hash (RunContextHash), which the run tracker's outer
	// takeover matches. Empty for a drive outside any dispatched run. (change 0359)
	RunContext string

	// RunID links this drive's top-level execution to the workflow run
	// (runtracker_run_record.go) the starting run tracker minted, and is recorded on the worktree
	// execution slot the start reserves (admission.go). It is a LOCATOR, never a
	// credential: it authorizes nothing, but it fences the worktree — a later gate
	// in the same worktree that does not carry this run is refused ErrStaleRunID,
	// so an omitted or stale run cannot detach a workflow-owned worktree from its
	// run. Empty for a standalone gate that owns no implementation run (finalize's
	// local gate). (change 0375 Task 9)
	RunID string
}

// Driver is the gate-drive state machine. It holds no mutable per-drive state:
// the durable record in the Store is the sole source of truth, so a fresh
// Driver over the same store resumes any drive from disk. The slice/pollInterval/
// sleep fields are production plumbing tests override to run deterministically
// without sleeping for production durations.
type Driver struct {
	store *Store
	clock Clock
	proc  ProcessSeam
	git   GitSeam

	slice        time.Duration
	pollInterval time.Duration
	sleep        func(time.Duration)

	// runLaunch, when set, is the app-owned authoritative run liveness read the
	// launch/reservation paths run their durable reservation body under (change
	// 0437). Every run-backed reservation/launch authorization in this package
	// flows through the runLaunchGated helper, which consults this seam only when both
	// it and a run id are present. It is injected once at composition
	// (SetRunLaunchGate), before any concurrent start, so it needs no lock.
	runLaunch RunLaunchGate
}

// RunLaunchGate is the app-injected authority that validates a run is
// LIVE (active, uniquely resolved in this repository's registry, and bound to
// worktree) and, while the registry's per-key run lock is held, runs reserve —
// the driver's durable admission/reservation body — so a concurrent cancellation
// fence either lands before the liveness read (reserve never runs) or observes
// the durable reservation reserve produced. A validation failure returns a typed
// error and reserve is NEVER called. The gate performs no run write. A nil gate
// or an empty runID runs reserve directly (a genuinely no-run-record standalone
// gate keeps its existing behavior).
type RunLaunchGate func(runID, worktree string, reserve func() error) error

// SetRunLaunchGate injects the gate at composition, before any concurrent
// start, so it needs no lock (like SetRunSettledResolver). Passing nil
// clears it (the launch gate is then skipped and the no-run-record standalone
// behavior governs).
func (d *Driver) SetRunLaunchGate(g RunLaunchGate) { d.runLaunch = g }

// RunLaunchGateWired reports whether a RunLaunchGate has been injected. It is a
// read-only composition probe the app-layer wiring test keys on (change 0437 Task 5:
// the wiring is where the takeover-only defect lived) — never part of the drive
// protocol and never consulted by a drive operation.
func (d *Driver) RunLaunchGateWired() bool { return d.runLaunch != nil }

// runLaunchGated runs reserve under the injected gate when both the gate and the
// run id are present, else directly. Every run-backed reservation/launch
// authorization in this package flows through this ONE helper (the launch-site
// guard in change 0437 Task 8 keys on it).
func (d *Driver) runLaunchGated(runID, worktree string, reserve func() error) error {
	if d.runLaunch == nil || runID == "" {
		return reserve()
	}
	return d.runLaunch(runID, worktree, reserve)
}

// NewDriver builds a Driver over the composed seams with production slice bounds
// and a real sleep. Tests set the unexported slice/pollInterval/sleep fields to
// inject a deterministic clock/slice.
func NewDriver(store *Store, clock Clock, proc ProcessSeam, git GitSeam) *Driver {
	return &Driver{
		store:        store,
		clock:        clock,
		proc:         proc,
		git:          git,
		slice:        productionSlice,
		pollInterval: productionPollInterval,
		sleep:        time.Sleep,
	}
}

// startDocWithLegacy attaches a ticket's first-admission legacy recovery summary to
// a freshly produced START document, but ONLY when the census actually assessed
// legacy history (Checked > 0) — an ordinary start over a store with no legacy
// records narrates nothing. A launch error propagates unchanged with no summary.
func startDocWithLegacy(doc DriveDoc, err error, legacy *LegacyHistorySummary) (DriveDoc, error) {
	if err == nil && legacy != nil && legacy.Checked > 0 {
		doc.LegacyHistory = legacy
	}
	return doc, err
}

// reserveWorktreeExecution inventories legacy records with this driver's exact
// process-recovery seam before a new slot is durably reserved, returning the
// legacy history summary the census produced (nil when no legacy history was
// relevant) so the start paths can carry it on the returned document.
func (d *Driver) reserveWorktreeExecution(rec admissionRecord) (string, *LegacyHistorySummary, error) {
	return d.store.reserveWorktreeExecution(rec, d.proc)
}

// CleanupHistory runs the shared manual legacy-history assessment over this
// driver's store with its own process-recovery seam (the same recoverySeam the
// first-admission inventory consults). With an empty HistoryCleanupRequest.DriveID
// it scans the whole drive registry in ascending id order; a non-empty DriveID
// assesses exactly that one record. DryRun previews without writing any abandoned
// marker. Unlike admission, it is a REPORT, never a refusal — every candidate is
// returned with its class — and it takes NO admission/scope/drive lock: it mutates
// no gate state, and the only write is the process layer's own lock-guarded
// abandoned marker under apply.
func (d *Driver) CleanupHistory(req HistoryCleanupRequest) (HistoryCleanupOutcome, error) {
	return d.store.cleanupHistory(req, d.proc)
}

// NewSystemDriver builds a production Driver over the real monotonic clock and
// the real git seam, composing the given store and process seam. The application
// service seam (internal/app) uses it so an in-process caller composes the same
// state machine the CLI drives, without shelling out to docket's own CLI.
func NewSystemDriver(store *Store, proc ProcessSeam) *Driver {
	return NewDriver(store, systemClock{}, proc, realGit{})
}

// AdmissionTicket is the opaque result of Admit: a worktree execution slot durably
// reserved and a RESERVED drive record persisted, but with NO process launched
// yet. StartAdmitted consumes the ticket to launch the raw run and drive one
// slice; AbandonAdmission releases it when the caller decides between admission
// and launch not to proceed.
//
// The two-phase split exists so a caller can interpose exactly one accounting
// decision BETWEEN admission and launch: the BUILD owner reserves one full-suite
// attempt only after Admit succeeds, so a refused admission (worktree-busy,
// unresolved) charges nothing, while a launch/persistence failure in
// StartAdmitted keeps the charge — admission precedes charging, and once charged
// there are no refunds (change 0375 Task 8, ADR-0116). Every other caller composes
// the two through the thin Start.
type AdmissionTicket struct {
	id       string
	ownerGen string
	rec      driveRecord
	token    string
	// legacy is the first-admission legacy-drive recovery summary the worktree
	// reservation produced (nil when no legacy history was relevant). The launch
	// half carries it onto the returned START document.
	legacy *LegacyHistorySummary
	// runID retains, in memory only, the run this admission was gated
	// under so the launch half can revalidate the SAME run before launching
	// (change 0437). It is NEVER persisted — the durable linkage stays the
	// worktree slot record; an empty value is a genuinely no-run-record standalone gate.
	runID string
}

// Start creates a drive, validates and fingerprints the execution context,
// launches the first raw run through the process seam, persists the drive
// identity, and advances through at most one slice — returning the same typed
// outcome document Advance returns. A malformed request or a launch failure is a
// command failure (an error), not a drive outcome.
//
// Start is the thin composition of Admit (the pre-launch admission half) and
// StartAdmitted (the launch half) for callers that charge nothing between the two
// — finalize's local gate and the commandless resumption service. The BUILD owner composes the two halves itself so it can
// reserve one full-suite attempt between them (gate_drive.go); its behavior is
// otherwise identical to this composition.
func (d *Driver) Start(req StartRequest) (DriveDoc, error) {
	ticket, err := d.Admit(req)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.StartAdmitted(ticket)
}

// Admit performs the pre-launch half of a start: it validates the request,
// fingerprints the execution context, reserves the worktree execution slot, and
// persists a RESERVED drive record — but launches NO process. Every refusal here
// (a malformed request, or a worktree-busy / launch-unconfirmed slot) returns a
// typed error and reserves nothing the caller must later account for: no process
// was launched, and a freshly minted worktree slot is released before returning. A worktree-busy / unresolved refusal
// is returned only after finished-incumbent reconciliation could not settle the
// incumbent (its bounded finding rides OwnershipError.Reconciliation); admission
// never stops an incumbent to make room. On success it returns the ticket
// StartAdmitted (or AbandonAdmission) consumes.
func (d *Driver) Admit(req StartRequest) (*AdmissionTicket, error) {
	if len(req.Command) == 0 || req.Command[0] == "" {
		return nil, fmt.Errorf("gatedrive: start requires a non-empty command")
	}
	if req.Budget < 0 {
		return nil, fmt.Errorf("gatedrive: start requires a non-negative budget")
	}

	fp, err := ComputeFingerprint(req.Worktree, d.git)
	if err != nil {
		return nil, fmt.Errorf("gatedrive: start fingerprint: %w", err)
	}

	now := d.clock.Now()
	ownerGen, err := randomToken(genNBytes)
	if err != nil {
		return nil, storeErr(ErrIO, "start", err)
	}

	rec := driveRecord{
		RepoIdentity:        req.RepoDir,
		WorktreePath:        req.Worktree,
		ChangeID:            req.ChangeID,
		TaskID:              req.TaskID,
		Phase:               req.Phase,
		Branch:              req.Branch,
		Ref:                 req.Ref,
		HeadOID:             fp.Head,
		Fingerprint:         fp,
		Command:             append([]string(nil), req.Command...),
		Cwd:                 req.Cwd,
		ConfigProvenance:    req.ConfigProvenance,
		Budget:              req.Budget,
		EnvHash:             req.EnvHash,
		RunRoot:             req.RunRoot,
		IdempotentSuiteGate: req.IdempotentSuiteGate,
		StartedAt:           now,
		UpdatedAt:           now,
		Deadline:            computeDeadline(now, req.Budget),
		LastClock:           now,
		ProtocolVersion:     ProtocolVersion,
		Attempt:             1,
		OwnerGeneration:     ownerGen,
	}
	// RunContextHash links a drive started inside a dispatched run to that run's
	// outer recovery scope (empty for a drive outside any dispatched run).
	if req.RunContext != "" {
		rec.RunContextHash = capHash(req.RunContext)
	}

	// Fence the durable reservation behind the app-owned run liveness read: the
	// reservation body runs while the run registry lock is held, so a concurrent
	// cancellation fence either lands before the read (reserve never runs, nothing
	// is reserved) or observes the durable reservation reserve produced. The
	// fingerprint (above) stays OUTSIDE the gate; the lock order inside reserve is
	// unchanged (admission → drive).
	var ticket *AdmissionTicket
	admit := func() error {
		return d.runLaunchGated(req.RunID, req.Worktree, func() error {
			var aerr error
			ticket, aerr = d.admitScopeless(rec, ownerGen, req.RunID)
			return aerr
		})
	}
	err = admit()
	// Finished-incumbent reconciliation (change 0446 spec §3). A worktree-busy /
	// launch-unconfirmed refusal decided on an occupying incumbent is final only
	// after the exact incumbent was inspected: a proven-finished one is settled and
	// this SAME admission retries its reservation once. Reconciliation probes
	// processes, so it runs OUTSIDE the run launch gate (probe outside outer locks) and
	// applies its release under the slot CAS with the expected token; the retry then
	// re-enters the gate, which revalidates the run. The pre-reserve checks above
	// (command/budget, ComputeFingerprint) validate the request and never depend on
	// the incumbent, so none of them can refuse a finished incumbent before this
	// point.
	if oe, ok := isIncumbentRefusal(err); ok {
		settled, finding, _ := d.reconcileFinishedIncumbent(req.Worktree, req.RunID)
		if settled {
			err = admit()
		} else {
			oe.Reconciliation = finding
		}
	}
	if err != nil {
		return nil, err
	}
	ticket.runID = req.RunID
	return ticket, nil
}

// StartAdmitted performs the launch half of a start: it launches the admitted
// drive's raw run, persists the launch handle, confirms the worktree slot, and
// advances through at most one slice — returning the same typed outcome
// document Advance returns. A launch or persistence failure is a command failure
// (an error), not a drive outcome, and — because the caller may already have
// charged a suite attempt against this ticket — it never refunds: it either proves
// the fresh process stopped and releases the worktree slot, or marks the slot
// unresolved and fails future admission closed.
func (d *Driver) StartAdmitted(t *AdmissionTicket) (DriveDoc, error) {
	if t == nil {
		return DriveDoc{}, fmt.Errorf("gatedrive: StartAdmitted requires an admission ticket")
	}
	// Revalidate the run and the EXACT durable reservation this ticket minted,
	// and acquire the drive's claimant flock, before any launch (change 0437 Task
	// 2). A fence that landed between Admit and here — or a foreign reservation, a
	// busy claim, or a settled record — refuses with a typed error
	// and launches nothing. On success the returned claim is HELD across
	// launch/attach so a concurrent cancellation observes pending work rather than
	// a free slot; the launch half releases it once the launch is confirmed.
	claim, err := d.revalidateAdmittedLaunch(t)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.launchScopeless(t, claim)
}

// revalidateAdmittedLaunch re-reads, under the run launch gate, the EXACT durable
// reservation this ticket minted — the worktree slot must still carry the
// ticket's ReservationToken and still be reserved, and the RESERVED
// drive record must still exist under the ticket's owner generation and stay
// nonterminal — and acquires the drive's claimant flock NONBLOCKING. Any
// mismatch, a busy claim, or a revoked run refuses with a typed error and
// launches nothing.
//
// The claim is taken as the nonblocking per-drive launch claimant (the SAME lock
// file tryRelaunchClaim/reserveRelaunch use), so a concurrent cancellation that
// probes the claim reports busy — pending work, never proof of a crashed caller.
// The run lock is held only inside the gate; the returned claim is retained by
// the caller across the out-of-gate launch. A run refusal (the gate refused
// before running reserve) fail-closes the delayed ticket: it settles the drive
// HALTED "run-cancelled" and releases the ticket's slot — nothing launched,
// provably idle — then returns the gate's error
// unchanged so the app surfaces the fence token.
func (d *Driver) revalidateAdmittedLaunch(t *AdmissionTicket) (*relaunchClaim, error) {
	var claim *relaunchClaim
	reserveEntered := false
	err := d.runLaunchGated(t.runID, t.rec.WorktreePath, func() error {
		reserveEntered = true
		// (a) The drive's claimant flock, nonblocking. A busy claim is a launch
		// still in flight (or a cancellation probing it), never a free slot.
		c, busy, cerr := d.store.tryRelaunchClaim(t.id)
		if cerr != nil {
			return cerr
		}
		if busy {
			return ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted")
		}
		// (b) The worktree slot must still carry this ticket's reservation token and
		// still be reserved.
		if verr := d.verifyAdmittedSlot(t); verr != nil {
			c.close()
			return verr
		}
		// (c) The RESERVED drive record must still exist under this owner
		// generation and remain nonterminal.
		cur, lerr := d.store.Load(t.id)
		if lerr != nil {
			c.close()
			return lerr
		}
		if verr := verifyOwner(&cur, t.ownerGen); verr != nil {
			c.close()
			return verr
		}
		if isTerminalOutcome(cur.LastOutcome) {
			c.close()
			return ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted")
		}
		claim = c
		return nil
	})
	if err != nil {
		if claim != nil {
			claim.close()
			claim = nil
		}
		if !reserveEntered {
			// Run refusal: the gate refused before running reserve, so nothing was
			// claimed. Fail-close the delayed ticket and surface the gate's error.
			d.settleAdmittedAfterRunRefusal(t)
		}
		return nil, err
	}
	return claim, nil
}

// verifyAdmittedSlot confirms the worktree slot still carries this ticket's
// reservation token and is still reserved (the ticket minted the reservation and
// confirms it to executing only after launch). Any load error, token mismatch, or
// other state is a fail-closed ErrUnresolvedLaunchTransition — the exact
// reservation the ticket minted is gone.
func (d *Driver) verifyAdmittedSlot(t *AdmissionTicket) error {
	slot, _, err := d.store.LoadWorktreeExecution(t.rec.WorktreePath)
	if err != nil {
		return ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted")
	}
	if slot.ReservationToken != t.token {
		return ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted")
	}
	if slot.State != admissionReserved {
		return ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted")
	}
	return nil
}

// settleAdmittedAfterRunRefusal fail-closes a delayed ticket whose run was
// revoked between Admit and StartAdmitted. It settles the reserved drive record
// HALTED "run-cancelled" (mirroring the launch-failed CAS block in
// launchScopeless) and releases the worktree slot the ticket minted — nothing
// launched, so it is provably idle.
func (d *Driver) settleAdmittedAfterRunRefusal(t *AdmissionTicket) {
	_ = d.store.ownerCAS(t.id, func(r *driveRecord) error {
		if err := verifyOwner(r, t.ownerGen); err != nil {
			return err
		}
		if isTerminalOutcome(r.LastOutcome) {
			return errAlreadyTerminal
		}
		r.LastOutcome = HALTED
		r.LastCause = "run-cancelled"
		return nil
	})
	_ = d.store.ReleaseWorktreeExecution(t.rec.WorktreePath, t.token)
}

// AbandonAdmission releases an admission the caller decided, between Admit and
// StartAdmitted, not to launch (change 0375 Task 8: a build-owned start whose
// full-suite attempt could not be reserved after admission). Because no process
// was launched, the reserved drive record is a pure orphan (removed so it is not a
// spurious recovery candidate) and the worktree slot the start reserved is
// provably idle (released). AbandonAdmission is only reachable when the advisory
// budget precheck (gate_drive.go) was raced or a store fault intervened; it is
// deliberately best-effort and fail-closed, never a refund path.
func (d *Driver) AbandonAdmission(t *AdmissionTicket) error {
	if t == nil {
		return nil
	}
	_ = d.store.removeReservedDrive(t.id)
	if t.token == "" {
		return nil
	}
	return d.store.ReleaseWorktreeExecution(t.rec.WorktreePath, t.token)
}

// admitScopeless runs the pre-launch admission half of a start:
//
//	ReserveWorktreeExecution -> NewReservedDrive.
//
// RunRoot only scopes process-supervisor allocation. Worktree admission still
// keys on WorktreePath, so independent callers cannot use distinct private run
// roots to launch concurrently against one worktree. It launches no process;
// launchScopeless does. A NewReservedDrive failure releases the freshly reserved
// slot before returning, so a refused admission leaks nothing.
func (d *Driver) admitScopeless(rec driveRecord, ownerGen, runID string) (*AdmissionTicket, error) {
	token, legacy, err := d.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: rec.RepoIdentity,
		WorktreeRoot: rec.WorktreePath,
		RunID:        runID,
		Kind:         "scopeless",
	})
	if err != nil {
		return nil, err
	}
	rec.AdmissionToken = token

	// Persist a drive with no run handle before Launch. This makes a post-launch
	// persist error recoverable and ensures the admission reservation is never
	// detached from the drive that carries its token.
	id, _, err := d.store.NewReservedDrive(rec)
	if err != nil {
		_ = d.store.ReleaseWorktreeExecution(rec.WorktreePath, token)
		return nil, err
	}
	return &AdmissionTicket{id: id, ownerGen: ownerGen, rec: rec, token: token, legacy: legacy}, nil
}

// launchScopeless runs the launch half of a start:
//
//	Launch -> attachLaunch -> ConfirmWorktreeExecution -> driveAndPersist.
//
// Every post-launch failure either proves the fresh process stopped before
// releasing the slot, or marks the slot unresolved and fails future admission
// closed — never an automatic second launch. The claimant flock
// revalidateAdmittedLaunch acquired is HELD across Launch and attach (so a
// concurrent cancellation observes pending work), then released before the drive
// slice so a first-slice relaunch can reserve its own claim; the deferred close is
// an idempotent safety net for every failure leg.
func (d *Driver) launchScopeless(t *AdmissionTicket, claim *relaunchClaim) (DriveDoc, error) {
	defer claim.close()
	rec := t.rec
	id := t.id
	ownerGen := t.ownerGen
	token := t.token

	out, lerr := d.proc.Launch(rec.launchRequest())
	if lerr != nil {
		// Keep the attempted drive as durable recovery evidence (HALTED
		// "launch-failed"). ResolveReservation is the only proof that an error
		// response means no process was launched; every other outcome leaves the
		// worktree slot unresolved.
		_ = d.store.ownerCAS(id, func(r *driveRecord) error {
			if err := verifyOwner(r, ownerGen); err != nil {
				return err
			}
			if isTerminalOutcome(r.LastOutcome) {
				return errAlreadyTerminal
			}
			r.LastOutcome = HALTED
			r.LastCause = "launch-failed"
			return nil
		})
		d.resolveWorktreeAfterLaunchFailure(rec.WorktreePath, rec.RunRoot, token)
		return DriveDoc{}, fmt.Errorf("gatedrive: start launch: %w", lerr)
	}

	if err := d.store.attachLaunch(id, ownerGen, out.RunDir, out.RunID); err != nil {
		stopped := d.stopIfOwned(out.RunDir)
		d.releaseOrUnresolveWorktree(rec.WorktreePath, token, stopped)
		return DriveDoc{}, err
	}
	if err := d.store.ConfirmWorktreeExecution(rec.WorktreePath, token, out.RunID, out.RunDir); err != nil {
		stopped := d.stopIfOwned(out.RunDir)
		d.releaseOrUnresolveWorktree(rec.WorktreePath, token, stopped)
		return DriveDoc{}, err
	}

	// Launch and attach are confirmed: release the launch claim so the drive slice
	// can reserve its own single relaunch (the claim is the SAME lock file
	// reserveRelaunch takes). close is idempotent with the deferred safety net.
	claim.close()

	rec.RawRunDir = out.RunDir
	rec.RawOwnership = out.RunID
	doc, derr := d.driveAndPersist(id, ownerGen, rec)
	return startDocWithLegacy(doc, derr, t.legacy)
}

// resolveWorktreeAfterLaunchFailure consults the process backend for the fate of a
// launch that returned an error, then releases or fails the worktree slot closed. A
// PROVEN never-launched (a clean census carried no matching run) frees the slot — the
// worktree is genuinely idle. Every other verdict (unresolved, an identified run from a
// lost response, or a resolve error) fails the slot closed to unresolved, so a possibly
// live process never frees the worktree (change 0375, fail-closed everywhere).
func (d *Driver) resolveWorktreeAfterLaunchFailure(worktree, runRoot, token string) {
	res, rerr := d.proc.ResolveReservation(runRoot, token)
	if rerr == nil && res != nil && res.Disposition == "never-launched" {
		_ = d.store.ReleaseWorktreeExecution(worktree, token)
		return
	}
	_ = d.store.MarkWorktreeExecutionUnresolved(worktree, token)
}

// releaseOrUnresolveWorktree frees the worktree slot when a teardown was PROVEN and
// fails it closed to unresolved otherwise. A post-launch persist/confirm failure orphans
// a freshly launched run; only a stop this drive proved it owned lets the slot be
// released, else the run might still be live and the slot must block admission until
// recovery (change 0375).
func (d *Driver) releaseOrUnresolveWorktree(worktree, token string, stopProven bool) {
	if stopProven {
		_ = d.store.ReleaseWorktreeExecution(worktree, token)
		return
	}
	_ = d.store.MarkWorktreeExecutionUnresolved(worktree, token)
}

// Advance resumes a drive through at most one slice. It loads the durable record
// (the only source of truth), verifies the presented owner generation, and — for
// a still-live drive — runs one slice and persists the transition. A record that
// cannot be read at all is a command failure; a recognized-but-unusable record
// (unknown schema, corrupt) or a stale owner fails closed to HALTED.
func (d *Driver) Advance(id, ownerGen string) (DriveDoc, error) {
	rec, err := d.store.Load(id)
	if err != nil {
		// A missing or malformed id is a command failure — there is no drive to
		// report on. A recognized drive whose schema/content is unusable is a
		// fail-closed HALT (spec "unknown schema ... => HALTED").
		if se, ok := AsStoreError(err); ok {
			switch se.Kind {
			case ErrUnknownSchema, ErrCorruptRecord:
				return d.haltDoc(id, ownerGen, driveRecord{}, CauseSchemaMismatch), nil
			default:
				return DriveDoc{}, err
			}
		}
		return DriveDoc{}, err
	}

	// A stale/wrong owner generation is an identity disagreement: HALT, never a
	// silent continuation, and mutate nothing.
	if err := verifyOwner(&rec, ownerGen); err != nil {
		return d.haltDoc(id, ownerGen, rec, "owner-superseded"), nil
	}

	// A terminal drive is idempotent: return the recorded verdict without
	// re-driving the (already consumed or torn-down) run.
	if isTerminalOutcome(rec.LastOutcome) {
		return d.settledTerminalDoc(id, ownerGen, rec), nil
	}
	var claim *relaunchClaim
	if rec.RelaunchReserved {
		// Validate the drive's run (read-only) BEFORE recoverReservedRelaunch takes
		// the per-drive claim: a revoked run must not authorize a NEW launch for a
		// crash-window reservation, though an already-identified replacement is still
		// reconciled (attach/report — reconciliation, not authorization). The run
		// lock is thus acquired without holding the claim (change 0437 Task 3).
		revoked := d.recoveryRunRevoked(rec)
		var resolved *DriveDoc
		rec, claim, resolved, err = d.recoverReservedRelaunch(id, ownerGen, rec, revoked)
		if err != nil {
			return DriveDoc{}, err
		}
		if resolved != nil {
			return *resolved, nil
		}
	}

	return d.driveAndPersistClaim(id, ownerGen, rec, claim)
}

// Handoff transfers a live drive to a fresh owner through the single-use handoff
// receipt. It loads the durable record, recomputes the current repository
// fingerprint through the injected git seam, and — under the ownership CAS —
// invalidates the presented owner and writes the receipt only when that owner is
// current and the worktree still matches the drive-start identity (spec
// "Explicit handoff and nearest-owner continuation"). The returned document
// carries, in Generation, the single-use handoff token a claimant presents to
// Claim. A record that cannot be read at all is a command failure; a
// recognized-but-unusable record, a stale owner, an outstanding handoff, or a
// drifted worktree fails closed to HALTED — never a silent transfer.
func (d *Driver) Handoff(id, ownerGen string) (DriveDoc, error) {
	rec, err := d.store.Load(id)
	if err != nil {
		return d.loadHalt(id, ownerGen, err)
	}
	current, ferr := ComputeFingerprint(rec.WorktreePath, d.git)
	if ferr != nil {
		return d.haltDoc(id, ownerGen, rec, "fingerprint-error"), nil
	}
	receipt, herr := d.store.writeHandoffReceipt(id, ownerGen, current)
	if herr != nil {
		if oe, ok := AsOwnershipError(herr); ok {
			return d.haltDoc(id, ownerGen, rec, string(oe.Kind)), nil
		}
		return DriveDoc{}, herr
	}
	cur, err := d.store.Load(id)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.transferDoc(id, receipt.HandoffGeneration, cur), nil
}

// Claim consumes an outstanding single-use handoff receipt for a fresh owner. It
// loads the durable record, recomputes the current repository fingerprint through
// the injected git seam, and — under the ownership CAS — installs a new owner only
// when the presented handoff token is the drive's current outstanding one and the
// recomputed identity still matches the drive-start fingerprint. The returned
// document carries, in Generation, the fresh owner generation the claimant
// advances with. A record that cannot be read at all is a command failure; no
// outstanding handoff, a mismatched token, or a drifted worktree fails closed to
// HALTED, so a claimant that lost the race or no longer matches acquires no
// authority.
func (d *Driver) Claim(id, handoffID string) (DriveDoc, error) {
	rec, err := d.store.Load(id)
	if err != nil {
		return d.loadHalt(id, "", err)
	}
	current, ferr := ComputeFingerprint(rec.WorktreePath, d.git)
	if ferr != nil {
		return d.haltDoc(id, "", rec, "fingerprint-error"), nil
	}
	newOwner, cerr := d.store.consumeHandoffCAS(id, handoffID, current)
	if cerr != nil {
		if oe, ok := AsOwnershipError(cerr); ok {
			return d.haltDoc(id, "", rec, string(oe.Kind)), nil
		}
		return DriveDoc{}, cerr
	}
	// A normal claim moves the nearest-owner chain up: if the drive was dispatched
	// under a recovery scope, close it best-effort so a parent never later takes
	// over a drive that was already handed off and claimed cooperatively.
	if rec.ScopeID != "" {
		_ = d.store.closeScope(rec.ScopeID)
	}
	cur, err := d.store.Load(id)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.transferDoc(id, newOwner, cur), nil
}

// loadHalt maps a store Load error at an ownership boundary the same way Advance
// does: a recognized-but-unusable record (unknown schema, corrupt) fails closed
// to a HALTED document, while a missing or malformed id is a command failure
// (there is no drive to report on).
func (d *Driver) loadHalt(id, gen string, err error) (DriveDoc, error) {
	if se, ok := AsStoreError(err); ok {
		switch se.Kind {
		case ErrUnknownSchema, ErrCorruptRecord:
			return d.haltDoc(id, gen, driveRecord{}, CauseSchemaMismatch), nil
		}
	}
	return DriveDoc{}, err
}

// transferDoc builds the document a successful ownership transfer returns:
// Generation carries the credential the caller presents next — the single-use
// handoff token after Handoff, or the fresh owner generation after Claim — while
// Outcome/Cause report the drive's last recorded verdict and only a PASSED drive
// exposes its raw run dir.
func (d *Driver) transferDoc(id, generation string, rec driveRecord) DriveDoc {
	doc := DriveDoc{
		ProtocolVersion: ProtocolVersion,
		DriveID:         id,
		Generation:      generation,
		Attempt:         rec.Attempt,
		Deadline:        rec.Deadline,
		Outcome:         rec.LastOutcome,
		Cause:           rec.LastCause,
	}
	if rec.LastOutcome == PASSED {
		doc.RawRunDir = rec.RawRunDir
	}
	return doc
}

// driveAndPersist runs one slice over rec, persists the resulting transition
// atomically under the owner CAS, and returns the outcome document built from
// the authoritative post-transition record.
func (d *Driver) driveAndPersist(id, ownerGen string, rec driveRecord) (DriveDoc, error) {
	return d.driveAndPersistClaim(id, ownerGen, rec, nil)
}

func (d *Driver) driveAndPersistClaim(id, ownerGen string, rec driveRecord, claim *relaunchClaim) (DriveDoc, error) {
	if claim != nil {
		defer claim.close()
	}
	res := d.driveSlice(id, ownerGen, rec, claim)
	if res.err != nil {
		return DriveDoc{}, res.err
	}
	if res.relaunchRaceLost {
		cur, err := d.store.Load(id)
		if err != nil {
			return DriveDoc{}, err
		}
		return d.recordedDoc(id, ownerGen, cur), nil
	}

	err := d.store.ownerCAS(id, func(r *driveRecord) error {
		if err := verifyOwner(r, ownerGen); err != nil {
			return err
		}
		// A concurrent writer may already have driven this drive to a terminal
		// state; never clobber a recorded verdict.
		if isTerminalOutcome(r.LastOutcome) {
			return errAlreadyTerminal
		}
		r.UpdatedAt = res.lastClock
		r.LastClock = res.lastClock
		r.LastOutcome = res.outcome
		r.LastCause = res.cause
		return nil
	})
	if err != nil {
		if errors.Is(err, errAlreadyTerminal) || errors.Is(err, errRelaunchRaceLost) {
			// Relaunch attachment is committed before its observation slice, so a
			// stale outcome writer owns no orphan to stop here. Return the current
			// authoritative verdict.
			cur, lerr := d.store.Load(id)
			if lerr != nil {
				return DriveDoc{}, lerr
			}
			return d.settledTerminalDoc(id, ownerGen, cur), nil
		}
		return DriveDoc{}, err
	}

	cur, err := d.store.Load(id)
	if err != nil {
		return DriveDoc{}, err
	}
	return d.settledTerminalDoc(id, ownerGen, cur), nil
}

// settledTerminalDoc runs the terminal slot release for rec and builds the outcome
// document from what that release PROVED (change 0446 spec §5 audit). The release
// error is never discarded: a failed release/stopping/unresolved write rides the
// document as a bounded ReleaseFinding, and the private run root is advertised only
// when the slot's release evidence is settled — a caller that removes the root at
// the terminal must never delete the evidence a still-required reconciliation needs.
// A non-terminal record has nothing to release and is returned as recorded.
func (d *Driver) settledTerminalDoc(id, ownerGen string, rec driveRecord) DriveDoc {
	if !isTerminalOutcome(rec.LastOutcome) {
		return d.recordedDoc(id, ownerGen, rec)
	}
	settled, err := d.releaseAdmissionIfProven(rec)
	doc := d.recordedDocWithRoot(id, ownerGen, rec, settled && err == nil)
	if err != nil {
		doc.ReleaseFinding = releaseFinding(err)
	}
	return doc
}

// releaseFinding reduces a release-step error to a bounded, credential-free token:
// the typed store/ownership op and kind only — never the wrapped error text, which
// can carry host paths. An untyped error is reported as a generic io finding.
func releaseFinding(err error) string {
	if oe, ok := AsOwnershipError(err); ok {
		return "release-unsettled:" + oe.Op + ":" + string(oe.Kind)
	}
	if se, ok := AsStoreError(err); ok {
		return "release-unsettled:" + se.Op + ":" + string(se.Kind)
	}
	return "release-unsettled:io"
}

// releaseAdmissionIfProven frees the drive's worktree execution slot once the
// drive's teardown is proven, so the next top-level execution can admit onto the
// same worktree. It reports settled=true only when the slot is PROVEN not to be held by
// this drive's execution any more (released under its token, already released, or
// already carrying a successor's token), and returns every write/read error rather
// than dropping it (change 0446).
//
// A record with no admission token carries no slot this driver can free: every
// admission path (admitScopeless) stamps a token, so an
// empty token is a legacy/raw-history record. For PASSED/FAILED its process has
// finished, so it is settled; for HALTED nothing proves teardown, so it is not.
//
// On a PASSED or FAILED verdict the slot is released outright: the supervisor
// reports those only after writing its terminal record, so the child group has
// ended and the worktree is genuinely idle, and the release is proven by the same
// observation that produced the verdict. A HALTED verdict is handled here too, but
// its process may still be live, so the document alone does not prove teardown:
// the slot is Observe/Stopped and released only when that proves the group has
// ended (Stop performed, or the observation proves teardown); when it cannot be
// resolved the slot is marked unresolved or stopping rather than freed, and
// settled is false. HALTED is never itself release proof. The release verifies the
// slot's reservation token and is idempotent under it, so a stale drive cannot free
// a successor's slot and a concurrent-writer race that both observe the terminal
// never double-frees.
func (d *Driver) releaseAdmissionIfProven(rec driveRecord) (settled bool, err error) {
	finished := rec.LastOutcome == PASSED || rec.LastOutcome == FAILED
	if rec.AdmissionToken == "" {
		return finished, nil
	}
	if !finished && rec.LastOutcome != HALTED {
		return false, nil
	}
	// A launch failure can already have released this token after a proven
	// never-launched resolution, and a successor may already hold the slot under
	// its own token. Either way this drive no longer holds it: preserve that
	// historical release instead of rewriting (or converting it to uncertainty).
	slot, _, err := d.store.LoadWorktreeExecution(rec.WorktreePath)
	if err != nil {
		return false, err
	}
	if slot.ReservationToken != rec.AdmissionToken || slot.State == admissionReleased {
		return true, nil
	}
	if finished {
		if err := d.store.ReleaseWorktreeExecution(rec.WorktreePath, rec.AdmissionToken); err != nil {
			return false, err
		}
		return true, nil
	}
	if rec.RawRunDir == "" {
		return false, d.store.MarkWorktreeExecutionUnresolved(rec.WorktreePath, rec.AdmissionToken)
	}
	stopped, serr := d.proc.Stop(rec.RawRunDir, "gatedrive-halt-release")
	if serr != nil {
		return false, d.store.MarkWorktreeExecutionUnresolved(rec.WorktreePath, rec.AdmissionToken)
	}
	if stopped != nil && (stopped.Performed || stopProvesTeardown(stopped.State)) {
		if err := d.store.ReleaseWorktreeExecution(rec.WorktreePath, rec.AdmissionToken); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, d.store.MarkWorktreeExecutionStopping(rec.WorktreePath, rec.AdmissionToken)
}

// stopProvesTeardown reports whether a Stop outcome's resulting process state
// proves the child group is gone. A passed, failed, stopped, or vanished run is
// proven torn down; a signalled run is NOT (its group may still hold
// descendants), and a live run obviously is not. It governs only the HALTED
// release-on-stop leg above — the legacy inventory now assesses teardown through
// the shared classifier and the process-recovery seam, not a bare observation.
func stopProvesTeardown(st process.State) bool {
	switch st {
	case process.StatePassed, process.StateFailed, process.StateStopped, process.StateVanished:
		return true
	default:
		return false
	}
}

// errAlreadyTerminal is a sentinel raised inside an owner CAS (the persist CAS
// and reserveRelaunch's reservation CAS) to abort a write over a drive a
// concurrent same-owner writer already finished; it never escapes as a workflow
// error. Every caller — the persist path, the attach-race branch, and the
// reserveRelaunch branch — treats it as "reload the authoritative recorded
// state", so a loser that reaches its reservation CAS only after the winner
// settled the drive terminally returns that verdict rather than the raw sentinel.
var errAlreadyTerminal = errors.New("gatedrive: drive already terminal")

// errRelaunchRaceLost reports that another same-owner advance has already
// reserved or consumed the single automatic replacement. The loser reloads the
// authoritative drive state and never issues a second backend launch.
var errRelaunchRaceLost = errors.New("gatedrive: relaunch already consumed by a concurrent advance")

// resolveDriveRun resolves the run a durable drive is linked to, from
// existing records only. A scoped drive answers from its scope's RunID; a
// scopeless drive with an AdmissionToken answers from the worktree slot ONLY when
// the slot's ReservationToken still equals that token (an exact-reservation
// match). ok=false with cause set means the linkage is LOST or inconsistent — the
// drive can no longer prove whether it is run-backed, so new execution is
// refused (never demoted to standalone). A scoped drive whose scope cannot be read
// reports CauseRunRecordUnreadable; an absent, unreadable, or reassigned worktree
// slot reports CauseRunLinkLost. ("", true, "") is a genuinely no-run-record
// drive (a legacy empty token, or a slot recording no run). (change 0437 Task 3)
func (d *Driver) resolveDriveRun(rec driveRecord) (runID string, ok bool, cause string) {
	if rec.ScopeID != "" {
		scope, err := d.store.LoadScope(rec.ScopeID)
		if err != nil {
			// The scope's run cannot be read: the drive can no longer prove its
			// linkage, so refuse rather than treat it as standalone (CauseRunRecordUnreadable).
			return "", false, CauseRunRecordUnreadable
		}
		return scope.RunID, true, ""
	}
	if rec.AdmissionToken == "" {
		return "", true, ""
	}
	slot, _, err := d.store.LoadWorktreeExecution(rec.WorktreePath)
	if err != nil {
		return "", false, CauseRunLinkLost
	}
	if slot.ReservationToken != rec.AdmissionToken {
		return "", false, CauseRunLinkLost
	}
	return slot.RunID, true, ""
}

// authorizeRelaunch validates, under the run launch gate, that the drive's run (if
// any) is live and reserves the single automatic replacement while the gate is
// held. It returns the held claim; the caller launches OUTSIDE the gate. A LOST
// linkage returns a non-empty halt cause and reserves nothing (never demoted to
// standalone). A gate refusal (the run is revoked) maps to halt cause
// "run-cancelled" — a bounded token matching the fence vocabulary; reserveRelaunch's
// own race-lost/terminal/IO error is returned unchanged so the caller's existing
// sentinel handling applies. Lock order: the run launch gate is acquired FIRST and
// reserveRelaunch takes the per-drive claim INSIDE it, so the run lock is never
// acquired while the claim is already held (spec "Serialize with existing locks").
func (d *Driver) authorizeRelaunch(id, ownerGen string, rec driveRecord) (*relaunchClaim, string, error) {
	runID, ok, cause := d.resolveDriveRun(rec)
	if !ok {
		return nil, cause, nil
	}
	var claim *relaunchClaim
	reserveEntered := false
	err := d.runLaunchGated(runID, rec.WorktreePath, func() error {
		reserveEntered = true
		c, rerr := d.store.reserveRelaunch(id, ownerGen)
		if rerr != nil {
			return rerr
		}
		claim = c
		return nil
	})
	if err != nil {
		if claim != nil {
			claim.close()
			claim = nil
		}
		if !reserveEntered {
			// The gate refused before running reserve: the run is revoked.
			return nil, "run-cancelled", nil
		}
		// reserveRelaunch's own error: hand it back for the existing sentinel handling.
		return nil, "", err
	}
	return claim, "", nil
}

// recoveryRunRevoked reports whether a reserved-relaunch drive's linked run is
// no longer live, via a read-only pass through the run launch gate (a no-op reserve
// body). It runs BEFORE recoverReservedRelaunch takes the per-drive claim, so the
// run lock is never acquired while the claim is held. A lost or unreadable
// linkage is treated as revoked (fail closed: recovery may still attach or report,
// but must never authorize a NEW launch for a drive that cannot prove it is still
// run-backed). A genuinely no-run-record drive (runID "") runs the no-op directly
// and is never revoked, so the standalone recovery path is unchanged. (change 0437
// Task 3)
func (d *Driver) recoveryRunRevoked(rec driveRecord) bool {
	runID, ok, _ := d.resolveDriveRun(rec)
	if !ok {
		return true
	}
	return d.runLaunchGated(runID, rec.WorktreePath, func() error { return nil }) != nil
}

// reserveRelaunch acquires the short-lived claimant fence before durably
// consuming the drive's one automatic replacement. The unique process token is
// written in the same CAS. A competitor cannot mistake the interval between
// this transition and Launch for a crashed caller because it cannot acquire the
// claimant flock; a crashed caller releases the flock while leaving the durable
// token available for exact ResolveReservation recovery.
func (s *Store) reserveRelaunch(id, ownerGen string) (*relaunchClaim, error) {
	claim, busy, err := s.tryRelaunchClaim(id)
	if err != nil {
		return nil, err
	}
	if busy {
		return nil, errRelaunchRaceLost
	}
	token, err := randomToken(genNBytes)
	if err != nil {
		claim.close()
		return nil, storeErr(ErrIO, "reserve-relaunch-token", err)
	}
	claim.token = token
	err = s.ownerCAS(id, func(rec *driveRecord) error {
		if err := verifyOwner(rec, ownerGen); err != nil {
			return err
		}
		if isTerminalOutcome(rec.LastOutcome) {
			return errAlreadyTerminal
		}
		if rec.RelaunchCount > 0 || rec.RelaunchReserved || rec.RelaunchToken != "" {
			return errRelaunchRaceLost
		}
		rec.RelaunchReserved = true
		rec.RelaunchToken = token
		return nil
	})
	if err != nil {
		claim.close()
		return nil, err
	}
	return claim, nil
}

// recoverReservedRelaunch resolves a crash window after reserveRelaunch and
// before the replacement was attached. The claimant flock is checked before
// the census: contention proves the original reserving call is still within
// launch/attach, so this caller returns authoritative state without resolving
// or launching. After a crash, the new claimant resolves the replacement's own
// token, never the admission token used by the original run.
//
// When revoked is true (the caller's read-only run pass found the drive's run
// no longer live), the ONLY behavioral change is the proven-never-launched arm: it
// settles the drive HALTED "run-cancelled" instead of returning a live claim for a
// new launch. The identified, busy, and ambiguous arms are unchanged — reconciling
// an already-live replacement (attach/report/halt) is teardown, not authorization
// (change 0437 Task 3, spec AC4).
func (d *Driver) recoverReservedRelaunch(id, ownerGen string, rec driveRecord, revoked bool) (driveRecord, *relaunchClaim, *DriveDoc, error) {
	claim, busy, err := d.store.tryRelaunchClaim(id)
	if err != nil {
		return driveRecord{}, nil, nil, err
	}
	if busy {
		doc := d.recordedDoc(id, ownerGen, rec)
		return rec, nil, &doc, nil
	}
	cur, err := d.store.Load(id)
	if err != nil {
		claim.close()
		return driveRecord{}, nil, nil, err
	}
	if err := verifyOwner(&cur, ownerGen); err != nil {
		claim.close()
		return driveRecord{}, nil, nil, err
	}
	if isTerminalOutcome(cur.LastOutcome) || !cur.RelaunchReserved {
		claim.close()
		doc := d.recordedDoc(id, ownerGen, cur)
		return cur, nil, &doc, nil
	}
	if cur.RelaunchToken == "" {
		claim.close()
		return d.haltReservedRelaunch(id, ownerGen, cur)
	}
	claim.token = cur.RelaunchToken
	resolution, err := d.proc.ResolveReservation(cur.RunRoot, cur.RelaunchToken)
	if err != nil || resolution == nil || resolution.Disposition == "unresolved" {
		claim.close()
		return d.haltReservedRelaunch(id, ownerGen, cur)
	}
	switch resolution.Disposition {
	case "never-launched":
		if revoked {
			// The run was revoked before this crash-window reservation ever
			// launched: it is provably idle, so settle it closed rather than
			// authorizing a new launch under a dead run.
			claim.close()
			return d.haltReservedRelaunchCause(id, ownerGen, cur, "run-cancelled")
		}
		return cur, claim, nil, nil
	case "identified":
		if resolution.RunID == "" || resolution.RunDir == "" {
			claim.close()
			return d.haltReservedRelaunch(id, ownerGen, cur)
		}
		err := d.attachReservedRelaunch(id, ownerGen, claim, resolution.RunDir, resolution.RunID)
		claim.close()
		if err != nil {
			if !errors.Is(err, errRelaunchRaceLost) {
				return driveRecord{}, nil, nil, err
			}
			cur, lerr := d.store.Load(id)
			if lerr != nil {
				return driveRecord{}, nil, nil, lerr
			}
			doc := d.recordedDoc(id, ownerGen, cur)
			return cur, nil, &doc, nil
		}
		cur, err := d.store.Load(id)
		if err != nil {
			return driveRecord{}, nil, nil, err
		}
		return cur, nil, nil, nil
	default:
		claim.close()
		return d.haltReservedRelaunch(id, ownerGen, cur)
	}
}

func (d *Driver) haltReservedRelaunch(id, ownerGen string, rec driveRecord) (driveRecord, *relaunchClaim, *DriveDoc, error) {
	return d.haltReservedRelaunchCause(id, ownerGen, rec, "launch-unconfirmed")
}

// haltReservedRelaunchCause settles a reserved-but-unattached relaunch HALTED with
// the given cause, preserving the consumed reservation (the CAS never clears
// RelaunchReserved, so the sole relaunch is never refunded). "launch-unconfirmed"
// is the crash-window uncertainty default; "run-cancelled" is used when the drive's
// run was revoked before the replacement launched (change 0437 Task 3).
func (d *Driver) haltReservedRelaunchCause(id, ownerGen string, rec driveRecord, cause string) (driveRecord, *relaunchClaim, *DriveDoc, error) {
	err := d.store.ownerCAS(id, func(r *driveRecord) error {
		if err := verifyOwner(r, ownerGen); err != nil {
			return err
		}
		if isTerminalOutcome(r.LastOutcome) {
			return errAlreadyTerminal
		}
		if !r.RelaunchReserved {
			return errRelaunchRaceLost
		}
		r.LastOutcome = HALTED
		r.LastCause = cause
		return nil
	})
	if err != nil && !errors.Is(err, errAlreadyTerminal) && !errors.Is(err, errRelaunchRaceLost) {
		return driveRecord{}, nil, nil, err
	}
	cur, lerr := d.store.Load(id)
	if lerr != nil {
		return driveRecord{}, nil, nil, lerr
	}
	doc := d.recordedDoc(id, ownerGen, cur)
	return cur, nil, &doc, nil
}

// attachReservedRelaunch makes the replacement authoritative before any
// observation slice begins. The claimant token prevents a stale launcher from
// attaching over a recovered claimant, and clearing RelaunchReserved lets later
// advances observe the replacement normally after the liveness flock closes.
func (d *Driver) attachReservedRelaunch(id, ownerGen string, claim *relaunchClaim, runDir, runID string) error {
	return d.store.ownerCAS(id, func(r *driveRecord) error {
		if err := verifyOwner(r, ownerGen); err != nil {
			return err
		}
		if r.RelaunchCount > 0 || !r.RelaunchReserved || claim == nil || r.RelaunchToken != claim.token {
			return errRelaunchRaceLost
		}
		r.PriorRawRunDir = r.RawRunDir
		r.RawRunDir = runDir
		r.RawOwnership = runID
		r.Attempt++
		r.RelaunchCount++
		r.RelaunchReserved = false
		return nil
	})
}

// sliceResult is one slice's decision: the outcome/cause to persist and, on the
// single admitted relaunch, the new raw run identity. lastClock is the freshly
// accepted clock value bound to the record.
type sliceResult struct {
	outcome   Outcome
	cause     string
	rawRunDir string // PASSED only

	// relaunchRaceLost reports that a same-owner competitor already owns this
	// drive's single automatic replacement: it either reserved/consumed the
	// relaunch (errRelaunchRaceLost) or settled the drive terminally before this
	// caller's reservation CAS (errAlreadyTerminal). The loser reloads the
	// authoritative record and returns it, issuing no backend launch and mutating
	// no attempt/relaunch state.
	relaunchRaceLost bool
	err              error

	lastClock time.Time
}

// driveSlice observes the drive's live raw run for at most one slice and maps
// the native observation to a typed outcome. The mapping fails closed: only an
// exact running state is retryable, a pass is accepted only after the
// fingerprint revalidates, a death earns at most one relaunch under the five
// conjoined conditions, and every other uncertainty HALTs (never red). rec is
// read-only; the persisted mutations travel back in the sliceResult.
func (d *Driver) driveSlice(id, ownerGen string, rec driveRecord, claim *relaunchClaim) sliceResult {
	sliceStart := d.clock.Now()
	runDir := rec.RawRunDir
	relaunchUsed := rec.RelaunchCount > 0
	res := sliceResult{lastClock: sliceStart}

	for {
		now := d.clock.Now()
		res.lastClock = now

		observation, err := d.proc.Observe(runDir)
		if err != nil {
			// An unreadable observation is fail-closed: HALT, never a guessed state.
			return halt(&res, CauseObservationUnreadable)
		}

		switch observation.State {
		case process.StateRunning:
			// Time governs a live run. Terminal states above never reach here, so
			// only a live run consults the clock and the slice bound.
			expired, backward := rec.deadlineState(now)
			if backward {
				// A backward clock jump could lengthen the budget; distrust it and
				// stop the tree we can still prove we own.
				d.stopIfOwned(runDir)
				return halt(&res, "clock-backward")
			}
			if expired {
				// Deadline expired with a live run: stop and HALT. No relaunch is
				// earned. Stop's own outcome IS the re-observed result (it re-reads
				// the durable terminal record), so no separate observation is taken
				// — a zero budget therefore takes exactly one observation before the
				// stop (deadline == start expires the first observation). A stop
				// that cannot prove ownership says so in the cause.
				if _, serr := d.proc.Stop(runDir, "gatedrive-halt"); serr != nil {
					return halt(&res, "deadline-expired-stop-unproven")
				}
				return halt(&res, CauseDeadlineExpired)
			}
			if d.clock.Since(sliceStart) >= d.slice {
				// The slice ended with the run still live: record the observation
				// and WAIT. No shell monitor, sleep loop, or notification.
				res.outcome = WAITING
				return res
			}
			d.sleep(d.pollInterval)
			continue

		case process.StatePassed:
			// A pass certifies the exact drive-start bytes: revalidate the
			// fingerprint before accepting it; any drift stops-if-owned and HALTs,
			// never red.
			cur, ferr := ComputeFingerprint(rec.WorktreePath, d.git)
			if ferr != nil {
				d.stopIfOwned(runDir)
				return halt(&res, "fingerprint-error")
			}
			if !cur.Equal(rec.Fingerprint) {
				d.stopIfOwned(runDir)
				return halt(&res, "worktree-changed")
			}
			res.outcome = PASSED
			res.rawRunDir = observation.RunDir
			return res

		case process.StateFailed:
			// The suite itself completed red — the only path to FAILED.
			res.outcome = FAILED
			return res

		case process.StateStopped:
			// A stop this drive did not initiate as a continuing transition is a
			// fail-closed HALT, never red.
			return halt(&res, "stopped-not-initiated")

		case process.StateSignaled, process.StateVanished:
			// A death without a verdict. Prove no owned tree survives, then admit
			// the single relaunch only if every condition holds.
			gone, derr := d.proveNoTreeSurvives(runDir, observation)
			if derr != nil || !gone {
				return halt(&res, "uncertain-ownership")
			}
			if relaunchUsed {
				return halt(&res, "relaunch-exhausted")
			}
			if refusal := d.relaunchRefusal(&rec, now); refusal != "" {
				return halt(&res, refusal)
			}
			if claim == nil {
				// Fence the single automatic relaunch behind the run launch gate: the
				// reservation commits while the run registry lock is held, so a
				// concurrent cancellation either lands first (nothing is reserved) or
				// observes the held claim. A lost linkage or a revoked run refuses
				// with a HALT cause and launches nothing (change 0437 Task 3).
				var haltCause string
				var err error
				claim, haltCause, err = d.authorizeRelaunch(id, ownerGen, rec)
				if haltCause != "" {
					return halt(&res, haltCause)
				}
				if err != nil {
					// A same-owner competitor may have consumed the relaunch
					// (errRelaunchRaceLost) or already settled the drive
					// terminally before this reservation CAS (errAlreadyTerminal).
					// Either way the loser reloads and returns the authoritative
					// recorded state; it never launches, spends no attempt, and
					// surfaces no error — mirroring the attach-race branch below.
					if errors.Is(err, errRelaunchRaceLost) || errors.Is(err, errAlreadyTerminal) {
						res.relaunchRaceLost = true
						return res
					}
					res.err = err
					return res
				}
				rec.RelaunchReserved = true
				rec.RelaunchToken = claim.token
			}
			out, lerr := d.proc.Launch(rec.launchRequestWithReservation(claim.token))
			if lerr != nil {
				resolution, rerr := d.proc.ResolveReservation(rec.RunRoot, claim.token)
				if rerr != nil || resolution == nil || resolution.Disposition == "unresolved" {
					claim.close()
					return halt(&res, "launch-unconfirmed")
				}
				if resolution.Disposition != "identified" || resolution.RunID == "" || resolution.RunDir == "" {
					claim.close()
					return halt(&res, "relaunch-failed")
				}
				out = &process.LaunchOutcome{RunID: resolution.RunID, RunDir: resolution.RunDir, State: resolution.State}
			}
			if err := d.attachReservedRelaunch(id, ownerGen, claim, out.RunDir, out.RunID); err != nil {
				claim.close()
				d.stopIfOwned(out.RunDir)
				if errors.Is(err, errRelaunchRaceLost) || errors.Is(err, errAlreadyTerminal) {
					res.relaunchRaceLost = true
					return res
				}
				res.err = err
				return res
			}
			claim.close()
			claim = nil
			cur, err := d.store.Load(id)
			if err != nil {
				res.err = err
				return res
			}
			rec = cur
			// The second raw run belongs to the same drive and deadline. It was
			// never started alongside the first (proven gone above). Continue the
			// same slice observing the new run.
			runDir = out.RunDir
			relaunchUsed = true
			continue

		default:
			// An unrecognized native state fails closed.
			return halt(&res, CauseUnknownObservation)
		}
	}
}

// halt stamps a HALTED outcome and cause onto res and returns it.
func halt(res *sliceResult, cause string) sliceResult {
	res.outcome = HALTED
	res.cause = cause
	return *res
}

// proveNoTreeSurvives establishes that no owned process tree survives a death
// before any relaunch is considered (spec "Death and the single relaunch"). A
// vanished observation already proves the supervisor is gone with no terminal to
// consume. A signaled run is already terminal: an already-terminal stop no-op
// confirms it, and a re-observe consumes that terminal state before deciding. A
// stop that cannot prove ownership (an error) leaves the outcome uncertain.
func (d *Driver) proveNoTreeSurvives(runDir string, observation *process.Observation) (bool, error) {
	if observation.State == process.StateVanished {
		return true, nil
	}
	if _, err := d.proc.Stop(runDir, "gatedrive-death-probe"); err != nil {
		return false, err
	}
	if _, err := d.proc.Observe(runDir); err != nil {
		return false, err
	}
	return true, nil
}

// relaunchRefusal returns the typed reason a second launch is refused, or "" when
// all admittable conditions hold. It checks conditions 1 (idempotent gate), 5
// (deadline remains), and 4 (worktree identity still matches, recomputed here).
// Condition 2 (former tree proven gone) is established by proveNoTreeSurvives and
// condition 3 (no prior relaunch) by the caller's relaunchUsed. Command,
// configuration, and environment identity are intrinsic to the immutable record
// and unchanged within a drive; a live environment re-hash belongs to the
// application seam that resolves config/env (Task 9).
func (d *Driver) relaunchRefusal(rec *driveRecord, now time.Time) string {
	if rec.RelaunchCount > 0 {
		return "relaunch-exhausted"
	}
	if !rec.IdempotentSuiteGate {
		return "not-idempotent"
	}
	if expired, _ := rec.deadlineState(now); expired {
		return CauseDeadlineExpired
	}
	cur, err := ComputeFingerprint(rec.WorktreePath, d.git)
	if err != nil {
		return "fingerprint-error"
	}
	if !cur.Equal(rec.Fingerprint) {
		return "worktree-changed"
	}
	return ""
}

// stopIfOwned issues a best-effort stop of a run this drive owns and reports
// whether the stop was performed or the run was already terminal (an ownership-
// proven no-op). It is used at fail-closed boundaries — a drifted pass, a
// backward clock, a deadline expiry — where a live owned tree must not leak. A
// stop it cannot prove ownership for returns false so the caller can say so.
func (d *Driver) stopIfOwned(runDir string) bool {
	out, err := d.proc.Stop(runDir, "gatedrive-halt")
	return err == nil && out != nil
}

// recordedDoc builds the outcome document from an authoritative persisted record
// on a path that ran NO slot release (a busy relaunch claim, a crash-window
// reservation settled HALTED, an acknowledgement). Only PASSED exposes the raw run
// dir. PASSED/FAILED expose the private run root (the supervisor wrote its terminal
// record, so the process has ended); HALTED withholds it, because nothing on such a
// path proved the slot's teardown (change 0446). A path that just ran the release
// uses settledTerminalDoc, which gates the root on what the release proved.
func (d *Driver) recordedDoc(id, ownerGen string, rec driveRecord) DriveDoc {
	return d.recordedDocWithRoot(id, ownerGen, rec, rec.LastOutcome != HALTED)
}

// recordedDocWithRoot is recordedDoc with the run-root exposure made explicit:
// exposeRoot is the caller's statement that the drive's slot release/teardown
// evidence is settled. A terminal document carries RunRoot only when it is true.
func (d *Driver) recordedDocWithRoot(id, ownerGen string, rec driveRecord, exposeRoot bool) DriveDoc {
	doc := DriveDoc{
		ProtocolVersion: ProtocolVersion,
		DriveID:         id,
		Generation:      ownerGen,
		Attempt:         rec.Attempt,
		Deadline:        rec.Deadline,
		Outcome:         rec.LastOutcome,
		Cause:           rec.LastCause,
	}
	if rec.LastOutcome == PASSED {
		doc.RawRunDir = rec.RawRunDir
	}
	// A terminal document exposes the private run root so the owning caller that
	// minted it removes it at the terminal (WAITING retains it — a relaunch may
	// still replay under it) — but only once release evidence is settled
	// (exposeRoot). haltDoc paths (empty or stale-owner records) never reach here,
	// so a superseded owner never deletes a live drive's root.
	if isTerminalOutcome(rec.LastOutcome) && exposeRoot {
		doc.RunRoot = rec.RunRoot
	}
	return doc
}

// haltDoc builds a HALTED document for a boundary reached before (or without) a
// persisted transition — a stale owner or an unusable record. It carries the
// identity it can prove and never exposes a raw run dir.
func (d *Driver) haltDoc(id, ownerGen string, rec driveRecord, cause string) DriveDoc {
	return DriveDoc{
		ProtocolVersion: ProtocolVersion,
		DriveID:         id,
		Generation:      ownerGen,
		Attempt:         rec.Attempt,
		Deadline:        rec.Deadline,
		Outcome:         HALTED,
		Cause:           cause,
	}
}

// isTerminalOutcome reports whether an outcome is a settled verdict. WAITING is
// the sole nonterminal outcome; PASSED/FAILED/HALTED are terminal and make a
// re-advance idempotent.
func isTerminalOutcome(o Outcome) bool {
	return o == PASSED || o == FAILED || o == HALTED
}

// launchRequest builds the deterministic raw launch input the drive replays on
// the single relaunch: the same allocation root, working directory, and argv.
// ReservationToken carries the drive's worktree admission token so a lost launch
// response is resolvable to this exact run (ResolveReservation, change 0375); it
// is empty for a legacy drive that admitted through no slot, which the process
// backend accepts as an unset optional token.
func (rec *driveRecord) launchRequest() process.LaunchRequest {
	return rec.launchRequestWithReservation(rec.AdmissionToken)
}

func (rec *driveRecord) launchRequestWithReservation(token string) process.LaunchRequest {
	return process.LaunchRequest{
		Root:             rec.RunRoot,
		Cwd:              rec.Cwd,
		Argv:             rec.Command,
		ReservationToken: token,
	}
}
