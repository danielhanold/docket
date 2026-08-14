package repository

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The dependency-direction pin. `internal/domain` is the pure core: it may not
// reach sideways into any other `internal/...` package, and it may not touch
// the process environment (filesystem, network, exec). `internal/repository`
// is the only adapter allowed to sit above it, and only over the three
// packages named in repositoryAllowedInternal. Neither may read the wall
// clock: every time-dependent decision takes an explicit `now` argument, so a
// `time.Now` call site inside either package is a design regression, not a
// convenience.
//
// The scan is over production files only — `_test.go` files legitimately reach
// for `os`, `path/filepath`, and fixtures, and pinning them would make the
// guard unusable.

const modulePath = "github.com/danielhanold/docket/"

// domainBannedStdlib names the environment-touching stdlib packages that must
// never appear in `internal/domain` production sources.
var domainBannedStdlib = []string{"os", "os/exec", "net", "net/http", "io/fs", "path/filepath"}

// repositoryBannedStdlib names the environment-touching stdlib packages that
// must never appear in `internal/repository` production sources. `io/fs` and
// `path/filepath` are absent deliberately: repository handles caller-supplied
// paths, so a future path-manipulation need there is legitimate.
var repositoryBannedStdlib = []string{"os", "os/exec", "net", "net/http"}

// repositoryAllowedInternal is the closed set of in-module packages
// `internal/repository` may import.
var repositoryAllowedInternal = []string{
	"internal/domain",
	"internal/config",
	"internal/document",
}

// productionGoFiles returns the non-test .go files in dir, sorted.
func productionGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir %s: %v", dir, err)
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	if len(files) == 0 {
		t.Fatalf("no production .go files found under %s — the boundary guard would pass vacuously", dir)
	}
	slices.Sort(files)
	return files
}

// fileImports parses path in ImportsOnly mode and returns the import paths it
// declares, along with the local name bound to `time` (empty when `time` is
// not imported, "." for a dot-import).
func fileImports(t *testing.T, path string) (paths []string, timeLocal string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports of %s: %v", path, err)
	}
	for _, spec := range f.Imports {
		unquoted, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("unquote import path %s in %s: %v", spec.Path.Value, path, err)
		}
		paths = append(paths, unquoted)
		if unquoted == "time" {
			timeLocal = "time"
			if spec.Name != nil {
				timeLocal = spec.Name.Name
			}
		}
	}
	return paths, timeLocal
}

func TestDomainProductionFilesImportNoInternalPackageAndNoEnvironment(t *testing.T) {
	for _, file := range productionGoFiles(t, filepath.Join("..", "domain")) {
		imports, _ := fileImports(t, file)
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath) {
				t.Errorf("%s imports %q: internal/domain is the pure core and must import no in-module package", file, imp)
			}
			if slices.Contains(domainBannedStdlib, imp) {
				t.Errorf("%s imports %q: internal/domain must not touch the process environment", file, imp)
			}
		}
	}
}

func TestRepositoryProductionFilesImportOnlyTheAllowedInternalPackages(t *testing.T) {
	for _, file := range productionGoFiles(t, ".") {
		imports, _ := fileImports(t, file)
		for _, imp := range imports {
			if strings.HasPrefix(imp, modulePath) {
				rel := strings.TrimPrefix(imp, modulePath)
				if !slices.Contains(repositoryAllowedInternal, rel) {
					t.Errorf("%s imports %q: internal/repository may import only %v from this module", file, imp, repositoryAllowedInternal)
				}
			}
			if slices.Contains(repositoryBannedStdlib, imp) {
				t.Errorf("%s imports %q: internal/repository must not touch the process environment", file, imp)
			}
		}
	}
}

func TestNeitherPackageReadsTheWallClockInProductionCode(t *testing.T) {
	for _, dir := range []string{filepath.Join("..", "domain"), "."} {
		for _, file := range productionGoFiles(t, dir) {
			_, timeLocal := fileImports(t, file)
			if timeLocal == "" {
				continue
			}
			if timeLocal == "." {
				t.Errorf("%s dot-imports \"time\": a dot-import hides `Now` from this guard", file)
				continue
			}
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, file, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", file, err)
			}
			ast.Inspect(parsed, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Now" {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok || ident.Name != timeLocal {
					return true
				}
				t.Errorf("%s:%d references %s.Now: time-dependent decisions take an explicit `now` argument",
					file, fset.Position(sel.Pos()).Line, timeLocal)
				return true
			})
		}
	}
}
