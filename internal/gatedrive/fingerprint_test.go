package gatedrive

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newDirtyRepo seeds a temporary git repository with a committed regular file,
// a second committed file, and a committed symlink, then returns its path. The
// repository is clean at return; each test mutates exactly one dimension so the
// resulting inequality isolates that dimension.
func newDirtyRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitInit(t, repo)
	writeFile(t, repo, "x.sh", "echo hello\n")
	writeFile(t, repo, "keep.txt", "keep\n")
	symlink(t, repo, "x.sh", "link")
	gitAdd(t, repo, "x.sh", "keep.txt", "link")
	gitCommit(t, repo, "seed")
	return repo
}

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func gitInit(t *testing.T, repo string) {
	t.Helper()
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "config", "core.fileMode", "true")
	git(t, repo, "config", "core.symlinks", "true")
}

func gitAdd(t *testing.T, repo string, paths ...string) {
	t.Helper()
	git(t, repo, append([]string{"add", "--"}, paths...)...)
}

func gitCommit(t *testing.T, repo, msg string) {
	t.Helper()
	git(t, repo, "commit", "-q", "-m", msg)
}

func writeFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	p := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, repo, target, rel string) {
	t.Helper()
	p := filepath.Join(repo, rel)
	_ = os.Remove(p)
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
}

func chmod(t *testing.T, repo, rel string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(filepath.Join(repo, rel), mode); err != nil {
		t.Fatal(err)
	}
}

func fingerprint(t *testing.T, repo string) Fingerprint {
	t.Helper()
	fp, err := ComputeFingerprint(repo, realGit{})
	if err != nil {
		t.Fatalf("ComputeFingerprint: %v", err)
	}
	return fp
}

// TestFingerprintStagedByteChange proves that staging a content change to a
// tracked file alters the fingerprint.
func TestFingerprintStagedByteChange(t *testing.T) {
	repo := newDirtyRepo(t)
	a := fingerprint(t, repo)
	writeFile(t, repo, "x.sh", "echo goodbye\n")
	gitAdd(t, repo, "x.sh")
	b := fingerprint(t, repo)
	if a.Equal(b) {
		t.Fatalf("staged byte change must alter fingerprint")
	}
}

// TestFingerprintUnstagedByteChange proves that an unstaged content change to a
// tracked file alters the fingerprint.
func TestFingerprintUnstagedByteChange(t *testing.T) {
	repo := newDirtyRepo(t)
	a := fingerprint(t, repo)
	writeFile(t, repo, "x.sh", "echo goodbye\n") // not staged
	b := fingerprint(t, repo)
	if a.Equal(b) {
		t.Fatalf("unstaged byte change must alter fingerprint")
	}
}

// TestFingerprintUntrackedFileAdded proves that adding an untracked file alters
// the fingerprint.
func TestFingerprintUntrackedFileAdded(t *testing.T) {
	repo := newDirtyRepo(t)
	a := fingerprint(t, repo)
	writeFile(t, repo, "new.txt", "brand new\n")
	b := fingerprint(t, repo)
	if a.Equal(b) {
		t.Fatalf("untracked file must alter fingerprint")
	}
}

// TestFingerprintDetectsModeChange proves an executable-bit change alters the
// fingerprint. (Plan Task 3 snippet.)
func TestFingerprintDetectsModeChange(t *testing.T) {
	repo := newDirtyRepo(t)
	a := fingerprint(t, repo)
	chmod(t, repo, "x.sh", 0o755)
	b := fingerprint(t, repo)
	if a.Equal(b) {
		t.Fatalf("mode change must alter fingerprint")
	}
}

// TestFingerprintFileDeleted proves that deleting a tracked file from the
// worktree alters the fingerprint.
func TestFingerprintFileDeleted(t *testing.T) {
	repo := newDirtyRepo(t)
	a := fingerprint(t, repo)
	if err := os.Remove(filepath.Join(repo, "keep.txt")); err != nil {
		t.Fatal(err)
	}
	b := fingerprint(t, repo)
	if a.Equal(b) {
		t.Fatalf("deletion must alter fingerprint")
	}
}

// TestFingerprintFileRenamed proves that renaming a tracked file alters the
// fingerprint.
func TestFingerprintFileRenamed(t *testing.T) {
	repo := newDirtyRepo(t)
	a := fingerprint(t, repo)
	git(t, repo, "mv", "keep.txt", "kept.txt")
	b := fingerprint(t, repo)
	if a.Equal(b) {
		t.Fatalf("rename must alter fingerprint")
	}
}

// TestFingerprintSymlinkTargetChanged proves that repointing a symlink alters
// the fingerprint — the link is hashed by its value.
func TestFingerprintSymlinkTargetChanged(t *testing.T) {
	repo := newDirtyRepo(t)
	a := fingerprint(t, repo)
	symlink(t, repo, "keep.txt", "link") // was -> x.sh
	b := fingerprint(t, repo)
	if a.Equal(b) {
		t.Fatalf("symlink retarget must alter fingerprint")
	}
}

// TestFingerprintIdenticalDirtyStateEqual proves that recomputing the
// fingerprint over an unchanged (dirty) worktree yields Equal.
func TestFingerprintIdenticalDirtyStateEqual(t *testing.T) {
	repo := newDirtyRepo(t)
	// Make it genuinely dirty: an unstaged edit plus an untracked file.
	writeFile(t, repo, "x.sh", "echo dirty\n")
	writeFile(t, repo, "untracked.txt", "loose\n")
	a := fingerprint(t, repo)
	b := fingerprint(t, repo)
	if !a.Equal(b) {
		t.Fatalf("identical dirty state must be Equal")
	}
}

// TestFingerprintDanglingSymlinkHashedByValue proves that a symlink is hashed by
// its link value and never followed: a dangling symlink still fingerprints, and
// changing its (nonexistent) target changes the fingerprint.
func TestFingerprintDanglingSymlinkHashedByValue(t *testing.T) {
	repo := newDirtyRepo(t)
	symlink(t, repo, "does-not-exist-1", "dangling") // untracked, dangling
	a, err := ComputeFingerprint(repo, realGit{})
	if err != nil {
		t.Fatalf("dangling symlink must still fingerprint, got error: %v", err)
	}
	symlink(t, repo, "does-not-exist-2", "dangling")
	b, err := ComputeFingerprint(repo, realGit{})
	if err != nil {
		t.Fatalf("dangling symlink must still fingerprint, got error: %v", err)
	}
	if a.Equal(b) {
		t.Fatalf("dangling symlink value change must alter fingerprint (link not followed)")
	}
}
