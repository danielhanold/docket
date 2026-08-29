package suiterunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExec creates an executable placeholder file at path, making any missing
// parent directories first.
func writeExec(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDiscoverSortsByByteValue(t *testing.T) {
	dir := t.TempDir()
	// Deliberately create out of order; a non-matching file and a subdir member
	// that must be excluded (maxdepth 1).
	for _, name := range []string{"test_b.sh", "test_a.sh", "test_Z.sh", "helper.sh"} {
		writeExec(t, filepath.Join(dir, name))
	}
	writeExec(t, filepath.Join(dir, "sub", "test_c.sh"))

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	// Uppercase 'Z' (0x5A) sorts before lowercase 'a' (0x61) in C collation.
	want := []string{
		filepath.Join(dir, "test_Z.sh"),
		filepath.Join(dir, "test_a.sh"),
		filepath.Join(dir, "test_b.sh"),
	}
	if len(got) != len(want) {
		t.Fatalf("Discover returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Discover[%d] = %q, want %q (full result %v)", i, got[i], want[i], got)
		}
	}
}

func TestDiscoverFailsClosed(t *testing.T) {
	// Nonexistent directory is an error.
	if _, err := Discover(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("Discover on a nonexistent dir returned nil error; want fail-closed")
	}

	// A readable directory with zero matches is an error, never an empty pass.
	empty := t.TempDir()
	writeExec(t, filepath.Join(empty, "helper.sh")) // present but non-matching
	_, err := Discover(empty)
	if err == nil {
		t.Fatal("Discover on a zero-match dir returned nil error; want fail-closed")
	}
	if !strings.Contains(err.Error(), "no test files") {
		t.Fatalf("Discover zero-match error = %q, want it to contain %q", err.Error(), "no test files")
	}
}

func TestResolveTargetsRejectsDuplicateBasenamesAndMissingFilesTogether(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a", "test_x.sh")
	pathB := filepath.Join(dir, "b", "test_x.sh")
	writeExec(t, pathA)
	writeExec(t, pathB)
	missing := filepath.Join(dir, "c", "test_y.sh") // never created

	_, err := ResolveTargets([]string{pathA, pathB, missing}, nil)
	if err == nil {
		t.Fatal("ResolveTargets accepted duplicate basenames and a missing file; want an error")
	}
	msg := err.Error()
	// The whole input set is validated: all three problems are reported together,
	// not just the first violation.
	for _, want := range []string{pathA, pathB, missing} {
		if !strings.Contains(msg, want) {
			t.Fatalf("ResolveTargets error = %q, missing reference to %q", msg, want)
		}
	}
}

func TestResolveTargetsJoinsBudgets(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "test_a.sh")
	pathB := filepath.Join(dir, "test_b.sh")
	writeExec(t, pathA)
	writeExec(t, pathB)

	budgets := map[string]budgetRow{
		"test_a.sh": {Ceiling: 20, Mode: ModeSerial},
	}
	targets, err := ResolveTargets([]string{pathA, pathB}, budgets)
	if err != nil {
		t.Fatalf("ResolveTargets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("ResolveTargets returned %d targets, want 2", len(targets))
	}
	if targets[0] != (Target{Path: pathA, Base: "test_a.sh", Ceiling: 20, Mode: ModeSerial}) {
		t.Fatalf("target 0 = %+v, want budget-joined serial/20", targets[0])
	}
	// test_b.sh has no row -> DefaultCeiling / ModeParallel.
	if targets[1] != (Target{Path: pathB, Base: "test_b.sh", Ceiling: DefaultCeiling, Mode: ModeParallel}) {
		t.Fatalf("target 1 = %+v, want default ceiling %d / parallel", targets[1], DefaultCeiling)
	}
}
