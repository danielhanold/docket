package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/release"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "releasepkg-bin-*")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "releasepkg")
	out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
	if err != nil {
		panic("building test binary: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// runCmd executes the built command and returns its captured streams and exit code.
func runCmd(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	cmd := exec.Command(binPath, args...)
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	err := cmd.Run()
	code = 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errBuf.String(), code
}

// repoRoot resolves the module root that owns this test file:
// cmd/releasepkg/main_test.go -> ../.. — the same checkout Package cross-builds.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate the repo root")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	root = filepath.Clean(root)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolved repo root %q has no go.mod: %v", root, err)
	}
	return root
}

// TestNoFlagsNamesEveryMissingFlag proves the usage error (exit 2) lists all
// five required flags at once, not one at a time (learning
// validate-the-whole-input-set-first).
func TestNoFlagsNamesEveryMissingFlag(t *testing.T) {
	out, errS, code := runCmd(t)
	if code != 2 {
		t.Fatalf("code = %d, want 2 (out=%q err=%q)", code, out, errS)
	}
	for _, want := range []string{"--source", "--version", "--commit", "--source-epoch", "--out"} {
		if !strings.Contains(errS, want) {
			t.Errorf("usage error does not name %q:\n%s", want, errS)
		}
	}
}

// TestBadEpochIsUsageError proves a non-decimal --source-epoch is rejected as a
// usage error (exit 2) before any packaging work.
func TestBadEpochIsUsageError(t *testing.T) {
	out, errS, code := runCmd(t,
		"--source", "/nonexistent",
		"--version", "v0.0.1-x",
		"--commit", strings.Repeat("ab", 20),
		"--source-epoch", "notanumber",
		"--out", "/nonexistent-out",
	)
	if code != 2 {
		t.Fatalf("code = %d, want 2 (out=%q err=%q)", code, out, errS)
	}
}

// TestHappyRunPackagesBundle drives one full packaging run against the repo
// source with the Task 5 fixture identity, and asserts exit 0, the success
// line, and the six bundle files. Deep artifact assertions live in Task 5's
// internal/release integration test; this is the command-surface check.
func TestHappyRunPackagesBundle(t *testing.T) {
	outDir := t.TempDir()
	const version = "v0.0.1-cmdintegration"
	stdout, errS, code := runCmd(t,
		"--source", repoRoot(t),
		"--version", version,
		"--commit", strings.Repeat("ab", 20),
		"--source-epoch", strconv.FormatInt(1700000000, 10),
		"--out", outDir,
	)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (err=%q)", code, errS)
	}
	if want := "packaged " + version + " -> " + outDir + "\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}

	want := []string{"install.sh", "checksums.txt"}
	for _, tp := range release.Tuples() {
		want = append(want, release.ArchiveName(version, tp))
	}
	for _, name := range want {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Errorf("bundle missing %s: %v", name, err)
		}
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(want) {
		t.Fatalf("OutDir holds %d entries, want exactly %d", len(entries), len(want))
	}
}
