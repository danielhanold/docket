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
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
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

// TestTransactionRequestsUseResolvedMetadataRemote pins that every metadata
// transaction pushes to the RESOLVED metadata remote. In a private repository
// the metadata branch lives on the bare dckt remote, not origin, so a
// transaction.Request that names origin (or any other fixed spelling) would
// lease-push metadata to the wrong place.
//
// Shape: every transaction.Request composite literal in a non-test .go file
// under internal/app must set its Remote field to a call of metadataRemote —
// the one accessor over the pinned layout. Keyed on that positive shape, so
// originRemote, setupRemote(), a string literal, and an omitted Remote all
// redden alike. Its TargetRef twin is shape 1 of
// TestNoIntegrationPushOutsidePRMerge (integration_push_test.go).
//
// Mutation-tested: putting `Remote: originRemote` back in ChangeClaim reddens
// it (and TestIntegrationRecordOpsClaimTransactionTargetsPinnedMetadataLayout).
func TestTransactionRequestsUseResolvedMetadataRemote(t *testing.T) {
	root := guardRoot(t)
	fset := token.NewFileSet()
	literals := 0
	var violations []string
	walkGo(t, root, func(rel string) {
		if dirOf(rel) != "internal/app" {
			return
		}
		file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		n, v := metadataRemoteViolations(fset, rel, file)
		literals += n
		violations = append(violations, v...)
	})
	// Population floor: an empty enumeration passes every negative.
	if literals < 20 {
		t.Fatalf("population floor: found %d transaction.Request literals under internal/app (want >= 20); the literal detector drifted", literals)
	}
	for _, v := range violations {
		t.Error(v)
	}
}

// metadataRemoteViolations reports each transaction.Request literal in file
// whose Remote is not a metadataRemote(...) call, and how many literals it saw.
func metadataRemoteViolations(fset *token.FileSet, rel string, file *ast.File) (int, []string) {
	literals := 0
	var violations []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isTransactionRequest(lit.Type) {
			return true
		}
		literals++
		pos := rel + ":" + strconv.Itoa(fset.Position(lit.Pos()).Line)
		var remote ast.Expr
		for _, el := range lit.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "Remote" {
					remote = kv.Value
				}
			}
		}
		if remote == nil {
			violations = append(violations, pos+": a transaction.Request sets no Remote; it must be metadataRemote(<pinned layout>)")
			return true
		}
		if call, ok := remote.(*ast.CallExpr); ok {
			if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "metadataRemote" {
				return true
			}
		}
		var buf bytes.Buffer
		_ = printer.Fprint(&buf, fset, remote)
		violations = append(violations, pos+": a transaction.Request pushes to Remote "+buf.String()+", not the resolved metadata remote (metadataRemote(<pinned layout>))")
		return true
	})
	return literals, violations
}

// TestTransactionRequestsUseResolvedMetadataRemoteDetects is the guard's own
// negative control: originRemote, a string literal, and an omitted Remote each
// produce a violation, and the metadataRemote shape produces none.
func TestTransactionRequestsUseResolvedMetadataRemoteDetects(t *testing.T) {
	src := `package app

func good() { _ = transaction.Request{Remote: metadataRemote(pin.Layout)} }
func origin() { _ = transaction.Request{Remote: originRemote} }
func literal() { _ = transaction.Request{Remote: "dckt"} }
func missing() { _ = transaction.Request{TargetRef: metadataRef(pin.Layout)} }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "internal/app/x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	n, v := metadataRemoteViolations(fset, "internal/app/x.go", f)
	if n != 4 {
		t.Errorf("literals = %d, want 4", n)
	}
	joined := strings.Join(v, "\n")
	if len(v) != 3 || !strings.Contains(joined, "originRemote") || !strings.Contains(joined, `"dckt"`) || !strings.Contains(joined, "sets no Remote") {
		t.Errorf("violations = %v, want exactly originRemote, the literal, and the omitted Remote", v)
	}
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
