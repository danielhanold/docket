package domain

import (
	"cmp"
	"maps"
	"slices"
)

// Severity ranks a validation finding. Only errors are fatal; warnings report
// something worth fixing that does not invalidate the snapshot.
type Severity string

// The closed set of severities.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// EntityKind names the kind of thing a finding is about.
type EntityKind string

// The closed set of entity kinds.
const (
	EntityChange   EntityKind = "change"
	EntityADR      EntityKind = "adr"
	EntityLearning EntityKind = "learning"
	EntityArtifact EntityKind = "artifact"
	EntityDerived  EntityKind = "derived-view"
	EntityRepo     EntityKind = "repository"
)

// EntityRef identifies the subject of a finding. ID is 0 when the entity's
// identity is non-numeric (learnings, artifacts, the repository itself); Slug
// and Path are empty when they do not apply.
type EntityRef struct {
	Kind EntityKind
	ID   int // 0 when identity is non-numeric
	Slug string
	Path string
}

// Finding is a single typed validation result. Code is the stable machine
// identifier; Detail carries code-specific context as plain strings so that
// callers never have to type-switch on an any.
type Finding struct {
	Code     string
	Severity Severity
	Entity   EntityRef
	Field    string
	Related  []EntityRef
	Detail   map[string]string
}

// clone returns a deep copy of f, so that neither the caller's slices/maps nor
// a returned copy's aliases can reach into a constructed report.
func (f Finding) clone() Finding {
	out := f
	out.Related = slices.Clone(f.Related)
	out.Detail = maps.Clone(f.Detail)
	return out
}

// compareFindings implements the documented sort key: Code, Entity.Kind,
// Entity.ID, Entity.Slug, Entity.Path, Field.
func compareFindings(a, b Finding) int {
	if c := cmp.Compare(a.Code, b.Code); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Entity.Kind, b.Entity.Kind); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Entity.ID, b.Entity.ID); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Entity.Slug, b.Entity.Slug); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Entity.Path, b.Entity.Path); c != 0 {
		return c
	}
	return cmp.Compare(a.Field, b.Field)
}

// ValidationReport is an immutable, deterministically ordered set of findings.
type ValidationReport struct {
	findings []Finding
}

// NewValidationReport copies the given findings and orders them by the
// documented sort key, so that two runs over the same findings in any input
// order produce byte-identical output.
func NewValidationReport(findings []Finding) ValidationReport {
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.clone())
	}
	slices.SortStableFunc(out, compareFindings)
	return ValidationReport{findings: out}
}

// Findings returns a fresh deep copy of the report's findings; mutating the
// result cannot affect the report or any other caller's copy.
func (r ValidationReport) Findings() []Finding {
	out := make([]Finding, 0, len(r.findings))
	for _, f := range r.findings {
		out = append(out, f.clone())
	}
	return out
}

// HasErrors reports whether any finding carries SeverityError.
func (r ValidationReport) HasErrors() bool {
	return slices.ContainsFunc(r.findings, func(f Finding) bool {
		return f.Severity == SeverityError
	})
}
