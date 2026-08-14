package domain

import (
	"fmt"
	"slices"
	"testing"
)

// acceptedADR builds an Accepted ADR carrying no relationship edges.
func acceptedADR(id ADRID) ADR {
	return NewADR(ADRSpec{ID: id, Status: ADRStatus{Kind: ADRAccepted}})
}

// adrSnapshot builds a snapshot carrying only the supplied ADRs.
func adrSnapshot(adrs ...ADR) Snapshot {
	return NewSnapshot(SnapshotSpec{ADRs: adrs})
}

// findingFor returns the first finding matching code, entity ID, and field.
func findingFor(fs []Finding, code string, id int, field string) (Finding, bool) {
	for _, f := range fs {
		if f.Code == code && f.Entity.ID == id && f.Field == field {
			return f, true
		}
	}
	return Finding{}, false
}

// findingCodes renders the findings as "<code>/<id>/<field>" for diagnostics.
func findingCodes(fs []Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, fmt.Sprintf("%s/%d/%s", f.Code, f.Entity.ID, f.Field))
	}
	return out
}

func TestValidateADRGraphCleanSupersedeAndReverse(t *testing.T) {
	s := adrSnapshot(
		NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRSupersededBy, Ref: 2}}),
		NewADR(ADRSpec{ID: 2, Status: ADRStatus{Kind: ADRAccepted}, Supersedes: []ADRID{1}, RelatesTo: []ADRID{3}}),
		NewADR(ADRSpec{ID: 3, Status: ADRStatus{Kind: ADRReversedBy, Ref: 4}}),
		NewADR(ADRSpec{ID: 4, Status: ADRStatus{Kind: ADRAccepted}, Reverses: []ADRID{3}}),
	)

	got := ValidateADRGraph(s)

	if len(got) != 0 {
		t.Fatalf("clean graph produced findings: %v", findingCodes(got))
	}
}

func TestValidateADRGraphWrongVerbIsAMismatch(t *testing.T) {
	s := adrSnapshot(
		NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRReversedBy, Ref: 2}}),
		NewADR(ADRSpec{ID: 2, Status: ADRStatus{Kind: ADRAccepted}, Supersedes: []ADRID{1}}),
	)

	got := ValidateADRGraph(s)

	f, ok := findingFor(got, "adr-backpointer-mismatch", 2, "supersedes")
	if !ok {
		t.Fatalf("no supersedes mismatch for ADR 2: %v", findingCodes(got))
	}
	if f.Severity != SeverityError {
		t.Fatalf("Severity = %q; want error", f.Severity)
	}
	if f.Detail["expected"] != "Superseded by ADR-0002" || f.Detail["actual"] != "Reversed by ADR-0002" {
		t.Fatalf("Detail = %v; want expected/actual statuses", f.Detail)
	}
	if len(f.Related) != 1 || f.Related[0].ID != 1 || f.Related[0].Kind != EntityADR {
		t.Fatalf("Related = %+v; want the target ADR 1", f.Related)
	}
}

func TestValidateADRGraphStillAcceptedTargetIsAMismatch(t *testing.T) {
	s := adrSnapshot(
		acceptedADR(1),
		NewADR(ADRSpec{ID: 2, Status: ADRStatus{Kind: ADRAccepted}, Supersedes: []ADRID{1}}),
	)

	got := ValidateADRGraph(s)

	if len(got) != 1 {
		t.Fatalf("findings = %v; want exactly the supersedes mismatch", findingCodes(got))
	}
	if _, ok := findingFor(got, "adr-backpointer-mismatch", 2, "supersedes"); !ok {
		t.Fatalf("no supersedes mismatch for ADR 2: %v", findingCodes(got))
	}
}

func TestValidateADRGraphStatusTargetWithoutReciprocalEdge(t *testing.T) {
	s := adrSnapshot(
		NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRSupersededBy, Ref: 3}}),
		acceptedADR(2),
		acceptedADR(3),
	)

	got := ValidateADRGraph(s)

	if len(got) != 1 {
		t.Fatalf("findings = %v; want exactly the status mismatch", findingCodes(got))
	}
	f, ok := findingFor(got, "adr-backpointer-mismatch", 1, "status")
	if !ok {
		t.Fatalf("no status mismatch for ADR 1: %v", findingCodes(got))
	}
	if len(f.Related) != 1 || f.Related[0].ID != 3 {
		t.Fatalf("Related = %+v; want the named ADR 3", f.Related)
	}
}

func TestValidateADRGraphReverseStatusNeedsReversesEdge(t *testing.T) {
	s := adrSnapshot(
		NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRReversedBy, Ref: 2}}),
		NewADR(ADRSpec{ID: 2, Status: ADRStatus{Kind: ADRAccepted}, Reverses: []ADRID{1}}),
	)

	if got := ValidateADRGraph(s); len(got) != 0 {
		t.Fatalf("matched reverse pair produced findings: %v", findingCodes(got))
	}
}

func TestValidateADRGraphDanglingReferences(t *testing.T) {
	cases := []struct {
		name     string
		adr      ADR
		field    string
		severity Severity
	}{
		// relates_to is associative and gates nothing, so a dangling target
		// warns rather than erroring — the same rationale the repository layer
		// applies to a change's associative links.
		{"relates_to", NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRAccepted}, RelatesTo: []ADRID{9}}), "relates_to", SeverityWarning},
		{"supersedes", NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRAccepted}, Supersedes: []ADRID{9}}), "supersedes", SeverityError},
		{"reverses", NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRAccepted}, Reverses: []ADRID{9}}), "reverses", SeverityError},
		{"status", NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRSupersededBy, Ref: 9}}), "status", SeverityError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateADRGraph(adrSnapshot(tc.adr))

			f, ok := findingFor(got, "adr-dangling-reference", 1, tc.field)
			if !ok {
				t.Fatalf("no dangling finding on %s: %v", tc.field, findingCodes(got))
			}
			if f.Severity != tc.severity {
				t.Fatalf("Severity = %q; want %q", f.Severity, tc.severity)
			}
			if f.Detail["lookup"] != "absent" {
				t.Fatalf("Detail[lookup] = %q; want absent", f.Detail["lookup"])
			}
		})
	}
}

func TestValidateADRGraphAmbiguousReferenceIsReported(t *testing.T) {
	s := NewSnapshot(SnapshotSpec{ADRs: []ADR{
		NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRAccepted}, RelatesTo: []ADRID{2}}),
		acceptedADR(2),
		acceptedADR(2),
	}})

	got := ValidateADRGraph(s)

	f, ok := findingFor(got, "adr-dangling-reference", 1, "relates_to")
	if !ok {
		t.Fatalf("no finding for the ambiguous reference: %v", findingCodes(got))
	}
	if f.Detail["lookup"] != "ambiguous" {
		t.Fatalf("Detail[lookup] = %q; want ambiguous", f.Detail["lookup"])
	}
}

func TestValidateADRGraphDanglingChangeBacklink(t *testing.T) {
	s := adrSnapshot(NewADR(ADRSpec{
		ID:     1,
		Status: ADRStatus{Kind: ADRAccepted},
		Change: OptionalInt{State: FieldPresent, Value: 5},
	}))

	got := ValidateADRGraph(s)

	f, ok := findingFor(got, "adr-dangling-reference", 1, "change")
	if !ok {
		t.Fatalf("no finding for the dangling change back-link: %v", findingCodes(got))
	}
	if len(f.Related) != 1 || f.Related[0].Kind != EntityChange || f.Related[0].ID != 5 {
		t.Fatalf("Related = %+v; want change 5", f.Related)
	}
	// The producing-change back-link is associative: a repository holding only
	// part of the corpus legitimately cannot resolve it.
	if f.Severity != SeverityWarning {
		t.Fatalf("Severity = %q; want warning", f.Severity)
	}
}

func TestValidateADRGraphResolvedChangeBacklinkIsClean(t *testing.T) {
	s := NewSnapshot(SnapshotSpec{
		Changes: []Change{NewChange(ChangeSpec{ID: 5, Status: StatusDone})},
		ADRs: []ADR{NewADR(ADRSpec{
			ID:     1,
			Status: ADRStatus{Kind: ADRAccepted},
			Change: OptionalInt{State: FieldPresent, Value: 5},
		})},
	})

	if got := ValidateADRGraph(s); len(got) != 0 {
		t.Fatalf("resolved back-link produced findings: %v", findingCodes(got))
	}
}

func TestValidateADRGraphNumberingGapIsAWarning(t *testing.T) {
	s := adrSnapshot(acceptedADR(1), acceptedADR(2), acceptedADR(4))

	got := ValidateADRGraph(s)

	if len(got) != 1 {
		t.Fatalf("findings = %v; want exactly the gap warning", findingCodes(got))
	}
	f, ok := findingFor(got, "adr-id-gap", 3, "")
	if !ok {
		t.Fatalf("no gap finding for ADR 3: %v", findingCodes(got))
	}
	if f.Severity != SeverityWarning {
		t.Fatalf("Severity = %q; want warning", f.Severity)
	}
	if f.Entity.Kind != EntityADR {
		t.Fatalf("Entity.Kind = %q; want adr", f.Entity.Kind)
	}
}

func TestValidateADRGraphSelfReference(t *testing.T) {
	cases := []struct {
		name  string
		adr   ADR
		field string
	}{
		{"supersedes", NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRAccepted}, Supersedes: []ADRID{1}}), "supersedes"},
		{"reverses", NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRAccepted}, Reverses: []ADRID{1}}), "reverses"},
		{"relates_to", NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRAccepted}, RelatesTo: []ADRID{1}}), "relates_to"},
		{"status", NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRSupersededBy, Ref: 1}}), "status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateADRGraph(adrSnapshot(tc.adr))

			f, ok := findingFor(got, "adr-self-reference", 1, tc.field)
			if !ok {
				t.Fatalf("no self-reference finding on %s: %v", tc.field, findingCodes(got))
			}
			if f.Severity != SeverityError {
				t.Fatalf("Severity = %q; want error", f.Severity)
			}
			if len(got) != 1 {
				t.Fatalf("findings = %v; want only the self-reference", findingCodes(got))
			}
		})
	}
}

func TestValidateADRGraphResultDoesNotAliasStoredState(t *testing.T) {
	s := adrSnapshot(NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRAccepted}, RelatesTo: []ADRID{9}}))

	first := ValidateADRGraph(s)
	first[0].Related = append(first[0].Related, EntityRef{Kind: EntityADR, ID: 42})
	first[0].Detail["lookup"] = "tampered"

	second := ValidateADRGraph(s)
	if len(second[0].Related) != 1 || second[0].Detail["lookup"] != "absent" {
		t.Fatalf("second call observed mutation: %+v", second[0])
	}
}

func TestNextADRID(t *testing.T) {
	cases := []struct {
		name string
		s    Snapshot
		want ADRID
	}{
		{"empty", adrSnapshot(), 1},
		{"gapped", adrSnapshot(acceptedADR(1), acceptedADR(2), acceptedADR(4)), 5},
		{"unordered", adrSnapshot(acceptedADR(7), acceptedADR(3)), 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextADRID(tc.s); got != tc.want {
				t.Fatalf("NextADRID = %d; want %d", got, tc.want)
			}
		})
	}
}

func TestSupersedeAndReverseFlipAcceptedTarget(t *testing.T) {
	s := adrSnapshot(acceptedADR(1))

	got, fail := Supersede(s, 1, 9)
	if fail != nil {
		t.Fatalf("Supersede failed: %v", fail)
	}
	want := ADRActionResult{Target: 1, NewStatus: ADRStatus{Kind: ADRSupersededBy, Ref: 9}}
	if got != want {
		t.Fatalf("Supersede = %+v; want %+v", got, want)
	}

	got, fail = Reverse(s, 1, 9)
	if fail != nil {
		t.Fatalf("Reverse failed: %v", fail)
	}
	want = ADRActionResult{Target: 1, NewStatus: ADRStatus{Kind: ADRReversedBy, Ref: 9}}
	if got != want {
		t.Fatalf("Reverse = %+v; want %+v", got, want)
	}
}

func TestSupersedeAndReverseRefusals(t *testing.T) {
	superseded := NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRSupersededBy, Ref: 2}})
	dup := NewSnapshot(SnapshotSpec{ADRs: []ADR{acceptedADR(1), acceptedADR(1)}})

	cases := []struct {
		name   string
		s      Snapshot
		target ADRID
		succ   ADRID
		kind   PolicyFailureKind
		reason string
	}{
		{"missing target", adrSnapshot(acceptedADR(2)), 1, 9, FailInvalidInput, "unknown-adr"},
		{"ambiguous target", dup, 1, 9, FailInvalidInput, "ambiguous-adr"},
		{"non-positive successor", adrSnapshot(acceptedADR(1)), 1, 0, FailInvalidInput, "invalid-successor-id"},
		{"successor is target", adrSnapshot(acceptedADR(1)), 1, 1, FailInvalidInput, "self-reference"},
		{"already superseded", adrSnapshot(superseded), 1, 9, FailInvalidState, "adr-not-accepted"},
		{"deprecated target", adrSnapshot(NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRDeprecated}})), 1, 9, FailInvalidState, "adr-not-accepted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, act := range []struct {
				name string
				fn   func(Snapshot, ADRID, ADRID) (ADRActionResult, *PolicyFailure)
			}{{"supersede", Supersede}, {"reverse", Reverse}} {
				got, fail := act.fn(tc.s, tc.target, tc.succ)
				if fail == nil {
					t.Fatalf("%s: expected refusal, got %+v", act.name, got)
				}
				if fail.Kind != tc.kind || fail.Reason != tc.reason {
					t.Fatalf("%s: Kind/Reason = %q/%q; want %q/%q", act.name, fail.Kind, fail.Reason, tc.kind, tc.reason)
				}
				if fail.Detail["adr"] == "" {
					t.Fatalf("%s: Detail[adr] empty: %+v", act.name, fail.Detail)
				}
				if (got != ADRActionResult{}) {
					t.Fatalf("%s: result on refusal = %+v; want zero value", act.name, got)
				}
			}
		})
	}
}

func TestValidateADRGraphOrderIsStable(t *testing.T) {
	s := adrSnapshot(
		NewADR(ADRSpec{ID: 1, Status: ADRStatus{Kind: ADRAccepted}, RelatesTo: []ADRID{9}}),
		NewADR(ADRSpec{ID: 3, Status: ADRStatus{Kind: ADRAccepted}, RelatesTo: []ADRID{8}}),
	)

	first := findingCodes(ValidateADRGraph(s))
	second := findingCodes(ValidateADRGraph(s))
	if !slices.Equal(first, second) {
		t.Fatalf("unstable order:\n%v\n%v", first, second)
	}
}
