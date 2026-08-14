package repository

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
)

// buildSources builds one snapshot side from path → exact source bytes, using
// the same path-to-kind classification a composer would apply.
func buildSources(t *testing.T, sources map[string]string) BuildResult {
	t.Helper()
	docs := make([]InputDocument, 0, len(sources))
	for _, recordPath := range slices.Sorted(maps.Keys(sources)) {
		kind, location := classifyCorpusPath(recordPath)
		docs = append(docs, record(t, kind, location, recordPath, sources[recordPath]))
	}
	return buildRecords(t, docs...)
}

// rawSources converts the literal fixtures into the exact-byte maps evolution
// compares. The bytes handed to the builder and the bytes compared here are
// the same bytes: nothing in this file normalizes a record.
func rawSources(sources map[string]string) map[string][]byte {
	out := make(map[string][]byte, len(sources))
	for recordPath, source := range sources {
		out[recordPath] = []byte(source)
	}
	return out
}

// evolutionInput pairs two literal repositories into one before/after input.
func evolutionInput(t *testing.T, before, after map[string]string) EvolutionInput {
	t.Helper()
	return EvolutionInput{
		Before:        buildSources(t, before),
		After:         buildSources(t, after),
		BeforeSources: rawSources(before),
		AfterSources:  rawSources(after),
	}
}

// adrPath is the ledger path a well-formed ADR record sits at.
func adrPath(id int, slug string) string {
	return fmt.Sprintf("docs/adrs/%0*d-%s.md", idDigits, id, slug)
}

// changePath is the active-directory path a well-formed change sits at.
func changePath(id int, slug string) string {
	return fmt.Sprintf("docs/changes/active/%0*d-%s.md", idDigits, id, slug)
}

// acceptedADR is the frozen before-state every ADR case starts from.
func acceptedADR() string { return minimalADR(7, "a-decision", "Accepted") }

// codesOf lists the codes of the findings a case produced.
func codesOf(findings []domain.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Code)
	}
	slices.Sort(out)
	return out
}

func TestValidateEvolutionStatusOnlyFlipIsClean(t *testing.T) {
	before := map[string]string{adrPath(7, "a-decision"): acceptedADR()}
	after := map[string]string{
		adrPath(7, "a-decision"):  minimalADR(7, "a-decision", "Superseded by ADR-0099"),
		adrPath(99, "the-sequel"): minimalADR(99, "the-sequel", "Accepted", "supersedes: [7]"),
	}
	if got := ValidateEvolution(evolutionInput(t, before, after)); len(got) != 0 {
		t.Fatalf("status-only flip should be clean, got %v", codesOf(got))
	}
}

func TestValidateEvolutionAcceptsAppendedUpdateSections(t *testing.T) {
	cases := map[string]string{
		"legacy heading":   "\n## Update\n\nSomething changed.\n",
		"current heading":  "\n## Update — 2026-08-14\n\nSomething changed.\n",
		"two appended":     "\n## Update — 2026-08-14\n\nOne.\n\n## Update — 2026-08-15\n\nTwo.\n",
		"blank lines only": "\n\n\n## Update — 2026-08-14\n\nSomething changed.\n",
	}
	for name, tail := range cases {
		t.Run(name, func(t *testing.T) {
			before := map[string]string{adrPath(7, "a-decision"): acceptedADR()}
			after := map[string]string{adrPath(7, "a-decision"): acceptedADR() + tail}
			if got := ValidateEvolution(evolutionInput(t, before, after)); len(got) != 0 {
				t.Fatalf("append while Accepted should be clean, got %v", codesOf(got))
			}
		})
	}
}

func TestValidateEvolutionRejectsEveryFrozenByteChange(t *testing.T) {
	withUpdate := acceptedADR() + "\n## Update — 2026-08-01\n\nFirst update.\n"
	cases := []struct {
		name          string
		before, after string
	}{
		{
			name:   "decision body edited",
			before: acceptedADR(),
			after:  strings.Replace(acceptedADR(), "## Decision\n\nBody.", "## Decision\n\nDifferent body.", 1),
		},
		{
			name:   "earlier update edited",
			before: withUpdate,
			after:  strings.Replace(withUpdate, "First update.", "Rewritten update.", 1),
		},
		{
			name:   "body comment edited",
			before: acceptedADR() + "\n<!-- note: one -->\n",
			after:  acceptedADR() + "\n<!-- note: two -->\n",
		},
		{
			name:   "unknown frontmatter field edited",
			before: minimalADR(7, "a-decision", "Accepted", "house_field: one"),
			after:  minimalADR(7, "a-decision", "Accepted", "house_field: two"),
		},
		{
			name:   "title edited",
			before: acceptedADR(),
			after:  strings.Replace(acceptedADR(), "'A decision'", "'A different decision'", 1),
		},
		{
			name:   "status flip with a body edit in the same diff",
			before: acceptedADR(),
			after: strings.Replace(
				strings.Replace(acceptedADR(), "status: Accepted", "status: Superseded by ADR-0099", 1),
				"Body.", "Edited body.", 1),
		},
		{
			name:   "trailing content appended that is not an update section",
			before: acceptedADR(),
			after:  acceptedADR() + "\nAn afterthought.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := evolutionInput(t,
				map[string]string{adrPath(7, "a-decision"): tc.before},
				map[string]string{adrPath(7, "a-decision"): tc.after})
			got := ValidateEvolution(in)
			if len(got) != 1 || got[0].Code != CodeADRFrozenContentModified {
				t.Fatalf("want one %s, got %v", CodeADRFrozenContentModified, codesOf(got))
			}
			if got[0].Severity != domain.SeverityError {
				t.Fatalf("severity = %q, want error", got[0].Severity)
			}
			want := domain.EntityRef{
				Kind: domain.EntityADR, ID: 7, Slug: "a-decision", Path: adrPath(7, "a-decision"),
			}
			if got[0].Entity != want {
				t.Fatalf("entity = %+v, want %+v", got[0].Entity, want)
			}
		})
	}
}

func TestValidateEvolutionRejectsUpdateAfterTerminalStatus(t *testing.T) {
	for _, status := range []string{"Superseded by ADR-0099", "Reversed by ADR-0099", "Deprecated"} {
		t.Run(status, func(t *testing.T) {
			terminal := minimalADR(7, "a-decision", status)
			in := evolutionInput(t,
				map[string]string{adrPath(7, "a-decision"): terminal},
				map[string]string{adrPath(7, "a-decision"): terminal + "\n## Update — 2026-08-14\n\nLate.\n"})
			got := ValidateEvolution(in)
			if len(got) != 1 || got[0].Code != CodeADRUpdateAfterTerminal {
				t.Fatalf("want one %s, got %v", CodeADRUpdateAfterTerminal, codesOf(got))
			}
			if got[0].Severity != domain.SeverityError {
				t.Fatalf("severity = %q, want error", got[0].Severity)
			}
		})
	}
}

func TestValidateEvolutionRejectsIllegalStatusFlips(t *testing.T) {
	cases := []struct {
		name          string
		before, after string
	}{
		{"terminal reopened", "Superseded by ADR-0099", "Accepted"},
		{"terminal re-aimed", "Superseded by ADR-0099", "Superseded by ADR-0100"},
		{"accepted respelled", "Accepted", "'Accepted'"},
		{"flipped to an unparseable status", "Accepted", "Retired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := evolutionInput(t,
				map[string]string{adrPath(7, "a-decision"): minimalADR(7, "a-decision", tc.before)},
				map[string]string{adrPath(7, "a-decision"): minimalADR(7, "a-decision", tc.after)})
			got := ValidateEvolution(in)
			if len(got) != 1 || got[0].Code != CodeADRStatusFlipIllegal {
				t.Fatalf("want one %s, got %v", CodeADRStatusFlipIllegal, codesOf(got))
			}
		})
	}
}

func TestValidateEvolutionRejectsIdentityMutation(t *testing.T) {
	cases := []struct {
		name          string
		path          string
		before, after string
		field         string
	}{
		{
			name:   "change slug rewritten in place",
			path:   changePath(42, "alpha"),
			before: minimalChange(42, "alpha", "proposed"),
			after:  minimalChange(42, "renamed", "proposed"),
			field:  "slug",
		},
		{
			name:   "change id rewritten in place",
			path:   changePath(42, "alpha"),
			before: minimalChange(42, "alpha", "proposed"),
			after:  minimalChange(43, "alpha", "proposed"),
			field:  "id",
		},
		{
			name:   "adr slug rewritten in place",
			path:   adrPath(7, "a-decision"),
			before: acceptedADR(),
			after:  minimalADR(7, "renamed", "Accepted"),
			field:  "slug",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := evolutionInput(t,
				map[string]string{tc.path: tc.before},
				map[string]string{tc.path: tc.after})
			got := ValidateEvolution(in)
			mutations := findingsWithCode(got, CodeIdentityMutated)
			if len(mutations) != 1 {
				t.Fatalf("want one %s, got %v", CodeIdentityMutated, codesOf(got))
			}
			if mutations[0].Field != tc.field {
				t.Fatalf("field = %q, want %q", mutations[0].Field, tc.field)
			}
			if mutations[0].Severity != domain.SeverityError {
				t.Fatalf("severity = %q, want error", mutations[0].Severity)
			}
		})
	}
}

func TestValidateEvolutionRejectsIdentityReuseAtANewPath(t *testing.T) {
	before := map[string]string{changePath(42, "alpha"): minimalChange(42, "alpha", "proposed")}
	after := map[string]string{
		changePath(42, "alpha"): minimalChange(42, "alpha", "proposed"),
		changePath(42, "beta"):  minimalChange(42, "beta", "proposed"),
	}
	got := ValidateEvolution(evolutionInput(t, before, after))
	reuse := findingsWithCode(got, CodeIdentityReused)
	if len(reuse) != 1 {
		t.Fatalf("want one %s, got %v", CodeIdentityReused, codesOf(got))
	}
	if reuse[0].Entity.Path != changePath(42, "beta") {
		t.Fatalf("entity path = %q, want the new record's path", reuse[0].Entity.Path)
	}
	if reuse[0].Severity != domain.SeverityError {
		t.Fatalf("severity = %q, want error", reuse[0].Severity)
	}
}

func TestValidateEvolutionADRIDReuseAtANewPath(t *testing.T) {
	before := map[string]string{adrPath(7, "a-decision"): acceptedADR()}
	after := map[string]string{
		adrPath(7, "a-decision"): acceptedADR(),
		adrPath(7, "a-retread"):  minimalADR(7, "a-retread", "Accepted"),
	}
	got := ValidateEvolution(evolutionInput(t, before, after))
	if len(findingsWithCode(got, CodeIdentityReused)) != 1 {
		t.Fatalf("want one %s, got %v", CodeIdentityReused, codesOf(got))
	}
}

func TestValidateEvolutionIgnoresAnUnchangedRepository(t *testing.T) {
	same := map[string]string{
		adrPath(7, "a-decision"): acceptedADR(),
		changePath(42, "alpha"):  minimalChange(42, "alpha", "proposed"),
	}
	if got := ValidateEvolution(evolutionInput(t, same, same)); len(got) != 0 {
		t.Fatalf("identical repositories should be clean, got %v", codesOf(got))
	}
}

func TestValidateEvolutionIsDeterministic(t *testing.T) {
	before := map[string]string{
		adrPath(7, "a-decision"): acceptedADR(),
		adrPath(8, "b-decision"): minimalADR(8, "b-decision", "Accepted"),
		adrPath(9, "c-decision"): minimalADR(9, "c-decision", "Accepted"),
		changePath(42, "alpha"):  minimalChange(42, "alpha", "proposed"),
	}
	after := maps.Clone(before)
	after[adrPath(7, "a-decision")] = strings.Replace(acceptedADR(), "Body.", "Edited.", 1)
	after[adrPath(9, "c-decision")] = strings.Replace(minimalADR(9, "c-decision", "Accepted"), "Body.", "Edited.", 1)
	after[changePath(42, "alpha")] = minimalChange(42, "renamed", "proposed")

	in := evolutionInput(t, before, after)
	first := ValidateEvolution(in)
	for i := 0; i < 5; i++ {
		if got := ValidateEvolution(in); !slices.Equal(codesOf(got), codesOf(first)) ||
			!slices.Equal(entityPaths(got), entityPaths(first)) {
			t.Fatalf("run %d differs: %v vs %v", i, entityPaths(got), entityPaths(first))
		}
	}
}

// findingsWithCode filters by code.
func findingsWithCode(findings []domain.Finding, code string) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

// entityPaths lists the subject paths in the order the pass returned them.
func entityPaths(findings []domain.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Entity.Path)
	}
	return out
}

// --- mutation probes: the two predicates the byte rules rest on ------------
//
// Each probe exercises the predicate directly with tampered inputs, so the
// guard's own failure mode is observable without editing the implementation:
// a predicate weakened to a raw byte comparison, or to no comparison at all,
// flips exactly these assertions.

func TestStatusMaskedEqualIgnoresOnlyTheStatusValueSpan(t *testing.T) {
	before := []byte(acceptedADR())
	after := []byte(minimalADR(7, "a-decision", "Superseded by ADR-0099"))
	beforeSpan := mustStatusSpan(t, before)
	afterSpan := mustStatusSpan(t, after)

	if !statusMaskedEqual(before, after, beforeSpan, afterSpan) {
		t.Fatal("a status-only flip must compare equal under the mask")
	}
	// The probe: without the mask this is a plain byte comparison, and the
	// clean row above would redden.
	if bytes.Equal(before, after) {
		t.Fatal("fixture is not a real flip: the bytes are identical")
	}

	// The mask covers the status VALUE and nothing else: a byte changed
	// outside the span is still caught.
	edited := []byte(strings.Replace(minimalADR(7, "a-decision", "Superseded by ADR-0099"), "Body.", "Edited.", 1))
	if statusMaskedEqual(before, edited, beforeSpan, mustStatusSpan(t, edited)) {
		t.Fatal("a body edit alongside the flip must not compare equal")
	}
	// A byte changed in the frontmatter BEFORE the status line is caught too.
	retitled := []byte(strings.Replace(minimalADR(7, "a-decision", "Superseded by ADR-0099"), "'A decision'", "'Another'", 1))
	if statusMaskedEqual(before, retitled, beforeSpan, mustStatusSpan(t, retitled)) {
		t.Fatal("a title edit alongside the flip must not compare equal")
	}
}

func TestFrozenPrefixIntactRejectsAnyTamperedPreexistingByte(t *testing.T) {
	before := []byte(acceptedADR())
	appended := append(append([]byte(nil), before...), []byte("\n## Update — 2026-08-14\n\nLate news.\n")...)
	if !frozenPrefixIntact(before, appended) {
		t.Fatal("a pure append must leave the frozen prefix intact")
	}

	// The probe: tamper with one pre-existing byte while still appending a
	// legal-looking update section. Without the prefix check this reads as a
	// clean append.
	tampered := []byte(strings.Replace(string(appended), "Body.", "Bodyx", 1))
	if !bytes.Contains(tampered, []byte("## Update")) {
		t.Fatal("fixture lost its appended update section")
	}
	if frozenPrefixIntact(before, tampered) {
		t.Fatal("a tampered pre-existing byte must break the frozen prefix")
	}
	// A shorter after can never contain the whole before.
	if frozenPrefixIntact(before, before[:len(before)-1]) {
		t.Fatal("a truncated after must break the frozen prefix")
	}
}

// mustStatusSpan locates the status value span, failing when the document
// layer cannot supply one.
func mustStatusSpan(t *testing.T, source []byte) document.Span {
	t.Helper()
	span, ok := statusValueSpan(source)
	if !ok {
		t.Fatalf("no status value span in %q", source)
	}
	return span
}
