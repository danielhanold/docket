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
// KNOWN GAPS (deferred, out of Task 3's stated census+ReasonStatus+domain
// composition): codes minted through the change-create validateChangeCreateShape
// add() closure (invalid-request_id, empty-{title,why,what_changes,out_of_scope},
// invalid-stacked_on) and the app-local ReasonBacklink*/ReasonCloseout* reason
// families are not enumerated here; they are not literal Code:/constructor mints
// the shape guard reaches, and closing the vocabulary over them is Task 6's call.
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
	FCEmptyAttempt,
	FCEmptyChildPRVersion,
	FCEmptyCommit,
	FCEmptyEvidence,
	FCEmptyNoteEntry,
	FCEmptyPath,
	FCEmptyPR,
	FCEmptyReason,
	FCEmptyReport,
	FCEmptyVersion,
	FCEmptyWhyDeferred,
	FCEmptyWhyKilled,
	FindingCode(ReasonStatusExternal),
	FindingCode("illegal-source-status"),
	FindingCode(ReasonStatusInternalError),
	FindingCode(ReasonStatusInterrupted),
	FCInvalidADRs,
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
	FCInvalidRelated,
	FCInvalidRelatesTo,
	FCInvalidSectionHeading,
	FCInvalidSectionIntent,
	FCInvalidSectionMarkdown,
	FCInvalidSlug,
	FindingCode("invalid-successor-id"),
	FindingCode("lease-not-expired"),
	FCLocalMetadataAhead,
	FCLocalMetadataDiverged,
	FindingCode("malformed-claim-stamp"),
	FCMetadataRootForeign,
	FCMetadataRootUnresolved,
	FCMetadataWorktreeDirty,
	FCMigrationIncomplete,
	FindingCode("missing-claim-stamp"),
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
