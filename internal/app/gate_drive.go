// The in-process gate-drive service seam. It is the application-layer adapter
// over the native gate driver (internal/gatedrive): finalize (Task 11) and the
// CLI adapter (Task 10) both compose the SAME state machine through this seam,
// and it MUST NOT shell out to docket's own CLI. It owns the resolved,
// authoritative-config observation budget and suite command a drive Start
// requires (never agent input), roots the durable drive store at the repository's
// Git common directory, and maps every driver outcome into the shared protocol-v1
// document — the one typed DriveDoc the CLI, this seam, and the tests share, never
// a re-flattened copy.
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
)

// Gate-drive operation names — the fixed protocol identifiers for the
// slice-bounded gate-driver operations this seam exposes.
const (
	OperationGateDriveStart        = "gate.drive.start"
	OperationGateDriveAdvance      = "gate.drive.advance"
	OperationGateDriveAcknowledge  = "gate.drive.acknowledge"
	OperationGateDriveHandoff      = "gate.drive.handoff"
	OperationGateDriveClaim        = "gate.drive.claim"
	OperationGateDrivePrepareScope = "gate.drive.prepare-scope"
	OperationGateDriveTakeover     = "gate.drive.takeover"
)

// GateDriveResult is the protocol document for the gate-drive operations. It
// carries the shared gatedrive.DriveDoc verbatim on a successful operation (Drive
// is a pointer so a command failure omits it), and a bounded safe reason on a
// command failure (Reason is set only then). Because DriveDoc itself exposes
// RawRunDir on PASSED only, no mapping here can leak a raw run dir on a non-PASSED
// outcome.
type GateDriveResult struct {
	Envelope
	Drive   *gatedrive.DriveDoc `json:"drive,omitempty"`
	Reason  string              `json:"reason,omitempty"`
	Message string              `json:"message,omitempty"`
	// Stage + Locator carry the typed refusal site for an inventory refusal:
	// Stage "legacy-inventory", Locator "inventory-legacy-drive-<id>" (validated
	// id) or the safe "inventory-legacy-drives". Empty for every other refusal.
	Stage   string `json:"stage,omitempty"`
	Locator string `json:"locator,omitempty"`
	// LegacyHistory mirrors the drive document's summary onto refusals, where
	// no drive document exists.
	LegacyHistory *gatedrive.LegacyHistorySummary `json:"legacy_history,omitempty"`
}

// GateScopeResult is the protocol document for gate.drive.prepare-scope. It
// carries the scope locator and the two SEPARATE opaque capabilities in JSON,
// but its HumanText prints ONLY the scope id: the capabilities are authority and
// travel exclusively in the protocol document, never in diagnostic prose (spec
// "Capabilities and owner generations never appear in human text").
type GateScopeResult struct {
	Envelope
	ScopeID          string `json:"scope_id,omitempty"`
	ChildCapability  string `json:"child_capability,omitempty"`
	ParentCapability string `json:"parent_capability,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

// HumanText renders GateScopeResult naming ONLY the scope id (and a bounded
// reason on a command failure). The child and parent capabilities are
// deliberately omitted — they are authority, carried only in the JSON document.
func (r GateScopeResult) HumanText() string {
	var lines []string
	if r.ScopeID != "" {
		lines = append(lines, "scope_id: "+r.ScopeID)
	}
	if r.Reason != "" {
		lines = append(lines, "reason: "+r.Reason)
	}
	return strings.Join(lines, "\n")
}

// driveEngine is the native gate-drive state machine this seam maps to the
// protocol. *gatedrive.Driver satisfies it; unit tests inject a fake engine to
// prove the outcome mapping and the authoritative-config injection independent of
// a real store, process supervisor, or repository.
type driveEngine interface {
	Start(gatedrive.StartRequest) (gatedrive.DriveDoc, error)
	// Admit / StartAdmitted / AbandonAdmission are the two-phase split of Start: the
	// build owner reserves one full-suite attempt BETWEEN admission and launch so a
	// refused admission charges nothing (change 0375 Task 8). Every other owner uses
	// the thin Start.
	Admit(gatedrive.StartRequest) (*gatedrive.AdmissionTicket, error)
	StartAdmitted(*gatedrive.AdmissionTicket) (gatedrive.DriveDoc, error)
	AbandonAdmission(*gatedrive.AdmissionTicket) error
	Advance(id, ownerGen string) (gatedrive.DriveDoc, error)
	Acknowledge(scopeID, childCapability, driveID, ownerGen string) (gatedrive.DriveDoc, error)
	Handoff(id, ownerGen string) (gatedrive.DriveDoc, error)
	Claim(id, handoffID string) (gatedrive.DriveDoc, error)
	Takeover(scopeID, parentCap, driveID string) (gatedrive.DriveDoc, error)
	PrepareScope(gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error)
}

// GateDriveService is the in-process seam over the native gate driver. It owns
// the resolved observation budget, suite command, and config provenance a Start
// requires — authoritative config, never agent input — and delegates every
// operation to the composed engine.
type GateDriveService struct {
	engine     driveEngine
	budget     time.Duration
	command    string
	provenance string
	// owner names the policy that owns this drive's command ("build" or
	// "finalize"), set by the owner constructors. It is used only to compose the
	// unresolved-command refusal's human message; the commandless resumption path
	// leaves it empty, and Start (the only operation that reads a command) is never
	// reached on that path.
	owner string
	// taskIntent marks the TASK-INTENT owner (NewTaskGateDriveService): its Start
	// uses the agent-supplied argv verbatim (no /bin/sh -c wrapping and no config
	// command to resolve) and forces IdempotentSuiteGate false. argv holds that raw
	// argv. Both are empty/false for the config-owned (build/finalize) and
	// commandless services, which keep their existing command-argv and idempotency
	// behavior.
	taskIntent bool
	argv       []string
	// budgetStore + maxAttempts wire the durable per-phase suite-attempt budget the
	// BUILD owner reserves against before a change-scoped build drive is created
	// (change 0421). budgetStore is the SAME store the engine composes; maxAttempts
	// is the snapshotted build.max_attempts from the SAME authoritative config
	// resolution that resolved the build-owned command — never a second resolver.
	// Only the build owner reserves (Start keys on owner=="build"), so a
	// finalize/task/commandless service never charges an attempt even though a
	// finalize service also stores a non-nil budgetStore.
	budgetStore *gatedrive.Store
	maxAttempts int
}

// GateDriveStartRequest is the caller-supplied identity and launch context for a
// new drive. The caller supplies only the work identity and launch context; the
// SERVICE supplies the authoritative-config command, budget, and provenance, so a
// caller can never substitute the suite command or the observation budget.
type GateDriveStartRequest struct {
	RepoDir             string
	Worktree            string
	ChangeID            string
	TaskID              string
	Phase               string
	Branch              string
	Ref                 string
	Cwd                 string
	EnvHash             string
	RunRoot             string
	IdempotentSuiteGate bool
	// Scope binding (change 0359): ScopeID + ChildCapability bind the new drive
	// into a recovery scope the parent prepared, and GateContext is the raw outer
	// child-context token linking a nested drive to the outer gate. All optional;
	// empty means a scopeless drive (pre-0359 behavior). ChildCapability and
	// GateContext are raw tokens the driver verifies/hashes and persists nowhere in
	// the clear.
	ScopeID         string
	ChildCapability string
	GateContext     string
	// RunEpochID links this drive to the workflow run epoch (change 0375 Task 9): a
	// locator, not a credential, recorded on the worktree execution slot so an omitted
	// or stale epoch cannot detach a workflow-owned worktree. Empty for a standalone
	// gate (finalize's local gate, an ad-hoc task drive) that owns no epoch.
	RunEpochID string
	// Successor receipt (change 0405): a scoped drive that follows a predecessor in
	// the same recovery scope names the predecessor it acknowledges — both fields
	// together, or both empty for a scope's first drive. They are forwarded verbatim
	// to gatedrive.StartRequest, which retires exactly the named predecessor's
	// recovery authority as the journaled half of one logical transition.
	PredecessorDriveID  string
	PredecessorOwnerGen string
}

// newGateDriveService is the seam-injecting core constructor: it binds a drive
// engine to the resolved budget/command/provenance. Production wiring composes
// the real driver through the owner constructors below; unit tests inject a fake
// engine.
func newGateDriveService(engine driveEngine, budget time.Duration, command, provenance string) *GateDriveService {
	return &GateDriveService{engine: engine, budget: budget, command: command, provenance: provenance}
}

// Command selection is an explicit DOMAIN BOUNDARY, not a caller convenience: the
// build role reads only build policy and finalize only finalize policy, and (spec)
// "no agent or CLI caller may substitute an arbitrary command around authoritative
// configuration". The two owner constructors below are the only production entry
// points; each reads exactly one owner's test_command and records that owning key
// in the persisted provenance. There is deliberately no owner-agnostic constructor
// that takes a command — a Start command is always the resolved policy of a named
// owner.

// NewBuildGateDriveService composes the production gate-drive seam for the BUILD
// role. It reads ONLY build.test_command (never finalize's) and the shared
// observation budget, and names build.test_command in the persisted provenance.
func NewBuildGateDriveService(gitCommonDir, exePath string, eff config.Effective) (*GateDriveService, Result, string) {
	svc, res, reason := newOwnedGateDriveService(gitCommonDir, exePath, eff, "build", eff.Build.TestCommand)
	if svc != nil {
		// Snapshot the resolved build.max_attempts from the same authoritative
		// config load that resolved the build-owned command. A build-owned
		// change-scoped Start reserves one full-suite attempt against this bound.
		svc.maxAttempts = eff.Build.MaxAttempts.Value
	}
	return svc, res, reason
}

// NewFinalizeGateDriveService composes the production gate-drive seam for the
// FINALIZE role. It reads ONLY finalize.test_command (never build's) and the
// shared observation budget, and names finalize.test_command in the persisted
// provenance — byte-identical to the pre-0374 single-owner provenance so a
// finalize drive record is unchanged.
func NewFinalizeGateDriveService(gitCommonDir, exePath string, eff config.Effective) (*GateDriveService, Result, string) {
	return newOwnedGateDriveService(gitCommonDir, exePath, eff, "finalize", eff.Finalize.TestCommand)
}

// newOwnedGateDriveService is the shared private core the two owner constructors
// delegate to. It roots the durable drive store at the repository's Git common
// directory, resolves the config-provenanced observation budget
// (gate_observation_budget, in minutes) and the OWNER'S OWN suite command from the
// effective configuration — authoritative config, never agent input — and builds
// the native driver over the real process supervisor, monotonic clock, and git
// seam. It never shells out to docket's own CLI. owner ("build"|"finalize") is the
// owning-key stem: the persisted provenance names <owner>.test_command and the
// unresolved-command refusal names the owner. A process-service resolution failure
// returns a non-nil (result, reason) the caller surfaces directly; the reason is a
// fixed safe string, never a host path.
func newOwnedGateDriveService(gitCommonDir, exePath string, eff config.Effective, owner string, command config.Value[string]) (*GateDriveService, Result, string) {
	proc, err := process.NewService(exePath)
	if err != nil {
		r, reason := mapGateFailure(err)
		return nil, r, reason
	}
	store := gatedrive.OpenStore(gitCommonDir)
	engine := gatedrive.NewSystemDriver(store, proc)
	// A parent takeover must not revive a cancelled/superseded run epoch (change 0375
	// Task 12): wire the run-epoch revocation resolver over this repository's registry.
	// It fires only for a scope carrying a RunEpochID, so standalone/pre-linkage
	// scopes are unaffected.
	engine.SetEpochRevokedResolver(epochRevokedResolver(gitCommonDir))
	engine.SetEpochReplacementResolver(epochReplacementResolver(gitCommonDir))
	budget := time.Duration(eff.GateObservation.Value) * time.Minute
	// Provenance emits layer identities only — never a value — so it is safe to
	// persist in the drive record. The owning key is <owner>.test_command, derived
	// from owner so the stem and the message can never drift apart.
	prov := fmt.Sprintf("gate_observation_budget=%s;%s.test_command=%s",
		eff.GateObservation.Provenance.Layer, owner, command.Provenance.Layer)
	svc := newGateDriveService(engine, budget, command.Value, prov)
	svc.owner = owner
	// Reuse the engine's store for the build owner's suite-attempt reservation so a
	// build drive charges the same durable budget it composes. Non-build owners set
	// it too but never consult it (Start keys the reservation on owner=="build").
	svc.budgetStore = store
	return svc, "", ""
}

// NewCommandlessGateDriveService composes the gate-drive seam for the RESUMPTION
// operations (advance, handoff, claim), which never consult the suite command or
// the observation budget — they resume a drive the durable store already owns. It
// therefore resolves NO configuration: no owner, no command, no budget. Because
// Start fails closed on an empty command, a caller can never smuggle an
// unconfigured Start through this path; advance/handoff/claim stay
// config-resolution-free, exactly as before the owner split.
func NewCommandlessGateDriveService(gitCommonDir, exePath string) (*GateDriveService, Result, string) {
	proc, err := process.NewService(exePath)
	if err != nil {
		r, reason := mapGateFailure(err)
		return nil, r, reason
	}
	store := gatedrive.OpenStore(gitCommonDir)
	engine := gatedrive.NewSystemDriver(store, proc)
	// A parent takeover must not revive a cancelled/superseded run epoch (change 0375
	// Task 12): wire the run-epoch revocation resolver over this repository's registry.
	// It fires only for a scope carrying a RunEpochID, so standalone/pre-linkage
	// scopes are unaffected.
	engine.SetEpochRevokedResolver(epochRevokedResolver(gitCommonDir))
	engine.SetEpochReplacementResolver(epochReplacementResolver(gitCommonDir))
	return newGateDriveService(engine, 0, "", ""), "", ""
}

// NewTaskGateDriveService composes the gate-drive seam for TASK-INTENT
// (focused/ad-hoc) drives: the workflow role declares the test intent and
// supplies the argv EXPLICITLY, so there is no authoritative-config command to
// resolve (the domain boundary moves to "which constructor", not away — the
// build/finalize owner constructors still refuse caller argv). Its Start uses the
// agent-supplied argv verbatim, NEVER sets IdempotentSuiteGate (forced false
// regardless of the request), and records the fixed provenance
// "task.argv=agent-supplied". An empty argv fails closed here so no service can
// ever reach Start with an empty command.
//
// The COMMAND is agent-supplied, but the observation BUDGET is not: like the
// build/finalize owner constructors, this resolves the config-provenanced
// gate_observation_budget (minutes) from the effective configuration. A zero
// budget here would fix the deadline at start, so any focused test observed
// running even once would HALT deadline-expired instead of WAITING for its result
// — the exact defect Task 12 surfaced. The budget therefore always comes from
// authoritative config (default 30 minutes), never a hardcoded zero; only the
// command stays agent-supplied.
func NewTaskGateDriveService(gitCommonDir, exePath string, eff config.Effective, argv []string) (*GateDriveService, Result, string) {
	if len(argv) == 0 {
		return nil, ResultInvalidInput, "missing-argv"
	}
	proc, err := process.NewService(exePath)
	if err != nil {
		r, reason := mapGateFailure(err)
		return nil, r, reason
	}
	store := gatedrive.OpenStore(gitCommonDir)
	engine := gatedrive.NewSystemDriver(store, proc)
	// A parent takeover must not revive a cancelled/superseded run epoch (change 0375
	// Task 12): wire the run-epoch revocation resolver over this repository's registry.
	// It fires only for a scope carrying a RunEpochID, so standalone/pre-linkage
	// scopes are unaffected.
	engine.SetEpochRevokedResolver(epochRevokedResolver(gitCommonDir))
	budget := time.Duration(eff.GateObservation.Value) * time.Minute
	engine.SetEpochReplacementResolver(epochReplacementResolver(gitCommonDir))
	svc := newGateDriveService(engine, budget, "", "task.argv=agent-supplied")
	svc.owner = "task"
	svc.taskIntent = true
	svc.argv = argv
	return svc, "", ""
}

// Start begins a new drive over the resolved suite command and budget. An
// unresolved suite command (config resolved to unset) fails closed as a command
// failure before touching the engine — never a fabricated verdict.
//
// A build-owned change-scoped start is routed through startBudgetedBuild, which
// admits BEFORE it charges a full-suite attempt so a refused admission charges
// nothing (change 0375 Task 8). Every other owner — finalize, task-intent, the
// commandless resumption service — never charges and composes the driver's thin
// Start directly.
func (s *GateDriveService) Start(req GateDriveStartRequest) GateDriveResult {
	// The task-intent owner supplies its own argv, so the unresolved-command guard
	// applies only to the config-owned services (which have no argv).
	if !s.taskIntent && s.command == "" {
		return GateDriveResult{
			Envelope: NewEnvelope(OperationGateDriveStart, ResultInvalidInput),
			Reason:   "unresolved-command",
			Message:  s.unresolvedCommandMessage(),
		}
	}
	startReq := s.startRequest(req)
	// A build-role start that certifies a change (non-empty ChangeID — change 0416
	// guarantees a scoped start carries the full change/task/phase bundle) is the
	// only owner that charges the phase suite-attempt budget. The finalize and task
	// owners never reach this branch (different owner), and a build-owned start with
	// NO ChangeID (a scopeless ad-hoc drive) is deliberately unbudgeted — both
	// boundaries are pinned by tests.
	if s.owner == "build" && req.ChangeID != "" {
		return s.startBudgetedBuild(req, startReq)
	}
	doc, err := s.engine.Start(startReq)
	return mapDriveResult(OperationGateDriveStart, doc, err)
}

// startRequest builds the native StartRequest from a caller request, injecting the
// authoritative-config command/budget/provenance the caller can never substitute.
// A task-intent drive is never idempotent-suite-gated: the flag is forced false
// regardless of what the caller requested.
func (s *GateDriveService) startRequest(req GateDriveStartRequest) gatedrive.StartRequest {
	idempotent := req.IdempotentSuiteGate
	if s.taskIntent {
		idempotent = false
	}
	return gatedrive.StartRequest{
		RepoDir:             req.RepoDir,
		Worktree:            req.Worktree,
		ChangeID:            req.ChangeID,
		TaskID:              req.TaskID,
		Phase:               req.Phase,
		Branch:              req.Branch,
		Ref:                 req.Ref,
		Command:             s.commandArgv(),
		Cwd:                 req.Cwd,
		ConfigProvenance:    s.provenance,
		Budget:              s.budget,
		EnvHash:             req.EnvHash,
		RunRoot:             req.RunRoot,
		IdempotentSuiteGate: idempotent,
		ScopeID:             req.ScopeID,
		ChildCapability:     req.ChildCapability,
		GateContext:         req.GateContext,
		RunEpochID:          req.RunEpochID,
		PredecessorDriveID:  req.PredecessorDriveID,
		PredecessorOwnerGen: req.PredecessorOwnerGen,
	}
}

// startBudgetedBuild is the build owner's admission-precedes-charging start
// (change 0375 Task 8, ADR-0116). The ordering is load-bearing:
//
//  1. Advisory admission precheck — a plainly busy/unresolved worktree slot
//     short-circuits BEFORE any charge. It is advisory (WorktreeAdmissionRefusal
//     defers to the authoritative reserve on anything it cannot read).
//  2. Advisory budget precheck — an already-spent phase budget short-circuits
//     before admission, so an exhausted start neither reserves the worktree slot
//     nor mints a reserved drive it would have to abandon.
//  3. Authoritative admission (Admit). A refusal here — a worktree-busy /
//     unresolved-execution / scope-busy slot the advisory precheck missed under a
//     race — charges NO suite attempt: admission precedes charging.
//  4. Charge exactly one full-suite attempt BETWEEN admission and launch. Once
//     charged there are NO refunds: a launch/persistence failure in StartAdmitted
//     spends the attempt. A charge that cannot be reserved after admission (an
//     exhausted race the peek missed, or an IO fault) abandons the admission
//     fail-closed and refuses.
//  5. Launch the admitted, charged drive (StartAdmitted).
//
// Do not reorder the charge before the admission — that is exactly the defect this
// change fixes (a worktree-busy refusal must reserve no attempt).
func (s *GateDriveService) startBudgetedBuild(req GateDriveStartRequest, startReq gatedrive.StartRequest) GateDriveResult {
	if err := s.budgetStore.WorktreeAdmissionRefusal(req.Worktree); err != nil {
		return mapDriveResult(OperationGateDriveStart, gatedrive.DriveDoc{}, err)
	}
	if refusal, refused := s.suiteBudgetPrecheck(req); refused {
		return refusal
	}
	ticket, err := s.engine.Admit(startReq)
	if err != nil {
		return mapDriveResult(OperationGateDriveStart, gatedrive.DriveDoc{}, err)
	}
	if refusal, refused := s.reserveBuildSuiteAttempt(req); refused {
		_ = s.engine.AbandonAdmission(ticket)
		return refusal
	}
	doc, serr := s.engine.StartAdmitted(ticket)
	return mapDriveResult(OperationGateDriveStart, doc, serr)
}

// reserveBuildSuiteAttempt reserves one logical full-suite attempt for a
// build-owned, change-scoped Start. The budget key's phase is the LITERAL "build",
// never req.Phase, so the whole owning build phase shares one budget: a repair
// worker's build-owned rerun in the same phase is charged, while a task-owned
// focused-test start (a different owner) is never even reached. The limit is the
// snapshotted build.max_attempts; the store consults it only when creating the
// record (the phase's first reservation) and enforces the stored snapshot
// thereafter, so a config edit mid-phase never rewrites an owned budget.
//
// It returns (refusal, true) when the start must be refused and (zero, false) when
// the attempt was reserved and Start may proceed. A spent budget refuses with the
// stable "suite-attempts-exhausted" reason and a human message naming
// build.max_attempts and the used/limit fraction; there are no refunds, so a
// reservation whose later drive creation fails still counts. Any other reservation
// error (a sub-1 limit config validation should have caught, or an IO fault) fails
// the start closed rather than silently bypass the cap.
//
// The budget key carries no outer-attempt or epoch dimension, so it is scoped to the
// change's lifetime (repo + change + "build") and is intentionally NOT refreshed or
// reset by an outer-gate gate-retry-once re-dispatch of the same change: an outer
// retry inherits the remaining build budget by design, and can never acquire more
// build repairs. This errs safe.
func (s *GateDriveService) reserveBuildSuiteAttempt(req GateDriveStartRequest) (GateDriveResult, bool) {
	key := gatedrive.SuiteBudgetKey{
		RepoIdentity: req.RepoDir,
		ChangeID:     req.ChangeID,
		Phase:        "build",
	}
	_, _, err := s.budgetStore.ReserveSuiteAttempt(key, s.maxAttempts)
	if err == nil {
		return GateDriveResult{}, false
	}
	if se, ok := gatedrive.AsStoreError(err); ok && se.Kind == gatedrive.ErrSuiteBudgetExhausted {
		used, limit, _ := s.budgetStore.SuiteBudgetUsage(key)
		return s.suiteExhaustedRefusal(used, limit), true
	}
	res, reason := mapDriveFailure(err)
	return GateDriveResult{Envelope: NewEnvelope(OperationGateDriveStart, res), Reason: reason}, true
}

// suiteBudgetPrecheck is the ADVISORY, read-only budget short-circuit the build
// owner consults before admission (change 0375 Task 8): an already-spent phase
// budget refuses with the same exhausted refusal reserveBuildSuiteAttempt would,
// so an exhausted start never admits (and so never reserves a worktree slot or
// mints a reserved drive it would have to abandon). It never charges. A key with no
// record yet reports (0,0) → not exhausted → proceed to admission, where the
// authoritative ReserveSuiteAttempt creates the record. A corrupt/unknown-schema
// budget record fails the start closed rather than admitting over an unreadable
// budget.
func (s *GateDriveService) suiteBudgetPrecheck(req GateDriveStartRequest) (GateDriveResult, bool) {
	key := gatedrive.SuiteBudgetKey{RepoIdentity: req.RepoDir, ChangeID: req.ChangeID, Phase: "build"}
	used, limit, err := s.budgetStore.SuiteBudgetUsage(key)
	if err != nil {
		res, reason := mapDriveFailure(err)
		return GateDriveResult{Envelope: NewEnvelope(OperationGateDriveStart, res), Reason: reason}, true
	}
	if limit > 0 && used >= limit {
		return s.suiteExhaustedRefusal(used, limit), true
	}
	return GateDriveResult{}, false
}

// suiteExhaustedRefusal builds the stable spent-budget refusal shared by the
// advisory budget precheck and the authoritative reservation, so both surface the
// identical "suite-attempts-exhausted" reason and the human message naming
// build.max_attempts and the used/limit fraction.
func (s *GateDriveService) suiteExhaustedRefusal(used, limit int) GateDriveResult {
	return GateDriveResult{
		Envelope: NewEnvelope(OperationGateDriveStart, ResultGateFailed),
		Reason:   "suite-attempts-exhausted",
		Message: fmt.Sprintf("the build full-suite attempt budget is spent (%d/%d used); "+
			"raising build.max_attempts takes effect on the next build phase, not this one — "+
			"halt per the build skill's halting conditions", used, limit),
	}
}

// unresolvedCommandMessage names the owner and the setup remedy for the
// unresolved-command refusal. The reason TOKEN stays "unresolved-command"
// (stable); only this human message carries the owner and the remedy. The owner is
// always set on a service that can reach Start (an owner constructor); the "gate"
// fallback covers the unreachable commandless path defensively.
func (s *GateDriveService) unresolvedCommandMessage() string {
	owner := s.owner
	if owner == "" {
		owner = "gate"
	}
	return fmt.Sprintf("no resolved %s test command; run docket repository configure-tests", owner)
}

// Advance resumes the current attempt of a drive through at most one slice.
func (s *GateDriveService) Advance(id, ownerGen string) GateDriveResult {
	doc, err := s.engine.Advance(id, ownerGen)
	return mapDriveResult(OperationGateDriveAdvance, doc, err)
}

// Acknowledge consumes the scope's final drive result and closes the scope. It
// is the final drive's "successor" — the terminal counterpart of the successor
// receipt a Start carries — and needs no suite command or observation budget, so
// it composes over the commandless service exactly like advance/handoff/claim.
// The driver verifies the child capability, that driveID is the scope's current
// launched drive with a durable PASSED/FAILED outcome and no outstanding handoff,
// and current ownership; a byte-identical repeat after success is idempotent. Its
// outcome maps exactly like the other drive operations: a produced document is an
// applied result; a typed ownership rejection carries the bounded reason and its
// next-action message.
func (s *GateDriveService) Acknowledge(scopeID, childCap, driveID, ownerGen string) GateDriveResult {
	doc, err := s.engine.Acknowledge(scopeID, childCap, driveID, ownerGen)
	return mapDriveResult(OperationGateDriveAcknowledge, doc, err)
}

// Handoff transfers a live drive to a fresh owner, returning the single-use
// handoff token (in the document's generation) a claimant presents to Claim.
func (s *GateDriveService) Handoff(id, ownerGen string) GateDriveResult {
	doc, err := s.engine.Handoff(id, ownerGen)
	return mapDriveResult(OperationGateDriveHandoff, doc, err)
}

// Claim consumes an outstanding handoff receipt, returning the fresh owner
// generation (in the document's generation) the claimant advances with.
func (s *GateDriveService) Claim(id, handoffID string) GateDriveResult {
	doc, err := s.engine.Claim(id, handoffID)
	return mapDriveResult(OperationGateDriveClaim, doc, err)
}

// PrepareScope mints a recovery scope for one parent/child dispatch boundary and
// returns the grant. On success the JSON document carries all three grant fields
// (scope id + child/parent capabilities); a command failure carries a bounded
// safe reason and no grant. The two capabilities travel ONLY in the JSON
// document — never in the human text (GateScopeResult.HumanText).
func (s *GateDriveService) PrepareScope(req gatedrive.ScopeRequest) GateScopeResult {
	grant, err := s.engine.PrepareScope(req)
	if err != nil {
		res, reason := mapDriveFailure(err)
		return GateScopeResult{Envelope: NewEnvelope(OperationGateDrivePrepareScope, res), Reason: reason}
	}
	return GateScopeResult{
		Envelope:         NewEnvelope(OperationGateDrivePrepareScope, ResultApplied),
		ScopeID:          grant.ScopeID,
		ChildCapability:  grant.ChildCapability,
		ParentCapability: grant.ParentCapability,
	}
}

// Takeover performs the event-authorized exceptional transfer of a scope-bound
// drive to a fresh owner the parent mints. It delegates to the driver and maps
// the outcome exactly like the other drive operations: a produced document
// (including a HALTED refusal) is an applied result carrying the shared DriveDoc;
// a command failure carries a bounded safe reason and no document.
func (s *GateDriveService) Takeover(scopeID, parentCap, driveID string) GateDriveResult {
	doc, err := s.engine.Takeover(scopeID, parentCap, driveID)
	return mapDriveResult(OperationGateDriveTakeover, doc, err)
}

// commandArgv shells the resolved suite command exactly as the finalize gate does
// (`/bin/sh -c <command>`), so the driver launches the identical process tree. An
// empty command yields nil argv, but Start guards that before this is reached.
func (s *GateDriveService) commandArgv() []string {
	// A task-intent owner runs the agent-supplied argv verbatim — no shell wrapping.
	if s.taskIntent {
		return s.argv
	}
	if s.command == "" {
		return nil
	}
	return []string{"/bin/sh", "-c", s.command}
}

// mapDriveResult maps a driver call into the protocol document. A nil error is a
// SUCCESSFUL operation that produced a typed workflow verdict (WAITING, PASSED,
// FAILED, or HALTED) — always ResultApplied, with the verdict carried in the
// shared DriveDoc; callers key on doc.Outcome, not the envelope result, for the
// workflow decision (spec "Typed outcomes"). A non-nil error is a COMMAND FAILURE
// (unparseable request, unrecognized drive) distinct from any workflow verdict: a
// non-applied result carrying a bounded safe reason and no drive document.
func mapDriveResult(op string, doc gatedrive.DriveDoc, err error) GateDriveResult {
	if err != nil {
		res, reason := mapDriveFailure(err)
		result := GateDriveResult{Envelope: NewEnvelope(op, res), Reason: reason}
		// A typed ownership rejection also carries a valid-next-action message so
		// the caller knows what to do instead of retrying blindly (spec "Human
		// messages explain the valid next action for the actual state"). The reason
		// token stays the bounded kind; only this message explains the recourse.
		if oe, ok := gatedrive.AsOwnershipError(err); ok {
			// A legacy-inventory refusal carries the typed stage, a SAFE locator, and
			// the mirrored recovery summary (no drive document exists on a refusal), and
			// a cleanup-oriented message. Every other ownership kind keeps its existing
			// slot-recovery next-action message unchanged.
			if stage, locator, isInventory := legacyInventoryLocator(oe.Op); isInventory {
				result.Stage, result.Locator = stage, locator
				result.LegacyHistory = oe.Legacy
				result.Message = "historical gate drives block this admission; inspect or recover them with docket gate history cleanup (--dry-run first); run.cancel applies only to a live run with an owning epoch"
			} else {
				result.Message = ownershipNextAction(oe.Kind)
			}
		} else if fe, ok := AsMutationFenceError(err); ok {
			result.Message = fenceNextAction(fe.Reason)
		}
		return result
	}
	d := doc
	return GateDriveResult{Envelope: NewEnvelope(op, ResultApplied), Drive: &d}
}

// mapDriveFailure classifies a driver command failure into a protocol result and
// a bounded safe reason token. A store error naming a bad or unknown drive id is
// invalid input; any other store error is an internal error. The reason is a
// stable kind token, never the raw error text, so no argv/env/path can leak.
func mapDriveFailure(err error) (Result, string) {
	// A typed ownership rejection surfaces its bounded kind token instead of
	// collapsing to the generic invalid-request (spec "Map known ownership errors
	// to their bounded, stable reason tokens"). The kind is the whole reason, so no
	// wrapped free text (argv, env, path, stored error text) can leak.
	if oe, ok := gatedrive.AsOwnershipError(err); ok {
		return ResultInvalidInput, string(oe.Kind)
	}
	// A run-epoch mutation fence (rungate_fence.go) is a distinct refusal type
	// carrying its OWN stable token — "run-cancelled" (the owning epoch is
	// cancelling/cancelled) or "stale-run-epoch" (superseded by a resume). It never
	// reaches the standalone gate-drive path today, but classifying it here is
	// fail-safe: if a fenced-epoch error ever chains through this seam it surfaces its
	// bounded token instead of leaking the wrapped refusal text or collapsing to the
	// generic invalid-request. The Reason field is a fixed vocabulary token, never
	// record content, argv, env, or a credential.
	if fe, ok := AsMutationFenceError(err); ok {
		return ResultInvalidInput, fe.Reason
	}
	if se, ok := gatedrive.AsStoreError(err); ok {
		switch se.Kind {
		case gatedrive.ErrInvalidID, gatedrive.ErrNotFound:
			return ResultInvalidInput, string(se.Kind)
		default:
			return ResultInternalError, string(se.Kind)
		}
	}
	return ResultInvalidInput, "invalid-request"
}

// ownershipNextAction maps an ownership rejection kind to a one-line, credential-
// free description of the caller's valid next action for that actual state (spec
// "Human messages explain the valid next action for the actual state"). It never
// authorizes a blind start retry or a keyless fallback. An unrecognized kind
// yields the empty string, so callers omit the message rather than inventing one.
func ownershipNextAction(kind gatedrive.OwnershipErrorKind) string {
	switch kind {
	case gatedrive.ErrEpochScopeRequired:
		return "prepare a fresh scope bound to this run epoch, then start with its scope id, child capability and complete identity; the epoch alone is not authority"
	case gatedrive.ErrScopeBusy:
		return "another start or transition owns this scope's slot; do not retry blindly"
	case gatedrive.ErrHandoffOutstanding:
		return "claim the outstanding handoff instead of starting or taking over"
	case gatedrive.ErrScopeClosed:
		return "scope authority was transferred or finished; stop and return BLOCKED"
	case gatedrive.ErrStalePredecessor:
		return "the presented predecessor is not the scope's current drive"
	case gatedrive.ErrPredecessorNotReusable:
		return "the predecessor has no durable PASSED/FAILED result to acknowledge"
	case gatedrive.ErrUnresolvedLaunchTransition:
		return "a prior launch transition is unresolved; recover via the parent, not a retry"
	case gatedrive.ErrWorktreeBusy:
		return "this worktree already runs a gate execution; wait for it or cancel that run — do not start a second in the same worktree"
	case gatedrive.ErrUnresolvedExecution:
		return "a prior execution in this worktree is unresolved; recover it through the parent or run.cancel, never a blind re-start"
	case gatedrive.ErrStaleRunEpoch:
		return "an in-flight run owns this worktree; present that run's epoch or cancel it before starting"
	case gatedrive.ErrScopeCapabilityMismatch:
		return "use the complete identity bundle from your dispatch prompt"
	case gatedrive.ErrScopeIdentityMismatch:
		return "the scope identity does not match; use the complete identity bundle from your dispatch prompt"
	case gatedrive.ErrScopeSecondDrive:
		return "the scope already holds a drive; a successor start must present the predecessor receipt"
	default:
		return ""
	}
}

// legacyInventoryLocator recognizes the inventory refusal ops and returns a
// SAFE locator: a drive-id-bearing op is rendered verbatim only when the id
// validates; anything else collapses to the inventory-level locator so an
// arbitrary directory name can never render. ok is false for a non-inventory op,
// which keeps its existing next-action message untouched.
func legacyInventoryLocator(op string) (stage, locator string, ok bool) {
	const prefix = "inventory-legacy-drive-"
	switch {
	case op == "inventory-legacy-drives":
		return "legacy-inventory", op, true
	case strings.HasPrefix(op, prefix):
		if id := strings.TrimPrefix(op, prefix); gatedrive.ValidDriveID(id) {
			return "legacy-inventory", op, true
		}
		return "legacy-inventory", "inventory-legacy-drives", true
	}
	return "", "", false
}

// fenceNextAction maps a run-epoch mutation-fence reason (MutationFenceError.Reason)
// to a one-line, credential-free description of the caller's valid next action. It
// mirrors ownershipNextAction for the fence refusal family so a fenced-epoch error
// surfaced through this seam explains the recourse rather than inviting a blind
// retry. An unrecognized reason yields the empty string, so callers omit the message.
func fenceNextAction(reason string) string {
	switch reason {
	case "run-cancelled":
		return "the run epoch was cancelled; do not retry — a resume after confirmed cancellation admits exactly one replacement"
	case "stale-run-epoch":
		return "the run epoch was superseded by a resume; use the current run's identity, not this stale one"
	default:
		return ""
	}
}

// HumanText renders GateDriveResult as stable labeled lines. It names the outcome
// and identity only and DELIBERATELY omits the ownership generation: the shared
// JSON document carries the credential for the protocol, but diagnostic prose
// never emits an ownership credential (spec "Typed outcomes").
func (r GateDriveResult) HumanText() string {
	var lines []string
	if r.Drive != nil {
		lines = append(lines, "outcome: "+string(r.Drive.Outcome))
		if r.Drive.DriveID != "" {
			lines = append(lines, "drive_id: "+r.Drive.DriveID)
		}
		if r.Drive.Attempt != 0 {
			lines = append(lines, fmt.Sprintf("attempt: %d", r.Drive.Attempt))
		}
		if r.Drive.Cause != "" {
			lines = append(lines, "cause: "+r.Drive.Cause)
		}
		if r.Drive.RawRunDir != "" {
			lines = append(lines, "raw_run_dir: "+r.Drive.RawRunDir)
		}
	}
	if r.Reason != "" {
		lines = append(lines, "reason: "+r.Reason)
	}
	if r.Message != "" {
		lines = append(lines, "message: "+r.Message)
	}
	if r.Stage != "" {
		lines = append(lines, "stage: "+r.Stage)
	}
	if r.Locator != "" {
		lines = append(lines, "locator: "+r.Locator)
	}
	// The legacy-history summary renders as a COMPACT counts-only line — checked,
	// recovered, retained totals only, never per-record content — from the refusal
	// mirror or, on a success result, the drive document's own summary.
	if sum := r.legacySummary(); sum != nil {
		lines = append(lines, fmt.Sprintf("legacy_history: checked %d recovered %d retained %d",
			sum.Checked, len(sum.Recovered), len(sum.Retained)))
	}
	return strings.Join(lines, "\n")
}

// legacySummary returns the legacy-history summary to render: the refusal mirror
// when set (no drive document exists on a refusal), else the success document's
// own summary. Nil when neither carries one.
func (r GateDriveResult) legacySummary() *gatedrive.LegacyHistorySummary {
	if r.LegacyHistory != nil {
		return r.LegacyHistory
	}
	if r.Drive != nil {
		return r.Drive.LegacyHistory
	}
	return nil
}

// Compile-time seam assertions: the production driver satisfies the engine seam,
// and the result satisfies the presenter contract.
var (
	_ driveEngine     = (*gatedrive.Driver)(nil)
	_ OperationResult = GateDriveResult{}
	_ OperationResult = GateScopeResult{}
)
