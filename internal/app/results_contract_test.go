package app

import (
	"os"
	"path/filepath"
	"runtime"
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
