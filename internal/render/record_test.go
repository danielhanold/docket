package render_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
)

// goldenDate is the fixed clock the record goldens are frozen against.
var goldenDate = time.Date(2026, 8, 16, 13, 30, 0, 0, time.UTC)

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "records", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return b
}

func changeIDs(ids ...int) []domain.ChangeID {
	out := make([]domain.ChangeID, len(ids))
	for i, id := range ids {
		out[i] = domain.ChangeID(id)
	}
	return out
}

func adrIDs(ids ...int) []domain.ADRID {
	out := make([]domain.ADRID, len(ids))
	for i, id := range ids {
		out[i] = domain.ADRID(id)
	}
	return out
}

// TestChangeRecordMatchesGolden pins the canonical proposed-change shape.
func TestChangeRecordMatchesGolden(t *testing.T) {
	got, err := render.ChangeRecord(render.NewChangeRecord{
		ID:          312,
		Slug:        "add-planning-mutations",
		Title:       "Add planning mutations: board and ADRs",
		Type:        "feat",
		Priority:    "medium",
		Created:     goldenDate,
		Why:         "Docket needs typed metadata write operations.",
		WhatChanges: "Add ten planning operations across the render and app layers.",
		OutOfScope:  "GitHub board mirroring stays out of this slice.",
	})
	if err != nil {
		t.Fatalf("ChangeRecord: %v", err)
	}
	want := readGolden(t, "change.golden.md")
	if !bytes.Equal(got, want) {
		t.Fatalf("ChangeRecord mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestLearningRecordMatchesGolden pins the canonical learning shape.
func TestLearningRecordMatchesGolden(t *testing.T) {
	got, err := render.LearningRecord(render.NewLearningRecord{
		Slug:     "cached-runner-serves-a-mutated-tree",
		Hook:     "A runner cache can serve a stale PASS against a mutated tree; defeat it with -count=1.",
		Topics:   []string{"testing", "mutation"},
		Changes:  changeIDs(312),
		Created:  goldenDate,
		Apply:    "Any run whose purpose is to observe a change in outcome must defeat the cache.",
		WarStory: "2026-08-16 — a mutation probe reported a fabricated green until -count=1 was added.",
	})
	if err != nil {
		t.Fatalf("LearningRecord: %v", err)
	}
	want := readGolden(t, "learning.golden.md")
	if !bytes.Equal(got, want) {
		t.Fatalf("LearningRecord mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestADRRecordMatchesGolden pins the canonical Accepted-ADR shape.
func TestADRRecordMatchesGolden(t *testing.T) {
	change := domain.ChangeID(312)
	got, err := render.ADRRecord(render.NewADRRecord{
		ID:           74,
		Slug:         "canonical-record-serializers",
		Title:        "Canonical record serializers emit through the shared builder",
		Date:         goldenDate,
		Change:       &change,
		RelatesTo:    adrIDs(62, 71),
		Context:      "New records must be well-formed by construction.",
		Decision:     "All frontmatter is emitted through document.New.",
		Consequences: "Quoting becomes a construction property, not a runtime check.",
		Alternatives: "A conditional-quoting writer was rejected for needing an enumeration oracle.",
	})
	if err != nil {
		t.Fatalf("ADRRecord: %v", err)
	}
	want := readGolden(t, "adr.golden.md")
	if !bytes.Equal(got, want) {
		t.Fatalf("ADRRecord mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestRecordsReparseCleanly asserts every serializer's output round-trips back
// through document.Parse with no error (validator-must-match-the-reader-it-feeds).
func TestRecordsReparseCleanly(t *testing.T) {
	change := domain.ChangeID(3)
	cases := map[string][]byte{}

	ch, err := render.ChangeRecord(render.NewChangeRecord{
		ID: 5, Slug: "s", Title: "t", Type: "feat", Priority: "low", Created: goldenDate,
		DependsOn: changeIDs(3), Related: changeIDs(4), DiscoveredFrom: changeIDs(2),
		ADRs: adrIDs(7), StackedOn: &change,
		Why: "why", WhatChanges: "what", OutOfScope: "scope",
	})
	if err != nil {
		t.Fatalf("ChangeRecord: %v", err)
	}
	cases["change"] = ch

	ln, err := render.LearningRecord(render.NewLearningRecord{
		Slug: "s", Hook: "h", Topics: []string{"a", "b"}, Changes: changeIDs(1),
		Created: goldenDate, Apply: "apply", WarStory: "war",
	})
	if err != nil {
		t.Fatalf("LearningRecord: %v", err)
	}
	cases["learning"] = ln

	ad, err := render.ADRRecord(render.NewADRRecord{
		ID: 9, Slug: "s", Title: "t", Date: goldenDate, Change: &change,
		Supersedes: adrIDs(1), Reverses: adrIDs(2), RelatesTo: adrIDs(3),
		Context: "c", Decision: "d", Consequences: "q", Alternatives: "alt",
	})
	if err != nil {
		t.Fatalf("ADRRecord: %v", err)
	}
	cases["adr"] = ad

	for name, out := range cases {
		if _, err := document.Parse(out); err != nil {
			t.Errorf("%s output failed reparse: %v\n%s", name, err, out)
		}
		if n := bytes.Count(out[len(out)-1:], []byte("\n")); n != 1 {
			t.Errorf("%s does not end in exactly one newline", name)
		}
		if bytes.HasSuffix(out, []byte("\n\n")) {
			t.Errorf("%s ends in a blank line", name)
		}
	}
}

// TestChangeRecordQuotesColonTitle proves quoting-by-construction: a title with
// an embedded ": " and a trailing colon round-trips inside single quotes.
func TestChangeRecordQuotesColonTitle(t *testing.T) {
	out, err := render.ChangeRecord(render.NewChangeRecord{
		ID: 1, Slug: "s", Title: "Weird: title with a trailing colon:", Type: "feat",
		Priority: "high", Created: goldenDate, Why: "w", WhatChanges: "wc", OutOfScope: "oos",
	})
	if err != nil {
		t.Fatalf("ChangeRecord: %v", err)
	}
	if !bytes.Contains(out, []byte("title: 'Weird: title with a trailing colon:'\n")) {
		t.Fatalf("title not quoted by construction:\n%s", out)
	}
	if _, err := document.Parse(out); err != nil {
		t.Fatalf("colon title broke reparse: %v", err)
	}
}

// TestRelationshipCollectionsRenderFlowSeqs: empty collections render "[]",
// populated ones render "[a, b]" — never null, never quoted integers.
func TestRelationshipCollectionsRenderFlowSeqs(t *testing.T) {
	empty, err := render.ChangeRecord(render.NewChangeRecord{
		ID: 1, Slug: "s", Title: "t", Type: "feat", Priority: "low", Created: goldenDate,
		Why: "w", WhatChanges: "wc", OutOfScope: "oos",
	})
	if err != nil {
		t.Fatalf("ChangeRecord: %v", err)
	}
	if !bytes.Contains(empty, []byte("depends_on: []\n")) {
		t.Fatalf("empty depends_on not rendered as []:\n%s", empty)
	}

	full, err := render.ChangeRecord(render.NewChangeRecord{
		ID: 1, Slug: "s", Title: "t", Type: "feat", Priority: "low", Created: goldenDate,
		DependsOn: changeIDs(3, 5), Why: "w", WhatChanges: "wc", OutOfScope: "oos",
	})
	if err != nil {
		t.Fatalf("ChangeRecord: %v", err)
	}
	if !bytes.Contains(full, []byte("depends_on: [3, 5]\n")) {
		t.Fatalf("populated depends_on not rendered as [3, 5]:\n%s", full)
	}
}
