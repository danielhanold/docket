package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// fixedBase is an arbitrary well-formed base commit id stored in the manifest.
// candidate.go treats it as opaque, so the exact value only has to round-trip.
const fixedBase gitcli.ObjectID = "0123456789abcdef0123456789abcdef01234567"

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

// TestLiveLockExcludesSecondNonBlocking proves a second non-blocking acquire of
// a held lock reports the would-block sentinel rather than silently succeeding.
func TestLiveLockExcludesSecondNonBlocking(t *testing.T) {
	path := filepath.Join(testsupport.TempDir(t), "live.lock")
	l1, err := acquireLock(path, false)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer func() { _ = l1.release() }()

	l2, err := acquireLock(path, false)
	if err == nil {
		_ = l2.release()
		t.Fatal("second non-blocking acquire succeeded on a held lock")
	}
	if !errors.Is(err, errLockHeld) {
		t.Fatalf("err = %v, want errLockHeld", err)
	}
}

// TestRegistryLockMutualExclusion proves withRegistryLock serializes callers and
// releases the lock once fn returns — coordinated by channels, never sleeps.
func TestRegistryLockMutualExclusion(t *testing.T) {
	root := testsupport.TempDir(t)
	registryPath := filepath.Join(root, "registry.lock")

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- withRegistryLock(root, func() error {
			close(entered)
			<-release
			return nil
		})
	}()

	<-entered
	// While fn holds the lock, a non-blocking acquire of the same file must fail.
	if l, err := acquireLock(registryPath, false); err == nil {
		_ = l.release()
		close(release)
		<-done
		t.Fatal("acquired registry.lock while withRegistryLock held it")
	} else if !errors.Is(err, errLockHeld) {
		close(release)
		<-done
		t.Fatalf("contended acquire err = %v, want errLockHeld", err)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("withRegistryLock returned: %v", err)
	}

	// The lock is free once fn returned: a non-blocking acquire now succeeds.
	l, err := acquireLock(registryPath, false)
	if err != nil {
		t.Fatalf("registry.lock still held after fn returned: %v", err)
	}
	_ = l.release()
}
