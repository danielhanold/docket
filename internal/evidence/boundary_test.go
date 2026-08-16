package evidence

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

// This file is the evidence package's import-boundary pin, both directions.
//
// Dependency direction (spec §"Package and dependency boundaries"): evidence
// reuses internal/document for whole-population marker validation and exact
// source patching, and NOTHING else in-module — no gitcli (a local hex check
// keeps it process-free), no GitHub, no filesystem/process/workflow/config
// behavior. And nothing landed may import evidence, since the future 0315
// composition maps its closed outcomes rather than depending on it. Both
// directions are pinned mechanically here.
//
// The scans cover production (non-_test) files only.

const modulePrefix = "github.com/danielhanold/docket/"

// evidenceAllowedInModule is the exact set of in-module packages an evidence
// production file may import.
var evidenceAllowedInModule = []string{
	"internal/document",
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found walking up from package directory")
		}
		dir = parent
	}
}

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
	slices.Sort(files)
	return files
}

func fileImports(t *testing.T, path string) []string {
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

func allProductionGoFilesUnder(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	slices.Sort(files)
	return files
}

// TestEvidenceImportsOnlyDocument pins the OUTWARD boundary: every in-module
// import an evidence production file declares must be internal/document. An
// import of gitcli/githubcli/os/etc. beyond that set reddens.
func TestEvidenceImportsOnlyDocument(t *testing.T) {
	files := productionGoFiles(t, ".")
	if len(files) == 0 {
		t.Fatal("no evidence production .go files found — the boundary guard would pass vacuously")
	}
	for _, file := range files {
		for _, imp := range fileImports(t, file) {
			if !strings.HasPrefix(imp, modulePrefix) {
				continue
			}
			rel := strings.TrimPrefix(imp, modulePrefix)
			if !slices.Contains(evidenceAllowedInModule, rel) {
				t.Errorf("%s imports %q: evidence may import only %v in-module", file, imp, evidenceAllowedInModule)
			}
		}
	}
}

// TestNoLandedPackageImportsEvidence pins the INWARD boundary: no production file
// under internal/ (outside evidence itself) may import internal/evidence. The
// importer set is DERIVED by walking internal/.
func TestNoLandedPackageImportsEvidence(t *testing.T) {
	root := moduleRoot(t)
	selfDir := filepath.Join(root, "internal", "evidence")
	target := modulePrefix + "internal/evidence"
	files := allProductionGoFilesUnder(t, filepath.Join(root, "internal"))
	if len(files) == 0 {
		t.Fatal("no internal/ production files found — the reverse guard would pass vacuously")
	}
	scanned := 0
	for _, file := range files {
		if filepath.Dir(file) == selfDir {
			continue
		}
		scanned++
		for _, imp := range fileImports(t, file) {
			if imp == target {
				t.Errorf("%s imports %q: nothing landed may depend on internal/evidence (direction must stay outward)", file, imp)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("reverse guard scanned no files outside the evidence package — would pass vacuously")
	}
}
