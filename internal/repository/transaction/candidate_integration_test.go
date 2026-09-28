//go:build integration

package transaction

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationTxnApplyAllocateCandidateStructureAndManifest proves allocation lays down the
// private candidate tree, mints a 32-hex id, publishes a fully populated
// manifest, and stays invisible to the primary checkout's status.
func TestIntegrationTxnApplyAllocateCandidateStructureAndManifest(t *testing.T) {
	client, repo := newTxnRepo(t)

	c, err := allocateCandidate(txnTestClock, repo, "origin", "refs/heads/main", fixedBase)
	if err != nil {
		t.Fatalf("allocateCandidate: %v", err)
	}
	defer func() { _ = c.live.release() }()

	// Directory shape: <transactionsRoot>/<id>/{worktree not yet, hooks empty}.
	wantRoot := filepath.Join(transactionsRoot(repo), c.id)
	if c.root != wantRoot {
		t.Errorf("c.root = %q, want %q", c.root, wantRoot)
	}
	if c.worktree != filepath.Join(wantRoot, "worktree") {
		t.Errorf("c.worktree = %q", c.worktree)
	}
	if c.hooks != filepath.Join(wantRoot, "hooks") {
		t.Errorf("c.hooks = %q", c.hooks)
	}
	if fi, err := os.Stat(c.root); err != nil || !fi.IsDir() {
		t.Fatalf("candidate root not a dir: %v", err)
	}
	entries, err := os.ReadDir(c.hooks)
	if err != nil {
		t.Fatalf("read hooks dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("hooks dir not empty: %v", entries)
	}
	if _, err := os.Stat(filepath.Join(c.root, "manifest.json")); err != nil {
		t.Errorf("manifest.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.root, "live.lock")); err != nil {
		t.Errorf("live.lock missing: %v", err)
	}

	// ID shape.
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(c.id) {
		t.Errorf("id %q does not match ^[0-9a-f]{32}$", c.id)
	}

	// Two allocations differ.
	c2, err := allocateCandidate(txnTestClock, repo, "origin", "refs/heads/main", fixedBase)
	if err != nil {
		t.Fatalf("second allocateCandidate: %v", err)
	}
	defer func() { _ = c2.live.release() }()
	if c.id == c2.id {
		t.Errorf("two allocations produced the same id %q", c.id)
	}

	// Manifest round-trips with every field populated.
	m := readManifestFile(t, c.root)
	wantStamp := txnTestClock.Now().UTC().Format(time.RFC3339)
	want := manifest{
		Schema:        manifestSchemaVersion,
		TransactionID: c.id,
		CommonDir:     repo.CommonDir,
		Remote:        "origin",
		TargetRef:     "refs/heads/main",
		BaseCommit:    fixedBase,
		WorktreeRel:   "worktree",
		Phase:         phaseAllocating,
		CreatedUTC:    wantStamp,
		UpdatedUTC:    wantStamp,
		PID:           os.Getpid(),
	}
	if m != want {
		t.Errorf("manifest =\n%+v\nwant\n%+v", m, want)
	}

	// The transactions tree lives under the common dir, invisible to status.
	changes, err := client.ChangedPaths(context.Background(), repo.PrimaryWorktree)
	if err != nil {
		t.Fatalf("ChangedPaths: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("primary checkout dirty after allocation: %v", changes)
	}
}

// TestIntegrationTxnApplyCandidateModesUnderUmask proves the promised modes are enforced with an
// explicit chmod: under a permissive umask (0o022) an unchmodded lock/dir would
// land at 0644/0755, so passing under BOTH umasks can only mean the chmod ran.
func TestIntegrationTxnApplyCandidateModesUnderUmask(t *testing.T) {
	for _, um := range []int{0o077, 0o022} {
		t.Run(fmt.Sprintf("umask_%04o", um), func(t *testing.T) {
			old := syscall.Umask(um)
			defer syscall.Umask(old)

			_, repo := newTxnRepo(t)
			c, err := allocateCandidate(txnTestClock, repo, "origin", "refs/heads/main", fixedBase)
			if err != nil {
				t.Fatalf("allocateCandidate: %v", err)
			}
			defer func() { _ = c.live.release() }()

			assertPerm(t, c.root, 0o700)
			assertPerm(t, c.hooks, 0o700)
			assertPerm(t, filepath.Join(c.root, "manifest.json"), 0o600)
			assertPerm(t, filepath.Join(c.root, "live.lock"), 0o600)
		})
	}
}

// txnTestClock is the pinned instant every candidate test stamps manifests with.
var txnTestClock = fakeClock{t: time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)}

// newTxnRepo builds a real, non-bare Git repository under testsupport.TempDir(t), makes one
// commit so HEAD exists, and resolves its canonical identity through the same
// gitcli.Discover the engine uses. It returns the client (for ChangedPaths) and
// the repository whose CommonDir roots the transactions tree. The test is
// skipped when git is unavailable.
func newTxnRepo(t *testing.T) (*gitcli.Client, gitcli.Repository) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	useBackgroundOffGit(t)
	dir := testsupport.TempDir(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.name", "t")
	run("config", "user.email", "t@t")
	run("config", "core.quotePath", "true")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-q", "-m", "initial")

	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: dir})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return client, repo
}

// assertPerm fails unless path's permission bits equal want. Lstat so a symlink
// is never followed to something else's mode.
func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Errorf("%s mode = %04o, want %04o", path, got, want)
	}
}

func readManifestFile(t *testing.T, root string) manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	return m
}
