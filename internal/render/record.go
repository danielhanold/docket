package render

import (
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
)

// artifactsBlockBody is the empty docket:artifacts managed block a new change
// record carries: the start/end marker pair with no interior rows. The marker
// spelling is the canonical one from internal/document/markers.go
// (startMarkerLine/endMarkerLine); it is reproduced verbatim here because those
// helpers are unexported and this package must not read the filesystem.
const artifactsBlockBody = "<!-- docket:artifacts:start (generated — do not hand-edit) -->\n" +
	"<!-- docket:artifacts:end -->"

// NewChangeRecord describes one canonical proposed change record.
type NewChangeRecord struct {
	ID             domain.ChangeID
	Slug, Title    string
	Type           string    // validated against config change_types by the app layer
	Priority       string    // stored spelling: critical|high|medium|low
	Created        time.Time // date part rendered YYYY-MM-DD UTC
	DependsOn      []domain.ChangeID
	StackedOn      *domain.ChangeID
	Related        []domain.ChangeID
	DiscoveredFrom []domain.ChangeID
	ADRs           []domain.ADRID
	Why            string // markdown body, no heading line
	WhatChanges    string // markdown body, no heading line
	OutOfScope     string // markdown body, no heading line
}

// ChangeRecord serializes r as a canonical brand-new proposed change record.
// Field names, order, and defaults mirror
// skills/docket-new-change/change-template.md (without the template's authoring
// comments); frontmatter is emitted through document.New, so every text scalar
// is single-quoted by construction (ADR-0071) and flow collections stay
// unquoted integer sequences.
func ChangeRecord(r NewChangeRecord) ([]byte, error) {
	created := utcDate(r.Created)
	fields := []document.FieldSpec{
		{Name: "id", Value: document.Int(int64(r.ID))},
		{Name: "slug", Value: document.String(r.Slug)},
		{Name: "title", Value: document.String(r.Title)},
		{Name: "status", Value: document.String(string(domain.StatusProposed))},
		{Name: "priority", Value: document.String(r.Priority)},
		{Name: "type", Value: document.String(r.Type)},
		{Name: "created", Value: document.String(created)},
		{Name: "updated", Value: document.String(created)},
		{Name: "depends_on", Value: changeIDSeq(r.DependsOn)},
		{Name: "stacked_on", Value: changeIDPtr(r.StackedOn)},
		{Name: "related", Value: changeIDSeq(r.Related)},
		{Name: "discovered_from", Value: changeIDSeq(r.DiscoveredFrom)},
		{Name: "adrs", Value: adrIDSeq(r.ADRs)},
		{Name: "spec", Value: document.Null()},
		{Name: "plan", Value: document.Null()},
		{Name: "results", Value: document.Null()},
		{Name: "trivial", Value: document.Bool(false)},
		{Name: "auto_groomable", Value: document.Null()},
		{Name: "branch_prefix", Value: document.Null()},
		{Name: "branch", Value: document.Null()},
		{Name: "pr", Value: document.Null()},
		{Name: "blocked_by", Value: document.Null()},
		{Name: "reconciled", Value: document.Bool(false)},
	}
	body := joinSections(
		section("## Artifacts", artifactsBlockBody),
		section("## Why", r.Why),
		section("## What changes", r.WhatChanges),
		section("## Out of scope", r.OutOfScope),
	)
	return document.New(fields, body)
}

// NewLearningRecord describes one canonical newly-recorded learning finding.
type NewLearningRecord struct {
	Slug, Hook string
	Topics     []string
	Changes    []domain.ChangeID
	Created    time.Time
	Apply      string // markdown body, no heading line
	WarStory   string // markdown body, no heading line
}

// LearningRecord serializes r as a canonical brand-new learning finding: a
// freshly recorded finding carries promotion_state: retained and an empty
// promoted_to. The hook is a free-text scalar, quoted by construction.
func LearningRecord(r NewLearningRecord) ([]byte, error) {
	created := utcDate(r.Created)
	fields := []document.FieldSpec{
		{Name: "slug", Value: document.String(r.Slug)},
		{Name: "hook", Value: document.String(r.Hook)},
		{Name: "topics", Value: stringSeq(r.Topics)},
		{Name: "changes", Value: changeIDSeq(r.Changes)},
		{Name: "created", Value: document.String(created)},
		{Name: "updated", Value: document.String(created)},
		{Name: "promotion_state", Value: document.String("retained")},
		{Name: "promoted_to", Value: document.Null()},
	}
	body := joinSections(
		section("## Apply", r.Apply),
		section("## War story", r.WarStory),
	)
	return document.New(fields, body)
}

// NewADRRecord describes one canonical newly-recorded Accepted ADR.
type NewADRRecord struct {
	ID           domain.ADRID
	Slug, Title  string
	Date         time.Time
	Change       *domain.ChangeID
	RelatesTo    []domain.ADRID
	Supersedes   []domain.ADRID // populated by adr supersede; empty for plain record
	Reverses     []domain.ADRID
	Context      string // markdown body, no heading line
	Decision     string // markdown body, no heading line
	Consequences string // markdown body, no heading line
	Alternatives string // markdown body, no heading line
}

// ADRRecord serializes r as a canonical brand-new Accepted ADR. Field names,
// order, and defaults mirror skills/docket-adr/adr-template.md and the live
// docs/adrs/*.md, with the canonical v1 body carrying ## Alternatives
// considered.
func ADRRecord(r NewADRRecord) ([]byte, error) {
	fields := []document.FieldSpec{
		{Name: "id", Value: document.Int(int64(r.ID))},
		{Name: "slug", Value: document.String(r.Slug)},
		{Name: "title", Value: document.String(r.Title)},
		{Name: "status", Value: document.String("Accepted")},
		{Name: "date", Value: document.String(utcDate(r.Date))},
		{Name: "supersedes", Value: adrIDSeq(r.Supersedes)},
		{Name: "reverses", Value: adrIDSeq(r.Reverses)},
		{Name: "relates_to", Value: adrIDSeq(r.RelatesTo)},
		{Name: "change", Value: changeIDPtr(r.Change)},
	}
	body := joinSections(
		section("## Context", r.Context),
		section("## Decision", r.Decision),
		section("## Consequences", r.Consequences),
		section("## Alternatives considered", r.Alternatives),
	)
	return document.New(fields, body)
}

// utcDate renders t's date part in the canonical YYYY-MM-DD UTC form.
func utcDate(t time.Time) string { return t.UTC().Format("2006-01-02") }

// section assembles one heading + body block. The body is trimmed of its outer
// newlines; an empty body yields the bare heading line.
func section(heading, body string) string {
	body = strings.Trim(body, "\n")
	if body == "" {
		return heading
	}
	return heading + "\n\n" + body
}

// joinSections separates section blocks with exactly one blank line. document.New
// frames the result with the leading blank line and the single trailing newline.
func joinSections(parts ...string) string {
	return strings.Join(parts, "\n\n")
}

// changeIDSeq renders a change-id slice as a flow sequence of integers; an empty
// slice renders "[]", never null.
func changeIDSeq(ids []domain.ChangeID) document.Value {
	items := make([]document.Value, len(ids))
	for i, id := range ids {
		items[i] = document.Int(int64(id))
	}
	return document.Seq(items...)
}

// adrIDSeq renders an ADR-id slice as a flow sequence of integers.
func adrIDSeq(ids []domain.ADRID) document.Value {
	items := make([]document.Value, len(ids))
	for i, id := range ids {
		items[i] = document.Int(int64(id))
	}
	return document.Seq(items...)
}

// stringSeq renders a string slice as a flow sequence of quoted scalars.
func stringSeq(ss []string) document.Value {
	items := make([]document.Value, len(ss))
	for i, s := range ss {
		items[i] = document.String(s)
	}
	return document.Seq(items...)
}

// changeIDPtr renders an optional change id: the integer when set, the bare
// "key:" null form when nil.
func changeIDPtr(id *domain.ChangeID) document.Value {
	if id == nil {
		return document.Null()
	}
	return document.Int(int64(*id))
}
