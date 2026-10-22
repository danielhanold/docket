package reposetup

import (
	"strings"
	"testing"
)

// repairTextBannedWords is the internal vocabulary a user-facing finding part
// must never carry (change 0553's plain-language rule).
var repairTextBannedWords = []string{
	"canonical render", "roster", "scalar", "corpus", "lease", "snapshot", "postcondition", "final",
}

// TestRepairFindingsCarryThreeParts pins that every frontmatter-repair shape
// says what is wrong, whether it matters, and what to do — with the remedy
// naming the record's real location and the repair command.
func TestRepairFindingsCarryThreeParts(t *testing.T) {
	rel := "docs/changes/archive/2026-05-28-0001-wisdom-ai-bastion.md"
	loc := RecordLocator{Checkout: ".docket"}
	fm := func(extra string) string {
		return "---\nid: 1\nslug: wisdom-ai-bastion\nstatus: done\ntitle: T\ntype: feature\n" + extra + "---\n\nBody.\n"
	}
	cases := []struct {
		name       string
		src        string
		field      string // "" selects the whole-record finding
		repairable bool
		msg        string
	}{
		{"quote-unsafe-scalar", strings.Replace(fm(""), "title: T", "title: yes", 1), "title", true, "`title: yes`"},
		{"scalar-to-list", fm("depends_on: 3, 4\n"), "depends_on", true, "`depends_on: 3, 4`"},
		{"ambiguous value", strings.Replace(fm(""), "title: T", "title: true", 1), "title", false, "`title: true`"},
		{"non-id list", fm("related: foo\n"), "related", false, "`related: foo`"},
		{"undecodable", "---\nid: 1\nid: 2\n---\nbody\n", "", false, "frontmatter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs, err := PlanRepairs(rel, []byte(tc.src), loc)
			if err != nil {
				t.Fatalf("PlanRepairs: %v", err)
			}
			if len(fs) != 1 {
				t.Fatalf("want exactly one finding, got %d: %+v", len(fs), fs)
			}
			f := fs[0]
			if f.Field != tc.field {
				t.Fatalf("field = %q, want %q", f.Field, tc.field)
			}
			if f.Repairable != tc.repairable {
				t.Fatalf("repairable = %v, want %v", f.Repairable, tc.repairable)
			}
			if f.Message == "" || f.Impact == "" || f.Remedy == "" {
				t.Fatalf("every part must be non-empty: %+v", f)
			}
			if !strings.Contains(f.Message, tc.msg) {
				t.Errorf("message %q does not contain %q", f.Message, tc.msg)
			}
			if want := "`.docket/" + rel + "`"; !strings.Contains(f.Remedy, want) {
				t.Errorf("remedy %q does not name %s", f.Remedy, want)
			}
			if !strings.Contains(f.Remedy, "docket repository repair") {
				t.Errorf("remedy %q does not name docket repository repair", f.Remedy)
			}
			for _, part := range []string{f.Message, f.Impact, f.Remedy} {
				low := strings.ToLower(part)
				for _, w := range repairTextBannedWords {
					if strings.Contains(low, w) {
						t.Errorf("part %q carries banned word %q", part, w)
					}
				}
			}
		})
	}
}
