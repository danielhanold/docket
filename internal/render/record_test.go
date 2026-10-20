package render_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

// TestChangeRecordHasNoTrivialKey proves a new change record writes no
// trivial: frontmatter line.
func TestChangeRecordHasNoTrivialKey(t *testing.T) {
	got, err := render.ChangeRecord(render.NewChangeRecord{
		ID: 312, Slug: "add-planning-mutations", Title: "Add planning mutations: board and ADRs",
		Type: "feat", Priority: "medium", Created: goldenDate,
		Why: "w", WhatChanges: "wc", OutOfScope: "oos",
	})
	if err != nil {
		t.Fatalf("ChangeRecord: %v", err)
	}
	lines := strings.Split(string(got), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		t.Fatalf("record does not open with frontmatter:\n%s", got)
	}
	for _, line := range lines[1:] {
		if line == "---" {
			return
		}
		if strings.HasPrefix(line, "trivial:") {
			t.Fatalf("new change record carries %q", line)
		}
	}
	t.Fatalf("frontmatter never closed:\n%s", got)
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

// TestChangeRecordDraftScalars pins the two create-time scalars: unset renders
// the canonical null (the golden's shape), a set value renders a boolean or a
// single-quoted string by construction.
func TestChangeRecordDraftScalars(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		name       string
		auto       *bool
		prefix     string
		wantAuto   string
		wantPrefix string
	}{
		{"unset", nil, "", "\nauto_groomable:\n", "\nbranch_prefix:\n"},
		{"true with prefix", &yes, "hotfix", "\nauto_groomable: true\n", "\nbranch_prefix: 'hotfix'\n"},
		{"explicit false", &no, "", "\nauto_groomable: false\n", "\nbranch_prefix:\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := render.ChangeRecord(render.NewChangeRecord{
				ID: 1, Slug: "s", Title: "T", Type: "feat", Priority: "medium", Created: goldenDate,
				AutoGroomable: c.auto, BranchPrefix: c.prefix,
				Why: "w", WhatChanges: "c", OutOfScope: "o",
			})
			if err != nil {
				t.Fatalf("ChangeRecord: %v", err)
			}
			s := string(got)
			if !strings.Contains(s, c.wantAuto) || !strings.Contains(s, c.wantPrefix) {
				t.Errorf("record missing %q / %q:\n%s", c.wantAuto, c.wantPrefix, s)
			}
			if _, err := document.Parse(got); err != nil {
				t.Errorf("record does not reparse: %v", err)
			}
		})
	}
}

// TestChangeRecordWritesDeliverableAfterType pins the deliverable line: an
// empty value renders as pr, and the line sits directly after type:.
func TestChangeRecordWritesDeliverableAfterType(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "deliverable: pr"},
		{"results-artifact", "deliverable: results-artifact"},
	} {
		got, err := render.ChangeRecord(render.NewChangeRecord{
			ID: 1, Slug: "s", Title: "t", Type: "feat", Priority: "low", Created: goldenDate,
			Deliverable: tc.in,
		})
		if err != nil {
			t.Fatalf("ChangeRecord: %v", err)
		}
		lines := strings.Split(string(got), "\n")
		typeAt, delAt := -1, -1
		for i, l := range lines {
			switch {
			case strings.HasPrefix(l, "type:"):
				typeAt = i
			case strings.HasPrefix(l, "deliverable:"):
				delAt = i
				if strings.ReplaceAll(l, "'", "") != tc.want {
					t.Errorf("deliverable %q: line = %q, want %q", tc.in, l, tc.want)
				}
			}
		}
		if typeAt < 0 || delAt != typeAt+1 {
			t.Errorf("deliverable %q: type at %d, deliverable at %d; want directly after", tc.in, typeAt, delAt)
		}
	}
}

// TestChangeRecordHold pins the hold field and its optional reason section: a
// held record writes hold: true directly after auto_groomable, an unheld one a
// valueless hold:, and a reason lands as the "## Why on hold" section after
// "## Out of scope".
func TestChangeRecordHold(t *testing.T) {
	base := render.NewChangeRecord{
		ID: 1, Slug: "s", Title: "T", Type: "feat", Priority: "medium", Created: goldenDate,
		Why: "w", WhatChanges: "c", OutOfScope: "oos",
	}

	held := base
	held.Hold = true
	got, err := render.ChangeRecord(held)
	if err != nil {
		t.Fatalf("ChangeRecord: %v", err)
	}
	if s := string(got); !strings.Contains(s, "\nauto_groomable:\nhold: true\nbranch_prefix:\n") {
		t.Errorf("held record missing hold: true after auto_groomable:\n%s", s)
	}
	if strings.Contains(string(got), "## Why on hold") {
		t.Errorf("held record without a reason wrote the reason section:\n%s", got)
	}

	unheld, err := render.ChangeRecord(base)
	if err != nil {
		t.Fatalf("ChangeRecord: %v", err)
	}
	if s := string(unheld); !strings.Contains(s, "\nauto_groomable:\nhold:\nbranch_prefix:\n") {
		t.Errorf("unheld record missing valueless hold:\n%s", s)
	}

	reasoned := held
	reasoned.WhyOnHold = "Needs a sign-in."
	got, err = render.ChangeRecord(reasoned)
	if err != nil {
		t.Fatalf("ChangeRecord: %v", err)
	}
	if s := string(got); !strings.HasSuffix(s, "## Out of scope\n\noos\n\n## Why on hold\n\nNeeds a sign-in.\n") {
		t.Errorf("reason section not appended after Out of scope:\n%s", s)
	}
	for _, b := range [][]byte{got, unheld} {
		if _, err := document.Parse(b); err != nil {
			t.Errorf("record does not reparse: %v", err)
		}
	}
}
