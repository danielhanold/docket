package document

import (
	"strings"
	"testing"
)

func TestClassifyLines(t *testing.T) {
	text := LineInfo{Kind: LineText}
	fm := LineInfo{Kind: LineFrontmatter}
	fence := LineInfo{Kind: LineFence}
	malformed := LineInfo{Kind: LineMarkerMalformed}
	start := func(name string) LineInfo { return LineInfo{Kind: LineMarkerStart, Name: name} }
	end := func(name string) LineInfo { return LineInfo{Kind: LineMarkerEnd, Name: name} }

	cases := []struct {
		name string
		src  string
		want []LineInfo
	}{
		{
			name: "frontmatter body and artifacts block",
			src: "---\nid: 1\nstatus: proposed\n---\nbody\n" +
				"<!-- docket:artifacts:start (generated) -->\nlink\n<!-- docket:artifacts:end -->\n",
			want: []LineInfo{fm, fm, fm, fm, text, start("artifacts"), text, end("artifacts")},
		},
		{
			name: "neutral namespace keeps its prefix",
			src:  "<!-- dckt:backlink:start -->\nx\n<!-- dckt:backlink:end -->",
			want: []LineInfo{start("dckt:backlink"), text, end("dckt:backlink")},
		},
		{
			name: "markers inside a backtick fence are fence lines",
			src:  "a\n```md\n<!-- docket:artifacts:start -->\n## Heading\n```\nb\n",
			want: []LineInfo{text, fence, fence, fence, fence, text},
		},
		{
			name: "tilde fence closed by a longer run",
			src:  "~~~\n<!-- docket:Bad -->\n```\n~~~~~\n<!-- docket:x:end -->\n",
			want: []LineInfo{fence, fence, fence, fence, end("x")},
		},
		{
			name: "a closer carrying an info string does not close",
			src:  "```\n``` go\n<!-- docket:x:start -->\n```\nafter\n",
			want: []LineInfo{fence, fence, fence, fence, text},
		},
		{
			name: "unclosed fence runs to EOF",
			src:  "```\n<!-- docket:x:start -->\ntext",
			want: []LineInfo{fence, fence, fence},
		},
		{
			name: "unclosed frontmatter opener is text",
			src:  "---\nid: 1\n<!-- docket:x:start -->\nbody\n",
			want: []LineInfo{text, text, start("x"), text},
		},
		{
			name: "end with no start and start with no end",
			src:  "<!-- docket:a:end -->\nmid\n<!-- docket:b:start -->\ntail\n",
			want: []LineInfo{end("a"), text, start("b"), text},
		},
		{
			name: "malformed markers",
			src:  "<!-- docket:Bad -->\n<!-- dckt:x:start\n<!-- docket:x:end (note) -->\n <!-- docket:x:start -->\n",
			want: []LineInfo{malformed, malformed, malformed, text},
		},
		{
			name: "frontmatter lines are never markers or fences",
			src:  "---\n<!-- docket:x:start -->\n```\n---\n<!-- docket:y:start -->\n",
			want: []LineInfo{fm, fm, fm, fm, start("y")},
		},
		{
			name: "empty source",
			src:  "",
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := func(t *testing.T, src string) {
				t.Helper()
				got := ClassifyLines([]byte(src))
				if n := len(scanLines([]byte(src))); len(got) != n {
					t.Fatalf("len(ClassifyLines) = %d, scanLines = %d", len(got), n)
				}
				if len(got) != len(tc.want) {
					t.Fatalf("got %d lines %v, want %d %v", len(got), got, len(tc.want), tc.want)
				}
				for i := range got {
					if got[i] != tc.want[i] {
						t.Errorf("line %d: got %+v, want %+v", i+1, got[i], tc.want[i])
					}
				}
			}
			t.Run("LF", func(t *testing.T) { check(t, tc.src) })
			t.Run("CRLF", func(t *testing.T) { check(t, strings.ReplaceAll(tc.src, "\n", "\r\n")) })
		})
	}
}
