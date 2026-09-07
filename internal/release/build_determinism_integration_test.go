//go:build integration

package release

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// gitFixtureModule creates a minimal committed Go module in its own git repo
// and returns its root. The module has a tracked non-Go file (README.md) whose
// edit dirties the tree without changing any compile input.
func gitFixtureModule(t *testing.T) string {
	t.Helper()
	dir := testsupport.TempDir(t)
	files := map[string]string{
		"go.mod":    "module detfixture\n\ngo 1.24\n",
		"main.go":   "package main\n\nfunc main() {}\n",
		"README.md": "clean\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		full := append([]string{"-C", dir, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)
		if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "fixture")
	return dir
}

// buildDefault runs a plain `go build -trimpath` (buildvcs at its default) —
// the positive control proving the fixture's git state reaches built bytes.
func buildDefault(t *testing.T, dir, out string, tuple Tuple) []byte {
	t.Helper()
	cmd := exec.Command("go", "build", "-trimpath", "-o", out, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+tuple.OS, "GOARCH="+tuple.Arch, "GOFLAGS=")
	if o, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("control build: %v\n%s", err, o)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read control binary: %v", err)
	}
	return b
}

// TestIntegrationReleaseBuildIgnoresAmbientGitState pins the demonstrated
// 0406 mechanism: ambient repository VCS state (vcs.modified et al.) is an
// undeclared build input under default -buildvcs, and buildTuple — the real
// production build invocation — must be immune to it. The control asserts the
// mechanism is live in this environment first, so the immunity assert cannot
// pass vacuously.
func TestIntegrationReleaseBuildIgnoresAmbientGitState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fixture-module builds in -short mode")
	}
	fixture := gitFixtureModule(t)
	scratch := testsupport.TempDir(t)
	host := Tuple{OS: runtime.GOOS, Arch: runtime.GOARCH}
	readme := filepath.Join(fixture, "README.md")

	// Clean-tree builds.
	if err := buildTuple("go", fixture, ".", "", host, filepath.Join(scratch, "release-clean")); err != nil {
		t.Fatalf("buildTuple clean: %v", err)
	}
	releaseClean, err := os.ReadFile(filepath.Join(scratch, "release-clean"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	controlClean := buildDefault(t, fixture, filepath.Join(scratch, "control-clean"), host)

	// Dirty the tree via a tracked NON-GO file: vcs.modified flips while every
	// compile input stays identical.
	if err := os.WriteFile(readme, []byte("clean\ndirty\n"), 0o644); err != nil {
		t.Fatalf("dirty README: %v", err)
	}

	// Positive control: with default buildvcs the dirty-tree binary MUST
	// differ, or this environment is not exercising the mechanism and the
	// assert below would prove nothing.
	controlDirty := buildDefault(t, fixture, filepath.Join(scratch, "control-dirty"), host)
	if bytes.Equal(controlClean, controlDirty) {
		t.Fatalf("control builds are byte-identical across a tree-state change; fixture does not exercise the vcs-stamping mechanism (git missing or buildvcs inert?)")
	}

	// The pinned property: the release build invocation is immune.
	if err := buildTuple("go", fixture, ".", "", host, filepath.Join(scratch, "release-dirty")); err != nil {
		t.Fatalf("buildTuple dirty: %v", err)
	}
	releaseDirty, err := os.ReadFile(filepath.Join(scratch, "release-dirty"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(releaseClean, releaseDirty) {
		t.Fatalf("buildTuple output depends on ambient git state: clean and dirty-tree builds differ (%d vs %d bytes)", len(releaseClean), len(releaseDirty))
	}
}
