package repoguard

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// Ports tests/test_comment_anchor_style.sh (change 0114, ADR-0054): a
// cross-reference in maintained source anchors on a symbol name or a
// verbatim-quoted clause, never on a filename-plus-line-number. This guard
// enforces exactly the ONE mechanically-measurable form — a filename with a
// source extension immediately followed by :<digits> — the same partial scope
// the Bash predecessor kept (the bare colon-number and prose "line N" forms
// carry too many false positives to gate without false negatives; they rest on
// the AGENTS.md authoring rule plus review).

// anchorRe is the explicit-file anchor: <name>.<source-ext> then :<digits>.
var anchorRe = regexp.MustCompile(`[A-Za-z0-9_-]+\.(sh|md|yml|yaml|mdc):[0-9]+`)

// anchorExts is the source-text surface the anchor scan covers.
var anchorExts = []string{".sh", ".md", ".yml", ".yaml", ".mdc"}

func scanAnchor(rel, content string) []string {
	var v []string
	for i, line := range strings.Split(content, "\n") {
		if m := anchorRe.FindString(line); m != "" {
			v = append(v, fmt.Sprintf("%s:%d: line-number cross-reference %q — anchor on a symbol name or a quoted clause", rel, i+1, m))
		}
	}
	return v
}

// anchorCorpus is the maintained source-text surface EXCLUDING tests/. The
// retired Bash suite embeds synthetic `name.ext:NN` tokens as probe fixtures
// (its predecessor self-excluded its own such file); tests/ carries no real
// cross-reference anchors, so it is scanned nowhere here. Go source is not in
// scope for the same reason it was not in the shell predecessor's scope — the
// explicit-file form was measured false-positive-free only over shell/markdown/
// yaml source. The .go anchor rule rests on AGENTS.md plus review.
func anchorCorpus(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range maintainedPop(t, root) {
		if underDir(rel, "tests") {
			continue
		}
		if hasExt(rel, anchorExts...) {
			out = append(out, rel)
		}
	}
	return out
}

func TestCommentAnchorStyle(t *testing.T) {
	root := guardRoot(t)
	corpus := anchorCorpus(t, root)
	if len(corpus) < 40 {
		t.Fatalf("population floor: anchor corpus collapsed to %d files (expected >= 40)", len(corpus))
	}
	var violations []string
	for _, rel := range corpus {
		violations = append(violations, scanAnchor(rel, readMaintained(t, root, rel))...)
	}
	if len(violations) != 0 {
		t.Errorf("line-number cross-reference anchors in maintained source:\n%s", strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		if got := scanAnchor("probe.sh", "# see render-board.sh:76 for the id gate"); len(got) == 0 {
			t.Errorf("anchor scanner missed an explicit-file anchor render-board.sh:76")
		}
		// The two deliberately-unguarded/never-flagged shapes stay clean.
		clean := []string{
			`PATH_STACK=("${PATH_STACK[@]:0:${#PATH_STACK[@]}-1}")`,
			`{"data":{"p10":{"number":101,"mergedAt":"2026-07-05T18:22:31Z"}}}`,
			`# the archive table renders from its own pass`,
		}
		for _, c := range clean {
			if got := scanAnchor("clean.sh", c); len(got) != 0 {
				t.Errorf("anchor scanner wrongly flagged a bash slice / JSON / prose line %q: %v", c, got)
			}
		}
	})
}
