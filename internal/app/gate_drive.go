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
	Advance(id, ownerGen string) (gatedrive.DriveDoc, error)
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
	return newOwnedGateDriveService(gitCommonDir, exePath, eff, "build", eff.Build.TestCommand)
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
	budget := time.Duration(eff.GateObservation.Value) * time.Minute
	// Provenance emits layer identities only — never a value — so it is safe to
	// persist in the drive record. The owning key is <owner>.test_command, derived
	// from owner so the stem and the message can never drift apart.
	prov := fmt.Sprintf("gate_observation_budget=%s;%s.test_command=%s",
		eff.GateObservation.Provenance.Layer, owner, command.Provenance.Layer)
	svc := newGateDriveService(engine, budget, command.Value, prov)
	svc.owner = owner
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
func NewTaskGateDriveService(gitCommonDir, exePath string, argv []string) (*GateDriveService, Result, string) {
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
	svc := newGateDriveService(engine, 0, "", "task.argv=agent-supplied")
	svc.owner = "task"
	svc.taskIntent = true
	svc.argv = argv
	return svc, "", ""
}

// Start begins a new drive over the resolved suite command and budget. An
// unresolved suite command (config resolved to unset) fails closed as a command
// failure before touching the engine — never a fabricated verdict.
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
	// A task-intent drive is never idempotent-suite-gated: the agent runs a focused
	// command, and no config command backs a suite-idempotency claim. The flag is
	// forced false here regardless of what the caller requested.
	idempotent := req.IdempotentSuiteGate
	if s.taskIntent {
		idempotent = false
	}
	doc, err := s.engine.Start(gatedrive.StartRequest{
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
	})
	return mapDriveResult(OperationGateDriveStart, doc, err)
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
		return GateDriveResult{Envelope: NewEnvelope(op, res), Reason: reason}
	}
	d := doc
	return GateDriveResult{Envelope: NewEnvelope(op, ResultApplied), Drive: &d}
}

// mapDriveFailure classifies a driver command failure into a protocol result and
// a bounded safe reason token. A store error naming a bad or unknown drive id is
// invalid input; any other store error is an internal error. The reason is a
// stable kind token, never the raw error text, so no argv/env/path can leak.
func mapDriveFailure(err error) (Result, string) {
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
	return strings.Join(lines, "\n")
}

// Compile-time seam assertions: the production driver satisfies the engine seam,
// and the result satisfies the presenter contract.
var (
	_ driveEngine     = (*gatedrive.Driver)(nil)
	_ OperationResult = GateDriveResult{}
	_ OperationResult = GateScopeResult{}
)
