package app

import (
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
)

// Vocabulary is one closed set, emitted once and referenced by name from
// FieldDescriptor.Enum. Exactly one of Members or Pattern is set: Members for
// a closed enumeration, Pattern for a shape-closed token set (change types,
// which domain deliberately validates by ValidTypeToken's shape, not a list —
// closing them here would reject stored corpora domain accepts, so the
// vocabulary reports the truth: the pattern).
type Vocabulary struct {
	Members []string `json:"members,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
}

// changeTypePattern is the regexp shape domain.ValidTypeToken enforces: a
// lowercase letter followed by lowercase letters, digits, or hyphens. It is
// quoted from ValidTypeToken's own documented shape (that function hand-rolls
// the char-class walk rather than compiling a regexp), and anchored here so a
// consumer can apply it directly. change_types is reported as a Pattern rather
// than a Members list because domain validates stored change types by this
// shape, not against a closed list — an enumerated vocabulary would reject
// stored corpora domain accepts.
const changeTypePattern = `^[a-z][a-z0-9-]*$`

// SchemaVocabularies returns the complete named-vocabulary map the schema
// surface publishes: the core vocabularies plus one Members set per disposition
// family declared by a bound result field's Disposition string.
//
// effects is passed in by the cli caller because package app cannot import
// package cli; every other vocabulary derives from an app/domain/render
// constant group, and change 0399's TestVocabularyConstCompleteness holds each
// emitted Members set in both-directions correspondence with the const group it
// is built from (finding_codes subset-only, because AllFindingCodes folds in
// non-const literal tokens).
//
// Scope note (change 0399, Task 6): the disposition families are derived from
// the existing prefixed const groups every disposition result struct's
// Disposition field draws from. Task 7 wires the per-op enum references in the
// result registry; it can extend this map for any family a later result field
// introduces, and the completeness guard keeps every such addition honest.
func SchemaVocabularies(effects []string) map[string]Vocabulary {
	members := func(count int, at func(int) string) []string {
		out := make([]string, count)
		for i := 0; i < count; i++ {
			out[i] = at(i)
		}
		return out
	}

	v := map[string]Vocabulary{}

	v["finding_codes"] = Vocabulary{Members: members(len(AllFindingCodes), func(i int) string {
		return string(AllFindingCodes[i])
	})}
	v["results"] = Vocabulary{Members: members(len(AllResults), func(i int) string {
		return string(AllResults[i])
	})}
	v["priorities"] = Vocabulary{Members: members(len(domain.AllPriorities), func(i int) string {
		return string(domain.AllPriorities[i])
	})}
	v["statuses"] = Vocabulary{Members: members(len(domain.AllStatuses), func(i int) string {
		return string(domain.AllStatuses[i])
	})}
	v["section_intents"] = Vocabulary{Members: members(len(render.AllSectionIntents), func(i int) string {
		return string(render.AllSectionIntents[i])
	})}
	v["groom_outcomes"] = Vocabulary{Members: []string{string(GroomSpec), string(GroomTrivial)}}
	v["change_types"] = Vocabulary{Pattern: changeTypePattern}
	v["effects"] = Vocabulary{Members: effects}

	// Disposition families: each Members list references the actual constants of
	// its prefixed const group, so a value respelling tracks automatically and
	// the completeness guard reddens on a forgotten or phantom member.
	v["claim_dispositions"] = Vocabulary{Members: []string{
		ClaimDispositionApplied, ClaimDispositionAlreadyClaimed,
		ClaimDispositionContended, ClaimDispositionFailed,
	}}
	v["halt_dispositions"] = Vocabulary{Members: []string{
		HaltDispHalted, HaltDispResumed, HaltDispContended,
		HaltDispRefused, HaltDispFailed,
	}}
	v["reclaim_dispositions"] = Vocabulary{Members: []string{
		ReclaimDispReclaimed, ReclaimDispSkipped,
		ReclaimDispContended, ReclaimDispFailed,
	}}
	v["reconcile_dispositions"] = Vocabulary{Members: []string{
		ReconcileDispositionApplied, ReconcileDispositionContended,
		ReconcileDispositionFailed,
	}}
	v["block_dispositions"] = Vocabulary{Members: []string{
		BlockDispRecorded, BlockDispAlready, BlockDispCleared,
		BlockDispNothingToClear, BlockDispUnknown, BlockDispContended,
		BlockDispRefused, BlockDispFailed,
	}}
	v["cleanup_dispositions"] = Vocabulary{Members: []string{
		CleanupDispCleaned, CleanupDispAlready, CleanupDispPending,
		CleanupDispRetained, CleanupDispChildrenRetargetRequired,
		CleanupDispRebaseScratchCleared,
	}}
	v["closeout_dispositions"] = Vocabulary{Members: []string{
		CloseoutDispDoneArchived, CloseoutDispStackedMerged, CloseoutDispRootArchived,
		CloseoutDispAlready, CloseoutDispChildrenRetargetRequired, CloseoutDispContended,
		CloseoutDispBlocked, CloseoutDispUnknown, CloseoutDispFailed,
	}}
	v["merge_dispositions"] = Vocabulary{Members: []string{
		MergeDispMerged, MergeDispAlreadyMerged, MergeDispContended,
		MergeDispNotMergeable, MergeDispDenied, MergeDispBlocked, MergeDispUnknown,
	}}
	v["publish_dispositions"] = Vocabulary{Members: []string{
		PublishDispPublished, PublishDispNoop, PublishDispContended,
		PublishDispUnknown, PublishDispBlocked,
	}}

	return v
}
