package repoguard

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// writeFile creates parent dirs and writes a file inside a fixture tree.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// fixtureTree builds a small tree exercising every inclusion and exclusion rule
// and returns its root.
func fixtureTree(t *testing.T) string {
	t.Helper()
	root := testsupport.TempDir(t)
	writeFile(t, root, "go.mod", "module x\n")
	// Included: ordinary maintained source across the surfaces guards care about.
	writeFile(t, root, "install.sh", "#!/bin/sh\n")
	writeFile(t, root, "AGENTS.md", "rules\n")
	writeFile(t, root, "internal/app/app.go", "package app\n")
	writeFile(t, root, "scripts/board-checks.sh", "#!/bin/sh\n")
	writeFile(t, root, "scripts/board-checks.md", "contract\n")
	writeFile(t, root, "scripts/runners/codex.sh", "#!/bin/sh\n")
	writeFile(t, root, "skills/docket-adr/SKILL.md", "skill\n")
	writeFile(t, root, "skills/docket-adr/references/x.md", "ref\n")
	writeFile(t, root, ".github/workflows/ci.yml", "on: push\n")
	// Excluded, each by a distinct categorical rule.
	writeFile(t, root, "docs/adrs/0001.md", "an ADR quoting docket.sh preflight\n")
	writeFile(t, root, "internal/repository/testdata/corpus/frozen.md", "docket.sh preflight\n")
	writeFile(t, root, "internal/document/testdata/crlf.md", "fixture\n")
	writeFile(t, root, "testdata/repositories/v0.9.2/x.md", "frozen release\n")
	writeFile(t, root, "internal/install/testdata/legacy/old.mdc", "legacy\n")
	writeFile(t, root, "internal/install/legacydata/block.md", "adopted legacy\n")
	writeFile(t, root, "tests/fixtures/hygiene/bad.sh", "mv $a $b\n")
	// .git internals must never be walked.
	writeFile(t, root, ".git/config", "[core]\n")
	writeFile(t, root, ".worktrees/sib/go.mod", "module y\n")
	return root
}

func TestMaintainedFilesIncludesAndExcludes(t *testing.T) {
	root := fixtureTree(t)
	files, err := MaintainedFiles(root)
	if err != nil {
		t.Fatalf("MaintainedFiles: %v", err)
	}
	mustInclude := []string{
		"go.mod", "install.sh", "AGENTS.md", "internal/app/app.go",
		"scripts/board-checks.sh", "scripts/board-checks.md",
		"scripts/runners/codex.sh", "skills/docket-adr/SKILL.md",
		"skills/docket-adr/references/x.md", ".github/workflows/ci.yml",
	}
	for _, want := range mustInclude {
		if !slices.Contains(files, want) {
			t.Errorf("MaintainedFiles omitted maintained file %q; got %v", want, files)
		}
	}
	mustExclude := []string{
		"docs/adrs/0001.md",
		"internal/repository/testdata/corpus/frozen.md",
		"internal/document/testdata/crlf.md",
		"testdata/repositories/v0.9.2/x.md",
		"internal/install/testdata/legacy/old.mdc",
		"internal/install/legacydata/block.md",
		"tests/fixtures/hygiene/bad.sh",
		".git/config",
		".worktrees/sib/go.mod",
	}
	for _, bad := range mustExclude {
		if slices.Contains(files, bad) {
			t.Errorf("MaintainedFiles included categorically excluded path %q", bad)
		}
	}
	if !slices.IsSorted(files) {
		t.Errorf("MaintainedFiles result is not sorted")
	}
}

// TestMaintainedFilesResolvesSymlinkToRegularFile pins the CLAUDE.md -> AGENTS.md
// alias shape: a symlink whose target is a regular file is included.
func TestMaintainedFilesResolvesSymlinkToRegularFile(t *testing.T) {
	root := fixtureTree(t)
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	files, err := MaintainedFiles(root)
	if err != nil {
		t.Fatalf("MaintainedFiles: %v", err)
	}
	if !slices.Contains(files, "CLAUDE.md") {
		t.Errorf("MaintainedFiles dropped the CLAUDE.md symlink; got %v", files)
	}
}

// TestMaintainedFilesBrokenSymlinkFailsClosed pins fail-closed on an unresolvable
// symlink — it must error, not silently drop the entry.
func TestMaintainedFilesBrokenSymlinkFailsClosed(t *testing.T) {
	root := fixtureTree(t)
	if err := os.Symlink("does-not-exist", filepath.Join(root, "dangling")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := MaintainedFiles(root); err == nil {
		t.Errorf("MaintainedFiles must fail closed on a broken symlink, got nil error")
	}
}

// TestMaintainedFilesUnreadableDirFailsClosed pins the fail-closed contract: an
// unreadable directory aborts the walk with an error rather than yielding a
// silently-truncated population.
func TestMaintainedFilesUnreadableDirFailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory permissions")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory read permissions")
	}
	root := fixtureTree(t)
	locked := filepath.Join(root, "internal", "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatalf("mkdir locked: %v", err)
	}
	writeFile(t, root, "internal/locked/secret.go", "package locked\n")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// Restore before TempDir cleanup so the harness can remove the tree.
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	if _, err := MaintainedFiles(root); err == nil {
		t.Errorf("MaintainedFiles must fail closed on an unreadable directory, got nil error")
	}
}

func TestExecutableSurfaceClassification(t *testing.T) {
	root := fixtureTree(t)
	// An executable-bit file with no telltale extension.
	writeFile(t, root, "bin/tool", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(root, "bin", "tool"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// A .bash file.
	writeFile(t, root, "lib/helper.bash", "echo hi\n")

	exec, err := ExecutableSurface(root)
	if err != nil {
		t.Fatalf("ExecutableSurface: %v", err)
	}
	mustInclude := []string{
		"install.sh", "scripts/board-checks.sh", "scripts/runners/codex.sh",
		"scripts/board-checks.md", "skills/docket-adr/SKILL.md",
		"skills/docket-adr/references/x.md", "bin/tool", "lib/helper.bash",
	}
	for _, want := range mustInclude {
		if !slices.Contains(exec, want) {
			t.Errorf("ExecutableSurface omitted %q; got %v", want, exec)
		}
	}
	// Non-executable, non-command markdown and plain Go source are prose/source,
	// not executable surface.
	mustExclude := []string{"AGENTS.md", "internal/app/app.go", ".github/workflows/ci.yml", "go.mod"}
	for _, bad := range mustExclude {
		if slices.Contains(exec, bad) {
			t.Errorf("ExecutableSurface wrongly included %q", bad)
		}
	}
}

// TestExecutableSurfaceIsSubsetOfMaintained pins the filter relationship: every
// executable-surface path is also a maintained path.
func TestExecutableSurfaceIsSubsetOfMaintained(t *testing.T) {
	root := fixtureTree(t)
	all, err := MaintainedFiles(root)
	if err != nil {
		t.Fatalf("MaintainedFiles: %v", err)
	}
	exec, err := ExecutableSurface(root)
	if err != nil {
		t.Fatalf("ExecutableSurface: %v", err)
	}
	for _, e := range exec {
		if !slices.Contains(all, e) {
			t.Errorf("ExecutableSurface returned %q which is not in MaintainedFiles", e)
		}
	}
}

// TestRootFindsModuleRoot pins that Root resolves the repo root from the package
// working directory (go.mod present here in the real module).
func TestRootFindsModuleRoot(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Errorf("Root returned %q with no go.mod: %v", root, err)
	}
}
