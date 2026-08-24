package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestLinkContextSoleConstructor is 0341's shape guard: outside
// link_context.go, no production file in this package may construct a
// field-carrying render.LinkContext literal — that inline idiom is exactly how
// all ~15 sites forgot RepoWebURL. The zero-value literal
// `render.LinkContext{}` on error paths is allowed (it renders nothing).
// A population floor rides along: the constructor must actually be in use at
// no fewer sites than the swap installed, so mass-deleting call sites cannot
// leave this guard vacuously green.
// Mutation probes (run with -count=1): (a) revert one call site to the inline
// literal -> the literal scan reddens; (b) delete linkContextOf calls below the
// floor -> the floor reddens.
func TestLinkContextSoleConstructor(t *testing.T) {
	literal := regexp.MustCompile(`render\.LinkContext\{[^}]`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	uses := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		uses += strings.Count(string(src), "linkContextOf(")
		if name == "link_context.go" {
			continue
		}
		if loc := literal.Find(src); loc != nil {
			t.Errorf("%s constructs a field-carrying render.LinkContext literal; use linkContextOf(pin) (change 0341)", name)
		}
	}
	// Floor = swapped call sites + 1 definition-adjacent use; adjust ONLY
	// upward when new operations adopt the constructor.
	const floor = 17
	if uses < floor {
		t.Errorf("linkContextOf used %d times in production files, want >= %d — call sites deleted or bypassed", uses, floor)
	}
}
