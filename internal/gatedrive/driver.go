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
	// Launch starts one detached native-supervisor run and returns its handle,
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
	// or unresolved (Task 2). The scoped-start launch-failure leg consults it to
	// decide whether to release the worktree execution slot (a proven
	// never-launched) or fail it closed to unresolved.
	ResolveReservation(root, token string) (*process.ReservationResolution, error)
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

	// Recovery-scope linkage (all optional; empty = a scopeless drive, the
	// pre-0359 behavior). ScopeID + ChildCapability enroll this new drive in a
	// recovery scope so a parent can take it over later (scope.go, takeover.go):
	// Start verifies the capability and scope identity BEFORE launching, reserves
	// the scope's single slot durably, and only then launches. ChildCapability is
	// the RAW capability, verified against the scope's stored hash and persisted
	// nowhere. GateContext is the RAW outer child-context token linking a nested
	// drive to the outer gate; it is stored only as its sha256 hash
	// (GateContextHash). (change 0359)
	ScopeID         string
	ChildCapability string
	GateContext     string

	// RunEpochID links this drive's top-level execution to the workflow run epoch
	// (rungate_epoch.go) the arming gate minted, and is recorded on the worktree
	// execution slot the start reserves (admission.go). It is a LOCATOR, never a
	// credential: it authorizes nothing (the scope's child capability carries
	// authority), but it fences the worktree — a later gate in the same worktree that
	// does not carry this epoch is refused ErrStaleRunEpoch, so an omitted or stale
	// epoch cannot detach a workflow-owned worktree from its epoch. Empty for a
	// standalone gate that owns no implementation epoch (finalize's local gate, an
	// ad-hoc task drive). (change 0375 Task 9)
	RunEpochID string

	// Recovery-scope successor receipt (change 0405 Task 4): the previous drive's
	// id and its current owner generation, captured from that drive's response. BOTH
	// are required together for a successor start over an occupied scope slot and
	// BOTH are forbidden for a scope's first start (a half-filled receipt is a
	// fail-closed ErrStalePredecessor). A successor acknowledges exactly this
	// predecessor result — retiring its recovery authority so it survives only as
	// history — before launching its own new drive over the same scope slot. A scope
	// carries a SEQUENCE of drives (baseline, RED, GREEN, verification) through one
	// slot; the receipt is the explicit hand-off between one drive and the next.
	PredecessorDriveID  string
	PredecessorOwnerGen string
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

// reserveWorktreeExecution inventories legacy records with this driver's exact
// process observer before a new slot is durably reserved.
func (d *Driver) reserveWorktreeExecution(rec admissionRecord) (string, error) {
	return d.store.reserveWorktreeExecution(rec, d.proc.Observe)
}

// NewSystemDriver builds a production Driver over the real monotonic clock and
// the real git seam, composing the given store and process seam. The application
// service seam (internal/app) uses it so an in-process caller composes the same
// state machine the CLI drives, without shelling out to docket's own CLI.
func NewSystemDriver(store *Store, proc ProcessSeam) *Driver {
	return NewDriver(store, systemClock{}, proc, realGit{})
}

// AdmissionTicket is the opaque result of Admit: a worktree execution slot (and,
// for a scoped start, its scope slot) durably reserved and a RESERVED drive record
// persisted, but with NO process launched yet. StartAdmitted consumes the ticket
// to launch the raw run and drive one slice; AbandonAdmission releases it when the
// caller decides between admission and launch not to proceed.
//
// The two-phase split exists so a caller can interpose exactly one accounting
// decision BETWEEN admission and launch: the BUILD owner reserves one full-suite
// attempt only after Admit succeeds, so a refused admission (worktree-busy,
// unresolved, scope-busy) charges nothing, while a launch/persistence failure in
// StartAdmitted keeps the charge — admission precedes charging, and once charged
// there are no refunds (change 0375 Task 8, ADR-0116). Every other caller composes
// the two through the thin Start.
type AdmissionTicket struct {
	// scoped selects StartAdmitted's launch path; the rest carry the reserved state.
	scoped   bool
	id       string
	ownerGen string
	rec      driveRecord
	token    string
	// reservedFresh reports that THIS start minted the worktree reservation (a
	// scoped start may instead reuse a same-scope peer's slot). ownsSlot reports
	// that the slot is still RESERVED and this start must confirm it to executing.
	// Both are meaningful only for a scoped ticket; a scopeless start always
	// freshly reserves and always confirms its own slot.
	reservedFresh bool
	ownsSlot      bool
}

// Start creates a drive, validates and fingerprints the execution context,
// launches the first raw run through the process seam, persists the drive
// identity, and advances through at most one slice — returning the same typed
// outcome document Advance returns. A malformed request or a launch failure is a
// command failure (an error), not a drive outcome.
//
// Start is the thin composition of Admit (the pre-launch admission half) and
// StartAdmitted (the launch half) for callers that charge nothing between the two
// — finalize's local gate, the commandless resumption service, and the
// task-intent owner. The BUILD owner composes the two halves itself so it can
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
// fingerprints the execution context, reserves the worktree execution slot (and,
// for a scoped start, the scope's single slot and the journaled predecessor
// retirement), and persists a RESERVED drive record — but launches NO process.
// Every refusal here (a malformed request, a scope pre-check rejection, a
// worktree-busy / unresolved-execution / scope-busy slot, or a lost scope
// reservation) returns a typed error and reserves nothing the caller must later
// account for: no process was launched, and any freshly minted worktree slot with
// no adopter is released before returning. On success it returns the ticket
// StartAdmitted (or AbandonAdmission) consumes.
func (d *Driver) Admit(req StartRequest) (*AdmissionTicket, error) {
	if len(req.Command) == 0 || req.Command[0] == "" {
		return nil, fmt.Errorf("gatedrive: start requires a non-empty command")
	}
	if req.Budget < 0 {
		return nil, fmt.Errorf("gatedrive: start requires a non-negative budget")
	}

	// Scope pre-check BEFORE any launch: an uncontended bad request short-circuits
	// here so proc.Launch is never reached on it and, crucially, so it consumes
	// nothing — no reserved drive record is minted and a legitimate predecessor
	// keeps its recovery authority. This unlocked block is a fast-fail ONLY;
	// reserveScopeDrive and retirePredecessor under their locks (below) are the
	// AUTHORITY that re-check every condition and arbitrate races, so a state
	// observed here but changed by a concurrent transition is caught there.
	if req.ScopeID != "" {
		if err := d.precheckScopedStart(req); err != nil {
			return nil, err
		}
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
	// Stamp the recovery-scope linkage onto the record (both empty for a scopeless
	// drive). ScopeID links the drive to the scope its owner was dispatched under;
	// GateContextHash links a nested drive to the outer gate.
	if req.ScopeID != "" {
		rec.ScopeID = req.ScopeID
	}
	if req.GateContext != "" {
		rec.GateContextHash = capHash(req.GateContext)
	}

	if req.ScopeID == "" {
		return d.admitScopeless(rec, ownerGen, req.RunEpochID)
	}
	return d.admitScoped(req, rec, ownerGen)
}

// StartAdmitted performs the launch half of a start: it launches the admitted
// drive's raw run, persists the launch handle, confirms the scope and worktree
// slots, and advances through at most one slice — returning the same typed outcome
// document Advance returns. A launch or persistence failure is a command failure
// (an error), not a drive outcome, and — because the caller may already have
// charged a suite attempt against this ticket — it never refunds: it either proves
// the fresh process stopped and releases the worktree slot, or marks the slot
// unresolved and fails future admission closed.
func (d *Driver) StartAdmitted(t *AdmissionTicket) (DriveDoc, error) {
	if t == nil {
		return DriveDoc{}, fmt.Errorf("gatedrive: StartAdmitted requires an admission ticket")
	}
	if t.scoped {
		return d.launchScoped(t)
	}
	return d.launchScopeless(t)
}

// AbandonAdmission releases an admission the caller decided, between Admit and
// StartAdmitted, not to launch (change 0375 Task 8: a build-owned start whose
// full-suite attempt could not be reserved after admission). Because no process
// was launched, the reserved drive record is a pure orphan (removed so it is not a
// spurious recovery candidate) and the worktree slot THIS start freshly reserved
// is provably idle (released). A start that reused an incumbent same-scope slot
// never disturbs the peer that owns it. AbandonAdmission is only reachable when the
// advisory budget precheck (gate_drive.go) was raced or a store fault intervened;
// it is deliberately best-effort and fail-closed, never a refund path.
func (d *Driver) AbandonAdmission(t *AdmissionTicket) error {
	if t == nil {
		return nil
	}
	_ = d.store.removeReservedDrive(t.id)
	// A scopeless start always freshly reserves its own slot; a scoped start
	// releases only the slot it minted (reservedFresh), never a peer's.
	if (t.scoped && !t.reservedFresh) || t.token == "" {
		return nil
	}
	return d.store.ReleaseWorktreeExecution(t.rec.WorktreePath, t.token)
}

// precheckScopedStart is the unlocked fast-fail gate for a scoped Start. It is NOT
// the authority — reserveScopeDrive (the slot) and retirePredecessor (the
// predecessor's recovery authority) re-check every condition under their locks and
// arbitrate races — but it rejects an uncontended bad request before any reserved
// drive record is minted or any process is launched, so an ordinary invalid request
// consumes nothing and a legitimate predecessor is never touched.
//
// A first start (empty receipt) succeeds only against an empty slot; an occupied
// slot is ErrScopeBusy (a reserved/in-flight or launch-failed start owns it, so no
// automatic second launch) or ErrScopeSecondDrive (a launched drive with no
// successor receipt). A successor start (both receipt fields set) must present the
// scope's complete pinned identity, name a scope whose slot holds a launched current
// drive, and name a predecessor with a durable PASSED/FAILED result still owned by
// the presented generation and carrying no outstanding handoff. A half-filled
// receipt is a fail-closed ErrStalePredecessor.
func (d *Driver) precheckScopedStart(req StartRequest) error {
	scope, err := d.store.LoadScope(req.ScopeID)
	if err != nil {
		return err
	}
	if scope.Closed {
		return ownershipErr(ErrScopeClosed, "start")
	}
	if req.ChildCapability == "" || scope.ChildCapHash != capHash(req.ChildCapability) {
		return ownershipErr(ErrScopeCapabilityMismatch, "start")
	}

	receipt := predecessorReceipt{DriveID: req.PredecessorDriveID, OwnerGen: req.PredecessorOwnerGen}
	if receipt.halfFilled() {
		return ownershipErr(ErrStalePredecessor, "start")
	}

	if receipt.empty() {
		// First start: an occupied slot short-circuits with the same typed rejection
		// reserveScopeDrive returns for an empty receipt, and identity is checked only
		// against an empty slot (a first start still fixes the scope's identity).
		if scope.CurrentDriveID != "" {
			if scope.CurrentDriveState == scopeStateReserved {
				return ownershipErr(ErrScopeBusy, "start")
			}
			return ownershipErr(ErrScopeSecondDrive, "start")
		}
		if !scopedIdentityMatch(scope, req) {
			return ownershipErr(ErrScopeIdentityMismatch, "start")
		}
		return nil
	}

	// Successor start: the complete pinned identity, a launched current slot, and a
	// durable reusable predecessor the receipt names.
	if !scopedIdentityMatch(scope, req) {
		return ownershipErr(ErrScopeIdentityMismatch, "start")
	}
	if scope.CurrentDriveID == "" {
		// A successor acknowledges a predecessor result, but the scope holds none.
		return ownershipErr(ErrStalePredecessor, "start")
	}
	if scope.CurrentDriveState == scopeStateReserved {
		return ownershipErr(ErrScopeBusy, "start")
	}
	if scope.PendingAckDriveID != "" {
		return ownershipErr(ErrUnresolvedLaunchTransition, "start")
	}
	// Validate the CLAIMED predecessor record (cheap, consumes nothing). Whether it is
	// the scope's CURRENT drive is reserveScopeDrive's authority — a wrong id there is
	// refused without consuming the slot; here we reject a predecessor that is not a
	// durable reusable result up front. A receipt naming a drive that cannot be loaded
	// is not a reusable predecessor.
	prec, lerr := d.store.Load(receipt.DriveID)
	if lerr != nil {
		if _, ok := AsStoreError(lerr); ok {
			return ownershipErr(ErrStalePredecessor, "start")
		}
		return lerr
	}
	return predecessorReusableError(&prec, receipt.OwnerGen)
}

// scopedIdentityMatch reports whether a scoped Start request carries the scope's
// complete pinned identity: the repo/branch/worktree/change/task/phase bundle
// scopeIdentityMatch checks, plus the gate-context token when the scope pinned one
// (Invariant 6 — omission or alteration must not detach a drive from outer
// recovery). A scope that pinned no gate context accepts any (the pre-0359 default).
func scopedIdentityMatch(scope scopeRecord, req StartRequest) bool {
	if !scopeIdentityMatch(scope, req.RepoDir, req.Branch, req.Worktree, req.ChangeID, req.TaskID, req.Phase) {
		return false
	}
	if scope.GateContextHash != "" && capHash(req.GateContext) != scope.GateContextHash {
		return false
	}
	return true
}

// admitScopeless runs the pre-launch admission half for a gate without a recovery
// scope (for example, finalize's local gate):
//
//	ReserveWorktreeExecution -> NewReservedDrive.
//
// RunRoot only scopes process-supervisor allocation. Worktree admission still
// keys on WorktreePath, so independent scopeless callers cannot use distinct
// private run roots to launch concurrently against one worktree. It launches no
// process; launchScopeless does. A NewReservedDrive failure releases the freshly
// reserved slot before returning, so a refused admission leaks nothing.
func (d *Driver) admitScopeless(rec driveRecord, ownerGen, runEpochID string) (*AdmissionTicket, error) {
	token, err := d.reserveWorktreeExecution(admissionRecord{
		RepoIdentity: rec.RepoIdentity,
		WorktreeRoot: rec.WorktreePath,
		RunEpochID:   runEpochID,
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
	return &AdmissionTicket{id: id, ownerGen: ownerGen, rec: rec, token: token}, nil
}

// launchScopeless runs the launch half for a scopeless admission:
//
//	Launch -> attachLaunch -> ConfirmWorktreeExecution -> driveAndPersist.
//
// Every post-launch failure either proves the fresh process stopped before
// releasing the slot, or marks the slot unresolved and fails future admission
// closed.
func (d *Driver) launchScopeless(t *AdmissionTicket) (DriveDoc, error) {
	rec := t.rec
	id := t.id
	ownerGen := t.ownerGen
	token := t.token

	out, lerr := d.proc.Launch(rec.launchRequest())
	if lerr != nil {
		// Keep the attempted drive as durable recovery evidence, mirroring the
		// scoped launch-failure leg. ResolveReservation is the only proof that
		// an error response means no process was launched; every other outcome
		// leaves the worktree slot unresolved.
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

	rec.RawRunDir = out.RunDir
	rec.RawOwnership = out.RunID
	return d.driveAndPersist(id, ownerGen, rec)
}

// admitScoped runs the pre-launch admission half of the pinned scoped-start order
// (change 0405 Tasks 3–4 plus change 0375 worktree admission):
//
//	ReserveWorktreeExecution → NewReservedDrive → reserveScopeDrive →
//	(successor: retirePredecessor → clearPendingAck).
//
// The worktree execution slot (admission.go) is the OUTERMOST admission authority:
// one canonical worktree carries at most one reserved-or-running top-level gate
// execution across DIFFERENT scopes, scopeless starts, and raw launches. A start
// belonging to the SAME scope as the incumbent slot — a concurrent first-start peer,
// or a successor continuing this scope's sequence — REUSES the slot the scope
// already holds rather than reserving a second one, so same-scope arbitration stays
// at the scope slot (reserveScopeDrive) and only a genuine cross-scope/scopeless/raw
// overlap is refused ErrWorktreeBusy (admitScopedWorktree). The durable reservation
// precedes the process (launchScoped), so a crash or failure between reservation and
// launch leaves a recoverable slot rather than a silently double-launched one, and
// every ambiguous launch/persist failure fails closed with NO automatic second
// launch. A first start passes an empty receipt; a successor passes the predecessor
// receipt so reserveScopeDrive advances the slot and the journaled retire/clear pair
// retires the predecessor as one logical transition. Every admission-half failure
// leg releases the freshly reserved worktree slot (unless a same-scope peer adopted
// it) and removes the orphan reserved record, so a refused admission leaks nothing.
func (d *Driver) admitScoped(req StartRequest, rec driveRecord, ownerGen string) (*AdmissionTicket, error) {
	receipt := predecessorReceipt{DriveID: req.PredecessorDriveID, OwnerGen: req.PredecessorOwnerGen}

	// WORKTREE ADMISSION. reservedFresh marks whether THIS start minted the
	// reservation (so a genuine pre-launch failure with no adopter releases it, while
	// a same-scope race loss leaves the slot to the peer that adopted it). ownsSlot
	// marks whether the slot is still RESERVED and this start must confirm it to
	// executing and owns its post-launch failure legs; a successor reusing an
	// already-executing slot must not re-confirm or disturb it.
	token, reservedFresh, ownsSlot, aerr := d.admitScopedWorktree(req)
	if aerr != nil {
		return nil, aerr
	}
	rec.AdmissionToken = token

	// Persist a RESERVED drive record (no launch handle), then durably reserve the
	// scope's single slot. reserveScopeDrive under the scope lock is the authority
	// that arbitrates the SCOPE slot: for a first start it fills an empty slot, for a
	// successor it advances the slot only when the receipt names the current launched
	// drive (and journals the pending ack). A reserve failure (a concurrent start won
	// the slot, a stale receipt, the scope closed, an unresolved transition, or a
	// capability change) means this start owns nothing and never launched, so surface
	// the typed rejection.
	id, _, err := d.store.NewReservedDrive(rec)
	if err != nil {
		if reservedFresh {
			_ = d.store.ReleaseWorktreeExecution(req.Worktree, token)
		}
		return nil, err
	}
	if rerr := d.store.reserveScopeDrive(req.ScopeID, req.ChildCapability, id, receipt); rerr != nil {
		// The reservation never won the slot, so the just-minted reserved record is a
		// pure orphan (no launch, no scope binding). Remove it best-effort so its
		// nonterminal outcome never lingers as a spurious FindScopeDriveIDs recovery
		// candidate that would fail an outer takeover closed on ambiguity (removeReservedDrive).
		_ = d.store.removeReservedDrive(id)
		// Release the worktree slot ONLY when THIS start freshly reserved it AND the
		// loss is not a same-scope race: a same-scope peer that beat us to the scope slot
		// has adopted our reservation (there is at most one fresh reservation per worktree
		// at a time), so releasing it would free a slot the winner is using. A genuine
		// failure (scope closed, an IO fault, an identity mismatch) has no adopter, so the
		// fresh reservation must be released rather than leaked.
		if reservedFresh && !isSameScopeRaceLoss(rerr) {
			_ = d.store.ReleaseWorktreeExecution(req.Worktree, token)
		}
		return nil, rerr
	}

	// Successor: retire the predecessor's recovery authority and clear the pending-ack
	// journal as the journaled second half of this one logical transition — both AFTER
	// a won reservation and BEFORE any launch. A failure here is an ambiguous
	// launch/persistence transition (a concurrent transition moved the predecessor
	// between the unlocked pre-check and this locked retirement): fail closed
	// ErrUnresolvedLaunchTransition with the reservation retained and nothing launched.
	// The exit is parent recovery, never a blind retry or a fabricated second launch.
	if !receipt.empty() {
		if rerr := d.store.retirePredecessor(receipt.DriveID, receipt.OwnerGen); rerr != nil {
			// The reservation won the slot but the predecessor could not be retired: the
			// slot is left reserved+pending-ack, an unresolved launch transition every
			// consumer (Start, Takeover, Acknowledge) fails closed on WITHOUT loading the
			// reserved record. So removing that never-launched record severs no live
			// recovery; it only spares outer enumeration a spurious candidate (removeReservedDrive).
			_ = d.store.removeReservedDrive(id)
			if reservedFresh {
				_ = d.store.ReleaseWorktreeExecution(req.Worktree, token)
			}
			return nil, ownershipErr(ErrUnresolvedLaunchTransition, "start")
		}
		if cerr := d.store.clearPendingAck(req.ScopeID, receipt.DriveID); cerr != nil {
			_ = d.store.removeReservedDrive(id)
			if reservedFresh {
				_ = d.store.ReleaseWorktreeExecution(req.Worktree, token)
			}
			return nil, ownershipErr(ErrUnresolvedLaunchTransition, "start")
		}
	}

	return &AdmissionTicket{
		scoped:        true,
		id:            id,
		ownerGen:      ownerGen,
		rec:           rec,
		token:         token,
		reservedFresh: reservedFresh,
		ownsSlot:      ownsSlot,
	}, nil
}

// launchScoped runs the launch half of the pinned scoped-start order:
//
//	Launch → attachLaunch → confirmScopeLaunch → ConfirmWorktreeExecution →
//	driveAndPersist.
//
// Every ambiguous launch/persist failure fails closed with NO automatic second
// launch. The scope slot stays durably reserved so a subsequent start is refused
// rather than launching a duplicate; the worktree slot this start owns is released
// only on proven teardown/never-launched and marked unresolved otherwise.
func (d *Driver) launchScoped(t *AdmissionTicket) (DriveDoc, error) {
	rec := t.rec
	id := t.id
	ownerGen := t.ownerGen
	token := t.token
	ownsSlot := t.ownsSlot
	worktree := rec.WorktreePath
	scopeID := rec.ScopeID

	// Launch the raw run. A launch failure is a command failure that leaves the scope
	// slot durably reserved (no automatic second launch): persist the drive HALTED so
	// outer recovery can see a terminal-unconsumed record. For the worktree slot this
	// start owns, ResolveReservation decides: a proven never-launched releases it (the
	// worktree is genuinely free), any other verdict fails it closed to unresolved so
	// admission blocks until recovery. No fabricated verdict document flows.
	out, lerr := d.proc.Launch(rec.launchRequest())
	if lerr != nil {
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
		if ownsSlot {
			d.resolveWorktreeAfterLaunchFailure(worktree, rec.RunRoot, token)
		}
		return DriveDoc{}, fmt.Errorf("gatedrive: start launch: %w", lerr)
	}

	// Persist the launch handle onto the reserved record, then confirm the scope slot's
	// launch. On a persist failure the freshly launched run is orphaned: stop it
	// best-effort. The scope slot stays reserved (never treated as empty), so a
	// subsequent start is refused rather than launching a duplicate. For the worktree
	// slot this start owns, a proven stop releases it; an unproven stop fails it closed
	// to unresolved (a possibly-live process never frees the slot).
	if aerr := d.store.attachLaunch(id, ownerGen, out.RunDir, out.RunID); aerr != nil {
		stopped := d.stopIfOwned(out.RunDir)
		if ownsSlot {
			d.releaseOrUnresolveWorktree(worktree, token, stopped)
		}
		return DriveDoc{}, aerr
	}
	if cerr := d.store.confirmScopeLaunch(scopeID, id); cerr != nil {
		stopped := d.stopIfOwned(out.RunDir)
		if ownsSlot {
			d.releaseOrUnresolveWorktree(worktree, token, stopped)
		}
		return DriveDoc{}, cerr
	}

	// Confirm the worktree slot to executing, attaching the raw run identity — but ONLY
	// for the start that owns the reserved slot. A successor reusing an already-executing
	// slot leaves it untouched (its stored run identity is the current sequence's, and a
	// re-confirm with a new run id would be a fail-closed unresolved transition). A confirm
	// failure cannot prove the run torn down, so it stops-if-owned and fails the slot
	// closed the same way a persist failure does.
	if ownsSlot {
		if cerr := d.store.ConfirmWorktreeExecution(worktree, token, out.RunID, out.RunDir); cerr != nil {
			stopped := d.stopIfOwned(out.RunDir)
			d.releaseOrUnresolveWorktree(worktree, token, stopped)
			return DriveDoc{}, cerr
		}
	}

	rec.RawRunDir = out.RunDir
	rec.RawOwnership = out.RunID
	return d.driveAndPersist(id, ownerGen, rec)
}

// admitScopedWorktree reserves (or reuses) the worktree execution slot for a scoped
// start and reports how the start relates to it. It returns the reservation token to
// thread into the launch, whether THIS start freshly reserved the slot (reservedFresh)
// and whether the slot is still RESERVED and this start must drive it to executing
// (ownsSlot).
//
// A fresh reservation is the common first-start path: the slot was absent or released.
// When the reserve is refused ErrWorktreeBusy, the slot may already be held by THIS
// scope — a concurrent same-scope first-start peer that won the reservation, or the
// predecessor whose executing slot this scope's successor continues under. Such a start
// REUSES the incumbent token so the scope slot (not the worktree slot) arbitrates
// same-scope races; a slot held by a DIFFERENT scope, or in a stopping/unresolved
// state, is a genuine cross-scope refusal returned verbatim. ErrUnresolvedExecution and
// every other error (an unresolvable worktree, an IO fault) fail closed unchanged.
func (d *Driver) admitScopedWorktree(req StartRequest) (token string, reservedFresh, ownsSlot bool, err error) {
	rec := admissionRecord{
		RepoIdentity: req.RepoDir,
		WorktreeRoot: req.Worktree,
		ScopeID:      req.ScopeID,
		RunEpochID:   req.RunEpochID,
		Kind:         "scoped",
	}
	token, rerr := d.reserveWorktreeExecution(rec)
	if rerr == nil {
		return token, true, true, nil // freshly reserved: this start confirms it
	}
	if oe, ok := AsOwnershipError(rerr); !ok || oe.Kind != ErrWorktreeBusy {
		return "", false, false, rerr // unresolved / invalid / IO: fail closed
	}
	// Busy: reuse only when the incumbent slot belongs to THIS scope.
	slot, _, lerr := d.store.LoadWorktreeExecution(req.Worktree)
	if lerr != nil {
		return "", false, false, rerr // fail closed on the original busy error
	}
	if slot.ScopeID != "" && slot.ScopeID == req.ScopeID {
		switch slot.State {
		case admissionReserved:
			return slot.ReservationToken, false, true, nil // reuse; still confirm it
		case admissionExecuting:
			return slot.ReservationToken, false, false, nil // reuse; already executing
		}
	}
	return "", false, false, rerr // cross-scope or non-reusable state: ErrWorktreeBusy
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

// isSameScopeRaceLoss reports whether a reserveScopeDrive rejection means a same-scope
// peer won the scope slot (ErrScopeBusy) or a receipt-less start raced an already-launched
// same-scope drive (ErrScopeSecondDrive) — the losses under which a same-scope peer has
// ADOPTED this start's fresh worktree reservation, so it must not be released. Every other
// rejection (a closed scope, an identity mismatch, an IO fault) has no adopter and its
// fresh reservation is released rather than leaked.
func isSameScopeRaceLoss(err error) bool {
	oe, ok := AsOwnershipError(err)
	if !ok {
		return false
	}
	return oe.Kind == ErrScopeBusy || oe.Kind == ErrScopeSecondDrive
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
		_ = d.releaseAdmissionIfProven(rec)
		return d.recordedDoc(id, ownerGen, rec), nil
	}
	var claim *relaunchClaim
	if rec.RelaunchReserved {
		var resolved *DriveDoc
		rec, claim, resolved, err = d.recoverReservedRelaunch(id, ownerGen, rec)
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
			_ = d.releaseAdmissionIfProven(cur)
			return d.recordedDoc(id, ownerGen, cur), nil
		}
		return DriveDoc{}, err
	}

	cur, err := d.store.Load(id)
	if err != nil {
		return DriveDoc{}, err
	}
	_ = d.releaseAdmissionIfProven(cur)
	return d.recordedDoc(id, ownerGen, cur), nil
}

// releaseAdmissionOnTerminal frees the drive's worktree execution slot once the
// drive has committed a PASSED or FAILED verdict, so the next top-level execution —
// a different scope, a scopeless start, or this scope's next sequential drive — can
// admit onto the same worktree. The supervisor reports PASSED/FAILED only after it
// has written its terminal record, so the child group has ended and the worktree is
// genuinely idle; the release is thus proven by the same observation that produced
// the verdict. It is deliberately narrow for change 0375 Task 3: a scopeless drive
// (no admission token) and a HALTED verdict — whose process may still be live, so
// teardown is not proven by the document alone — are left untouched, and Task 6 adds
// the explicit Observe/Stop teardown proof, the HALTED stopping/unresolved handling,
// and the legacy inventory. The release verifies the slot's reservation token and is
// idempotent under it, so a stale drive cannot free a successor's slot and a
// concurrent-writer race that both observe the terminal never double-frees.
func (d *Driver) releaseAdmissionIfProven(rec driveRecord) error {
	if rec.AdmissionToken == "" {
		return nil
	}
	if rec.LastOutcome == PASSED || rec.LastOutcome == FAILED {
		return d.store.ReleaseWorktreeExecution(rec.WorktreePath, rec.AdmissionToken)
	}
	if rec.LastOutcome != HALTED {
		return nil
	}
	// A launch failure can already have released this token after a proven
	// never-launched resolution. A later idempotent Advance must preserve that
	// historical release instead of converting it to uncertainty.
	slot, _, err := d.store.LoadWorktreeExecution(rec.WorktreePath)
	if err != nil {
		return err
	}
	if slot.ReservationToken != rec.AdmissionToken || slot.State == admissionReleased {
		return nil
	}
	if rec.RawRunDir == "" {
		return d.store.MarkWorktreeExecutionUnresolved(rec.WorktreePath, rec.AdmissionToken)
	}
	stopped, serr := d.proc.Stop(rec.RawRunDir, "gatedrive-halt-release")
	if serr != nil {
		return d.store.MarkWorktreeExecutionUnresolved(rec.WorktreePath, rec.AdmissionToken)
	}
	if stopped != nil && (stopped.Performed || admissionObservationProvesTeardown(&process.Observation{State: stopped.State})) {
		return d.store.ReleaseWorktreeExecution(rec.WorktreePath, rec.AdmissionToken)
	}
	return d.store.MarkWorktreeExecutionStopping(rec.WorktreePath, rec.AdmissionToken)
}

// errAlreadyTerminal is a sentinel used inside the persist CAS to abort a write
// over a drive a concurrent writer already finished; it never escapes as a
// workflow error.
var errAlreadyTerminal = errors.New("gatedrive: drive already terminal")

// errRelaunchRaceLost reports that another same-owner advance has already
// reserved or consumed the single automatic replacement. The loser reloads the
// authoritative drive state and never issues a second backend launch.
var errRelaunchRaceLost = errors.New("gatedrive: relaunch already consumed by a concurrent advance")

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
func (d *Driver) recoverReservedRelaunch(id, ownerGen string, rec driveRecord) (driveRecord, *relaunchClaim, *DriveDoc, error) {
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
		r.LastCause = "unresolved-execution"
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
				return halt(&res, "identity-mismatch")
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
				var err error
				claim, err = d.store.reserveRelaunch(id, ownerGen)
				if err != nil {
					if errors.Is(err, errRelaunchRaceLost) {
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
					return halt(&res, "unresolved-execution")
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
		return "identity-mismatch"
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
// (a terminal re-advance, a concurrent-writer verdict, or a just-persisted
// transition). Only PASSED exposes the raw run dir; every terminal outcome
// exposes the private run root so the owning caller can remove it at the
// terminal.
func (d *Driver) recordedDoc(id, ownerGen string, rec driveRecord) DriveDoc {
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
	// still replay under it). haltDoc paths (empty or stale-owner records) never
	// reach here, so a superseded owner never deletes a live drive's root.
	if isTerminalOutcome(rec.LastOutcome) {
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
// is empty for a drive that admitted through no slot (a scopeless drive in this
// generation), which the process backend accepts as an unset optional token.
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
