package githubcli

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

// This file is the githubcli package's import-boundary pin, both directions.
//
// Dependency direction (spec §"Package and dependency boundaries"): githubcli
// starts only `gh`. It imports NONE of workspace, repository, repository/
// transaction, config, document, process, install, harness, cli, or app — in
// fact no in-module package at all: head commits arrive as plain validated
// strings, never a gitcli.ObjectID, so even gitcli stays out. On the inward
// side, only the application layer (internal/app and internal/cli) may import
// githubcli — 0315's composition wires it there and maps its closed outcomes.
// Every OTHER landed package (domain, document, repository, workspace, evidence,
// process, render, gitcli, config, buildinfo, harness, install, assets, …) must
// stay clear of it, so the inward direction is still pinned mechanically here.
//
// The scans cover production (non-_test) files only.

const modulePrefix = "github.com/danielhanold/docket/"

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

// TestGithubcliImportsNoInModulePackage pins the OUTWARD boundary: a githubcli
// production file declares NO in-module import — not gitcli, not domain, none.
// Any docket-module import reddens.
func TestGithubcliImportsNoInModulePackage(t *testing.T) {
	files := productionGoFiles(t, ".")
	if len(files) == 0 {
		t.Fatal("no githubcli production .go files found — the boundary guard would pass vacuously")
	}
	for _, file := range files {
		for _, imp := range fileImports(t, file) {
			if strings.HasPrefix(imp, modulePrefix) {
				t.Errorf("%s imports %q: githubcli must import no in-module package (stdlib + gh process only)", file, imp)
			}
		}
	}
}

// TestNoLandedPackageImportsGithubcli pins the INWARD boundary: no production
// file under internal/ (outside githubcli itself, and outside the application
// layer) may import internal/githubcli. The application layer — internal/app and
// internal/cli — is the ONLY allowed importer (0315 wires githubcli there); every
// other package must stay clear. The importer set is DERIVED by walking internal/.
func TestNoLandedPackageImportsGithubcli(t *testing.T) {
	root := moduleRoot(t)
	selfDir := filepath.Join(root, "internal", "githubcli")
	// The application layer is allowed to import githubcli. A file is exempt only
	// when it lives directly in one of these package directories (not merely under
	// them), so a hypothetical internal/app/foo subpackage is still guarded.
	allowedImporterDirs := []string{
		filepath.Join(root, "internal", "app"),
		filepath.Join(root, "internal", "cli"),
	}
	target := modulePrefix + "internal/githubcli"
	files := allProductionGoFilesUnder(t, filepath.Join(root, "internal"))
	if len(files) == 0 {
		t.Fatal("no internal/ production files found — the reverse guard would pass vacuously")
	}
	scanned := 0
	for _, file := range files {
		dir := filepath.Dir(file)
		if dir == selfDir || slices.Contains(allowedImporterDirs, dir) {
			continue
		}
		scanned++
		for _, imp := range fileImports(t, file) {
			if imp == target {
				t.Errorf("%s imports %q: only the application layer (internal/app, internal/cli) may depend on internal/githubcli", file, imp)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("reverse guard scanned no files outside the githubcli package — would pass vacuously")
	}
}
