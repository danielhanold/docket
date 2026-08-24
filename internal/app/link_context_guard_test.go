package app

import (
	"go/ast"
	"go/parser"
	"go/token"
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
// A population floor rides along: the constructor must actually be *called* at
// no fewer sites than the swap installed, so mass-deleting call sites cannot
// leave this guard vacuously green.
//
// The floor counts real linkContextOf call expressions via the AST, not textual
// occurrences of "linkContextOf(": a substring count would be perturbed by a doc
// comment or string literal that merely names the constructor, which could mask a
// genuinely deleted call site and let the floor stay green (review finding, 0341).
// The definition (a FuncDecl, not a call) is deliberately not counted.
//
// Mutation probes (run with -count=1): (a) revert one call site to the inline
// literal -> the literal scan reddens; (b) delete linkContextOf calls below the
// floor -> the floor reddens.
func TestLinkContextSoleConstructor(t *testing.T) {
	literal := regexp.MustCompile(`render\.LinkContext\{[^}]`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "linkContextOf" {
				calls++
			}
			return true
		})
		if name == "link_context.go" {
			continue
		}
		if loc := literal.Find(src); loc != nil {
			t.Errorf("%s constructs a field-carrying render.LinkContext literal; use linkContextOf(pin) (change 0341)", name)
		}
	}
	// Floor = the call sites the swap installed; adjust ONLY upward when new
	// operations adopt the constructor.
	const floor = 16
	if calls < floor {
		t.Errorf("linkContextOf called %d times in production files, want >= %d — call sites deleted or bypassed", calls, floor)
	}
}
