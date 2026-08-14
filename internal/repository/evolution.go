package repository

import (
	"bytes"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
)

// Evolution finding codes. Like the single-snapshot codes, every one is a
// stable lowercase-hyphen token a caller keys on.
const (
	// CodeADRFrozenContentModified marks an ADR whose already-published bytes
	// changed: anything other than a legal status flip or an appended update
	// section rewrites a record that is supposed to be immutable.
	CodeADRFrozenContentModified = "adr-frozen-content-modified"
	// CodeADRUpdateAfterTerminal marks an update section appended to an ADR
	// that is no longer Accepted. A superseded, reversed, or deprecated
	// decision is closed: the successor carries the new thinking.
	CodeADRUpdateAfterTerminal = "adr-update-after-terminal"
	// CodeADRStatusFlipIllegal marks a status value that changed in a way the
	// lifecycle does not allow — a terminal status reopened or re-aimed, an
	// Accepted status merely respelled, or a flip to an unparseable value.
	CodeADRStatusFlipIllegal = "adr-status-flip-illegal"
	// CodeIdentityMutated marks an existing record whose immutable identity
	// (id or slug) was rewritten in place at the same path.
	CodeIdentityMutated = "identity-mutated"
	// CodeIdentityReused marks a new record that claims an id an existing
	// before-snapshot record already holds at a different path.
	CodeIdentityReused = "identity-reused"
)

// updateHeading is the appended-section heading an ADR may legally grow while
// still Accepted, in both the legacy bare spelling and the current
// "## Update — <date>" spelling.
const updateHeading = "## Update"

// EvolutionInput pairs each record identity with its exact source bytes and
// built snapshot, for both sides of a proposed change. Sources are keyed by
// repository-relative path and are the EXACT bytes the record was read as:
// nothing in this package normalizes, re-encodes, or re-renders them.
type EvolutionInput struct {
	Before, After BuildResult
	// BeforeSources and AfterSources carry path → exact bytes for every
	// document in each snapshot. A path with no supplied bytes is skipped by
	// the byte rules — the composer decides what to supply.
	BeforeSources, AfterSources map[string][]byte
}

// ValidateEvolution checks the before→after rules, writing nothing on either
// side. Findings are returned in a deterministic order: the ADR frozen-content
// rules by path, then the identity rules by path.
func ValidateEvolution(in EvolutionInput) []domain.Finding {
	findings := validateFrozenADRs(in)
	return append(findings, validateIdentityEvolution(in)...)
}

// validateFrozenADRs compares every ADR that exists on both sides at the same
// path. A record that vanished is not this pass's business — deletion is a
// placement question the single-snapshot pass answers.
func validateFrozenADRs(in EvolutionInput) []domain.Finding {
	before := adrsByPath(in.Before.Snapshot)
	after := adrsByPath(in.After.Snapshot)

	var findings []domain.Finding
	for _, recordPath := range slices.Sorted(maps.Keys(before)) {
		afterADR, present := after[recordPath]
		if !present {
			continue
		}
		beforeBytes, haveBefore := in.BeforeSources[recordPath]
		afterBytes, haveAfter := in.AfterSources[recordPath]
		if !haveBefore || !haveAfter {
			continue
		}
		findings = append(findings,
			compareFrozenADR(before[recordPath], afterADR, beforeBytes, afterBytes)...)
	}
	return findings
}

// compareFrozenADR applies the byte rules to one ADR: identical bytes are
// clean, a difference confined to the status value span is a status flip
// judged by the lifecycle, a pure append of update sections is judged by the
// before status, and anything else rewrote a frozen byte.
func compareFrozenADR(before, after domain.ADR, beforeBytes, afterBytes []byte) []domain.Finding {
	if bytes.Equal(beforeBytes, afterBytes) {
		return nil
	}

	beforeSpan, beforeOK := statusValueSpan(beforeBytes)
	afterSpan, afterOK := statusValueSpan(afterBytes)
	if beforeOK && afterOK && statusMaskedEqual(beforeBytes, afterBytes, beforeSpan, afterSpan) {
		return statusFlipFindings(before, after)
	}

	if frozenPrefixIntact(beforeBytes, afterBytes) && appendedUpdateSections(beforeBytes, afterBytes) {
		if isTerminalADRStatus(before.Status()) {
			return []domain.Finding{evolutionFinding(CodeADRUpdateAfterTerminal, adrEntity(before), "", map[string]string{
				"status": before.RawStatus(),
			})}
		}
		return nil
	}

	return []domain.Finding{evolutionFinding(CodeADRFrozenContentModified, adrEntity(before), "", nil)}
}

// statusFlipFindings judges a difference the mask confined to the status value
// span. Only an Accepted ADR may flip, and only to a parseable terminal
// status: a terminal status is closed, and an Accepted status respelled to
// itself is a rewrite of a frozen field wearing a flip's clothes.
func statusFlipFindings(before, after domain.ADR) []domain.Finding {
	beforeStatus, beforeParsed := domain.ParseADRStatus(before.RawStatus())
	afterStatus, afterParsed := domain.ParseADRStatus(after.RawStatus())
	legal := beforeParsed && afterParsed &&
		beforeStatus.Kind == domain.ADRAccepted && afterStatus.Kind != domain.ADRAccepted
	if legal {
		return nil
	}
	return []domain.Finding{evolutionFinding(CodeADRStatusFlipIllegal, adrEntity(before), "status", map[string]string{
		"before": before.RawStatus(),
		"after":  after.RawStatus(),
	})}
}

// statusMaskedEqual reports whether two records are byte-identical once each
// side's status VALUE span is masked out — the prefix before the value and the
// suffix after it are compared directly, with no normalization and no
// re-encode. Masking the value span is what makes a legal status flip
// distinguishable from a rewrite: drop the mask and this degenerates into a
// raw byte comparison that calls every flip a modification.
func statusMaskedEqual(before, after []byte, beforeStatus, afterStatus document.Span) bool {
	if !spanWithin(beforeStatus, len(before)) || !spanWithin(afterStatus, len(after)) {
		return false
	}
	return bytes.Equal(before[:beforeStatus.Start], after[:afterStatus.Start]) &&
		bytes.Equal(before[beforeStatus.End:], after[afterStatus.End:])
}

// frozenPrefixIntact reports whether after begins with the ENTIRE before, byte
// for byte — the check that makes an append an append. Without it, a diff that
// tampers with a published byte while appending a legal-looking update section
// reads as clean.
func frozenPrefixIntact(before, after []byte) bool {
	return len(after) >= len(before) && bytes.Equal(after[:len(before)], before)
}

// appendedUpdateSections reports whether the tail after grew past before opens
// an update section: the first non-blank appended line must be an update
// heading, in either the legacy "## Update" or the current "## Update — …"
// spelling.
func appendedUpdateSections(before, after []byte) bool {
	tail := after[len(before):]
	if len(tail) == 0 {
		return false
	}
	for _, line := range strings.Split(string(tail), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		return line == updateHeading || strings.HasPrefix(line, updateHeading+" ")
	}
	return false
}

// statusValueSpan locates the status field's value token in exact source
// bytes. It parses to find the span and reads nothing else from the parse:
// the comparison always runs over the caller's bytes.
func statusValueSpan(source []byte) (document.Span, bool) {
	doc, err := document.Parse(source)
	if err != nil {
		return document.Span{}, false
	}
	field, found := doc.Field("status")
	if !found || field.Shape == document.ShapeUnsupported {
		return document.Span{}, false
	}
	return field.Value, true
}

// spanWithin reports whether a span addresses a real, non-inverted range of a
// buffer of the given length.
func spanWithin(span document.Span, length int) bool {
	return span.Start >= 0 && span.End >= span.Start && span.End <= length
}

// isTerminalADRStatus reports whether a status closes the decision to further
// updates.
func isTerminalADRStatus(status domain.ADRStatus) bool {
	switch status.Kind {
	case domain.ADRSupersededBy, domain.ADRReversedBy, domain.ADRDeprecated:
		return true
	default:
		return false
	}
}

// validateIdentityEvolution checks the two identity rules across both record
// kinds: an identity rewritten in place at a path that already existed, and a
// new path claiming an id a before-snapshot record already holds.
func validateIdentityEvolution(in EvolutionInput) []domain.Finding {
	findings := identityFindings(
		changeIdentities(in.Before.Snapshot), changeIdentities(in.After.Snapshot))
	return append(findings, identityFindings(
		adrIdentities(in.Before.Snapshot), adrIdentities(in.After.Snapshot))...)
}

// recordIdentity is one record's immutable identity at one path.
type recordIdentity struct {
	kind domain.EntityKind
	id   int
	slug string
	path string
}

// entityRef renders the identity as a finding subject.
func (r recordIdentity) entityRef() domain.EntityRef {
	return domain.EntityRef{Kind: r.kind, ID: r.id, Slug: r.slug, Path: r.path}
}

// identityFindings compares one kind's identities across the two snapshots.
func identityFindings(before, after map[string]recordIdentity) []domain.Finding {
	beforePathsByID := make(map[int][]string, len(before))
	for _, identity := range before {
		beforePathsByID[identity.id] = append(beforePathsByID[identity.id], identity.path)
	}

	var findings []domain.Finding
	for _, recordPath := range slices.Sorted(maps.Keys(after)) {
		now := after[recordPath]
		previous, existed := before[recordPath]
		if existed {
			findings = append(findings, mutationFindings(previous, now)...)
			continue
		}
		held := beforePathsByID[now.id]
		if len(held) == 0 {
			continue
		}
		slices.Sort(held)
		findings = append(findings, evolutionFinding(CodeIdentityReused, now.entityRef(), "id", map[string]string{
			"held_by": strings.Join(held, ", "),
		}))
	}
	return findings
}

// mutationFindings reports an id or slug rewritten in place. Both can be wrong
// at once, and each is its own finding: a repair addresses one field.
func mutationFindings(previous, now recordIdentity) []domain.Finding {
	var findings []domain.Finding
	if previous.id != now.id {
		findings = append(findings, identityMutation(previous, "id",
			strconv.Itoa(previous.id), strconv.Itoa(now.id)))
	}
	if previous.slug != now.slug {
		findings = append(findings, identityMutation(previous, "slug", previous.slug, now.slug))
	}
	return findings
}

// identityMutation builds one in-place identity-rewrite finding, subject to
// the identity the record USED to have: that is the identity anything else in
// the repository still points at.
func identityMutation(previous recordIdentity, field, was, is string) domain.Finding {
	return evolutionFinding(CodeIdentityMutated, previous.entityRef(), field, map[string]string{
		"before": was,
		"after":  is,
	})
}

// changeIdentities indexes a snapshot's changes by path.
func changeIdentities(s domain.Snapshot) map[string]recordIdentity {
	out := make(map[string]recordIdentity)
	for _, change := range s.Changes() {
		out[change.Path()] = recordIdentity{
			kind: domain.EntityChange, id: int(change.ID()), slug: change.Slug(), path: change.Path(),
		}
	}
	return out
}

// adrIdentities indexes a snapshot's ADRs by path.
func adrIdentities(s domain.Snapshot) map[string]recordIdentity {
	out := make(map[string]recordIdentity)
	for _, adr := range s.ADRs() {
		out[adr.Path()] = recordIdentity{
			kind: domain.EntityADR, id: int(adr.ID()), slug: adr.Slug(), path: adr.Path(),
		}
	}
	return out
}

// adrsByPath indexes a snapshot's ADR entities by path.
func adrsByPath(s domain.Snapshot) map[string]domain.ADR {
	out := make(map[string]domain.ADR)
	for _, adr := range s.ADRs() {
		out[adr.Path()] = adr
	}
	return out
}

// adrEntity renders an ADR as a finding subject.
func adrEntity(a domain.ADR) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityADR, ID: int(a.ID()), Slug: a.Slug(), Path: a.Path()}
}

// evolutionFinding builds one error finding; every evolution rule is an error.
func evolutionFinding(code string, entity domain.EntityRef, field string, detail map[string]string) domain.Finding {
	return domain.Finding{
		Code:     code,
		Severity: domain.SeverityError,
		Entity:   entity,
		Field:    field,
		Detail:   detail,
	}
}
