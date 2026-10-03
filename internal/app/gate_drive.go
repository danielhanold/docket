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
	"regexp"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
)

// Gate-drive operation names — the fixed protocol identifiers for the
// slice-bounded gate-driver operations this seam exposes.
const (
	OperationGateDriveStart   = "gate.drive.start"
	OperationGateDriveAdvance = "gate.drive.advance"
	OperationGateDriveHandoff = "gate.drive.handoff"
	OperationGateDriveClaim   = "gate.drive.claim"
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
	// Stage + Locator carry the typed refusal site of a worktree-busy refusal
	// (incumbentRefusalLocator): Stage "worktree-admission", Locator
	// "incumbent-drive:<id>" / "incumbent-run:<id>" (validated id) naming the
	// worktree lock's live holder, or "" when the holder is unknown. Empty for
	// every other refusal.
	Stage   string `json:"stage,omitempty"`
	Locator string `json:"locator,omitempty"`
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
	Handoff(id, ownerGen string) (gatedrive.DriveDoc, error)
	Claim(id, handoffID string) (gatedrive.DriveDoc, error)
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
	// budgetStore + maxAttempts wire the durable per-phase suite-attempt budget the
	// BUILD owner reserves against before a change-scoped build drive is created
	// (change 0421). budgetStore is the SAME store the engine composes; maxAttempts
	// is the snapshotted build.max_attempts from the SAME authoritative config
	// resolution that resolved the build-owned command — never a second resolver.
	// Only the build owner reserves (Start keys on owner=="build"), so a
	// finalize or commandless service never charges an attempt even though a
	// finalize service also stores a non-nil budgetStore.
	budgetStore *gatedrive.Store
	maxAttempts int
}

// GateDriveStartRequest is the caller-supplied identity and launch context for a
// new drive. The caller supplies only the work identity and launch context; the
// SERVICE supplies the authoritative-config command, budget, and provenance, so a
// caller can never substitute the suite command or the observation budget.
type GateDriveStartRequest struct {
	RepoDir  string
	Worktree string
	ChangeID string
	TaskID   string
	Phase    string
	Branch   string
	Ref      string
	Cwd      string
	EnvHash  string
	RunRoot  string
	// RunContext is the raw run-context token from run.start linking this drive to
	// the dispatched run; the driver persists only its hash, which run.verdict's
	// outer scan matches a run's drives on. Optional: empty for an untracked run.
	RunContext string
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
	return newGateDriveService(engine, 0, "", ""), "", ""
}

// Start begins a new drive over the resolved suite command and budget. An
// unresolved suite command (config resolved to unset) fails closed as a command
// failure before touching the engine — never a fabricated verdict.
//
// A build-owned change-scoped start is routed through startBudgetedBuild, which
// admits BEFORE it charges a full-suite attempt so a refused admission charges
// nothing (change 0375 Task 8). Every other owner — finalize, the commandless
// resumption service — never charges and composes the driver's thin Start
// directly.
func (s *GateDriveService) Start(req GateDriveStartRequest) GateDriveResult {
	if s.command == "" {
		return GateDriveResult{
			Envelope: NewEnvelope(OperationGateDriveStart, ResultInvalidInput),
			Reason:   "unresolved-command",
			Message:  s.unresolvedCommandMessage(),
		}
	}
	startReq := s.startRequest(req)
	// A build-role start that certifies a change (non-empty ChangeID) is the
	// only owner that charges the phase suite-attempt budget. The finalize owner
	// never reaches this branch (different owner), and a build-owned start with
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
func (s *GateDriveService) startRequest(req GateDriveStartRequest) gatedrive.StartRequest {
	return gatedrive.StartRequest{
		RepoDir:          req.RepoDir,
		Worktree:         req.Worktree,
		ChangeID:         req.ChangeID,
		TaskID:           req.TaskID,
		Phase:            req.Phase,
		Branch:           req.Branch,
		Ref:              req.Ref,
		Command:          s.commandArgv(),
		Cwd:              req.Cwd,
		ConfigProvenance: s.provenance,
		Budget:           s.budget,
		EnvHash:          req.EnvHash,
		RunRoot:          req.RunRoot,
		RunContext:       req.RunContext,
		Owner:            s.owner,
	}
}

// startBudgetedBuild is the build owner's admission-precedes-charging start
// (change 0375 Task 8, ADR-0116). The ordering is load-bearing:
//
//  1. Advisory budget precheck — an already-spent phase budget short-circuits
//     before admission, so an exhausted start neither takes the worktree lock nor
//     mints a reserved drive it would have to abandon.
//  2. Authoritative admission (Admit): the worktree lock is taken before the
//     charge. A refusal here — another gate's supervisor holds the worktree
//     (worktree-busy) — charges NO suite attempt
//     and creates no drive: admission precedes charging.
//  3. Charge exactly one full-suite attempt BETWEEN admission and launch. Once
//     charged there are NO refunds: a launch/persistence failure in StartAdmitted
//     spends the attempt. A charge that cannot be reserved after admission (an
//     exhausted race the peek missed, or an IO fault) abandons the admission —
//     closing the worktree lock — and refuses.
//  4. Launch the admitted, charged drive (StartAdmitted).
//
// Do not reorder the charge before the admission — a worktree-busy refusal must
// reserve no attempt.
func (s *GateDriveService) startBudgetedBuild(req GateDriveStartRequest, startReq gatedrive.StartRequest) GateDriveResult {
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
// worker's build-owned rerun in the same phase is charged, while a finalize-owned
// start (a different owner) is never even reached. The limit is the
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
// The budget key carries no outer-attempt or run dimension, so it is scoped to the
// change's lifetime (repo + change + "build") and is intentionally NOT refreshed or
// reset by a run-tracker run-retry-once re-dispatch of the same change: such a
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
// so an exhausted start never admits (and so never takes the worktree lock or
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

// commandArgv shells the resolved suite command exactly as the finalize gate does
// (`/bin/sh -c <command>`), so the driver launches the identical process tree. An
// empty command yields nil argv, but Start guards that before this is reached.
func (s *GateDriveService) commandArgv() []string {
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
			if oe.Kind == gatedrive.ErrWorktreeBusy {
				// A worktree-busy refusal always names its site, and diagnoses from
				// the holder snapshot the lock refusal validated as running (never a
				// re-read) — or says the holder is unknown when there is none.
				result.Stage = stageWorktreeAdmission
				result.Locator = incumbentRefusalLocator(oe.Incumbent)
				result.Message = incumbentRemedyMessage(oe.Incumbent)
			} else {
				result.Message = ownershipNextAction(oe.Kind)
			}
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
	// A run mutation fence (runtracker_fence.go) is a distinct refusal type carrying
	// its OWN stable token. No gate-drive path raises it since change 0491 deleted
	// the run launch check, but classifying it stays fail-safe: a fenced-run error
	// that ever chains through this seam surfaces its bounded token instead of
	// leaking the wrapped refusal text or collapsing to the generic invalid-request.
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
	case gatedrive.ErrHandoffOutstanding:
		return "claim the outstanding handoff instead of starting another drive"
	case gatedrive.ErrUnresolvedLaunchTransition:
		return "a prior launch transition is unresolved; settle it with run.cancel or wait for it, never a blind retry"
	case gatedrive.ErrWorktreeBusy:
		return "another gate's supervisor holds this worktree's lock; wait for it to finish — the worktree frees itself when that gate ends — or stop that gate through its own route (run.cancel for a tracked run, gate stop for a raw launch); never start a second gate in the same worktree"
	case gatedrive.ErrWorktreeUnresolved:
		return "the launch working directory is not inside a git worktree; start the gate from the change's worktree"
	default:
		return ""
	}
}

// stageWorktreeAdmission is the typed refusal site for a worktree-busy refusal.
const stageWorktreeAdmission = "worktree-admission"

// rawRunIDShape matches the supervisor's run-id shape (32 lowercase hex); the
// canonical pattern lives in internal/process (see paths.go runIDPattern), and
// this diagnostic-only copy accepts exactly the same ids.
var rawRunIDShape = regexp.MustCompile("^[0-9a-f]{32}$")

// incumbentRefusalLocator returns the bounded safe locator for an admission
// refusal's incumbent: "incumbent-drive:<id>" / "incumbent-run:<id>", "" when no
// identity validates. (This is the single bounded-locator convention shared by
// every admission-refusal path, including the raw gate.launch's
// admissionRefusalCause in gate.go.) A drive
// id is validated with gatedrive.ValidDriveID and a raw run id with rawRunIDShape,
// so an arbitrary directory name or drive id can never render into the locator.
func incumbentRefusalLocator(inc *gatedrive.IncumbentSnapshot) string {
	if inc == nil {
		return ""
	}
	switch {
	case inc.DriveID != "" && gatedrive.ValidDriveID(inc.DriveID):
		return "incumbent-drive:" + inc.DriveID
	case inc.RawRunID != "" && rawRunIDShape.MatchString(inc.RawRunID):
		return "incumbent-run:" + inc.RawRunID
	default:
		return ""
	}
}

// quoteOperand renders a path as a safely single-quoted shell operand for human
// guidance: the path is wrapped in single quotes and each interior single quote
// is replaced by the close-quote, backslash-escaped-quote, reopen-quote sequence.
func quoteOperand(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// changeIDShape bounds the change id a holder note may render into guidance:
// change ids are decimal (finalize passes strconv.Itoa; the build controller its
// numeric id). Anything else renders generically, never verbatim.
var changeIDShape = regexp.MustCompile("^[0-9]{1,9}$")

// holderChangeLabel names the holder's change for guidance: "change <id>" for a
// validated id, else a generic phrase.
func holderChangeLabel(changeID string) string {
	if changeIDShape.MatchString(changeID) {
		return "change " + changeID
	}
	return "another change"
}

// incumbentRemedyMessage returns the credential-free next-action guidance for a
// worktree-busy refusal — always a bounded honest sentence, never "". inc is the
// worktree lock's holder snapshot, present only when the holder note's run was
// observed running (gatedrive.liveHolder); nil means the holder is unknown. The
// remedy names the right tool for the holder's kind: run.cancel for a build
// drive's owning run, waiting for finalize's gate, or gate stop for a raw launch.
// It renders a drive id, change id, or run dir only after validating it, and
// never projects a token, owner generation, capability, or run id.
func incumbentRemedyMessage(inc *gatedrive.IncumbentSnapshot) string {
	switch {
	case inc == nil:
		return "holder unknown: another gate's supervisor holds this worktree's lock and it could not be confirmed running; wait for it to finish — the worktree frees itself when that gate ends — or find the holding process with lsof on this worktree's busy.lock under the repository's Git common dir (docket/worktree-locks/<key>/busy.lock); never start a second gate here"
	case inc.Kind == "raw" && inc.RawRunDir != "" && rawRunIDShape.MatchString(inc.RawRunID):
		dir := quoteOperand(inc.RawRunDir)
		return "a raw gate run holds this worktree (run dir " + dir + "); wait for it, or stop it with docket gate stop " + dir + " --reason <why> — the worktree frees itself when the run ends"
	case inc.Kind == "raw":
		return "a raw gate run holds this worktree but its run identity did not validate; wait for it to finish — the worktree frees itself when the run ends; never start a second gate here"
	case inc.Owner == "finalize":
		return "finalize's local gate for " + holderChangeLabel(inc.ChangeID) + " holds this worktree; wait for it to finish — it frees the worktree when it ends"
	default:
		gate := holderChangeLabel(inc.ChangeID) + "'s build gate"
		if gatedrive.ValidDriveID(inc.DriveID) {
			gate += " (drive " + inc.DriveID + ")"
		}
		return gate + " holds this worktree; wait for it, or stop the owning run with the run.cancel operation (--key <key> --reason <why>) — the worktree frees itself when that gate ends"
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
	return strings.Join(lines, "\n")
}

// Compile-time seam assertions: the production driver satisfies the engine seam,
// and the result satisfies the presenter contract.
var (
	_ driveEngine     = (*gatedrive.Driver)(nil)
	_ OperationResult = GateDriveResult{}
)
