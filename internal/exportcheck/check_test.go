package exportcheck

import (
	"reflect"
	"strings"
	"testing"
)

// knownIDs reports the ids the crafted inputs treat as real changes.
func knownIDs(ids ...int) func(int) bool {
	set := map[int]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return func(id int) bool { return set[id] }
}

var known = knownIDs(559, 113, 120, 547, 88, 282)

type lineRule struct {
	Line int
	Rule string
}

func lineRules(r Report) []lineRule {
	var out []lineRule
	for _, l := range r.Leftovers {
		out = append(out, lineRule{l.Line, l.Rule})
	}
	return out
}

func TestCheckFindsEachLeftoverKind(t *testing.T) {
	cases := []struct {
		name string
		md   string
		want []lineRule
	}{
		{"marker docket", "# T\n\n<!-- docket:artifacts:start -->\n", []lineRule{{3, RuleMarker}}},
		{"marker dckt", "# T\n\ntext <!-- dckt:backlink:end --> more\n", []lineRule{{3, RuleMarker}}},
		{"frontmatter", "---\nid: 1\n---\n# T\n", []lineRule{{1, RuleFrontmatter}}},
		{"banner", "# T\n\n> ↩ **Back**\n", []lineRule{{3, RuleBanner}}},
		{"inline link", "# T\n\nsee [docs](https://x)\n", []lineRule{{3, RuleLink}}},
		{"image", "# T\n\n![alt](p.png)\n", []lineRule{{3, RuleLink}}},
		{"autolink", "# T\n\nsee <https://go.dev/x>\n", []lineRule{{3, RuleLink}}},
		{"banner indented", "# T\n\n  >↩ Back\n", []lineRule{{3, RuleBanner}}},
		{"reference link", "# T\n\nsee [text][Ref]\n\n[ref]: https://x\n", []lineRule{{3, RuleLink}, {5, RuleLink}}},
		{"collapsed reference link", "# T\n\nsee [Text][]\n\n[text]: https://x\n", []lineRule{{3, RuleLink}, {5, RuleLink}}},
		{"link definition", "# T\n\n[ref]: https://x\n", []lineRule{{3, RuleLink}}},
		{"html anchor", "# T\n\nsee <a href=\"x\">t</a>\n", []lineRule{{3, RuleLink}}},
		{"html anchor upper", "# T\n\nsee <A\tNAME='x'>t\n", []lineRule{{3, RuleLink}}},
		{"html anchor bare with close", "# T\n\nsee <a>t</A>\n", []lineRule{{3, RuleLink}}},
		{"word form", "# T\n\nfrom change 559 on\n", []lineRule{{3, RuleChangeRef}}},
		{"word form hash", "# T\n\nChange #547 holds\n", []lineRule{{3, RuleChangeRef}}},
		{"word form list", "# T\n\nchanges 9999, 4444 and 0120\n", []lineRule{{3, RuleChangeRef}}},
		{"padded hash", "# T\n\nsee #0559 now\n", []lineRule{{3, RuleChangeRef}}},
		{"padded paren", "# T\n\nfields (0113)\n", []lineRule{{3, RuleChangeRef}}},
		{"padded bare", "# T\n\nmirroring 0088\n", []lineRule{{3, RuleChangeRef}}},
		{"padded possessive", "# T\n\n0282's rule.\n", []lineRule{{3, RuleChangeRef}}},
		{"padded sentence end", "# T\n\nas in 0559.\n", []lineRule{{3, RuleChangeRef}}},
		{"stamp groomed", "# T\n\nChange #561, groomed 2026-10-20.\n", []lineRule{{3, RuleStamp}}},
		{"stamp groomed dot", "# T\n\nChange 9373. Groomed 2026-09-02 (interactive). Rest\n", []lineRule{{3, RuleStamp}}},
		{"stamp change only", "# T\n\nChange 9561.\n", []lineRule{{3, RuleStamp}}},
		{"stamp bold", "# T\n\n**Change:** 9003 · **Type:** feat\n", []lineRule{{3, RuleStamp}}},
		{"stamp bold converted title", "# T\n\n**Change:** \"Some title\" — x\n", []lineRule{{3, RuleStamp}}},
		{"stamp colon converted title", "# T\n\nChange: \"Some title\" · slug\n", []lineRule{{3, RuleStamp}}},
		{"stamp converted title", "# T\n\nChange \"Some title\".\n", []lineRule{{3, RuleStamp}}},
		{"stamp colon record path", "# T\n\nChange: docs/changes/active/0258-x.md\n", []lineRule{{3, RuleStamp}}},
		{"stamp dot-run", "# T\n\nChange #9036 · slug\n", []lineRule{{3, RuleStamp}}},
		{"stamp parenthesis", "# T\n\nChange 9462 (discovered). Groomed\n", []lineRule{{3, RuleStamp}}},
		{"stamp later field", "# T\n\n**Date:** 2026-07-10 · **Change:** \"Some title\" · **Type:** x\n", []lineRule{{3, RuleStamp}}},
		{"stamp dot then author sentence", "# T\n\nChange 9237. Supersedes it.\n", []lineRule{{3, RuleStamp}}},
		{"heading", "# T\n\n## A\n\n#### B\n", []lineRule{{5, RuleHeading}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lineRules(Check([]byte(tc.md), known))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Check(%q) leftovers = %v, want %v", tc.md, got, tc.want)
			}
		})
	}
}

func TestCheckLeftoverCarriesText(t *testing.T) {
	r := Check([]byte("# T\n\nsee [docs](https://x) here\n"), known)
	if len(r.Leftovers) != 1 || !strings.Contains(r.Leftovers[0].Text, "[docs](") {
		t.Fatalf("leftover text = %+v, want the matched link text", r.Leftovers)
	}
}

func TestCheckKeepsWhatTheExportKeeps(t *testing.T) {
	md := strings.Join([]string{
		"# Title",
		"",
		"Bare #559 and PR #12 and PR #0559 stay.",
		"ADR-0147, ADR 0559, and ADR-0065's rule stay.",
		"An unknown change 9999 and changes 9998 and 9997 stay.",
		"A bare https://go.dev/x URL stays, and a 0559-export.md file, docs/0559, 0559.md too.",
		"A converted \"Close a change to done without a PR\" title stays.",
		"Change management is prose, and so is Changes: none.",
		"Code: `<!-- docket:x:start -->`, `[a](b)`, `change 559`, ``a ` [x](y) ``, `#0559`, `↩`.",
		"",
		"## Section",
		"",
		"```md",
		"---",
		"<!-- dckt:backlink:start -->",
		"> ↩ back",
		"[a](b) ![c](d) <https://x> [r]: u <a href=\"x\">t</a>",
		"change 559, #0559, (0113), mirroring 0088",
		"# H1 in code",
		"#### skipped in code",
		"```",
		"",
		"~~~~",
		"Change #561, groomed 2026-10-20.",
		"```",
		"still code",
		"~~~~",
		"",
		"Tail.",
	}, "\n") + "\n"
	r := Check([]byte(md), known)
	if len(r.Leftovers) != 0 {
		t.Fatalf("Check flagged what the export keeps on purpose: %+v", r.Leftovers)
	}
}

// TestCheckKeepsProseThatOnlyResemblesMachinery covers author prose the transform
// leaves alone on purpose: a ↩ that is not the banner's "> ↩" line, an <a>
// placeholder that is not an anchor element, and a "[x][y]" with no definition
// behind it, which Markdown renders as literal text.
func TestCheckKeepsProseThatOnlyResemblesMachinery(t *testing.T) {
	cases := []struct{ name, md string }{
		{"arrow in prose", "# T\n\nThe ↩ backlink banner goes.\n"},
		{"arrow later in a quote", "> a quote about the ↩ banner\n"},
		{"placeholder a", "# T\n\nThe main tree is at <a>, the dckt branch at <b>.\n"},
		{"placeholder A", "# T\n\nCompare <A> with <B>.\n"},
		{"a then words", "# T\n\nwrite <a and b> here\n"},
		{"index pair", "# T\n\nread x[i][j] twice\n"},
		{"undefined reference", "# T\n\nsee [t][r] here\n"},
		{"undefined collapsed reference", "# T\n\nsee [t][] here\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			md := tc.md
			if !strings.HasPrefix(md, "# ") {
				md = "# T\n\n" + md
			}
			if r := Check([]byte(md), known); len(r.Leftovers) != 0 {
				t.Fatalf("Check(%q) flagged prose: %+v", md, r.Leftovers)
			}
		})
	}
}

func TestCheckStampOnlyBeforeFirstH2(t *testing.T) {
	r := Check([]byte("# T\n\n## Why\n\nChange 9561.\n"), known)
	if len(r.Leftovers) != 0 {
		t.Fatalf("a stamp shape after the first H2 is prose: %+v", r.Leftovers)
	}
}

func TestCheckFrontmatterOnlyOnFirstLine(t *testing.T) {
	r := Check([]byte("# T\n\n---\n\ntext\n"), known)
	if len(r.Leftovers) != 0 {
		t.Fatalf("a thematic break below the top is not frontmatter: %+v", r.Leftovers)
	}
}

func TestCheckMentions(t *testing.T) {
	md := "# T\n" +
		"docket keeps this\n" +
		"\n" +
		"```\n" +
		"run DCKT now\n" +
		"```\n" +
		"Docketed is not the word\n"
	got := Check([]byte(md), known).Mentions
	want := Mentions{Count: 2, Lines: []int{2, 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Mentions = %+v, want %+v", got, want)
	}
}

func TestCheckHeadingRules(t *testing.T) {
	cases := []struct {
		name string
		md   string
		want []lineRule
	}{
		{"two H1s", "# A\n\ntext\n\n# B\n", []lineRule{{5, RuleHeading}}},
		{"H2 before the H1", "## Pre\n\n# A\n", []lineRule{{3, RuleHeading}}},
		{"skipped level", "# A\n\n## B\n\n#### C\n", []lineRule{{5, RuleHeading}}},
		{"no H1", "## A\n\ntext\n", []lineRule{{1, RuleHeading}}},
		{"no heading at all", "text\n", []lineRule{{1, RuleHeading}}},
		{"clean", "# A\n\n## B\n\n### C\n\n## D\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lineRules(Check([]byte(tc.md), known))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Check(%q) = %v, want %v", tc.md, got, tc.want)
			}
		})
	}
}

// TestCheckMasksCodeSpansAcrossLines: a code span may cross a line break inside
// a paragraph, so marker-, link-, and reference-shaped text on either of its
// lines is code. A blank line ends the paragraph: a backtick run before it never
// pairs with one after it.
func TestCheckMasksCodeSpansAcrossLines(t *testing.T) {
	md := "# T\n\n- Markers (`<!-- docket:dispatch:start (managed by docket — do not\n" +
		"  hand-edit) -->` / `<!-- docket:dispatch:end -->`); see `x`.\n\n" +
		"open ` [a](b)\n\nchange 0559 ` close\n"
	got := lineRules(Check([]byte(md), known))
	want := []lineRule{{6, RuleLink}, {8, RuleChangeRef}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("leftovers = %v, want %v", got, want)
	}
}
