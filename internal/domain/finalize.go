package domain

import (
	"slices"
	"strings"
)

// PRFacts is the live pull-request state a finalize decision consults, copied
// out of a GitHub probe by the app layer. Every field is a plain value so the
// domain reads facts without touching GitHub itself. State and Mergeable are
// each drawn from a closed vocabulary; "unknown"/"UNKNOWN" carry a failed or
// indeterminate probe and are never laundered into a clean absence.
type PRFacts struct {
	Number, Version         string
	State                   string // "open" | "closed" | "merged" | "unknown"
	Draft, Approved         bool
	Mergeable               string // "MERGEABLE" | "CONFLICTING" | "UNKNOWN"
	HeadBranch              string
	HeadOID, BaseRef        string
	ChangedFiles, DiffLines int
	MergedAtUTC             string // RFC3339 or ""
	MergeCommit             string
}

// The closed set of PR state tokens PRFacts.State carries.
const (
	prStateOpen    = "open"
	prStateClosed  = "closed"
	prStateMerged  = "merged"
	prStateUnknown = "unknown"
)

// The closed set of mergeability tokens PRFacts.Mergeable carries.
const (
	mergeableYes         = "MERGEABLE"
	mergeableConflicting = "CONFLICTING"
	mergeableUnknown     = "UNKNOWN"
)

// FinalizeBand names the actionable band a finalize candidate falls in. A
// merged PR needing closeout sorts ahead of open PRs; open PRs split
// MERGEABLE (own band) from CONFLICTING/UNKNOWN (one band behind it).
const (
	bandMergedRecovery = "merged-recovery"
	bandMergeable      = "mergeable"
	bandConflicting    = "conflicting"
	bandUnknown        = "unknown"
)

// The closed set of finalize skip reasons. A skipped candidate is surfaced
// carrying exactly one of these tokens, never dropped.
const (
	skipNotImplemented     = "not-implemented"
	skipDraft              = "draft"
	skipPRClosed           = "pr-closed"
	skipApprovalRequired   = "approval-required"
	skipFinalizeBlocked    = "finalize-blocked"
	skipDependencyUnmerged = "dependency-unmerged"
	skipMalformed          = "malformed"
	skipPRUnknown          = "pr-unknown"
	// The identity skip reasons, surfaced only against a cleanly observed open or
	// merged PR (a closed/unknown PR classifies by the existing bands first —
	// identity repair is meaningless against unknown evidence). The interactive
	// skill and the CLI key on these exact tokens.
	skipBranchMissing   = "branch-missing"          // recorded branch absent/empty; the exact PR's head is the only candidate
	skipBranchMismatch  = "branch-pr-head-mismatch" // recorded branch and the exact PR's head differ
	skipBranchMalformed = "branch-malformed"        // recorded branch is shape-invalid
)

// skippedRank sorts every skipped candidate after every actionable one; it is
// larger than any actionable band rank.
const skippedRank = 1000

// FinalizeCandidate is one change's finalize disposition. Band is one of the
// actionable bands and SkipReason is empty when the change is actionable;
// otherwise Band is empty and SkipReason carries a closed skip token. The
// change is surfaced either way — a skip is reported, never omitted.
type FinalizeCandidate struct {
	ID         ChangeID
	Band       string // "merged-recovery" | "mergeable" | "conflicting" | "unknown"
	SkipReason string // "" when actionable; else closed reason token
}

// finalizeRow pairs a candidate with the sort keys the FinalizeCandidate shape
// does not itself carry, so ordering reads changed-file/line/priority/created
// out of the facts and snapshot rather than off the projected result.
type finalizeRow struct {
	cand     FinalizeCandidate
	sortRank int
	files    int
	lines    int
	priority Priority
	created  OptionalTime
}

// SelectFinalizeQueue returns the finalize disposition of every change that is
// a finalize candidate — non-terminal and carrying a PR reference — bounded by
// allowlist and ordered deterministically. When allowlist is non-empty only
// changes whose ID it names are considered; an empty or nil allowlist filters
// nothing. Population is read from the snapshot in authored order (never from
// facts map iteration), so the result is stable across calls with identical
// inputs.
//
// Ordering: merged-recovery first (closeout work), then dependency-eligible
// open PRs — MERGEABLE before CONFLICTING/UNKNOWN — then smaller ChangedFiles,
// smaller DiffLines, then priority, created date, and ID. Skipped candidates
// sort after every actionable one and are surfaced, not omitted.
//
// The function is nil-safe: a nil facts map, a nil blocked map, a nil
// allowlist, and an empty snapshot all yield an empty slice with no panic. A
// missing facts entry is treated as pr-unknown, never as a clean absence.
func SelectFinalizeQueue(s Snapshot, facts map[ChangeID]PRFacts, blocked map[ChangeID]bool, allowlist []ChangeID) []FinalizeCandidate {
	allow := allowlistSet(allowlist)

	var rows []finalizeRow
	for _, c := range s.Changes() {
		if len(allow) > 0 && !allow[c.ID()] {
			continue
		}
		if c.Status().Terminal() {
			continue
		}
		if !hasPRRef(c) {
			continue
		}

		band, skip, f := classifyFinalize(s, c, facts, blocked)
		rank := skippedRank
		if skip == "" {
			rank = bandRank(band)
		}
		rows = append(rows, finalizeRow{
			cand:     FinalizeCandidate{ID: c.ID(), Band: band, SkipReason: skip},
			sortRank: rank,
			files:    f.ChangedFiles,
			lines:    f.DiffLines,
			priority: c.Priority(),
			created:  c.Created(),
		})
	}

	slices.SortStableFunc(rows, compareFinalizeRow)

	out := make([]FinalizeCandidate, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.cand)
	}
	return out
}

// classifyFinalize decides one change's band or skip reason. Precedence runs
// most-authoritative first: a malformed identity or an unknown PR probe is
// reported before any state-dependent decision; a merged PR takes the recovery
// band regardless of stored status (the merge already happened and needs
// closeout), unless its recorded branch cannot be reconciled with the merged
// PR's own head; then status, PR state, draft, block, dependency, and approval
// gates in turn; an otherwise-actionable open PR whose recorded branch cannot be
// reconciled with the exact PR's head is surfaced with an identity skip rather
// than banded; anything surviving is an actionable open PR banded by
// mergeability. Identity is computed ONLY against cleanly observed open/merged
// evidence — a closed/unknown PR classifies by the existing bands first. f is
// the resolved facts (zero value when absent).
func classifyFinalize(s Snapshot, c Change, facts map[ChangeID]PRFacts, blocked map[ChangeID]bool) (band, skip string, f PRFacts) {
	if _, out := s.Change(c.ID()); out == LookupAmbiguous {
		return "", skipMalformed, PRFacts{}
	}
	if c.ID() <= 0 || !ValidSlugToken(c.Slug()) {
		return "", skipMalformed, PRFacts{}
	}

	f, ok := facts[c.ID()]
	if !ok || f.State == "" || f.State == prStateUnknown {
		return "", skipPRUnknown, f
	}
	if f.State == prStateMerged {
		if skip := identitySkip(c.Branch(), f.HeadBranch); skip != "" {
			return "", skip, f
		}
		return bandMergedRecovery, "", f
	}
	if c.Status() != StatusImplemented && c.Status() != StatusStackedMerged {
		return "", skipNotImplemented, f
	}
	if f.State == prStateClosed {
		return "", skipPRClosed, f
	}
	if f.Draft {
		return "", skipDraft, f
	}
	if blocked[c.ID()] {
		return "", skipFinalizeBlocked, f
	}
	if !EvaluateDependencies(s, c).Satisfied {
		return "", skipDependencyUnmerged, f
	}
	if !f.Approved {
		return "", skipApprovalRequired, f
	}
	if skip := identitySkip(c.Branch(), f.HeadBranch); skip != "" {
		return "", skip, f
	}
	return mergeBand(f.Mergeable), "", f
}

// identitySkip reconciles the branch recorded at claim time with the exact PR's
// own head branch, returning the identity skip token for an unusable or
// disagreeing recorded branch, or "" when the recorded branch is present,
// well-formed, and equal to the head. It is consulted only for a cleanly
// observed open or merged PR. The shape rules mirror the app layer's
// recordedBranch (refs/ prefix, a leading "-", embedded whitespace, "@{", "..",
// or a NUL): a value carrying any of them cannot be a plain feature-branch ref.
func identitySkip(recorded OptionalString, headBranch string) string {
	present := recorded.State == FieldPresent && recorded.Value != ""
	if present && malformedBranchRef(recorded.Value) {
		return skipBranchMalformed
	}
	if !present {
		return skipBranchMissing
	}
	if headBranch != recorded.Value {
		return skipBranchMismatch
	}
	return ""
}

// malformedBranchRef reports whether s cannot be a plain feature-branch ref
// under the same shape rules the app layer's recordedBranch enforces.
func malformedBranchRef(s string) bool {
	return strings.HasPrefix(s, "refs/") || strings.HasPrefix(s, "-") ||
		strings.ContainsAny(s, " \t\r\n\v\f") || strings.Contains(s, "@{") ||
		strings.Contains(s, "..") || strings.IndexByte(s, 0) >= 0
}

// mergeBand maps a mergeability token to its actionable band. Anything that is
// not exactly MERGEABLE or CONFLICTING — UNKNOWN, a blank, or an unrecognized
// value — bands as unknown, so an indeterminate probe never masquerades as
// mergeable.
func mergeBand(mergeable string) string {
	switch mergeable {
	case mergeableYes:
		return bandMergeable
	case mergeableConflicting:
		return bandConflicting
	default:
		return bandUnknown
	}
}

// bandRank orders the actionable bands: merged-recovery, then mergeable, then
// the shared conflicting/unknown band.
func bandRank(band string) int {
	switch band {
	case bandMergedRecovery:
		return 0
	case bandMergeable:
		return 1
	case bandConflicting, bandUnknown:
		return 2
	default:
		return skippedRank
	}
}

// compareFinalizeRow is the total finalize order: sort rank (band, then
// skipped), then fewer changed files, fewer diff lines, higher priority,
// earlier created date, and finally ascending ID, which makes the order total
// because the snapshot excludes ambiguous IDs from a winner.
func compareFinalizeRow(a, b finalizeRow) int {
	if d := a.sortRank - b.sortRank; d != 0 {
		return d
	}
	if d := a.files - b.files; d != 0 {
		return d
	}
	if d := a.lines - b.lines; d != 0 {
		return d
	}
	if d := priorityRank(a.priority) - priorityRank(b.priority); d != 0 {
		return d
	}
	if d := compareCreated(a.created, b.created); d != 0 {
		return d
	}
	return int(a.cand.ID) - int(b.cand.ID)
}

// hasPRRef reports whether c carries a usable PR reference — the manifest
// signal that a change has reached finalize's population.
func hasPRRef(c Change) bool {
	pr := c.PR()
	return pr.State == FieldPresent && pr.Value != ""
}

// allowlistSet builds a membership set from ids, returning nil for an empty or
// nil list so the caller reads "no allowlist filter" rather than "match
// nothing".
func allowlistSet(ids []ChangeID) map[ChangeID]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[ChangeID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// MergeConjuncts is the closed set of preconditions a merge requires, each a
// Boolean that must hold. AllHold reports whether the merge may proceed;
// FirstFailure names the first unmet conjunct so a refusal carries a stable,
// distinct reason token.
type MergeConjuncts struct {
	Implemented, PRIdentityMatch, HeadsAgree, OpenNonDraft,
	BaseIsEffectiveBase, GateSatisfied, ApprovalSatisfied,
	NoOpenChildren, NotSuperseded bool
}

// AllHold reports whether every conjunct holds.
func (m MergeConjuncts) AllHold() bool { return m.FirstFailure() == "" }

// FirstFailure returns the closed token for the first conjunct that does not
// hold, evaluated in field-declaration order, or "" when every conjunct holds.
func (m MergeConjuncts) FirstFailure() string {
	switch {
	case !m.Implemented:
		return "not-implemented"
	case !m.PRIdentityMatch:
		return "pr-identity-mismatch"
	case !m.HeadsAgree:
		return "head-moved"
	case !m.OpenNonDraft:
		return "not-open-nondraft"
	case !m.BaseIsEffectiveBase:
		return "base-mismatch"
	case !m.GateSatisfied:
		return "gate-unsatisfied"
	case !m.ApprovalSatisfied:
		return "approval-required"
	case !m.NoOpenChildren:
		return "open-children"
	case !m.NotSuperseded:
		return "superseded"
	}
	return ""
}
