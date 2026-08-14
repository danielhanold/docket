package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
)

// mustParse parses a literal record, failing the test when the document layer
// rejects it: a fixture that will not parse tests nothing.
func mustParse(t *testing.T, source string) document.Document {
	t.Helper()
	doc, err := document.Parse([]byte(source))
	if err != nil {
		t.Fatalf("document.Parse: %v", err)
	}
	return doc
}

// input builds an InputDocument around a literal record.
func input(t *testing.T, kind RecordKind, loc RecordLocation, path, source string) InputDocument {
	t.Helper()
	return InputDocument{Kind: kind, Location: loc, Path: path, Document: mustParse(t, source)}
}

// findingCodes lists the codes of findings, for order-independent assertions.
func findingCodes(findings []domain.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Code)
	}
	return out
}

// hasFinding reports whether findings carry one with the given code and field.
func hasFinding(findings []domain.Finding, code, field string) bool {
	for _, f := range findings {
		if f.Code == code && f.Field == field {
			return true
		}
	}
	return false
}

const happyChange = `---
id: 307
slug: domain-snapshot-validation-graphs-and-selection
title: 'Domain snapshot, validation, graphs, and selection'
status: in-progress
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-14
depends_on: [305, 306]
stacked_on: 301
related: [298, 301]
discovered_from: [303]
adrs: [92, 71]
spec: docs/specs/design.md
plan: docs/plans/plan.md
results:
trivial: false
branch: feat/domain-snapshot-validation-graphs-and-selection
claimed_at: 2026-08-14T02:39:23Z
pr: https://github.com/danielhanold/docket/pull/7
issue: 42
blocked_by:
reconciled: true
---

## Why

Body prose.
`

func TestDecodeChangeHappyPath(t *testing.T) {
	in := input(t, KindChange, LocationActive,
		"docs/changes/active/0307-domain-snapshot-validation-graphs-and-selection.md", happyChange)
	change, findings := decodeChange(in)
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %v", findingCodes(findings))
	}

	if change.ID() != 307 {
		t.Errorf("ID = %d, want 307", change.ID())
	}
	if change.Slug() != "domain-snapshot-validation-graphs-and-selection" {
		t.Errorf("Slug = %q", change.Slug())
	}
	if change.Title() != "Domain snapshot, validation, graphs, and selection" {
		t.Errorf("Title = %q", change.Title())
	}
	if change.Status() != domain.StatusInProgress || change.RawStatus() != "in-progress" {
		t.Errorf("Status = %q / raw %q", change.Status(), change.RawStatus())
	}
	if change.Priority() != domain.PriorityCritical || change.RawPriority() != "critical" {
		t.Errorf("Priority = %q / raw %q", change.Priority(), change.RawPriority())
	}
	if change.Type() != "feat" {
		t.Errorf("Type = %q", change.Type())
	}
	created := change.Created()
	if created.State != domain.FieldPresent || !created.Value.Equal(time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)) || created.Raw != "2026-08-12" {
		t.Errorf("Created = %+v", created)
	}
	updated := change.Updated()
	if updated.State != domain.FieldPresent || updated.Raw != "2026-08-14" {
		t.Errorf("Updated = %+v", updated)
	}
	if got := change.DependsOn(); len(got) != 2 || got[0] != 305 || got[1] != 306 {
		t.Errorf("DependsOn = %v", got)
	}
	stacked := change.StackedOn()
	if stacked.State != domain.FieldPresent || stacked.Value != 301 || stacked.Raw != "301" {
		t.Errorf("StackedOn = %+v", stacked)
	}
	if got := change.Related(); len(got) != 2 || got[0] != 298 || got[1] != 301 {
		t.Errorf("Related = %v", got)
	}
	if got := change.DiscoveredFrom(); len(got) != 1 || got[0] != 303 {
		t.Errorf("DiscoveredFrom = %v", got)
	}
	if got := change.ADRs(); len(got) != 2 || got[0] != 92 || got[1] != 71 {
		t.Errorf("ADRs = %v", got)
	}
	if got := change.Spec(); got.State != domain.FieldPresent || got.Value != "docs/specs/design.md" {
		t.Errorf("Spec = %+v", got)
	}
	if got := change.Plan(); got.State != domain.FieldPresent || got.Value != "docs/plans/plan.md" {
		t.Errorf("Plan = %+v", got)
	}
	if got := change.Results(); got.State != domain.FieldEmpty {
		t.Errorf("Results = %+v", got)
	}
	if change.Trivial() {
		t.Error("Trivial = true, want false")
	}
	if got := change.Branch(); got.State != domain.FieldPresent ||
		got.Value != "feat/domain-snapshot-validation-graphs-and-selection" {
		t.Errorf("Branch = %+v", got)
	}
	claimed := change.ClaimedAt()
	if claimed.State != domain.FieldPresent ||
		!claimed.Value.Equal(time.Date(2026, 8, 14, 2, 39, 23, 0, time.UTC)) ||
		claimed.Raw != "2026-08-14T02:39:23Z" {
		t.Errorf("ClaimedAt = %+v", claimed)
	}
	if got := change.PR(); got.State != domain.FieldPresent ||
		got.Value != "https://github.com/danielhanold/docket/pull/7" {
		t.Errorf("PR = %+v", got)
	}
	if got := change.Issue(); got.State != domain.FieldPresent || got.Value != "42" {
		t.Errorf("Issue = %+v", got)
	}
	if got := change.BlockedBy(); got.State != domain.FieldEmpty {
		t.Errorf("BlockedBy = %+v", got)
	}
	if !change.Reconciled() {
		t.Error("Reconciled = false, want true")
	}
	if change.Location() != LocationActive {
		t.Errorf("Location = %q", change.Location())
	}
	if change.Path() != "docs/changes/active/0307-domain-snapshot-validation-graphs-and-selection.md" {
		t.Errorf("Path = %q", change.Path())
	}
	if got := change.ArchiveDate(); got.State != domain.FieldAbsent {
		t.Errorf("ArchiveDate = %+v, want absent for an active record", got)
	}
	if change.HasRunHalted() || change.HasAutoGroomBlocked() ||
		change.HasFinalizeBlocked() || change.HasPublishDeferred() {
		t.Error("no presence marker should be reported for a body with none")
	}
}

// TestDecodeChangeOptionalStates pins the absent/empty/malformed/present
// distinction: nothing is ever laundered into a usable zero value.
func TestDecodeChangeOptionalStates(t *testing.T) {
	tests := []struct {
		name        string
		frontmatter string
		want        func(domain.Change) (domain.FieldState, string)
		wantState   domain.FieldState
		wantRaw     string
		wantFinding bool
		field       string
	}{
		{
			name:        "stacked_on absent",
			frontmatter: "id: 1\nslug: s\n",
			want:        func(c domain.Change) (domain.FieldState, string) { return c.StackedOn().State, c.StackedOn().Raw },
			wantState:   domain.FieldAbsent,
		},
		{
			name:        "stacked_on empty",
			frontmatter: "id: 1\nslug: s\nstacked_on:\n",
			want:        func(c domain.Change) (domain.FieldState, string) { return c.StackedOn().State, c.StackedOn().Raw },
			wantState:   domain.FieldEmpty,
		},
		{
			name:        "stacked_on non-integer is malformed with raw kept",
			frontmatter: "id: 1\nslug: s\nstacked_on: seven\n",
			want:        func(c domain.Change) (domain.FieldState, string) { return c.StackedOn().State, c.StackedOn().Raw },
			wantState:   domain.FieldMalformed,
			wantRaw:     "seven",
			wantFinding: true,
			field:       "stacked_on",
		},
		{
			name:        "created malformed date",
			frontmatter: "id: 1\nslug: s\ncreated: 2026-13-45\n",
			want:        func(c domain.Change) (domain.FieldState, string) { return c.Created().State, c.Created().Raw },
			wantState:   domain.FieldMalformed,
			wantRaw:     "2026-13-45",
			wantFinding: true,
			field:       "created",
		},
		{
			name:        "claimed_at malformed timestamp",
			frontmatter: "id: 1\nslug: s\nclaimed_at: yesterday\n",
			want:        func(c domain.Change) (domain.FieldState, string) { return c.ClaimedAt().State, c.ClaimedAt().Raw },
			wantState:   domain.FieldMalformed,
			wantRaw:     "yesterday",
			wantFinding: true,
			field:       "claimed_at",
		},
		{
			name:        "claimed_at empty is never malformed",
			frontmatter: "id: 1\nslug: s\nclaimed_at:\n",
			want:        func(c domain.Change) (domain.FieldState, string) { return c.ClaimedAt().State, c.ClaimedAt().Raw },
			wantState:   domain.FieldEmpty,
		},
		{
			name:        "spec present",
			frontmatter: "id: 1\nslug: s\nspec: docs/a.md\n",
			want:        func(c domain.Change) (domain.FieldState, string) { return c.Spec().State, c.Spec().Value },
			wantState:   domain.FieldPresent,
			wantRaw:     "docs/a.md",
		},
		{
			name:        "spec quoted empty string counts as empty",
			frontmatter: "id: 1\nslug: s\nspec: \"\"\n",
			want:        func(c domain.Change) (domain.FieldState, string) { return c.Spec().State, c.Spec().Value },
			wantState:   domain.FieldEmpty,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md",
				"---\n"+tc.frontmatter+"---\n\nbody\n")
			change, findings := decodeChange(in)
			gotState, gotRaw := tc.want(change)
			if gotState != tc.wantState {
				t.Errorf("state = %v, want %v", gotState, tc.wantState)
			}
			if gotRaw != tc.wantRaw {
				t.Errorf("raw = %q, want %q", gotRaw, tc.wantRaw)
			}
			got := hasFinding(findings, CodeFieldMalformed, tc.field)
			if got != tc.wantFinding {
				t.Errorf("field-malformed finding on %q = %v, want %v (codes %v)",
					tc.field, got, tc.wantFinding, findingCodes(findings))
			}
		})
	}
}

// TestDecodeChangeIgnoresBodyKeylikeLines pins the body-prose hazard: the
// decoder reads frontmatter only, so a key-shaped line in a paragraph never
// becomes a field value.
func TestDecodeChangeIgnoresBodyKeylikeLines(t *testing.T) {
	const source = `---
id: 1
slug: s
---

## Why

status: done
priority: critical
stacked_on: 99

The lines above are prose about frontmatter, not frontmatter.
`
	change, findings := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", source))
	if change.Status() != "" || change.RawStatus() != "" {
		t.Errorf("status decoded from body prose: %q / %q", change.Status(), change.RawStatus())
	}
	if change.Priority() != "" || change.RawPriority() != "" {
		t.Errorf("priority decoded from body prose: %q / %q", change.Priority(), change.RawPriority())
	}
	if got := change.StackedOn(); got.State != domain.FieldAbsent {
		t.Errorf("stacked_on decoded from body prose: %+v", got)
	}
	if len(findings) != 0 {
		t.Errorf("body prose produced findings: %v", findingCodes(findings))
	}
}

// TestDecodeChangeUnknownFieldsTolerated: a record written by a newer Docket
// still decodes; unknown keys are compatibility data in the document.
func TestDecodeChangeUnknownFieldsTolerated(t *testing.T) {
	const source = `---
id: 1
slug: s
status: proposed
auto_groomable: true
future_field: [1, 2]
---

body
`
	change, findings := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", source))
	if change.ID() != 1 || change.Status() != domain.StatusProposed {
		t.Errorf("known fields did not decode: %+v", change)
	}
	if len(findings) != 0 {
		t.Errorf("unknown fields produced findings: %v", findingCodes(findings))
	}
}

// TestDecodeChangeListsPreserveAuthoredOrder — dependency diagnostics tie-break
// on authored order, and a malformed element is reported, not silently dropped.
func TestDecodeChangeListsPreserveAuthoredOrder(t *testing.T) {
	const source = `---
id: 1
slug: s
depends_on: [9, 2, 40, 2]
related: [7, x]
---

body
`
	change, findings := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", source))
	want := []domain.ChangeID{9, 2, 40, 2}
	got := change.DependsOn()
	if len(got) != len(want) {
		t.Fatalf("DependsOn = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DependsOn = %v, want %v", got, want)
		}
	}
	if rel := change.Related(); len(rel) != 1 || rel[0] != 7 {
		t.Errorf("Related = %v, want the usable ids in authored order", rel)
	}
	if !hasFinding(findings, CodeListItemMalformed, "related") {
		t.Errorf("no list-item-malformed finding for related: %v", findingCodes(findings))
	}
}

// TestDecodeChangeTypeToken — shape is validated, membership is not: a token
// absent from the repository's configured change types stays readable.
func TestDecodeChangeTypeToken(t *testing.T) {
	tests := []struct {
		token       string
		wantFinding bool
	}{
		{"feat", false},
		{"chore-x2", false},
		{"spike", false}, // not in any configured list — still not an error
		{"Feat", true},
		{"2fast", true},
		{"has space", true},
	}
	for _, tc := range tests {
		t.Run(tc.token, func(t *testing.T) {
			source := "---\nid: 1\nslug: s\ntype: \"" + tc.token + "\"\n---\n\nbody\n"
			change, findings := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", source))
			if change.Type() != tc.token {
				t.Errorf("Type = %q, want %q retained as stored", change.Type(), tc.token)
			}
			if got := hasFinding(findings, CodeChangeTypeInvalid, "type"); got != tc.wantFinding {
				t.Errorf("change-type-invalid = %v, want %v", got, tc.wantFinding)
			}
		})
	}
}

// TestDecodeChangePresenceMarkers — the four markers match as whole bare
// heading lines only.
func TestDecodeChangePresenceMarkers(t *testing.T) {
	read := func(c domain.Change) [4]bool {
		return [4]bool{c.HasRunHalted(), c.HasAutoGroomBlocked(), c.HasFinalizeBlocked(), c.HasPublishDeferred()}
	}
	tests := []struct {
		name string
		body string
		want [4]bool
	}{
		{"all four bare headings", "## Run halted\n\n## Auto-groom blocked\n\n## Finalize blocked\n\n## Publish deferred\n", [4]bool{true, true, true, true}},
		{"dated variant does not count", "## Run halted — 2026-08-14\n", [4]bool{false, false, false, false}},
		{"trailing text does not count", "## Finalize blocked (pending)\n", [4]bool{false, false, false, false}},
		{"indented does not count", "  ## Publish deferred\n", [4]bool{false, false, false, false}},
		{"deeper heading level does not count", "### Auto-groom blocked\n", [4]bool{false, false, false, false}},
		{"prose mention does not count", "The run halted; see ## Run halted below.\n", [4]bool{false, false, false, false}},
		{"CRLF body still matches", "## Run halted\r\n", [4]bool{true, false, false, false}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := "---\nid: 1\nslug: s\n---\n\n" + tc.body
			change, _ := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", source))
			if got := read(change); got != tc.want {
				t.Errorf("markers = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDecodeChangeMarkerInFrontmatterNotScanned — only the body is scanned.
func TestDecodeChangeMarkerInFrontmatterNotScanned(t *testing.T) {
	const source = `---
id: 1
slug: s
title: |
  ## Run halted
---

body
`
	change, _ := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", source))
	if change.HasRunHalted() {
		t.Error("a heading inside frontmatter must not count as a presence marker")
	}
}

// TestDecodeChangeArchiveDate — the archive filename's YYYY-MM-DD- prefix.
func TestDecodeChangeArchiveDate(t *testing.T) {
	tests := []struct {
		name      string
		loc       RecordLocation
		path      string
		wantState domain.FieldState
		wantRaw   string
	}{
		{"archived record", LocationArchive, "docs/changes/archive/2026-06-02-0001-results-artifact.md",
			domain.FieldPresent, "2026-06-02"},
		{"archived without prefix", LocationArchive, "docs/changes/archive/0001-results-artifact.md",
			domain.FieldMalformed, "0001-results-artifact.md"},
		{"archived with unparseable prefix", LocationArchive, "docs/changes/archive/2026-13-45-0001-x.md",
			domain.FieldMalformed, "2026-13-45-0001-x.md"},
		{"active record has none", LocationActive, "docs/changes/active/0001-results-artifact.md",
			domain.FieldAbsent, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := "---\nid: 1\nslug: results-artifact\nstatus: done\n---\n\nbody\n"
			change, _ := decodeChange(input(t, KindChange, tc.loc, tc.path, source))
			got := change.ArchiveDate()
			if got.State != tc.wantState || got.Raw != tc.wantRaw {
				t.Errorf("ArchiveDate = %+v, want state %v raw %q", got, tc.wantState, tc.wantRaw)
			}
			if tc.wantState == domain.FieldPresent &&
				!got.Value.Equal(time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)) {
				t.Errorf("ArchiveDate value = %v", got.Value)
			}
		})
	}
}

const happyADR = `---
id: 92
slug: a-stacked-changes-base-is-its-parents-merge-destination
title: "A stacked change's effective base is its parent's merge destination"
status: Accepted
date: 2026-08-12
supersedes: [7]
reverses: []
relates_to: [71, 54]
change: 298
---

## Context

Body.
`

func TestDecodeADRHappyPath(t *testing.T) {
	in := input(t, KindADR, LocationLedger, "docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md", happyADR)
	adr, findings := decodeADR(in)
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %v", findingCodes(findings))
	}
	if adr.ID() != 92 {
		t.Errorf("ID = %d", adr.ID())
	}
	if adr.Slug() != "a-stacked-changes-base-is-its-parents-merge-destination" {
		t.Errorf("Slug = %q", adr.Slug())
	}
	if adr.Title() != "A stacked change's effective base is its parent's merge destination" {
		t.Errorf("Title = %q", adr.Title())
	}
	if adr.Status() != (domain.ADRStatus{Kind: domain.ADRAccepted}) || adr.RawStatus() != "Accepted" {
		t.Errorf("Status = %+v / raw %q", adr.Status(), adr.RawStatus())
	}
	if got := adr.Date(); got.State != domain.FieldPresent || got.Raw != "2026-08-12" {
		t.Errorf("Date = %+v", got)
	}
	if got := adr.Supersedes(); len(got) != 1 || got[0] != 7 {
		t.Errorf("Supersedes = %v", got)
	}
	if got := adr.Reverses(); len(got) != 0 {
		t.Errorf("Reverses = %v", got)
	}
	if got := adr.RelatesTo(); len(got) != 2 || got[0] != 71 || got[1] != 54 {
		t.Errorf("RelatesTo = %v", got)
	}
	if got := adr.Change(); got.State != domain.FieldPresent || got.Value != 298 {
		t.Errorf("Change = %+v", got)
	}
	sum := sha256.Sum256([]byte(happyADR))
	if adr.ContentID() != hex.EncodeToString(sum[:]) {
		t.Errorf("ContentID = %q, want the SHA-256 of the exact source", adr.ContentID())
	}
}

func TestDecodeADRStatus(t *testing.T) {
	tests := []struct {
		status      string
		wantKind    domain.ADRStatusKind
		wantRef     domain.ADRID
		wantFinding bool
	}{
		{"Accepted", domain.ADRAccepted, 0, false},
		{"Deprecated", domain.ADRDeprecated, 0, false},
		{"Superseded by ADR-0099", domain.ADRSupersededBy, 99, false},
		{"Reversed by ADR-0042", domain.ADRReversedBy, 42, false},
		{"accepted", "", 0, true},
		{"Superseded by 99", "", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			source := "---\nid: 5\nslug: a-decision\nstatus: \"" + tc.status + "\"\n---\n\nbody\n"
			adr, findings := decodeADR(input(t, KindADR, LocationLedger, "docs/adrs/0005-a-decision.md", source))
			if adr.Status().Kind != tc.wantKind || adr.Status().Ref != tc.wantRef {
				t.Errorf("Status = %+v", adr.Status())
			}
			if adr.RawStatus() != tc.status {
				t.Errorf("RawStatus = %q, want the stored text retained", adr.RawStatus())
			}
			if adr.ID() != 5 {
				t.Errorf("an unparseable status must still retain the record: ID = %d", adr.ID())
			}
			if got := hasFinding(findings, CodeADRStatusUnparseable, "status"); got != tc.wantFinding {
				t.Errorf("adr-status-unparseable = %v, want %v", got, tc.wantFinding)
			}
		})
	}
}

const happyLearning = `---
slug: adr-update-delivery
hook: "Deliver an ADR body update atomically."
topics: [adr, publishing, git]
changes: [17, 74, 18]
created: 2026-06-17
updated: 2026-07-28
promotion_state: retained
promoted_to:
---

## Apply

Body content.
`

func TestDecodeLearningHappyPath(t *testing.T) {
	learning, findings := decodeLearning(input(t, KindLearning, LocationLedger,
		"docs/changes/learnings/adr-update-delivery.md", happyLearning))
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %v", findingCodes(findings))
	}
	if learning.Slug() != "adr-update-delivery" {
		t.Errorf("Slug = %q", learning.Slug())
	}
	if learning.Hook() != "Deliver an ADR body update atomically." {
		t.Errorf("Hook = %q", learning.Hook())
	}
	if got := learning.Topics(); len(got) != 3 || got[0] != "adr" || got[2] != "git" {
		t.Errorf("Topics = %v", got)
	}
	if got := learning.Changes(); len(got) != 3 || got[0] != 17 || got[1] != 74 || got[2] != 18 {
		t.Errorf("Changes = %v, want authored order", got)
	}
	if got := learning.Created(); got.State != domain.FieldPresent || got.Raw != "2026-06-17" {
		t.Errorf("Created = %+v", got)
	}
	if got := learning.Updated(); got.State != domain.FieldPresent || got.Raw != "2026-07-28" {
		t.Errorf("Updated = %+v", got)
	}
	if learning.Promotion() != domain.PromotionRetained {
		t.Errorf("Promotion = %q", learning.Promotion())
	}
	if got := learning.PromotedTo(); got.State != domain.FieldEmpty {
		t.Errorf("PromotedTo = %+v", got)
	}
	if want := "\n## Apply\n\nBody content.\n"; learning.Content() != want {
		t.Errorf("Content = %q, want %q", learning.Content(), want)
	}
	if learning.Path() != "docs/changes/learnings/adr-update-delivery.md" {
		t.Errorf("Path = %q", learning.Path())
	}
}

func TestDecodeLearningPromotionState(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		want        domain.PromotionState
		wantFinding bool
	}{
		{"retained", "promotion_state: retained\n", domain.PromotionRetained, false},
		{"candidate", "promotion_state: candidate\n", domain.PromotionCandidate, false},
		{"promoted", "promotion_state: promoted\n", domain.PromotionPromoted, false},
		{"empty value is retained", "promotion_state:\n", domain.PromotionRetained, false},
		{"key absent is retained", "", domain.PromotionRetained, false},
		{"unknown token", "promotion_state: pending\n", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := "---\nslug: l\n" + tc.line + "---\n\nbody\n"
			learning, findings := decodeLearning(input(t, KindLearning, LocationLedger, "docs/changes/learnings/l.md", source))
			if learning.Promotion() != tc.want {
				t.Errorf("Promotion = %q, want %q", learning.Promotion(), tc.want)
			}
			if got := hasFinding(findings, CodeLearningPromotionUnknown, "promotion_state"); got != tc.wantFinding {
				t.Errorf("learning-promotion-unknown = %v, want %v", got, tc.wantFinding)
			}
		})
	}
}

func TestDecodeArtifact(t *testing.T) {
	const withBacklink = `<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0307**
<!-- docket:backlink:end -->

# Design
`
	tests := []struct {
		name         string
		path         string
		source       string
		wantKind     domain.ArtifactKind
		wantBacklink bool
	}{
		{"spec", "docs/superpowers/specs/2026-08-13-design.md", withBacklink, domain.ArtifactSpecKind, true},
		{"plan", "docs/superpowers/plans/2026-08-14-plan.md", withBacklink, domain.ArtifactPlan, true},
		{"results", "docs/changes/results/0307-results.md", "# Results\n", domain.ArtifactResults, false},
		{"other", "docs/notes/scratch.md", "# Notes\n", domain.ArtifactOther, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			artifact, findings := decodeArtifact(input(t, KindArtifact, LocationArtifact, tc.path, tc.source))
			if len(findings) != 0 {
				t.Fatalf("unexpected findings: %v", findingCodes(findings))
			}
			if artifact.Path() != tc.path {
				t.Errorf("Path = %q", artifact.Path())
			}
			if artifact.Kind() != tc.wantKind {
				t.Errorf("Kind = %q, want %q", artifact.Kind(), tc.wantKind)
			}
			if artifact.HasBacklinkMarker() != tc.wantBacklink {
				t.Errorf("HasBacklinkMarker = %v, want %v", artifact.HasBacklinkMarker(), tc.wantBacklink)
			}
			sum := sha256.Sum256([]byte(tc.source))
			if artifact.ContentID() != hex.EncodeToString(sum[:]) {
				t.Errorf("ContentID = %q, want the SHA-256 of the exact source", artifact.ContentID())
			}
		})
	}
}

func TestDecodeDerivedView(t *testing.T) {
	tests := []struct {
		path string
		want domain.DerivedViewKind
	}{
		{"docs/changes/BOARD.md", domain.DerivedBoard},
		{"docs/adrs/README.md", domain.DerivedADRIndex},
		{"docs/changes/LEARNINGS.md", domain.DerivedLearningsIndex},
		{"docs/changes/README.md", domain.DerivedOther},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			view, findings := decodeDerived(input(t, KindDerived, LocationDerived, tc.path, "# Generated\n"))
			if len(findings) != 0 {
				t.Fatalf("unexpected findings: %v", findingCodes(findings))
			}
			if view.Path() != tc.path {
				t.Errorf("Path = %q", view.Path())
			}
			if view.Kind() != tc.want {
				t.Errorf("Kind = %q, want %q", view.Kind(), tc.want)
			}
		})
	}
}

// TestDecodeUndecodableRecordRetained — a record with no frontmatter is
// reported, never silently dropped, and keeps its path identity.
func TestDecodeUndecodableRecordRetained(t *testing.T) {
	change, findings := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", "# No frontmatter\n"))
	if !hasFinding(findings, CodeRecordUndecodable, "") {
		t.Errorf("no record-undecodable finding: %v", findingCodes(findings))
	}
	if change.Path() != "docs/changes/active/0001-s.md" || change.Location() != LocationActive {
		t.Errorf("identity lost: path %q location %q", change.Path(), change.Location())
	}

	adr, adrFindings := decodeADR(input(t, KindADR, LocationLedger, "docs/adrs/0001-x.md", "# No frontmatter\n"))
	if !hasFinding(adrFindings, CodeRecordUndecodable, "") {
		t.Errorf("no record-undecodable finding for the ADR: %v", findingCodes(adrFindings))
	}
	if adr.Path() != "docs/adrs/0001-x.md" || adr.ContentID() == "" {
		t.Errorf("ADR identity lost: %+v", adr)
	}
}

// TestDecodeFindingEntityRefs — findings name the entity they are about, so
// the report can be read without the document at hand.
func TestDecodeFindingEntityRefs(t *testing.T) {
	const source = "---\nid: 42\nslug: broken\nstacked_on: nope\n---\n\nbody\n"
	_, findings := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0042-broken.md", source))
	if len(findings) != 1 {
		t.Fatalf("findings = %v, want exactly one", findingCodes(findings))
	}
	got := findings[0]
	want := domain.EntityRef{Kind: domain.EntityChange, ID: 42, Slug: "broken", Path: "docs/changes/active/0042-broken.md"}
	if got.Entity != want {
		t.Errorf("Entity = %+v, want %+v", got.Entity, want)
	}
	if got.Severity != domain.SeverityError {
		t.Errorf("Severity = %q", got.Severity)
	}
	if got.Detail["raw"] != "nope" {
		t.Errorf("Detail = %v, want the raw value preserved", got.Detail)
	}
}

// TestDecodeChangeIDMalformed — an unusable id is reported and never laundered.
func TestDecodeChangeIDMalformed(t *testing.T) {
	const source = "---\nid: seven\nslug: s\n---\n\nbody\n"
	change, findings := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", source))
	if change.ID() != 0 {
		t.Errorf("ID = %d, want 0 for an unusable id", change.ID())
	}
	if !hasFinding(findings, CodeFieldMalformed, "id") {
		t.Errorf("no field-malformed finding for id: %v", findingCodes(findings))
	}
}

// TestDecodeChangeBooleanMalformed — trivial/reconciled are booleans; a
// non-boolean is a finding, not a silent false.
func TestDecodeChangeBooleanMalformed(t *testing.T) {
	const source = "---\nid: 1\nslug: s\ntrivial: maybe\nreconciled: true\n---\n\nbody\n"
	change, findings := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", source))
	if change.Trivial() {
		t.Error("Trivial = true for a malformed value")
	}
	if !change.Reconciled() {
		t.Error("Reconciled = false, want true")
	}
	if !hasFinding(findings, CodeFieldMalformed, "trivial") {
		t.Errorf("no field-malformed finding for trivial: %v", findingCodes(findings))
	}
}

// TestDecodeChangeNonScalarWhereScalarExpected — a collection written where a
// scalar belongs is malformed, not a panic and not a zero value.
func TestDecodeChangeNonScalarWhereScalarExpected(t *testing.T) {
	const source = "---\nid: 1\nslug: s\nbranch: [a, b]\n---\n\nbody\n"
	change, findings := decodeChange(input(t, KindChange, LocationActive, "docs/changes/active/0001-s.md", source))
	if got := change.Branch(); got.State != domain.FieldMalformed {
		t.Errorf("Branch = %+v, want malformed", got)
	}
	if !hasFinding(findings, CodeFieldMalformed, "branch") {
		t.Errorf("no field-malformed finding for branch: %v", findingCodes(findings))
	}
}
