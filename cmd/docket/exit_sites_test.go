package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/repoguard"
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

	// The population is the shared repo-guard walk (repoguard.MaintainedFiles):
	// it skips hidden directories (so sibling checkouts under .worktrees/ and
	// .git are never scanned), testdata, docs, and fixture trees.
	files, err := repoguard.MaintainedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	scannedSet := map[string]bool{}
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		scannedSet[rel] = true
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if exitSiteRE.Match(body) && !exitSiteAllowed[rel] {
			t.Errorf("%s calls os.Exit/log.Fatal; only %v may end the process — return an error or an exit code instead", rel, keys(exitSiteAllowed))
		}
	}
	if len(scannedSet) == 0 {
		t.Fatal("scanned no non-test Go files; the guard would pass vacuously")
	}
	// Every allowlisted file must be in the scanned population, or the
	// allowlist has rotted into a permission for a path the guard never reads.
	for rel := range exitSiteAllowed {
		if !scannedSet[rel] {
			t.Errorf("allowlisted exit site %s is not in the scanned population", rel)
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
