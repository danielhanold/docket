package repoguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file holds shared helpers for the ported repo-guard scans (change 0370,
// Gate 2). Every guard resolves the repo root through Root, draws its population
// from MaintainedFiles / ExecutableSurface, and reads file content fail-closed
// (a read error is a test failure, never a silent clean-miss) — the same
// discipline the retired Bash suite kept.

// guardRoot resolves the module root for a guard scan, failing the test closed.
func guardRoot(t *testing.T) string {
	t.Helper()
	root, err := Root()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

// maintainedPop returns the maintained-file population, failing closed.
func maintainedPop(t *testing.T, root string) []string {
	t.Helper()
	files, err := MaintainedFiles(root)
	if err != nil {
		t.Fatalf("MaintainedFiles: %v", err)
	}
	return files
}

// execPop returns the executable-surface population, failing closed.
func execPop(t *testing.T, root string) []string {
	t.Helper()
	files, err := ExecutableSurface(root)
	if err != nil {
		t.Fatalf("ExecutableSurface: %v", err)
	}
	return files
}

// alwaysLoadedPop returns the always-loaded agent-instruction population, failing
// closed.
func alwaysLoadedPop(t *testing.T, root string) []string {
	t.Helper()
	files, err := AlwaysLoadedSurface(root)
	if err != nil {
		t.Fatalf("AlwaysLoadedSurface: %v", err)
	}
	return files
}

// readMaintained reads a maintained file (rel slash path) fail-closed: a read
// error is a guard failure, matching probe-error-is-not-clean-absence — an
// unreadable file in the scanned surface is an error, not an absence of hits.
func readMaintained(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read maintained file %s: %v (fail closed)", rel, err)
	}
	return string(b)
}

// hasExt reports whether rel ends in one of the given extensions.
func hasExt(rel string, exts ...string) bool {
	for _, e := range exts {
		if strings.HasSuffix(rel, e) {
			return true
		}
	}
	return false
}

// underDir reports whether rel is inside one of the named top-level directories
// (dir or dir/...).
func underDir(rel string, dirs ...string) bool {
	for _, d := range dirs {
		if rel == d || strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}
