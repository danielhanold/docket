package export

import (
	"reflect"
	"strings"
	"testing"
)

func assertMarkdown(t *testing.T, got Output, want string) {
	t.Helper()
	if string(got.Markdown) != want {
		t.Fatalf("markdown mismatch\n--- got ---\n%s\n--- want ---\n%s", got.Markdown, want)
	}
}

func TestTransformSpec(t *testing.T) {
	src := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **[Change 0561 — Export](../../changes/active/0561-export.md)**\n" +
		"<!-- docket:backlink:end -->\n" +
		"\n" +
		"# Export design\n" +
		"\n" +
		"Change #561, groomed 2026-10-20.\n" +
		"\n" +
		"## Why\n" +
		"\n" +
		"See [the docs](https://example.com/d) and change 0559.\n"
	got := Transform(Input{
		Kind:   KindSpec,
		Source: []byte(src),
		Titles: Titles{559: "Close a change to done without a PR", 561: "Export"},
	})
	assertMarkdown(t, got, "# Export design\n"+
		"\n"+
		"## Why\n"+
		"\n"+
		"See the docs and \"Close a change to done without a PR\".\n")
	if len(got.Stray) != 0 {
		t.Fatalf("Stray = %v, want none", got.Stray)
	}
}

func TestTransformSpecStampKeepsRest(t *testing.T) {
	src := "# T\n" +
		"\n" +
		"Change #12, groomed 2026-01-02. Found while building change #13 (PR #14).\n" +
		"\n" +
		"Change 0373. Groomed 2026-09-02 (interactive). Priority high: x\n" +
		"\n" +
		"**Change:** 0003\n" +
		"\n" +
		"**Change:** 5 · **Type:** feat · **Priority:** high\n" +
		"\n" +
		"Change 12.\n" +
		"\n" +
		"## Body\n" +
		"\n" +
		"Change 12.\n"
	got := Transform(Input{
		Kind:   KindSpec,
		Source: []byte(src),
		Titles: Titles{12: "Twelve", 13: "Thirteen", 373: "Three seven three", 3: "Three", 5: "Five"},
	})
	assertMarkdown(t, got, "# T\n"+
		"\n"+
		"Found while building \"Thirteen\" (PR #14).\n"+
		"\n"+
		"Priority high: x\n"+
		"\n"+
		"## Body\n"+
		"\n"+
		"\"Twelve\".\n")
}

// TestTransformSpecStampShapes covers the stamp shapes older specs carry. A stamp
// is matched by its shape (a line opening "Change", "Change:", or "**Change:**"
// and the change id), never by a list of spellings, and on link-stripped text.
func TestTransformSpecStampShapes(t *testing.T) {
	titles := Titles{36: "Thirty-six", 31: "Thirty-one", 51: "Fifty-one", 205: "Two oh five",
		462: "Four six two", 398: "Three nine eight", 257: "Two five seven", 282: "Two eight two",
		104: "One oh four", 234: "Two three four", 474: "Four seven four", 237: "Two three seven",
		236: "Two three six", 401: "Four oh one", 74: "Seventy-four", 72: "Seventy-two", 554: "Five five four"}
	cases := []struct{ name, line, want string }{
		{"colon dot-run", "Change: #0036 · slug `s` · spec drafted 2026-06-21 (autonomous groom)", ""},
		{"bold dash title", "**Change:** 0031 — match the supersede verb", ""},
		{"bold paren slug", "**Change:** 0051 (`global-agents-middle-layer`)", ""},
		{"spec date", "Change 0205. Spec date 2026-08-04.", ""},
		{"paren then groomed", "Change 0462 (discovered from 0398). Groomed 2026-09-27.", ""},
		{"plain dot-run", "Change 0257 · groomed 2026-08-07 (auto-groom) · consolidates the surviving items", ""},
		{"colon auto-groomed", "Change: 0282. Auto-groomed 2026-08-09 (critic-gated).", ""},
		{"bold linked", "**Change:** [#0104](../../changes/active/0104-x.md)", ""},
		{"linked sentence", "Change [#0234](../../changes/active/0234-x.md).", ""},
		{"colon slug", "Change: 0074-bootstrap-facade-verb", ""},
		{"colon record path", "Change: docs/changes/active/0258-guard-x.md", ""},
		{"bold code slug", "**Change:** #0072 `facade-skill-rewiring`", ""},
		{"bold dot-separated fields", "**Change:** 0401. **Type:** docs. **Path:** bounded (files added).", ""},
		{"later change field", "**Date:** 2026-07-10 · **Change:** 0053 · **Depends on:** 0051", ""},
		{"author sentence stays", "Change 0237. Supersedes and absorbs change 0236 (killed into this one).",
			"Supersedes and absorbs \"Two three six\" (killed into this one)."},
		{"groomed then author sentence", "Change #554, groomed 2026-10-16. Revised the same day.", "Revised the same day."},
		{"unfinished sentence stays", "Change: 0261. Discovered in review (merged 2026-08-08), which introduced the",
			"Discovered in review (merged 2026-08-08), which introduced the"},
		{"unclosed parenthesis stays", "Change: 0247 (consolidates two killed stubs and", "(consolidates two killed stubs and"},
		{"prose opening with a reference", "Change 0474 is the last family change.", "\"Four seven four\" is the last family change."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Transform(Input{Kind: KindSpec, Source: []byte("# T\n\n" + tc.line + "\n\n## Body\n\nx\n"), Titles: titles})
			want := "# T\n\n## Body\n\nx\n"
			if tc.want != "" {
				want = "# T\n\n" + tc.want + "\n\n## Body\n\nx\n"
			}
			assertMarkdown(t, got, want)
		})
	}
}

func TestTransformSpecStampOnlyBeforeFirstH2(t *testing.T) {
	// A non-spec artifact never loses a stamp-shaped line; it converts instead.
	src := "# T\n\nChange 12.\n"
	got := Transform(Input{Kind: KindPlan, Source: []byte(src), Titles: Titles{12: "Twelve"}})
	assertMarkdown(t, got, "# T\n\n\"Twelve\".\n")
}

const issueFixture = "---\n" +
	"id: 7\n" +
	"title: Seven\n" +
	"---\n" +
	"\n" +
	"# Preamble heading\n" +
	"\n" +
	"## Artifacts\n" +
	"\n" +
	"<!-- docket:artifacts:start -->\n" +
	"| a | [b](x.md) |\n" +
	"<!-- docket:artifacts:end -->\n" +
	"\n" +
	"## Why\n" +
	"\n" +
	"Because of change 0008.\n" +
	"\n" +
	"## What changes\n" +
	"\n" +
	"Things.\n" +
	"\n" +
	"## Reconcile log\n" +
	"\n" +
	"- log\n" +
	"\n" +
	"## Why deferred\n" +
	"\n" +
	"Later.\n" +
	"\n" +
	"## Out of scope\n" +
	"\n" +
	"None.\n"

const issueWant = "# Seven\n" +
	"\n" +
	"## Why\n" +
	"\n" +
	"Because of \"Eight\".\n" +
	"\n" +
	"## What changes\n" +
	"\n" +
	"Things.\n" +
	"\n" +
	"## Out of scope\n" +
	"\n" +
	"None.\n"

func TestTransformIssue(t *testing.T) {
	got := Transform(Input{
		Kind:   KindIssue,
		Source: []byte(issueFixture),
		Title:  "Seven (0008)",
		Titles: Titles{8: "Eight"},
	})
	assertMarkdown(t, got, issueWant)
}

func TestTransformNormalizesCRLF(t *testing.T) {
	crlf := strings.ReplaceAll(issueFixture, "\n", "\r\n")
	got := Transform(Input{
		Kind:   KindIssue,
		Source: []byte(crlf),
		Title:  "Seven (0008)",
		Titles: Titles{8: "Eight"},
	})
	assertMarkdown(t, got, issueWant)
	if strings.Contains(string(got.Markdown), "\r") {
		t.Fatalf("output still holds a carriage return: %q", got.Markdown)
	}
}

func TestTransformDamagedSourceKeepsContent(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		want  string
		stray []Stray
	}{
		{
			name:  "start with no end",
			src:   "# T\n\n<!-- docket:build-evidence:start -->\n\nLater prose.\n\n## More\n\nStill here.\n",
			want:  "# T\n\nLater prose.\n\n## More\n\nStill here.\n",
			stray: []Stray{{SourceLine: 3, Text: "<!-- docket:build-evidence:start -->"}},
		},
		{
			name:  "end with no start",
			src:   "# T\n\nBefore.\n<!-- docket:build-evidence:end -->\nAfter.\n",
			want:  "# T\n\nBefore.\nAfter.\n",
			stray: []Stray{{SourceLine: 4, Text: "<!-- docket:build-evidence:end -->"}},
		},
		{
			name: "start closed only after another start",
			src: "# T\n<!-- docket:a:start -->\nkept\n<!-- docket:b:start -->\ngone\n<!-- docket:b:end -->\n" +
				"<!-- docket:Bad -->\ntail\n",
			want: "# T\nkept\ntail\n",
			stray: []Stray{
				{SourceLine: 2, Text: "<!-- docket:a:start -->"},
				{SourceLine: 7, Text: "<!-- docket:Bad -->"},
			},
		},
		{
			name: "unclosed frontmatter opener",
			src:  "---\ntitle: x\n\nBody text.\n",
			want: "---\ntitle: x\n\nBody text.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Transform(Input{Kind: KindPlan, Source: []byte(tc.src)})
			assertMarkdown(t, got, tc.want)
			if !reflect.DeepEqual(got.Stray, tc.stray) {
				t.Fatalf("Stray = %#v, want %#v", got.Stray, tc.stray)
			}
		})
	}
}

func TestTransformFenceHeadingsAreContent(t *testing.T) {
	fence := "```text\n" +
		"## Reconcile log\n" +
		"`change 0559`\n" +
		"[x](y)\n" +
		"\n" +
		"\n" +
		"<!-- docket:x:start -->\n" +
		"```\n"
	src := "---\ntitle: T\n---\n\n## Why\n\n" + fence + "\nMore.\n\n## Reconcile log\n\n- gone\n"
	got := Transform(Input{Kind: KindIssue, Source: []byte(src), Title: "T", Titles: Titles{559: "Z"}})
	assertMarkdown(t, got, "# T\n\n## Why\n\n"+fence+"\nMore.\n")
}

func TestTransformCleanup(t *testing.T) {
	src := "\n\n# T\n\n## Artifacts\n\n\n\n## Body\n\nText\n\n\n\nMore\n\n" +
		"## A\n\n### B\n\n## C\n\nx\n\n\n"
	got := Transform(Input{Kind: KindPlan, Source: []byte(src)})
	assertMarkdown(t, got, "# T\n\n## Body\n\nText\n\nMore\n\n## C\n\nx\n")
}

func TestTransformSkipStepKeepsItsMachinery(t *testing.T) {
	// The unexported skip set lets the corpus mutation guard disable one step.
	src := "<!-- docket:a:start -->\nx\n<!-- docket:a:end -->\n# T\n"
	got := transform(Input{Kind: KindPlan, Source: []byte(src)}, map[step]bool{stepBlocks: true})
	if !strings.Contains(string(got.Markdown), "docket:a:start") {
		t.Fatalf("skipping stepBlocks still removed the block: %q", got.Markdown)
	}
	if len(allSteps) != 7 {
		t.Fatalf("allSteps has %d steps, want 7", len(allSteps))
	}
}
