package app

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// repoRootFromCaller locates the module root from this test file's own source
// path (two levels up from internal/app), the same root-discovery idiom the
// repoguard guards use — robust to the process working directory.
func repoRootFromCaller(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to locate this test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

// TestResultsTemplateFailsCheckpointValidation is the template<->validator
// coupling guard (change 0410). It reads the SHIPPED authoring scaffold —
// skills/docket-implement-next/results-template.md — and asserts the validator
// refuses it at the checkpoint phase with `results-placeholder`. This pins the
// correspondence between the template the coordinator copies and the reader that
// gates it: the raw scaffold, with its angle-bracket authoring instructions,
// can NEVER attach as a real artifact. If the template drops its placeholder
// scaffolding (or the validator stops recognizing it), this reddens — exactly
// the validator-must-match-the-reader coupling the phrase table cannot express.
func TestResultsTemplateFailsCheckpointValidation(t *testing.T) {
	root := repoRootFromCaller(t)
	path := filepath.Join(root, "skills", "docket-implement-next", "results-template.md")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shipped results template %s (fail closed): %v", path, err)
	}

	fs := ValidateResultsContent(src, ResultsPhaseCheckpoint)
	found := false
	for _, f := range fs {
		if f.Reason == reasonResultsPlaceholder {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("shipped results template must fail checkpoint validation with %q "+
			"(the raw scaffold must never attach); got findings %v", reasonResultsPlaceholder, fs)
	}
}

// testTemplateSlot is one angle-bracket authoring span located by the
// test-local scanner: its byte range in the authored template and its
// whitespace-normalized text.
type testTemplateSlot struct {
	start, end int // [start, end) byte offsets in the template source
	text       string
}

// testScanTemplateSlots is the INDEPENDENT slot oracle (change 0414): a
// deliberately separate implementation from extractTemplatePrompts, so the
// production extractor is never its own oracle. It blanks HTML comments
// in place (offsets preserved), then records every <...> span. The template
// has no fenced code today; if one is ever added, this scanner and the
// production extractor will disagree and the set-equality assert below reddens
// — which is the request to extend both deliberately.
func testScanTemplateSlots(t *testing.T, src []byte) []testTemplateSlot {
	t.Helper()
	body := []byte(string(src))
	for {
		open := bytes.Index(body, []byte("<!--"))
		if open < 0 {
			break
		}
		close := bytes.Index(body[open:], []byte("-->"))
		if close < 0 {
			t.Fatalf("authored template has an unclosed HTML comment at byte %d", open)
		}
		for i := open; i < open+close+3; i++ {
			body[i] = ' '
		}
	}
	var slots []testTemplateSlot
	for i := 0; i < len(body); i++ {
		if body[i] != '<' {
			continue
		}
		j := bytes.IndexByte(body[i:], '>')
		if j < 0 {
			t.Fatalf("authored template has an unterminated < span at byte %d", i)
		}
		slots = append(slots, testTemplateSlot{
			start: i, end: i + j + 1,
			text: strings.Join(strings.Fields(string(body[i:i+j+1])), " "),
		})
		i += j
	}
	return slots
}

// testFillSlots returns the template with every slot EXCEPT keep replaced by
// substantive filler text (keep < 0 fills all). Splicing runs back-to-front so
// recorded offsets stay valid.
func testFillSlots(src []byte, slots []testTemplateSlot, keep int) []byte {
	out := []byte(string(src))
	for i := len(slots) - 1; i >= 0; i-- {
		if i == keep {
			continue
		}
		out = append(out[:slots[i].start], append([]byte("Real substantive content"), out[slots[i].end:]...)...)
	}
	return out
}

// TestResultsTemplateEverySlotFailsValidation exercises EVERY authoring slot
// of the shipped template independently: an otherwise fully-filled template
// with exactly one slot left raw must fail checkpoint validation with
// results-placeholder — including the lowercase, wrapped, inline-Expected,
// title, and Human-action slots the retired uppercase heuristic could not all
// see. The slot population comes from the test-local scanner, and the
// fully-filled control must PASS, so a scanner that misses a slot (leaving raw
// scaffolding behind in the control) reddens here rather than passing
// vacuously.
func TestResultsTemplateEverySlotFailsValidation(t *testing.T) {
	root := repoRootFromCaller(t)
	src, err := os.ReadFile(filepath.Join(root, "skills", "docket-implement-next", "results-template.md"))
	if err != nil {
		t.Fatalf("read shipped results template (fail closed): %v", err)
	}
	slots := testScanTemplateSlots(t, src)
	if len(slots) < 10 {
		t.Fatalf("population floor: found only %d slots; the template carries more — the scanner is broken", len(slots))
	}

	// Mirror correspondence with the production extractor, both directions
	// via unique-set equality (correspondence-guard-runs-one-way).
	prod, err := extractTemplatePrompts(src)
	if err != nil {
		t.Fatalf("extractTemplatePrompts(authored source): %v", err)
	}
	uniq := map[string]bool{}
	for _, s := range slots {
		uniq[s.text] = true
	}
	prodSet := map[string]bool{}
	for _, p := range prod {
		prodSet[p] = true
	}
	for p := range prodSet {
		if !uniq[p] {
			t.Errorf("production extractor emits %q; the independent scan never saw it", p)
		}
	}
	for s := range uniq {
		if !prodSet[s] {
			t.Errorf("independent scan found slot %q; the production extractor missed it", s)
		}
	}

	// The embedded derivation must equal the authored-source derivation: the
	// asset guards prove byte parity, this pins the derivation seam end to end.
	embedded, err := resultsTemplatePrompts()
	if err != nil {
		t.Fatalf("resultsTemplatePrompts (embedded): %v", err)
	}
	if !reflect.DeepEqual(embedded, prod) {
		t.Fatalf("embedded prompts != authored-source prompts:\n%#v\n%#v", embedded, prod)
	}

	// Control: with EVERY slot filled the template passes checkpoint — proves
	// the fill works, so each per-slot failure below is attributable to the
	// one retained slot, not to leftover scaffolding.
	filled := testFillSlots(src, slots, -1)
	if fs := ValidateResultsContent(filled, ResultsPhaseCheckpoint); len(fs) != 0 {
		t.Fatalf("fully-filled template must pass checkpoint validation; got %v", fs)
	}

	// Each slot alone must refuse.
	for i, s := range slots {
		fixture := testFillSlots(src, slots, i)
		fs := ValidateResultsContent(fixture, ResultsPhaseCheckpoint)
		found := false
		for _, f := range fs {
			if f.Reason == reasonResultsPlaceholder {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("slot %d %q alone did not fail checkpoint validation; findings %v", i, s.text, fs)
		}
	}
}
