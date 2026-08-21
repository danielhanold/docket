package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// exitSiteRE keys on SHAPE, not on an enumerated list of spellings: any call
// through the os.Exit or log.Fatal* families terminates the process from
// wherever it sits.
var exitSiteRE = regexp.MustCompile(`\bos\.Exit\(|\blog\.Fatal`)

// exitSiteAllowed is the complete set of non-test files permitted to end the
// process. Everything else must return an error or an exit code to its caller:
// a library that exits takes the decision away from cli.Run's single-document
// presentation contract, and an os.Exit in package code is untestable in
// process. cmd/docket/main.go is the product's one exit site (it converts
// cli.Run's return value); cmd/genassets/main.go is a build-time-only
// generator with no presenter, where log.Fatalf IS its error path;
// cmd/releasepkg/main.go is repository-owned dev/release tooling (the release
// packager), a main-package entrypoint whose contract is a process exit code
// (0 success / 2 usage / 1 packager error), exactly like the other two.
//
// The allowlist admits only main-package entrypoints — commands that ARE the
// process — never library code.
var exitSiteAllowed = map[string]bool{
	"cmd/docket/main.go":     true,
	"cmd/genassets/main.go":  true,
	"cmd/releasepkg/main.go": true,
}

func TestProcessExitSitesAreAllowlisted(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// Sanity: the walk must be rooted at the module, or it would scan nothing
	// and pass vacuously.
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("module root not found at %s: %v", root, err)
	}

	scanned := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "testdata", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		scanned++
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if exitSiteRE.Match(body) && !exitSiteAllowed[rel] {
			t.Errorf("%s calls os.Exit/log.Fatal; only %v may end the process — return an error or an exit code instead", rel, keys(exitSiteAllowed))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatal("scanned no non-test Go files; the guard would pass vacuously")
	}
	// Both allowlisted files must still exist, or the allowlist has rotted
	// into a permission for a path nothing occupies.
	for rel := range exitSiteAllowed {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("allowlisted exit site %s does not exist: %v", rel, err)
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
