package export

import (
	"strings"
	"testing"
	"unicode"

	"github.com/danielhanold/docket/internal/document"
)

func TestMaskCode(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		spans []string // the code spans that must be hidden from the masked string
	}{
		{"single", "a `x [y](z)` b", []string{"`x [y](z)`"}},
		{"double backtick", "a `` a ` b `` c", []string{"`` a ` b ``"}},
		{"two spans", "`change 0559` and `#12`", []string{"`change 0559`", "`#12`"}},
		{"escaped backtick", "a \\` b [x](y)", nil},
		{"unmatched run", "a `` b ` c", nil},
		{"no code", "plain [x](y) text", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			masked, restore := maskCode(tc.in)
			for _, span := range tc.spans {
				if strings.Contains(masked, span) {
					t.Errorf("masked %q still contains span %q", masked, span)
				}
			}
			if len(tc.spans) == 0 && masked != tc.in {
				t.Errorf("masked = %q, want input unchanged", masked)
			}
			if got := restore(masked); got != tc.in {
				t.Errorf("restore(masked) = %q, want %q", got, tc.in)
			}
		})
	}

	// The placeholder for a span is private-use runes only: no ASCII, no digits.
	masked, _ := maskCode("a `x [y](z) 0559` b")
	inner := strings.TrimSuffix(strings.TrimPrefix(masked, "a "), " b")
	if inner == "" {
		t.Fatalf("masked %q left no placeholder", masked)
	}
	for _, r := range inner {
		if !unicode.Is(unicode.Co, r) {
			t.Errorf("placeholder rune %U is not private-use (masked %q)", r, masked)
		}
	}
}

func TestStripLinks(t *testing.T) {
	labels := map[string]bool{"ref": true, "text": true, "a b": true}
	cases := []struct{ in, want string }{
		{"[Run Modes](https://cursor.com/docs/x)", "Run Modes"},
		{"[PR 265](https://github.com/o/r/pull/265)", "PR 265"},
		{"[#559](../changes/archive/x.md)", "#559"},
		{"[text](#anchor)", "text"},
		{"[m](mailto:a@b.c)", "m"},
		{"[Foo](https://en.wikipedia.org/wiki/Foo_(bar)) end", "Foo end"},
		{"[a](u \"t\")", "a"},
		{"[a](u 't') and [b](<u v> (t))", "a and b"},
		{"<https://go.dev/x>", "https://go.dev/x"},
		{"![alt](p.png)", "alt"},
		{"![](p.png) x", " x"},
		{"[![badge](b.svg)](https://x)", "badge"},
		{"[text][ref]", "text"},
		{"[text][]", "text"},
		{"[ref]", "ref"},
		{"see [A  B] here", "see A  B here"},
		{"[note]", "[note]"},
		{"[text][missing]", "[text][missing]"},
		{"<a href=\"https://x\">text</a>", "text"},
		{"see https://go.dev/doc/toolchain", "see https://go.dev/doc/toolchain"},
		{"<!-- docket:x:start -->", "<!-- docket:x:start -->"},
		{"a \\[x](y) b", "a \\[x](y) b"},
		{"[unclosed](https://x", "[unclosed](https://x"},
		{"x < y and a <b> tag", "x < y and a <b> tag"},
		{"two [a](1) and [b](2).", "two a and b."},
	}
	for _, tc := range cases {
		if got := stripLinks(tc.in, labels); got != tc.want {
			t.Errorf("stripLinks(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	// A link wrapping a code span: the link goes, the code span stays.
	in := "[`internal/app/open.go`](../x)"
	masked, restore := maskCode(in)
	if got, want := restore(stripLinks(masked, nil)), "`internal/app/open.go`"; got != want {
		t.Errorf("code-wrapping link = %q, want %q", got, want)
	}
	// Link syntax inside a code span is untouched.
	in = "keep `[x](y)` and drop [z](w)"
	masked, restore = maskCode(in)
	if got, want := restore(stripLinks(masked, nil)), "keep `[x](y)` and drop z"; got != want {
		t.Errorf("code-span link = %q, want %q", got, want)
	}
}

func TestIsLinkDefinition(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"[ref]: https://x", true},
		{"  [a b]: <u> \"t\"", true},
		{"[ref]: u 'title'", true},
		{"[ref]: u (title)", true},
		{"[ref] text", false},
		{"    [ref]: u", false},
		{"[ref]:", false},
		{"[ref]: u trailing words", false},
		{"[^1]: a footnote", false},
		{"[ ]: u", false},
	}
	for _, tc := range cases {
		if got := isLinkDefinition(tc.in); got != tc.want {
			t.Errorf("isLinkDefinition(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestLinkLabels(t *testing.T) {
	src := "---\n[fm]: u\n---\n[Ref  One]: https://x\n```\n[code]: u\n```\ntext\n"
	info := document.ClassifyLines([]byte(src))
	lines := strings.Split(strings.TrimSuffix(src, "\n"), "\n")
	got := linkLabels(lines, info)
	if !got["ref one"] || len(got) != 1 {
		t.Errorf("linkLabels = %v, want only {ref one}", got)
	}
}
