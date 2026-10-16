package repoguard

// This file guards against test-shaped functions that go test never runs. A
// top-level func in a _test.go file that takes exactly one *testing.T, *testing.B,
// or *testing.F (under any local name for the "testing" import, including a dot
// import), returns nothing, and has no receiver or type parameters looks like a
// test. If its name is not one go test runs (Test/Benchmark/Fuzz matching its
// parameter type, followed by nothing or a non-lower-case rune), it only runs when
// something calls it. The guard reports such a function when no identifier with its
// name appears in any .go file of the same directory, across every package and
// build tag there (parsing ignores build constraints).
//
// Limitation: a use from another directory, or by reflection only, is invisible to
// this guard, so a function used only that way is reported as unrunnable. The trace
// that added the guard found no such use in the tree.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// unrunnableTestFunc names one test-shaped function go test never runs. File is the
// slash-relative path of the file that declares it.
type unrunnableTestFunc struct{ File, Name string }

// goTestRuns reports whether go test runs a function named name whose parameter
// type calls for prefix (Test, Benchmark, or Fuzz): the name must start with
// prefix, and the rest must be empty or start with a rune that is not lower case.
func goTestRuns(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	rest := name[len(prefix):]
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(r)
}

// testingParamPrefix maps the testing type a test-shaped function takes to the
// name prefix go test requires for it.
var testingParamPrefix = map[string]string{"T": "Test", "B": "Benchmark", "F": "Fuzz"}

// testShapedPrefix returns the go test name prefix fd's signature calls for, or ""
// when fd is not test-shaped. aliases holds the file's local names for the
// "testing" import; dot reports whether the file dot-imports it.
func testShapedPrefix(fd *ast.FuncDecl, aliases map[string]bool, dot bool) string {
	if fd.Recv != nil || (fd.Type.TypeParams != nil && len(fd.Type.TypeParams.List) > 0) {
		return ""
	}
	if fd.Type.Results != nil && len(fd.Type.Results.List) > 0 {
		return ""
	}
	params := fd.Type.Params
	if params == nil || len(params.List) != 1 || len(params.List[0].Names) > 1 {
		return ""
	}
	star, ok := params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return ""
	}
	var typeName string
	switch x := star.X.(type) {
	case *ast.SelectorExpr:
		pkg, ok := x.X.(*ast.Ident)
		if !ok || !aliases[pkg.Name] {
			return ""
		}
		typeName = x.Sel.Name
	case *ast.Ident:
		if !dot {
			return ""
		}
		typeName = x.Name
	default:
		return ""
	}
	return testingParamPrefix[typeName]
}

// findUnrunnableTestFuncs parses every source in srcs (keyed by slash-relative .go
// path) and returns the test-shaped functions go test never runs and nothing in
// their directory references, sorted by File then Name. A parse error returns an
// error naming the file.
func findUnrunnableTestFuncs(srcs map[string][]byte) ([]unrunnableTestFunc, error) {
	keys := make([]string, 0, len(srcs))
	for k := range srcs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fset := token.NewFileSet()
	refs := map[string]map[string]bool{} // directory -> referenced identifier names
	var candidates []unrunnableTestFunc
	for _, rel := range keys {
		f, err := parser.ParseFile(fset, rel, srcs[rel], parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", rel, err)
		}
		dir := path.Dir(rel)
		if refs[dir] == nil {
			refs[dir] = map[string]bool{}
		}
		declNames := map[*ast.Ident]bool{}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				declNames[fd.Name] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && !declNames[id] {
				refs[dir][id.Name] = true
			}
			return true
		})

		if !strings.HasSuffix(rel, "_test.go") {
			continue
		}
		aliases := map[string]bool{}
		dot := false
		for _, imp := range f.Imports {
			if imp.Path.Value != `"testing"` {
				continue
			}
			switch {
			case imp.Name == nil:
				aliases["testing"] = true
			case imp.Name.Name == ".":
				dot = true
			case imp.Name.Name != "_":
				aliases[imp.Name.Name] = true
			}
		}
		if len(aliases) == 0 && !dot {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			prefix := testShapedPrefix(fd, aliases, dot)
			if prefix == "" || goTestRuns(fd.Name.Name, prefix) {
				continue
			}
			candidates = append(candidates, unrunnableTestFunc{File: rel, Name: fd.Name.Name})
		}
	}

	var out []unrunnableTestFunc
	for _, c := range candidates {
		if !refs[path.Dir(c.File)][c.Name] {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// TestEveryTestShapedFunctionRuns fails for each test-shaped function in the
// maintained tree that go test never runs and nothing in its directory references.
func TestEveryTestShapedFunctionRuns(t *testing.T) {
	root := guardRoot(t)
	srcs := map[string][]byte{}
	for _, rel := range maintainedPop(t, root) {
		if strings.HasSuffix(rel, ".go") {
			srcs[rel] = []byte(readMaintained(t, root, rel))
		}
	}
	// Population floor: this file is e2e-tagged, so its presence proves tagged
	// files are in the population.
	const floor = "internal/app/finalize_e2e_test.go"
	if _, ok := srcs[floor]; !ok {
		t.Fatalf("population floor: %s is missing from the maintained .go files (%d found); the guard would scan too little", floor, len(srcs))
	}
	found, err := findUnrunnableTestFuncs(srcs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, f := range found {
		t.Errorf("%s: %s has a test signature, but go test never runs it and nothing in its directory references it — rename it to a name go test runs (Test/Benchmark/Fuzz followed by a non-lower-case rune or nothing), or call it from one", f.File, f.Name)
	}
}

func TestUnrunnableTestFuncsDetector(t *testing.T) {
	srcs := map[string][]byte{
		"p/a_test.go": []byte(`package p
import "testing"
func TestRuns(t *testing.T) { t.Run("sub", referencedSub) }
func referencedSub(t *testing.T) {}
var table = []func(*testing.T){tableSub}
func tableSub(t *testing.T) {}
func testOrphanExample(t *testing.T) {}
func Testlower(t *testing.T) {}
func Test(t *testing.T) {}
func Test_under(t *testing.T) {}
func Test2(t *testing.T) {}
func benchmarkOrphan(b *testing.B) {}
func BenchmarkFine(b *testing.B) {}
func fuzzOrphan(f *testing.F) {}
func BenchmarkWrongType(t *testing.T) {}
func helperTwoParams(t *testing.T, n int) {}
func helperResult(t *testing.T) error { return nil }
type s struct{}
func (s) methodOrphan(t *testing.T) {}
func usedFromNonTest(t *testing.T) {}
func usedFromExternal(t *testing.T) {}
`),
		"p/b.go":          []byte("package p\nvar _ = usedFromNonTest\n"),
		"p/c_test.go":     []byte("package p_test\nvar _ = usedFromExternal\n"),
		"q/alias_test.go": []byte("package q\nimport tt \"testing\"\nfunc aliasOrphan(t *tt.T) {}\n"),
		"q/dot_test.go":   []byte("package q\nimport . \"testing\"\nfunc dotOrphan(t *T) {}\n"),
		"q/x_test.go":     []byte("package q\nimport \"testing\"\nfunc referencedSub(t *testing.T) {}\n"),
	}
	want := []unrunnableTestFunc{
		{"p/a_test.go", "BenchmarkWrongType"},
		{"p/a_test.go", "Testlower"},
		{"p/a_test.go", "benchmarkOrphan"},
		{"p/a_test.go", "fuzzOrphan"},
		{"p/a_test.go", "testOrphanExample"},
		{"q/alias_test.go", "aliasOrphan"},
		{"q/dot_test.go", "dotOrphan"},
		{"q/x_test.go", "referencedSub"}, // referenced only in p/: scoping is per directory
	}
	got, err := findUnrunnableTestFuncs(srcs)
	if err != nil {
		t.Fatalf("findUnrunnableTestFuncs: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findUnrunnableTestFuncs:\n got  %v\n want %v", got, want)
	}

	_, err = findUnrunnableTestFuncs(map[string][]byte{"r/bad_test.go": []byte("package r\nfunc (")})
	if err == nil || !strings.Contains(err.Error(), "r/bad_test.go") {
		t.Errorf("parse error: got %v, want a non-nil error naming r/bad_test.go", err)
	}
}
