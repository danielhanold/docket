package app

import (
	"fmt"
	"sort"
	"strings"
)

// OperationBinding joins one capabilities operation id to the live Go types its
// handler decodes and returns. Request is exactly the JSON document the
// operation strictly decodes from a file (`--request`, `--input`, or `--body`),
// or nil when it decodes none. Scalar flags are described only by the
// capability catalog's `signature`, and non-JSON file inputs (the canonical
// build-evidence record, `agent.enter`'s plain-text request) are not requests
// (ADR-0138, which refines ADR-0109's request surface).
// The id is the SAME stable id the capability catalog uses — the join key across
// the two surfaces.
type OperationBinding struct {
	ID      string
	Request any // prototype struct value, e.g. ChangeBlockRequest{}
	Result  any // prototype struct value embedding Envelope
}

// operationBindings is the authoritative registry, one entry per capabilities
// catalog operation that emits a protocol-v1 document. It is DERIVED from the
// live catalog (`docket capabilities --json`) and each cli command's RunE, never
// hand-guessed: every entry's Request is exactly the JSON document the operation
// strictly decodes from a file (`--request`, `--input`, or `--body`), or nil when
// it decodes none, and every Result is the Envelope-embedding document its app
// function returns. A flag-assembled *Request struct is an internal app input and
// is never bound (ADR-0138; ADR-0109). TestPublishedRequestIsTheDecodedJSONFile
// (internal/cli) proves each binding's Request is the type the command declares
// through declareJSONFile. Each derivation names the app function symbol it was read
// from — a symbol name, greppable and drift-visible, never a line number
// (AGENTS.md, ADR-0054).
//
// One catalog operation is deliberately absent: `development.test` emits NO
// protocol document — its RunE (runDevelopmentTest) streams the suite report and
// returns only an exit code — so there is no Envelope-embedding result to bind.
// The cli-side correspondence guard names it as the sole no-document exception, in
// the open, so a second such op is a conscious addition rather than a silent gap.
//
// The list is declared sorted by id; TestOperationBindingsSortedUniqueAndDescribable
// holds that invariant.
var operationBindings = []OperationBinding{
	{ID: "adr.record", Request: ADRRecordRequest{}, Result: ADRResult{}},                                   // ADRRecordOp
	{ID: "adr.reverse", Request: ADRReplaceRequest{}, Result: ADRResult{}},                                 // ADRReverse
	{ID: "adr.supersede", Request: ADRReplaceRequest{}, Result: ADRResult{}},                               // ADRSupersede
	{ID: "agent.enter", Request: nil, Result: AgentEnterResult{}},                                          // AgentEnter
	{ID: "capabilities", Request: nil, Result: CapabilitiesResult{}},                                       // Capabilities
	{ID: "change.attach-plan", Request: nil, Result: ChangeAttachResult{}},                                 // ChangeAttachPlan
	{ID: "change.attach-results", Request: nil, Result: ChangeAttachResult{}},                              // ChangeAttachResults
	{ID: "change.block", Request: ChangeBlockRequest{}, Result: ChangeLifecycleResult{}},                   // ChangeBlock
	{ID: "change.claim", Request: nil, Result: ChangeClaimResult{}},                                        // ChangeClaim
	{ID: "change.create", Request: ChangeCreateRequest{}, Result: ChangeCreateResult{}},                    // ChangeCreate
	{ID: "change.defer", Request: ChangeDeferRequest{}, Result: ChangeLifecycleResult{}},                   // ChangeDefer
	{ID: "change.groom", Request: ChangeGroomRequest{}, Result: ChangeGroomResult{}},                       // ChangeGroom
	{ID: "change.halt", Request: ChangeHaltInput{}, Result: HaltResult{}},                                  // ChangeHalt
	{ID: "change.kill", Request: ChangeKillRequest{}, Result: ChangeKillResult{}},                          // ChangeKill
	{ID: "change.mark-implemented", Request: nil, Result: ChangeLifecycleResult{}},                         // ChangeMarkImplemented
	{ID: "change.reclaim", Request: nil, Result: ChangeReclaimResult{}},                                    // ChangeReclaim
	{ID: "change.reconcile", Request: ChangeReconcileRequest{}, Result: ChangeReconcileResult{}},           // ChangeReconcile
	{ID: "change.refresh-claim", Request: nil, Result: ChangeClaimResult{}},                                // ChangeRefreshClaim
	{ID: "change.relink", Request: nil, Result: RelinkResult{}},                                            // Relink
	{ID: "change.resume-halted", Request: nil, Result: HaltResult{}},                                       // ChangeResumeHalted
	{ID: "change.revive", Request: ChangeReviveRequest{}, Result: ChangeLifecycleResult{}},                 // ChangeRevive
	{ID: "change.unblock", Request: ChangeUnblockRequest{}, Result: ChangeLifecycleResult{}},               // ChangeUnblock
	{ID: "context.finalize", Request: nil, Result: FinalizeContextResult{}},                                // ContextFinalize
	{ID: "context.implementation", Request: nil, Result: ImplementationContextResult{}},                    // ContextImplementation
	{ID: "development.install", Request: nil, Result: InstallResult{}},                                     // RunDevelopmentInstall
	{ID: "diagnostic.config", Request: nil, Result: ConfigInspectionResult{}},                              // DiagnosticConfig
	{ID: "diagnostic.runtime", Request: nil, Result: RuntimeResult{}},                                      // DiagnosticRuntime
	{ID: "evidence.recertify", Request: nil, Result: EvidenceRecertifyResult{}},                            // EvidenceRecertify
	{ID: "evidence.record", Request: nil, Result: EvidenceOpResult{}},                                      // EvidenceRecord
	{ID: "evidence.verify", Request: nil, Result: EvidenceOpResult{}},                                      // EvidenceVerify
	{ID: "finalize.block", Request: FinalizeBlockInput{}, Result: BlockResult{}},                           // FinalizeBlock
	{ID: "finalize.cleanup", Request: nil, Result: CleanupOpResult{}},                                      // FinalizeCleanup
	{ID: "finalize.clear-block", Request: nil, Result: BlockResult{}},                                      // FinalizeClearBlock
	{ID: "finalize.closeout", Request: CloseoutNotes{}, Result: CloseoutResult{}},                          // FinalizeCloseout
	{ID: "finalize.merge", Request: nil, Result: FinalizeMergeResult{}},                                    // FinalizeMerge
	{ID: "finalize.publish", Request: nil, Result: FinalizePublishResult{}},                                // FinalizePublish
	{ID: "finalize.rebase", Request: nil, Result: FinalizeRebaseResult{}},                                  // FinalizeRebase
	{ID: "finalize.rebase-abort", Request: ResolverReport{}, Result: FinalizeRebaseResult{}},               // FinalizeRebaseAbort
	{ID: "finalize.rebase-continue", Request: ResolverReport{}, Result: FinalizeRebaseResult{}},            // FinalizeRebaseContinue
	{ID: "finalize.resolver-reserve", Request: nil, Result: FinalizeReserveResult{}},                       // FinalizeResolverReserve
	{ID: "finalize.retarget-children", Request: RetargetChildrenInput{}, Result: RetargetChildrenResult{}}, // FinalizeRetargetChildren
	{ID: "gate.cleanup", Request: nil, Result: CleanupOpResult{}},                                          // GateCleanup
	{ID: "gate.drive.advance", Request: nil, Result: GateDriveResult{}},                                    // GateDriveService.Advance
	{ID: "gate.drive.claim", Request: nil, Result: GateDriveResult{}},                                      // GateDriveService.Claim
	{ID: "gate.drive.handoff", Request: nil, Result: GateDriveResult{}},                                    // GateDriveService.Handoff
	{ID: "gate.drive.start", Request: nil, Result: GateDriveResult{}},                                      // GateDriveService.Start
	{ID: "gate.launch", Request: nil, Result: GateResult{}},                                                // GateLaunch
	{ID: "gate.observe", Request: nil, Result: GateResult{}},                                               // GateObserve
	{ID: "gate.recover", Request: nil, Result: GateRecoverResult{}},                                        // GateRecover
	{ID: "gate.stop", Request: nil, Result: GateResult{}},                                                  // GateStop
	{ID: "install", Request: nil, Result: InstallResult{}},                                                 // RunInstall
	{ID: "install.check", Request: nil, Result: InstallResult{}},                                           // RunInstallCheck
	{ID: "install.collect", Request: nil, Result: InstallResult{}},                                         // RunInstallCollect
	{ID: "instructions", Request: nil, Result: InstructionsResult{}},                                       // Instructions
	{ID: "learning.record", Request: LearningRecordRequest{}, Result: LearningResult{}},                    // LearningRecordOp
	{ID: "learning.update", Request: LearningUpdateRequest{}, Result: LearningResult{}},                    // LearningUpdate
	{ID: "maintenance.preflight", Request: nil, Result: MaintenancePreflightResult{}},                      // MaintenancePreflight
	{ID: "maintenance.sweep", Request: nil, Result: MaintenanceResult{}},                                   // MaintenanceSweep
	{ID: "open", Request: nil, Result: OpenResult{}},                                                       // Open
	{ID: "pr.publish", Request: PRPublishInput{}, Result: PRPublishResult{}},                               // PRPublish
	{ID: "repository.check", Request: nil, Result: RepositoryCheckResult{}},                                // RunRepositoryCheck
	{ID: "repository.configure-harnesses", Request: nil, Result: RepositoryOpResult{}},                     // RunRepositoryConfigureHarnesses
	{ID: "repository.configure-tests", Request: nil, Result: RepositoryOpResult{}},                         // RunRepositoryConfigureTests
	{ID: "repository.init", Request: nil, Result: RepositoryOpResult{}},                                    // RunRepositoryInit
	{ID: "repository.migrate", Request: nil, Result: RepositoryMigrateResult{}},                            // RunRepositoryMigrate
	{ID: "repository.prepare", Request: nil, Result: RepositoryPrepareResult{}},                            // RunRepositoryPrepare
	{ID: "repository.repair", Request: nil, Result: RepositoryRepairResult{}},                              // RunRepositoryRepair
	{ID: "repository.set-visibility", Request: nil, Result: RepositorySetVisibilityResult{}},               // RunRepositorySetVisibility
	{ID: "repository.sync-integration", Request: nil, Result: RepositorySyncResult{}},                      // RunRepositorySyncIntegration
	{ID: "run.cancel", Request: nil, Result: RunCancelResult{}},                                            // RunCancel
	{ID: "run.continue", Request: nil, Result: RunContinueResult{}},                                        // RunContinue
	{ID: "run.start", Request: nil, Result: RunStartResult{}},                                              // RunStart
	{ID: "run.verdict", Request: nil, Result: RunVerdictResult{}},                                          // RunVerdict (observe mode returns RunVerdictObserveResult)
	{ID: "run.verify", Request: nil, Result: RunVerifyResult{}},                                            // RunVerify
	{ID: "status", Request: nil, Result: StatusResult{}},                                                   // Status
	{ID: "uninstall", Request: nil, Result: InstallResult{}},                                               // RunUninstall
	{ID: "version", Request: nil, Result: VersionResult{}},                                                 // Version
	{ID: "workspace.commit-spec", Request: nil, Result: WorkspaceOpResult{}},                               // WorkspaceCommitSpec
	{ID: "workspace.inspect", Request: nil, Result: WorkspaceOpResult{}},                                   // WorkspaceInspect
	{ID: "workspace.prepare", Request: nil, Result: WorkspaceOpResult{}},                                   // WorkspacePrepare
	{ID: "workspace.publish", Request: nil, Result: WorkspaceOpResult{}},                                   // WorkspacePublish
}

// OperationBindings returns the complete registry sorted by id. The returned
// slice is a copy, so a caller cannot mutate the package-level registry.
func OperationBindings() []OperationBinding {
	out := make([]OperationBinding, len(operationBindings))
	copy(out, operationBindings)
	return out
}

// OperationSchema is one operation's emitted request/result shape. Request is
// omitted for a leaf that decodes no body; Result excludes the shared envelope
// keys (they are emitted once as SchemaResult.EnvelopeShape).
type OperationSchema struct {
	ID      string          `json:"id"`
	Request *TypeDescriptor `json:"request,omitempty"`
	Result  TypeDescriptor  `json:"result"`
}

// SchemaResult is the assembled schema document — itself a protocol-v1 result.
// The envelope shape is emitted once (EnvelopeShape); each per-op Result excludes
// the envelope's keys so the document does not restate them per operation.
//
// SchemaResult is deliberately NOT itself a schema binding: its own shape is
// self-referential (FieldDescriptor nests []FieldDescriptor) and carries a
// map[string]Vocabulary, so reflectDescriptor cannot describe it — the descriptor
// reflector is for docket's request/result shapes, never for the descriptor
// container that reports them. The schema op's own shape is pinned by
// SchemaVersion instead, and the cli-side correspondence guard accounts for the
// `schema` catalog entry as the sole self-referential no-binding exception.
type SchemaResult struct {
	Envelope
	SchemaVersion int                   `json:"schema_version"`
	EnvelopeShape TypeDescriptor        `json:"envelope"`
	Operations    []OperationSchema     `json:"operations"`
	Vocabularies  map[string]Vocabulary `json:"vocabularies"`
	// Findings normalizes to [] on the success path; the unknown-operation
	// refusal (SchemaUnknownOperation) carries a single FCUnknownOperation entry.
	Findings []StatusFinding `json:"findings"`
}

// Env satisfies OperationResult via the embedded Envelope; HumanText renders the
// compact default text form, mirroring CapabilitiesResult.HumanText: a versioned
// header with the operation count, then one line per operation naming its
// request and result field counts. The document is consumed as JSON, so the
// human form is deliberately compact.
func (r SchemaResult) HumanText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "schema v%d — %d operations", r.SchemaVersion, len(r.Operations))
	for _, op := range r.Operations {
		reqN := 0
		if op.Request != nil {
			reqN = len(op.Request.Fields)
		}
		fmt.Fprintf(&b, "\n  %s  request:%d  result:%d", op.ID, reqN, len(op.Result.Fields))
	}
	return b.String()
}

// SchemaUnknownOperation is the fail-closed refusal for `schema --operation
// <id>` when id names no bound operation: an invalid-input schema document
// carrying a single FCUnknownOperation finding. The cli maps SchemaFor's
// ok=false to this, so an unknown id reads with the same finding vocabulary a
// machine consumer sees everywhere else, and exits 2 (ResultInvalidInput).
func SchemaUnknownOperation(id string) SchemaResult {
	return SchemaResult{
		Envelope:      NewEnvelope("schema", ResultInvalidInput),
		SchemaVersion: SchemaVersion,
		Findings: []StatusFinding{{
			Code:     string(FCUnknownOperation),
			Severity: "error",
			Field:    "operation",
			Message:  fmt.Sprintf("no operation with id %q; consult `docket schema` for the bound ids", id),
		}},
	}
}

// Schema assembles the full schema document over every binding.
func Schema(effects []string) (SchemaResult, error) {
	return schemaFrom(operationBindings, effects)
}

// SchemaFor filters the document to one operation id. ok is false for an id that
// names no binding; the cli maps that to ResultInvalidInput carrying
// FCUnknownOperation.
func SchemaFor(id string, effects []string) (SchemaResult, bool, error) {
	for _, b := range operationBindings {
		if b.ID == id {
			res, err := schemaFrom([]OperationBinding{b}, effects)
			return res, true, err
		}
	}
	return SchemaResult{}, false, nil
}

// schemaFrom builds the document from the given bindings. The envelope shape is
// reflected once from Envelope{}; each per-op result descriptor is reflected from
// its prototype and then filtered to drop the envelope's own keys — the key set
// is COMPUTED from reflectDescriptor(Envelope{}), never a hand-maintained list, so
// an envelope field rename cannot leave a stale per-op copy behind.
func schemaFrom(bindings []OperationBinding, effects []string) (SchemaResult, error) {
	envDesc, err := reflectDescriptor(Envelope{})
	if err != nil {
		return SchemaResult{}, err
	}
	envKeys := make(map[string]bool, len(envDesc.Fields))
	for _, f := range envDesc.Fields {
		envKeys[f.Key] = true
	}

	ops := make([]OperationSchema, 0, len(bindings))
	for _, b := range bindings {
		op := OperationSchema{ID: b.ID}
		if b.Request != nil {
			reqDesc, err := reflectDescriptor(b.Request)
			if err != nil {
				return SchemaResult{}, err
			}
			op.Request = &reqDesc
		}
		resDesc, err := reflectDescriptor(b.Result)
		if err != nil {
			return SchemaResult{}, err
		}
		var fields []FieldDescriptor
		for _, f := range resDesc.Fields {
			if envKeys[f.Key] {
				continue
			}
			fields = append(fields, f)
		}
		op.Result = TypeDescriptor{Fields: fields}
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })

	return SchemaResult{
		Envelope:      NewEnvelope("schema", ResultApplied),
		SchemaVersion: SchemaVersion,
		EnvelopeShape: envDesc,
		Operations:    ops,
		Vocabularies:  SchemaVocabularies(effects),
		Findings:      []StatusFinding{},
	}, nil
}
