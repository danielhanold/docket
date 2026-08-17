package process

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestImportBoundaryStdlibOnly proves internal/process imports no
// github.com/danielhanold/docket package — the spec's dependency rule
// (cli -> app -> process -> stdlib). Test files are exempt: they may use
// helpers, but production code may not.
func TestImportBoundaryStdlibOnly(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		checked++
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if strings.Contains(path, ".") && strings.Contains(path, "/") {
				// Stdlib paths have no dot in the first element; any dotted
				// domain import (module-internal or third-party) is a breach.
				t.Errorf("%s imports %q — internal/process is stdlib-only", name, path)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("population floor: no production files checked — the guard is scanning the wrong directory")
	}
}
