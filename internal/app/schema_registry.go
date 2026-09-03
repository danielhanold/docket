package app

import "sort"

// OperationBinding joins one capabilities operation id to the live Go types its
// handler decodes and returns. Request is nil for leaves that take no JSON body
// (a pure read, or an op whose scalar flags assemble no *Request struct). The id
// is the SAME stable id the capability catalog uses — the join key across the two
// surfaces.
type OperationBinding struct {
	ID      string
	Request any // prototype struct value, e.g. ChangeBlockRequest{}
	Result  any // prototype struct value embedding Envelope
}

// operationBindings is the authoritative registry, one entry per capabilities
// catalog operation that emits a protocol-v1 document. It is DERIVED from the
// live catalog (`docket capabilities --json`) and each cli command's RunE, never
// hand-guessed: every entry's Request is the *Request struct that handler decodes
// or assembles (nil when it assembles none), and every Result is the
// Envelope-embedding document its app function returns. Each derivation names the
// app function symbol it was read from — a symbol name, greppable and drift-
// visible, never a line number (AGENTS.md, ADR-0054).
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
	{ID: "adr.record", Request: ADRRecordRequest{}, Result: ADRResult{}},                                           // ADRRecordOp
	{ID: "adr.reverse", Request: ADRReplaceRequest{}, Result: ADRResult{}},                                         // ADRReverse
	{ID: "adr.supersede", Request: ADRReplaceRequest{}, Result: ADRResult{}},                                       // ADRSupersede
	{ID: "artifact.backlink", Request: ArtifactBacklinkRequest{}, Result: ArtifactBacklinkResult{}},                // ArtifactBacklink
	{ID: "capabilities", Request: nil, Result: CapabilitiesResult{}},                                               // Capabilities
	{ID: "change.attach-plan", Request: ChangeAttachRequest{}, Result: ChangeAttachResult{}},                       // ChangeAttachPlan
	{ID: "change.attach-results", Request: ChangeAttachRequest{}, Result: ChangeAttachResult{}},                    // ChangeAttachResults
	{ID: "change.block", Request: ChangeBlockRequest{}, Result: ChangeLifecycleResult{}},                           // ChangeBlock
	{ID: "change.claim", Request: ChangeClaimRequest{}, Result: ChangeClaimResult{}},                               // ChangeClaim
	{ID: "change.create", Request: ChangeCreateRequest{}, Result: ChangeCreateResult{}},                            // ChangeCreate
	{ID: "change.defer", Request: ChangeDeferRequest{}, Result: ChangeLifecycleResult{}},                           // ChangeDefer
	{ID: "change.groom", Request: ChangeGroomRequest{}, Result: ChangeGroomResult{}},                               // ChangeGroom
	{ID: "change.halt", Request: HaltRequest{}, Result: HaltResult{}},                                              // ChangeHalt
	{ID: "change.kill", Request: ChangeKillRequest{}, Result: ChangeKillResult{}},                                  // ChangeKill
	{ID: "change.mark-implemented", Request: MarkImplementedRequest{}, Result: ChangeLifecycleResult{}},            // ChangeMarkImplemented
	{ID: "change.reclaim", Request: ChangeReclaimRequest{}, Result: ChangeReclaimResult{}},                         // ChangeReclaim
	{ID: "change.reconcile", Request: ChangeReconcileRequest{}, Result: ChangeReconcileResult{}},                   // ChangeReconcile
	{ID: "change.refresh-claim", Request: ChangeClaimRequest{}, Result: ChangeClaimResult{}},                       // ChangeRefreshClaim
	{ID: "change.repair-identity", Request: RepairIdentityRequest{}, Result: RepairIdentityResult{}},               // RepairIdentity
	{ID: "change.resume-halted", Request: ResumeRequest{}, Result: HaltResult{}},                                   // ChangeResumeHalted
	{ID: "context.finalize", Request: FinalizeContextRequest{}, Result: FinalizeContextResult{}},                   // ContextFinalize
	{ID: "context.implementation", Request: ImplementationContextRequest{}, Result: ImplementationContextResult{}}, // ContextImplementation
	{ID: "development.install", Request: nil, Result: InstallResult{}},                                             // RunDevelopmentInstall
	{ID: "diagnostic.config", Request: nil, Result: ConfigInspectionResult{}},                                      // DiagnosticConfig
	{ID: "diagnostic.runtime", Request: nil, Result: RuntimeResult{}},                                              // DiagnosticRuntime
	{ID: "evidence.record", Request: EvidenceRecordRequest{}, Result: EvidenceOpResult{}},                          // EvidenceRecord
	{ID: "evidence.verify", Request: EvidenceVerifyRequest{}, Result: EvidenceOpResult{}},                          // EvidenceVerify
	{ID: "finalize.block", Request: BlockRequest{}, Result: BlockResult{}},                                         // FinalizeBlock
	{ID: "finalize.cleanup", Request: nil, Result: CleanupOpResult{}},                                              // FinalizeCleanup
	{ID: "finalize.clear-block", Request: ClearBlockRequest{}, Result: BlockResult{}},                              // FinalizeClearBlock
	{ID: "finalize.closeout", Request: nil, Result: CloseoutResult{}},                                              // FinalizeCloseout
	{ID: "finalize.merge", Request: FinalizeMergeRequest{}, Result: FinalizeMergeResult{}},                         // FinalizeMerge
	{ID: "finalize.publish", Request: FinalizePublishRequest{}, Result: FinalizePublishResult{}},                   // FinalizePublish
	{ID: "finalize.rebase", Request: FinalizeRebaseRequest{}, Result: FinalizeRebaseResult{}},                      // FinalizeRebase
	{ID: "finalize.rebase-abort", Request: nil, Result: FinalizeRebaseResult{}},                                    // FinalizeRebaseAbort
	{ID: "finalize.rebase-continue", Request: nil, Result: FinalizeRebaseResult{}},                                 // FinalizeRebaseContinue
	{ID: "finalize.retarget-children", Request: RetargetChildrenRequest{}, Result: RetargetChildrenResult{}},       // FinalizeRetargetChildren
	{ID: "gate.cleanup", Request: nil, Result: CleanupOpResult{}},                                                  // GateCleanup
	{ID: "gate.drive.advance", Request: nil, Result: GateDriveResult{}},                                            // GateDriveService.Advance
	{ID: "gate.drive.claim", Request: nil, Result: GateDriveResult{}},                                              // GateDriveService.Claim
	{ID: "gate.drive.handoff", Request: nil, Result: GateDriveResult{}},                                            // GateDriveService.Handoff
	{ID: "gate.drive.prepare-scope", Request: nil, Result: GateScopeResult{}},                                      // GateDriveService.PrepareScope (request is gatedrive.ScopeRequest, not an app *Request)
	{ID: "gate.drive.start", Request: GateDriveStartRequest{}, Result: GateDriveResult{}},                          // GateDriveService.Start
	{ID: "gate.drive.takeover", Request: nil, Result: GateDriveResult{}},                                           // GateDriveService.Takeover
	{ID: "gate.launch", Request: nil, Result: GateResult{}},                                                        // GateLaunch
	{ID: "gate.observe", Request: nil, Result: GateResult{}},                                                       // GateObserve
	{ID: "gate.recover", Request: nil, Result: GateRecoverResult{}},                                                // GateRecover
	{ID: "gate.stop", Request: nil, Result: GateResult{}},                                                          // GateStop
	{ID: "install", Request: nil, Result: InstallResult{}},                                                         // RunInstall
	{ID: "install.check", Request: nil, Result: InstallResult{}},                                                   // RunInstallCheck
	{ID: "learning.record", Request: LearningRecordRequest{}, Result: LearningResult{}},                            // LearningRecordOp
	{ID: "learning.update", Request: LearningUpdateRequest{}, Result: LearningResult{}},                            // LearningUpdate
	{ID: "maintenance.preflight", Request: nil, Result: MaintenancePreflightResult{}},                              // MaintenancePreflight
	{ID: "maintenance.sweep", Request: nil, Result: MaintenanceResult{}},                                           // MaintenanceSweep
	{ID: "pr.publish", Request: PRPublishRequest{}, Result: PRPublishResult{}},                                     // PRPublish
	{ID: "repository.check", Request: nil, Result: RepositoryCheckResult{}},                                        // RunRepositoryCheck
	{ID: "repository.configure-tests", Request: nil, Result: RepositoryOpResult{}},                                 // RunRepositoryConfigureTests
	{ID: "repository.init", Request: nil, Result: RepositoryOpResult{}},                                            // RunRepositoryInit
	{ID: "repository.migrate", Request: nil, Result: RepositoryMigrateResult{}},                                    // RunRepositoryMigrate
	{ID: "repository.prepare", Request: nil, Result: RepositoryPrepareResult{}},                                    // RunRepositoryPrepare
	{ID: "run.gate-before", Request: nil, Result: RunGateBeforeResult{}},                                           // RunGateBefore
	{ID: "run.gate-claim", Request: nil, Result: RunGateClaimResult{}},                                             // RunGateClaim
	{ID: "run.gate-verdict", Request: nil, Result: RunGateVerdictResult{}},                                         // RunGateVerdict (observe mode returns RunGateVerdictObserveResult)
	{ID: "run.verify", Request: RunVerifyRequest{}, Result: RunVerifyResult{}},                                     // RunVerify
	{ID: "status", Request: nil, Result: StatusResult{}},                                                           // Status
	{ID: "version", Request: nil, Result: VersionResult{}},                                                         // Version
	{ID: "workspace.inspect", Request: WorkspaceIDRequest{}, Result: WorkspaceOpResult{}},                          // WorkspaceInspect
	{ID: "workspace.prepare", Request: WorkspaceIDRequest{}, Result: WorkspaceOpResult{}},                          // WorkspacePrepare
	{ID: "workspace.publish", Request: WorkspacePublishRequest{}, Result: WorkspaceOpResult{}},                     // WorkspacePublish
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
type SchemaResult struct {
	Envelope
	SchemaVersion int                   `json:"schema_version"`
	EnvelopeShape TypeDescriptor        `json:"envelope"`
	Operations    []OperationSchema     `json:"operations"`
	Vocabularies  map[string]Vocabulary `json:"vocabularies"`
}

// Env satisfies OperationResult via the embedded Envelope; HumanText renders a
// one-line summary. The schema document is consumed as JSON, so HumanText is
// intentionally terse.
func (r SchemaResult) HumanText() string { return "schema" }

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
	}, nil
}
