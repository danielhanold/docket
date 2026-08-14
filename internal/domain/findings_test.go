package domain

import (
	"math/rand"
	"reflect"
	"testing"
)

func sampleFindings() []Finding {
	return []Finding{
		{Code: "b-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityChange, ID: 2}, Field: "status"},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityADR, ID: 5}, Field: "title"},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityADR, ID: 5}, Field: "status"},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityChange, ID: 1, Slug: "b-slug"}},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityChange, ID: 1, Slug: "a-slug"}},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityChange, ID: 1, Slug: "a-slug", Path: "z.md"}},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityRepo}, Field: "root"},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityLearning, Slug: "l"}},
	}
}

func TestReportSortDeterministic(t *testing.T) {
	base := sampleFindings()

	first := NewValidationReport(base).Findings()

	shuffled := sampleFindings()
	rand.New(rand.NewSource(7)).Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	second := NewValidationReport(shuffled).Findings()

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("shuffled input produced different order:\n%#v\n%#v", first, second)
	}

	want := []Finding{
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityADR, ID: 5}, Field: "status"},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityADR, ID: 5}, Field: "title"},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityChange, ID: 1, Slug: "a-slug"}},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityChange, ID: 1, Slug: "a-slug", Path: "z.md"}},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityChange, ID: 1, Slug: "b-slug"}},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityLearning, Slug: "l"}},
		{Code: "a-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityRepo}, Field: "root"},
		{Code: "b-code", Severity: SeverityWarning, Entity: EntityRef{Kind: EntityChange, ID: 2}, Field: "status"},
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("sort key mismatch:\ngot  %#v\nwant %#v", first, want)
	}
}

func TestHasErrors(t *testing.T) {
	if NewValidationReport(nil).HasErrors() {
		t.Fatal("empty report reported errors")
	}
	warnings := []Finding{{Code: "w", Severity: SeverityWarning}}
	if NewValidationReport(warnings).HasErrors() {
		t.Fatal("warnings-only report reported errors")
	}
	mixed := []Finding{{Code: "w", Severity: SeverityWarning}, {Code: "e", Severity: SeverityError}}
	if !NewValidationReport(mixed).HasErrors() {
		t.Fatal("report with one error reported no errors")
	}
}

func TestReportImmutable(t *testing.T) {
	input := []Finding{
		{
			Code:     "a",
			Severity: SeverityWarning,
			Entity:   EntityRef{Kind: EntityChange, ID: 1},
			Related:  []EntityRef{{Kind: EntityADR, ID: 9}},
			Detail:   map[string]string{"k": "v"},
		},
	}
	report := NewValidationReport(input)
	want := report.Findings()

	// Mutating the caller's slice must not reach the report.
	input[0].Code = "mutated"
	input[0].Detail["k"] = "mutated"
	input[0].Related[0].ID = 99

	got := report.Findings()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("report changed after mutating input:\ngot  %#v\nwant %#v", got, want)
	}

	// Mutating a returned copy must not reach the report either.
	got[0].Code = "mutated"
	got[0].Detail["k"] = "mutated"
	got[0].Related[0].ID = 99

	again := report.Findings()
	if !reflect.DeepEqual(again, want) {
		t.Fatalf("report changed after mutating returned copy:\ngot  %#v\nwant %#v", again, want)
	}
}
