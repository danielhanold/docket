package suiterunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExec creates an executable placeholder file at path, making any missing
// parent directories first. The body carries no category declaration, so this is
// the fixture for the "undeclared file fails closed" cases.
func writeExec(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeCategorized writes an executable test file whose `# docket-suite:`
// declaration line sits at hdrLine (1-based) with the given category token, and
// trailing appended verbatim to that line (a non-empty trailing exercises the
// "trailing text is a discovery error" rule). Lines before the header are inert
// comment filler, so a hdrLine > 10 is the "below line 10" fixture.
func writeCategorized(t *testing.T, path, category string, hdrLine int, trailing string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	for i := 2; i < hdrLine; i++ {
		b.WriteString("# filler\n")
	}
	b.WriteString("# docket-suite: " + category + trailing + "\n")
	b.WriteString("echo 'ok - x'\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDiscoverReturnsSortedDeclaredCategories(t *testing.T) {
	dir := t.TempDir()
	// Deliberately create out of order, one of each category, plus a non-matching
	// file and a subdir member (both excluded by the maxdepth-1 test_*.sh rule).
	writeCategorized(t, filepath.Join(dir, "test_b.sh"), "posix-install", 3, "")
	writeCategorized(t, filepath.Join(dir, "test_a.sh"), "go", 1+1, "") // header on line 2
	writeCategorized(t, filepath.Join(dir, "test_Z.sh"), "posix-downloader", 10, "")
	writeExec(t, filepath.Join(dir, "helper.sh"))                            // non-matching name
	writeCategorized(t, filepath.Join(dir, "sub", "test_c.sh"), "go", 2, "") // subdir excluded

	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	// C collation: uppercase 'Z' (0x5A) sorts before lowercase 'a' (0x61).
	want := []DiscoveredTarget{
		{Path: filepath.Join(dir, "test_Z.sh"), Category: CategoryDownloader},
		{Path: filepath.Join(dir, "test_a.sh"), Category: CategoryGo},
		{Path: filepath.Join(dir, "test_b.sh"), Category: CategoryInstall},
	}
	if len(got) != len(want) {
		t.Fatalf("Discover returned %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Discover[%d] = %+v, want %+v (full %+v)", i, got[i], want[i], got)
		}
	}
}

func TestDiscoverFailsClosedOnMissingDeclaration(t *testing.T) {
	dir := t.TempDir()
	writeCategorized(t, filepath.Join(dir, "test_good.sh"), "go", 2, "")
	writeExec(t, filepath.Join(dir, "test_bad.sh")) // no declaration at all

	_, err := Discover(dir)
	if err == nil {
		t.Fatal("Discover accepted an undeclared test file; want a fail-closed error")
	}
	if !strings.Contains(err.Error(), "test_bad.sh") {
		t.Fatalf("error must name the offending file; got %q", err.Error())
	}
	// Fail-closed, never a lenient generic/legacy category.
	for _, forbidden := range []string{"legacy", "generic", "default"} {
		if strings.Contains(strings.ToLower(err.Error()), forbidden) {
			t.Fatalf("error hints a lenient fallback category %q: %q", forbidden, err.Error())
		}
	}
}

func TestDiscoverFailsClosedOnUnknownCategory(t *testing.T) {
	dir := t.TempDir()
	writeCategorized(t, filepath.Join(dir, "test_unknown.sh"), "bash", 2, "") // not a real category

	_, err := Discover(dir)
	if err == nil {
		t.Fatal("Discover accepted an unknown category; want a fail-closed error")
	}
	if !strings.Contains(err.Error(), "test_unknown.sh") {
		t.Fatalf("error must name the offending file; got %q", err.Error())
	}
}

func TestDiscoverFailsClosedOnBelowLineTenAndTrailingText(t *testing.T) {
	t.Run("below line 10", func(t *testing.T) {
		dir := t.TempDir()
		// A valid sibling so the only failure is the below-line-10 file.
		writeCategorized(t, filepath.Join(dir, "test_ok.sh"), "go", 2, "")
		writeCategorized(t, filepath.Join(dir, "test_late.sh"), "go", 11, "") // header on line 11
		_, err := Discover(dir)
		if err == nil || !strings.Contains(err.Error(), "test_late.sh") {
			t.Fatalf("a declaration below line 10 must fail closed naming the file; got err=%v", err)
		}
	})
	t.Run("trailing text", func(t *testing.T) {
		dir := t.TempDir()
		writeCategorized(t, filepath.Join(dir, "test_trail.sh"), "go", 2, " # nope") // trailing text
		_, err := Discover(dir)
		if err == nil || !strings.Contains(err.Error(), "test_trail.sh") {
			t.Fatalf("a declaration with trailing text must fail closed naming the file; got err=%v", err)
		}
	})
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
	writeCategorized(t, pathA, "go", 2, "")
	writeCategorized(t, pathB, "go", 2, "")
	missing := filepath.Join(dir, "c", "test_y.sh") // never created

	_, err := ResolveTargets([]DiscoveredTarget{
		{Path: pathA, Category: CategoryGo},
		{Path: pathB, Category: CategoryGo},
		{Path: missing, Category: CategoryGo},
	}, nil)
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

func TestResolveTargetsJoinsBudgetsAndCarriesCategory(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "test_a.sh")
	pathB := filepath.Join(dir, "test_b.sh")
	writeCategorized(t, pathA, "posix-install", 2, "")
	writeCategorized(t, pathB, "go", 2, "")

	budgets := map[string]budgetRow{
		"test_a.sh": {Ceiling: 20, Mode: ModeSerial},
	}
	targets, err := ResolveTargets([]DiscoveredTarget{
		{Path: pathA, Category: CategoryInstall},
		{Path: pathB, Category: CategoryGo},
	}, budgets)
	if err != nil {
		t.Fatalf("ResolveTargets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("ResolveTargets returned %d targets, want 2", len(targets))
	}
	want0 := Target{Path: pathA, Base: "test_a.sh", Ceiling: 20, Mode: ModeSerial, Category: CategoryInstall}
	if targets[0] != want0 {
		t.Fatalf("target 0 = %+v, want %+v", targets[0], want0)
	}
	// test_b.sh has no row -> DefaultCeiling / ModeParallel, category carried.
	want1 := Target{Path: pathB, Base: "test_b.sh", Ceiling: DefaultCeiling, Mode: ModeParallel, Category: CategoryGo}
	if targets[1] != want1 {
		t.Fatalf("target 1 = %+v, want %+v", targets[1], want1)
	}
}
