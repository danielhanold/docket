// Package domain holds docket's pure policy layer: typed immutable entities,
// deterministic validation, lifecycle actions, readiness, selection, and
// graph walks. It performs no filesystem, Git, or subprocess access, and
// imports neither internal/config nor internal/document.
package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// ChangeID is a change's numeric identifier (the NNNN in 0307-<slug>.md).
type ChangeID int

// ADRID is an ADR's numeric identifier (the NNNN in ADR-0071).
type ADRID int

// Status is a change's lifecycle status, drawn from a closed set.
type Status string

// The closed set of change statuses.
const (
	StatusProposed      Status = "proposed"
	StatusInProgress    Status = "in-progress"
	StatusBlocked       Status = "blocked"
	StatusDeferred      Status = "deferred"
	StatusImplemented   Status = "implemented"
	StatusStackedMerged Status = "stacked-merged"
	StatusDone          Status = "done"
	StatusKilled        Status = "killed"
)

// ParseStatus reports whether s is a member of the closed status set,
// returning the typed member when it is. Membership is exact: no case
// folding, no trimming.
func ParseStatus(s string) (Status, bool) {
	switch Status(s) {
	case StatusProposed, StatusInProgress, StatusBlocked, StatusDeferred,
		StatusImplemented, StatusStackedMerged, StatusDone, StatusKilled:
		return Status(s), true
	}
	return "", false
}

// Terminal reports whether the status is an end state — done or killed.
func (s Status) Terminal() bool {
	return s == StatusDone || s == StatusKilled
}

// Priority is a change's priority band, drawn from a closed set.
type Priority string

// The closed set of priorities, in rank order.
const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
)

// ParsePriority reports whether s is a member of the closed priority set,
// returning the typed member when it is.
func ParsePriority(s string) (Priority, bool) {
	switch Priority(s) {
	case PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow:
		return Priority(s), true
	}
	return "", false
}

// priorityRanks is the ONE package-owned rank table: lower sorts first.
var priorityRanks = map[Priority]int{
	PriorityCritical: 0,
	PriorityHigh:     1,
	PriorityMedium:   2,
	PriorityLow:      3,
}

// priorityRank returns the sort rank of p (critical=0 … low=3). An unknown
// or unset priority ranks as medium, so a malformed stored value never
// jumps or sinks the queue.
func priorityRank(p Priority) int {
	if rank, ok := priorityRanks[p]; ok {
		return rank
	}
	return priorityRanks[PriorityMedium]
}

// ADRStatusKind tags which of the four ADR statuses an ADRStatus carries.
type ADRStatusKind string

// The closed set of ADR status kinds.
const (
	ADRAccepted     ADRStatusKind = "accepted"
	ADRDeprecated   ADRStatusKind = "deprecated"
	ADRSupersededBy ADRStatusKind = "superseded-by"
	ADRReversedBy   ADRStatusKind = "reversed-by"
)

// ADRStatus is a tagged value: Accepted, Deprecated, SupersededBy(id), or
// ReversedBy(id). Ref is meaningful only for the two *By kinds.
type ADRStatus struct {
	Kind ADRStatusKind
	Ref  ADRID
}

const (
	supersededByPrefix = "Superseded by ADR-"
	reversedByPrefix   = "Reversed by ADR-"
)

// ParseADRStatus parses the exact v0.9.2 spellings — "Accepted",
// "Deprecated", "Superseded by ADR-NNNN", "Reversed by ADR-NNNN". Matching
// is exact (no case folding, no trimming) and the referenced ID must be a
// run of at least one digit parsing to a positive number. Anything else
// returns ok=false.
func ParseADRStatus(s string) (ADRStatus, bool) {
	switch s {
	case "Accepted":
		return ADRStatus{Kind: ADRAccepted}, true
	case "Deprecated":
		return ADRStatus{Kind: ADRDeprecated}, true
	}
	if tail, found := strings.CutPrefix(s, supersededByPrefix); found {
		return parseADRRef(ADRSupersededBy, tail)
	}
	if tail, found := strings.CutPrefix(s, reversedByPrefix); found {
		return parseADRRef(ADRReversedBy, tail)
	}
	return ADRStatus{}, false
}

// parseADRRef parses the numeric tail of an "ADR-<digits>" reference.
func parseADRRef(kind ADRStatusKind, tail string) (ADRStatus, bool) {
	if tail == "" {
		return ADRStatus{}, false
	}
	for _, r := range tail {
		if r < '0' || r > '9' {
			return ADRStatus{}, false
		}
	}
	n, err := strconv.Atoi(tail)
	if err != nil || n <= 0 {
		return ADRStatus{}, false
	}
	return ADRStatus{Kind: kind, Ref: ADRID(n)}, true
}

// String renders the exact v0.9.2 spelling of the status, zero-padding a
// referenced ADR ID to four digits. An unknown kind renders as its tag.
func (a ADRStatus) String() string {
	switch a.Kind {
	case ADRAccepted:
		return "Accepted"
	case ADRDeprecated:
		return "Deprecated"
	case ADRSupersededBy:
		return fmt.Sprintf("%s%04d", supersededByPrefix, int(a.Ref))
	case ADRReversedBy:
		return fmt.Sprintf("%s%04d", reversedByPrefix, int(a.Ref))
	}
	return string(a.Kind)
}

// PromotionState is a learning's promotion state, drawn from a closed set.
type PromotionState string

// The closed set of promotion states.
const (
	PromotionRetained  PromotionState = "retained"
	PromotionCandidate PromotionState = "candidate"
	PromotionPromoted  PromotionState = "promoted"
)

// ParsePromotionState reports whether s is a member of the closed promotion
// state set. The empty string is the legacy-missing spelling and resolves to
// retained; any other unknown value returns ok=false.
func ParsePromotionState(s string) (PromotionState, bool) {
	if s == "" {
		return PromotionRetained, true
	}
	switch PromotionState(s) {
	case PromotionRetained, PromotionCandidate, PromotionPromoted:
		return PromotionState(s), true
	}
	return "", false
}

// ValidTypeToken reports whether a stored change type matches [a-z][a-z0-9-]*
// — a lowercase letter followed by lowercase letters, digits, or hyphens.
func ValidTypeToken(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && (r >= '0' && r <= '9' || r == '-'):
		default:
			return false
		}
	}
	return true
}

// ValidSlugToken reports whether s matches the shared record-slug grammar —
// lowercase alphanumerics in hyphen-separated runs, with no leading, trailing,
// or doubled hyphen. It is the single rule every slug-shaped identifier is
// held to: change, ADR, and learning slugs, and the repository layer's
// validToken defers to it rather than restating the grammar.
func ValidSlugToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
		case ch == '-':
			if i == 0 || i == len(s)-1 || s[i-1] == '-' {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// BranchForSlug returns the deterministic feature branch name for a slug.
func BranchForSlug(slug string) string {
	return "feat/" + slug
}

// ValidBranchComponent reports whether s is usable as the single path
// component ahead of "/<slug>" in a minted branch name: non-empty, exactly one
// component (no slash, so never ref-qualified), and legal under the same
// shape rules workspace's validBranchRef applies to a full ref.
func ValidBranchComponent(s string) bool {
	if s == "" || strings.Contains(s, "/") {
		return false
	}
	// A component of "refs" mints a branch "refs/<slug>", which reads as
	// ref-qualified — exactly the shape workspace's NewTarget rejects with its
	// HasPrefix(base.Branch, "refs/") guard — so reject the reserved namespace
	// root here.
	if s == "refs" {
		return false
	}
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, ".") {
		return false
	}
	if strings.HasSuffix(s, ".lock") || strings.HasSuffix(s, ".") {
		return false
	}
	if strings.Contains(s, "..") || strings.Contains(s, "@{") {
		return false
	}
	if strings.ContainsAny(s, " \t\r\n\v\f~^:?*[\\") || strings.IndexByte(s, 0) >= 0 {
		return false
	}
	return true
}

// MintBranch constructs the full feature-branch name a claim records:
// (branch_prefix when present and non-empty, otherwise the change type) +
// "/" + slug. This is the ONLY branch-name constructor; every post-claim
// operation consumes the recorded branch: field instead.
func MintBranch(typeToken string, prefix OptionalString, slug string) string {
	p := typeToken
	if prefix.State == FieldPresent && prefix.Value != "" {
		p = prefix.Value
	}
	return p + "/" + slug
}
