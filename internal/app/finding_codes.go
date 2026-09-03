package app

// FindingCode is the typed vocabulary of stable machine reasons that surface as
// StatusFinding.Code and domain.Finding.Code on the request/result schema. Every
// production finding code in package app is minted from a constant declared
// here; this file is the one sanctioned site for a finding-code string literal.
// TestNoInlineFindingCodeLiterals enforces that rule by syntactic shape across
// the internal/app package.
type FindingCode string

// The census constants: one per distinct finding code minted at a production
// site in package app. Values are the exact stable tokens the schema emits and
// must not be respelled — a consumer keys on them.
const (
	// Request-shape findings on the change/lifecycle/finalize operations
	// (minted through lifecycleFinding and the StatusFinding literal form).
	FCAuthoredInputTooLarge       FindingCode = "authored-input-too-large"
	FCEmptyAttempt                FindingCode = "empty-attempt"
	FCEmptyChildPRVersion         FindingCode = "empty-child_pr_version"
	FCEmptyCommit                 FindingCode = "empty-commit"
	FCEmptyEvidence               FindingCode = "empty-evidence"
	FCEmptyNoteEntry              FindingCode = "empty-note-entry"
	FCEmptyPath                   FindingCode = "empty-path"
	FCEmptyPR                     FindingCode = "empty-pr"
	FCEmptyReason                 FindingCode = "empty-reason"
	FCEmptyReport                 FindingCode = "empty-report"
	FCEmptyVersion                FindingCode = "empty-version"
	FCEmptyWhyDeferred            FindingCode = "empty-why_deferred"
	FCEmptyWhyKilled              FindingCode = "empty-why_killed"
	FCDuplicateChildID            FindingCode = "duplicate-child_id"
	FCInvalidChangeID             FindingCode = "invalid-change_id"
	FCInvalidChildID              FindingCode = "invalid-child_id"
	FCInvalidChildPRNumber        FindingCode = "invalid-child_pr_number"
	FCInvalidHead                 FindingCode = "invalid-head"
	FCInvalidID                   FindingCode = "invalid-id"
	FCInvalidNoteEntry            FindingCode = "invalid-note-entry"
	FCWorkspaceServiceUnavailable FindingCode = "workspace-service-unavailable"

	// State-shape refusals built by refuseLifecycle inside a Plan closure.
	FCArtifactRenderFailed FindingCode = "artifact-render-failed"
	FCNotFound             FindingCode = "not-found"
	FCPathMismatch         FindingCode = "path-mismatch"
	FCSectionEditFailed    FindingCode = "section-edit-failed"

	// change create request-shape findings.
	FCInvalidSlug             FindingCode = "invalid-slug"
	FCUnknownType             FindingCode = "unknown-type"
	FCUnknownPriority         FindingCode = "unknown-priority"
	FCDanglingReference       FindingCode = "dangling-reference"
	FCInvalidDependsOn        FindingCode = "invalid-depends_on"
	FCInvalidRelated          FindingCode = "invalid-related"
	FCInvalidDiscoveredFrom   FindingCode = "invalid-discovered_from"
	FCInvalidADRs             FindingCode = "invalid-adrs"
	FCInvalidRelatesTo        FindingCode = "invalid-relates_to"
	FCInvalidChanges          FindingCode = "invalid-changes"
	FCDuplicateDependsOn      FindingCode = "duplicate-depends_on"
	FCDuplicateRelated        FindingCode = "duplicate-related"
	FCDuplicateDiscoveredFrom FindingCode = "duplicate-discovered_from"
	FCDuplicateADRs           FindingCode = "duplicate-adrs"
	FCDuplicateRelatesTo      FindingCode = "duplicate-relates_to"
	FCDuplicateChanges        FindingCode = "duplicate-changes"

	// change groom section-edit request-shape findings.
	FCInvalidSectionHeading  FindingCode = "invalid-section-heading"
	FCInvalidSectionMarkdown FindingCode = "invalid-section-markdown"
	FCInvalidSectionIntent   FindingCode = "invalid-section-intent"

	// Repository-preparation classifier findings (reposetup.Finding minted in
	// package app) and the corpus artifact/parse findings.
	FCPrepareTopologyUnresolved           FindingCode = "prepare-topology-unresolved"
	FCPrepareLocalStateUnknown            FindingCode = "prepare-local-state-unknown"
	FCDocketDirForeign                    FindingCode = "docket-dir-foreign"
	FCDocketWorktreeAmbiguousRegistration FindingCode = "docket-worktree-ambiguous-registration"
	FCMetadataRootForeign                 FindingCode = "metadata-root-foreign"
	FCMetadataRootUnresolved              FindingCode = "metadata-root-unresolved"
	FCMetadataWorktreeDirty               FindingCode = "metadata-worktree-dirty"
	FCMigrationIncomplete                 FindingCode = "migration-incomplete"
	FCLocalMetadataAhead                  FindingCode = "local-metadata-ahead"
	FCLocalMetadataDiverged               FindingCode = "local-metadata-diverged"
	FCRepositoryFresh                     FindingCode = "repository-fresh"
	FCRepositoryLegacy                    FindingCode = "repository-legacy"
	FCArtifactMissing                     FindingCode = "artifact-missing"
	FCParseFailed                         FindingCode = "parse-failed"
	FCSweepPRFactsUnresolved              FindingCode = "sweep-pr-facts-unresolved"

	// Schema-surface request-shape finding (change 0399, Task 7): SchemaFor
	// returns ok=false for an id absent from the operation-schema registry, and
	// the cli maps that to ResultInvalidInput carrying this code.
	FCUnknownOperation FindingCode = "unknown-operation"

	// Request-shape findings the change-create / adr.record / learning.record /
	// change-groom / change-reconcile / finalize block/clear-block shape
	// validators mint through their addShape/adrFinding/learningFinding closures
	// (change 0399, review). Each is a concrete, wire-emitted code; the composed
	// empty-<field> patterns are enumerated to one constant per field these ops
	// can produce, so the vocabulary is closed over every value that can appear.
	FCInvalidRequestID          FindingCode = "invalid-request_id"
	FCInvalidStackedOn          FindingCode = "invalid-stacked_on"
	FCEmptyTitle                FindingCode = "empty-title"
	FCEmptyWhy                  FindingCode = "empty-why"
	FCEmptyWhatChanges          FindingCode = "empty-what_changes"
	FCEmptyOutOfScope           FindingCode = "empty-out_of_scope"
	FCEmptyContext              FindingCode = "empty-context"
	FCEmptyDecision             FindingCode = "empty-decision"
	FCEmptyConsequences         FindingCode = "empty-consequences"
	FCEmptyAlternatives         FindingCode = "empty-alternatives"
	FCInvalidChangeDotID        FindingCode = "invalid-change-id"
	FCEmptyChangePath           FindingCode = "empty-change-path"
	FCEmptyChangeVersion        FindingCode = "empty-change-version"
	FCInvalidTargetID           FindingCode = "invalid-target-id"
	FCEmptyTargetPath           FindingCode = "empty-target-path"
	FCEmptyTargetVersion        FindingCode = "empty-target-version"
	FCEmptyHook                 FindingCode = "empty-hook"
	FCEmptyApply                FindingCode = "empty-apply"
	FCEmptyWarStory             FindingCode = "empty-war_story"
	FCInvalidTopics             FindingCode = "invalid-topics"
	FCEmptySpecMarkdown         FindingCode = "empty-spec_markdown"
	FCInvalidSpecMarkdown       FindingCode = "invalid-spec_markdown"
	FCMissingRationale          FindingCode = "missing-rationale"
	FCInvalidOutcome            FindingCode = "invalid-outcome"
	FCInvalidSpecSectionHeading FindingCode = "invalid-spec-section-heading"
	FCEmptyReconcileLogEntry    FindingCode = "empty-reconcile_log_entry"
	FCInvalidPRNumber           FindingCode = "invalid-pr_number"
	FCInvalidAttempt            FindingCode = "invalid-attempt"
	FCEmptyHead                 FindingCode = "empty-head"
)

// AllFindingCodes is the authoritative, sorted, deduplicated vocabulary of every
// finding code that surfaces on the request/result schema — the census constants
// above, the status classification reason tokens (ReasonStatus*, folded in by
// FindingCode conversion), and the domain policy-reason tokens that surface as
// finding codes through fail.Reason (the domain constants are unexported, so
// their tokens appear here as the registry's own sanctioned literals). It is
// hand-maintained and hand-sorted; TestFindingCodeRegistryIntegrity asserts the
// ordering, deduplication, and token shape. Task 6 (change 0399) adds the AST
// completeness guard that ties each declared constant to this list.
//
// The request-shape codes the change-create / adr.record / learning.record /
// change-groom / change-reconcile / finalize block/clear-block validators mint
// through their addShape/adrFinding/learningFinding closures are now registered
// FindingCode constants (change 0399, review): invalid-request_id,
// invalid-stacked_on, invalid-{target-id,topics,change-id,outcome,pr_number,
// attempt,spec_markdown,spec-section-heading}, missing-rationale, and the
// enumerated empty-<field> expansions (empty-{title,why,what_changes,
// out_of_scope,context,decision,consequences,alternatives,change-path,
// change-version,target-path,target-version,hook,apply,war_story,spec_markdown,
// reconcile_log_entry,head}). Each expands to exactly one registered member, so
// the vocabulary is closed over every value these ops can emit and the minting
// guard (addShape/adrFinding/learningFinding in ctorLit, plus the composite-
// literal and FindingCode("…") backstops) reddens on any unregistered mint.
//
// KNOWN GAPS (still deferred): the app-local ReasonBacklink*/ReasonCloseout*
// reason families surface through fail.Reason rather than a literal Code:/
// constructor mint the shape guard reaches, so they are not enumerated here.
// AllFindingCodes is authoritative for the const-backed vocabulary the AST
// completeness guard covers.
var AllFindingCodes = []FindingCode{
	FindingCode("adr-not-accepted"),
	FindingCode("ambiguous-adr"),
	FindingCode("ambiguous-change"),
	FCArtifactMissing,
	FCArtifactRenderFailed,
	FCAuthoredInputTooLarge,
	FindingCode("branch-still-exists"),
	FCDanglingReference,
	FCDocketDirForeign,
	FCDocketWorktreeAmbiguousRegistration,
	FCDuplicateADRs,
	FCDuplicateChanges,
	FCDuplicateChildID,
	FCDuplicateDependsOn,
	FCDuplicateDiscoveredFrom,
	FCDuplicateRelated,
	FCDuplicateRelatesTo,
	FCEmptyAlternatives,
	FCEmptyApply,
	FCEmptyAttempt,
	FCEmptyChangePath,
	FCEmptyChangeVersion,
	FCEmptyChildPRVersion,
	FCEmptyCommit,
	FCEmptyConsequences,
	FCEmptyContext,
	FCEmptyDecision,
	FCEmptyEvidence,
	FCEmptyHead,
	FCEmptyHook,
	FCEmptyNoteEntry,
	FCEmptyOutOfScope,
	FCEmptyPath,
	FCEmptyPR,
	FCEmptyReason,
	FCEmptyReconcileLogEntry,
	FCEmptyReport,
	FCEmptySpecMarkdown,
	FCEmptyTargetPath,
	FCEmptyTargetVersion,
	FCEmptyTitle,
	FCEmptyVersion,
	FCEmptyWarStory,
	FCEmptyWhatChanges,
	FCEmptyWhy,
	FCEmptyWhyDeferred,
	FCEmptyWhyKilled,
	FindingCode(ReasonStatusExternal),
	FindingCode("illegal-source-status"),
	FindingCode(ReasonStatusInternalError),
	FindingCode(ReasonStatusInterrupted),
	FCInvalidADRs,
	FCInvalidAttempt,
	FCInvalidChangeDotID,
	FCInvalidChangeID,
	FCInvalidChanges,
	FCInvalidChildID,
	FCInvalidChildPRNumber,
	FCInvalidDependsOn,
	FCInvalidDiscoveredFrom,
	FCInvalidHead,
	FCInvalidID,
	FindingCode(ReasonStatusInvalidInput),
	FCInvalidNoteEntry,
	FCInvalidOutcome,
	FCInvalidPRNumber,
	FCInvalidRelated,
	FCInvalidRelatesTo,
	FCInvalidRequestID,
	FCInvalidSectionHeading,
	FCInvalidSectionIntent,
	FCInvalidSectionMarkdown,
	FCInvalidSlug,
	FCInvalidSpecSectionHeading,
	FCInvalidSpecMarkdown,
	FCInvalidStackedOn,
	FindingCode("invalid-successor-id"),
	FCInvalidTargetID,
	FCInvalidTopics,
	FindingCode("lease-not-expired"),
	FCLocalMetadataAhead,
	FCLocalMetadataDiverged,
	FindingCode("malformed-claim-stamp"),
	FCMetadataRootForeign,
	FCMetadataRootUnresolved,
	FCMetadataWorktreeDirty,
	FCMigrationIncomplete,
	FindingCode("missing-claim-stamp"),
	FCMissingRationale,
	FCNotFound,
	FCParseFailed,
	FCPathMismatch,
	FCPrepareLocalStateUnknown,
	FCPrepareTopologyUnresolved,
	FCRepositoryFresh,
	FCRepositoryLegacy,
	FindingCode("root-ambiguous"),
	FindingCode("root-not-found"),
	FCSectionEditFailed,
	FindingCode("self-reference"),
	FCSweepPRFactsUnresolved,
	FindingCode("unknown-adr"),
	FindingCode("unknown-change"),
	FCUnknownOperation,
	FCUnknownPriority,
	FCUnknownType,
	FCWorkspaceServiceUnavailable,
}

// invalidIDCodeByKey selects the id-shape finding code for a lifecycle request
// by the JSON key the caller decodes the id under. This replaces the former
// "invalid-"+idKey composition (change 0399, Task 2) with a registry lookup over
// the closed key set the lifecycle operations use.
var invalidIDCodeByKey = map[string]FindingCode{
	"id":        FCInvalidID,
	"change_id": FCInvalidChangeID,
}

// invalidIDCode is the fail-closed lookup over invalidIDCodeByKey: it returns the
// registered id-shape FindingCode for idKey, falling back to the generic
// FCInvalidID for any key absent from the map rather than emitting an empty
// Code:"". Every lifecycle caller passes one of the two mapped keys today, so the
// fallback is unreachable in production; it exists so a future caller that adds a
// new key still mints a real, registered finding code (fail-closed ethos, change
// 0399 review). TestInvalidIDCodeByKeyCoversCallSiteKeys guards the closed key
// set the shape validators actually use.
func invalidIDCode(idKey string) FindingCode {
	if code, ok := invalidIDCodeByKey[idKey]; ok {
		return code
	}
	return FCInvalidID
}
