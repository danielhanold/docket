package render

import "testing"

func TestCloseoutNotesBody(t *testing.T) {
	cases := []struct {
		name         string
		verification []string
		late         []string
		want         string
	}{
		{
			name:         "both categories",
			verification: []string{"Production health check passed after deployment"},
			late:         []string{"The upgrade guide should mention the legacy config cleanup"},
			want: "### Verification\n\n" +
				"- Production health check passed after deployment\n\n" +
				"### Late findings\n\n" +
				"- The upgrade guide should mention the legacy config cleanup",
		},
		{
			name:         "verification only omits the late subsection",
			verification: []string{"Smoke test green", "Rollback drill passed"},
			want: "### Verification\n\n" +
				"- Smoke test green\n" +
				"- Rollback drill passed",
		},
		{
			name: "late only omits the verification subsection",
			late: []string{"Docs gap"},
			want: "### Late findings\n\n- Docs gap",
		},
		{
			name: "both empty renders nothing",
			want: "",
		},
		{
			name:         "multiline continuation is indented so it cannot escape the bullet",
			verification: []string{"line one\n## not a heading\nline three"},
			want: "### Verification\n\n" +
				"- line one\n" +
				"  ## not a heading\n" +
				"  line three",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CloseoutNotesBody(tc.verification, tc.late)
			if got != tc.want {
				t.Fatalf("CloseoutNotesBody = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCloseoutNotesBodySpliceRoundTrip proves the rendered section survives
// ApplySectionEdits and lands as the FINAL section: the writer's own reader
// accepts what it emits (learnings: validator-must-match-the-reader-it-feeds).
func TestCloseoutNotesBodySpliceRoundTrip(t *testing.T) {
	src := []byte("---\nid: 5\n---\n\n## Why\n\nBecause.\n")
	body := CloseoutNotesBody([]string{"ok\n## fake"}, nil)
	out, err := ApplySectionEdits(src, []string{CloseoutNotesHeading},
		[]SectionEdit{{Heading: CloseoutNotesHeading, Intent: SectionReplace, Markdown: body}})
	if err != nil {
		t.Fatalf("splice: %v", err)
	}
	heads := scanH2Headings(out)
	if len(heads) != 2 || heads[1].heading != CloseoutNotesHeading {
		t.Fatalf("headings = %+v, want [## Why, %s] with notes last", heads, CloseoutNotesHeading)
	}
}
