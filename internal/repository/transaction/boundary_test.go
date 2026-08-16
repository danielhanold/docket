package transaction

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The transaction subpackage's import-boundary pin. This package sits below the
// application layers and must never reach up into them: importing internal/cli,
// internal/app, internal/install, or internal/harness would invert the
// dependency direction the change's scope requires. The sources comply; this
// guard makes that a mechanically enforced fact rather than a convention.
//
// The scan is over production files only — `_test.go` files may import anything
// (tests legitimately reach for fixtures and helpers), so pinning them would
// make the guard unusable.

const transactionModulePath = "github.com/danielhanold/docket/"

// transactionForbiddenInternal names the in-module packages the transaction
// subpackage must never import.
var transactionForbiddenInternal = []string{
	"internal/cli",
	"internal/app",
	"internal/install",
	"internal/harness",
}

// transactionProductionGoFiles returns the non-test .go files in dir, sorted.
func transactionProductionGoFiles(t *testing.T, dir string) []string {
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

// transactionFileImports parses path in ImportsOnly mode and returns the import
// paths it declares.
func transactionFileImports(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports of %s: %v", path, err)
	}
	var paths []string
	for _, spec := range f.Imports {
		unquoted, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("unquote import path %s in %s: %v", spec.Path.Value, path, err)
		}
		paths = append(paths, unquoted)
	}
	return paths
}

func TestTransactionProductionFilesImportNoApplicationLayerPackage(t *testing.T) {
	for _, file := range transactionProductionGoFiles(t, ".") {
		for _, imp := range transactionFileImports(t, file) {
			if !strings.HasPrefix(imp, transactionModulePath) {
				continue
			}
			rel := strings.TrimPrefix(imp, transactionModulePath)
			if slices.Contains(transactionForbiddenInternal, rel) {
				t.Errorf("%s imports %q: the transaction subpackage must not import the application layers %v",
					file, imp, transactionForbiddenInternal)
			}
		}
	}
}
