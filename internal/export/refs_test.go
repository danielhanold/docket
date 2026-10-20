package export

import "testing"

func refsTestTitles() Titles {
	return Titles{
		547: "Measure whether the 1h cache TTL cut implement-next's cost",
		363: "Remove main-mode compatibility from Go v1",
		461: "A",
		550: "B",
		113: "C",
		114: "D",
		120: "E",
		466: "Bring test_go_race back under its 60s row",
		111: "Guard",
		559: "Close a change to done without a PR",
		282: "X",
		88:  "Y",
	}
}

func TestConvertRefs(t *testing.T) {
	titles := refsTestTitles()
	cases := []struct{ in, want string }{
		// Word form, singular, with and without '#'.
		{"Change #547 holds the full measurement", `"Measure whether the 1h cache TTL cut implement-next's cost" holds the full measurement`},
		{"from change 0363 forbids it", `from "Remove main-mode compatibility from Go v1" forbids it`},
		{"see change 559 here", `see "Close a change to done without a PR" here`},
		// Plural lists.
		{"changes #461 and #550", `"A" and "B"`},
		{"changes 0113, 0114 and 0120", `"C", "D" and "E"`},
		{"changes 0113, 0114, and 0120", `"C", "D" and "E"`},
		{"changes 0113, 9999 and 0120", `changes "C", 9999 and "E"`},
		// References-only parenthesis.
		{"fields (0461, 0550)", "fields"},
		{"fields (0461, 0550) stay", "fields stay"},
		{"(0461, ADR-0012)", "(ADR-0012)"},
		{"(0461, #12 and ADR-0012)", "(#12 and ADR-0012)"},
		{"(#461, ADR-0012)", "(#461, ADR-0012)"},
		{"It landed (0559) today.", "It landed today."},
		{"(see change 0547)", `(see "Measure whether the 1h cache TTL cut implement-next's cost")`},
		// Own-title dash.
		{"# Change 0466 — Bring test_go_race back under its 60s row budget", "# Bring test_go_race back under its 60s row budget"},
		{"# Change 0466 - Bring test_go_race back under its 60s row", "# Bring test_go_race back under its 60s row"},
		{"# Change 0466 — Something else", `# "Bring test_go_race back under its 60s row" — Something else`},
		// Padded form standing as its own word.
		{"the 0111 guard", `the "Guard" guard`},
		{"0282's", `"X"'s`},
		{"mirroring 0088", `mirroring "Y"`},
		{"#0559", `"Close a change to done without a PR"`},
		{"done in 0559.", `done in "Close a change to done without a PR".`},
		// The author's own quotes around a reference: the whole quoted text is the
		// reference, so the title takes those quotes; inside a longer quoted phrase
		// the title takes single quotes; a quoted references-only parenthesis never
		// leaves an empty quote pair.
		{`the "change 0559" note`, `the "Close a change to done without a PR" note`},
		{`"#0559"`, `"Close a change to done without a PR"`},
		{`"changes 0113 and 0114"`, `"C" and "D"`},
		{`"changes 0113 and 9999"`, `"changes 'C' and 9999"`},
		{`as "(0559)" said`, `as "Close a change to done without a PR" said`},
		{`as "(0113, 0114)" said`, `as "C" and "D" said`},
		{`("groom 0559")`, `("groom 'Close a change to done without a PR'")`},
		{`said "see change 0559 now" and change 0113`, `said "see 'Close a change to done without a PR' now" and "C"`},
		// Left exactly as written.
		{"#559", "#559"},
		{"PR #559", "PR #559"},
		{"PR #0559", "PR #0559"},
		{"ADR-0147", "ADR-0147"},
		{"ADR 0147", "ADR 0147"},
		{"adr 0559", "adr 0559"},
		{"ADR-0065's", "ADR-0065's"},
		{"change 9999", "change 9999"},
		{"0561-export.md", "0561-export.md"},
		{"docs/0561", "docs/0561"},
		{"0559.md", "0559.md"},
		{"v0559", "v0559"},
		{"change 3 files", "change 3 files"},
		{"exchange 559", "exchange 559"},
		{"0999 unknown", "0999 unknown"},
		{"12345 and 559", "12345 and 559"},
	}
	for _, tc := range cases {
		if got := convertRefs(tc.in, titles); got != tc.want {
			t.Errorf("convertRefs(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

func TestConvertRefsLeavesMaskedCode(t *testing.T) {
	titles := refsTestTitles()
	in := "a `change 0559` and change 0559"
	masked, restore := maskCode(in)
	got := restore(convertRefs(masked, titles))
	want := "a `change 0559` and \"Close a change to done without a PR\""
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCleanTitle(t *testing.T) {
	titles := refsTestTitles()
	titles[700] = "Rework change 0700 itself"
	cases := []struct{ in, want string }{
		{"Clear the unfixed review findings from change 0113", "Clear the unfixed review findings from another issue"},
		{`Fix "quoted" bits (0461, 0550)`, "Fix 'quoted' bits"},
		{"Split changes 0113 and 0114", "Split other issues"},
		{"Follow #559 and ADR-0012", "Follow #559 and ADR-0012"},
		{"Rework change 0700 itself", "Rework another issue itself"},
		{"Mirror 0088 here", "Mirror another issue here"},
		{"Keep change 9999", "Keep change 9999"},
	}
	for _, tc := range cases {
		if got := cleanTitle(tc.in, titles); got != tc.want {
			t.Errorf("cleanTitle(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

// An inserted title is cleaned once: a reference inside it never pulls in a
// second title, even when the referenced change's own title holds references.
func TestConvertRefsCleansInsertedTitleOnce(t *testing.T) {
	titles := Titles{1: "Follow up on change 0002", 2: "Follow up on change 0001"}
	got := convertRefs("see change 0001", titles)
	want := `see "Follow up on another issue"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
