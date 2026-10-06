package repoguard

// TestNoInlineStateFolderSpelling pins that no maintained Go source spells the
// per-repo state folder inline. The folder's name depends on the repository's
// visibility mode (layout.StateName), so every path beneath it must be built
// from layout.StateDirOf / layout.StateName, never from a literal "docket"
// path segment.
//
// Shape: every filepath.Join (or path.Join) call, at any nesting depth, with a
// string-literal argument equal to "docket", in a non-test .go file under
// internal/ or cmd/. Skipped: testdata/, the embedded asset tree, the layout
// package itself (which owns the spelling), and the two machine-path files
// whose "docket" segment names the binary's own per-user folders, not a
// repository's state folder (internal/config/fs.go and internal/install/roots.go).
//
// Mutation-tested: re-inlining "docket" in workspace.workspacesRoot reddens it.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// stateFolderGuardSkips are the files whose "docket" path segment is not a
// per-repo state folder.
var stateFolderGuardSkips = map[string]string{
	"internal/config/fs.go":     "global config path ${XDG_CONFIG_HOME}/docket/config.yml",
	"internal/install/roots.go": "the binary's per-user install roots",
}

// walkGo visits every non-test .go file under root's internal/ and cmd/ trees,
// passing its slash-separated path relative to root. It skips testdata/, the
// embedded asset tree, and the layout package, and fails the test closed on a
// walk error.
func walkGo(t *testing.T, root string, visit func(rel string)) {
	t.Helper()
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				switch {
				case d.Name() == "testdata",
					rel == "internal/assets/embedded",
					rel == "internal/layout":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			visit(rel)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", top, err)
		}
	}
}

// isPathJoin reports whether call is filepath.Join or path.Join.
func isPathJoin(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Join" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && (pkg.Name == "filepath" || pkg.Name == "path")
}

func TestNoInlineStateFolderSpelling(t *testing.T) {
	root := guardRoot(t)
	fset := token.NewFileSet()
	parsed := 0
	var offenders []string
	walkGo(t, root, func(rel string) {
		if _, skip := stateFolderGuardSkips[rel]; skip {
			return
		}
		file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		parsed++
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isPathJoin(call) {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if v, err := strconv.Unquote(lit.Value); err == nil && v == "docket" {
					pos := fset.Position(call.Pos())
					offenders = append(offenders, rel+":"+strconv.Itoa(pos.Line))
				}
			}
			return true
		})
	})
	// Population floor: a walk that silently parsed nothing would pass vacuously.
	if parsed < 100 {
		t.Fatalf("parsed only %d Go files under internal/ and cmd/; the walk is broken", parsed)
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("per-repo state folder spelled inline (build it from layout.StateDirOf / layout.StateName):\n  %s",
			strings.Join(offenders, "\n  "))
	}
}
