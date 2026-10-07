package gatedrive

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// mkLockDir creates <stateDir>/worktree-locks/<name>, with a busy.lock in it
// when withLock is set, and returns the directory.
func mkLockDir(t *testing.T, stateDir, name string, withLock bool) string {
	t.Helper()
	dir := filepath.Join(stateDir, "worktree-locks", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if withLock {
		if err := os.WriteFile(filepath.Join(dir, worktreeLockFile), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBusyWorktreeLocksReturnsOnlyHeldDirs(t *testing.T) {
	stateDir := testsupport.TempDir(t)
	held := mkLockDir(t, stateDir, "bbb", true)
	mkLockDir(t, stateDir, "aaa", true)  // free: nobody holds it
	mkLockDir(t, stateDir, "ccc", false) // no busy.lock: skipped
	if err := os.WriteFile(filepath.Join(stateDir, "worktree-locks", "stray"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f, busy, err := process.TryExclusiveLock(filepath.Join(held, worktreeLockFile))
	if err != nil || busy {
		t.Fatalf("hold lock: busy=%v err=%v", busy, err)
	}
	defer f.Close()

	got, err := BusyWorktreeLocks(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{held}; !reflect.DeepEqual(got, want) {
		t.Fatalf("BusyWorktreeLocks = %v, want %v", got, want)
	}
	// The probe never creates a lock file where none was.
	if _, err := os.Stat(filepath.Join(stateDir, "worktree-locks", "ccc", worktreeLockFile)); !os.IsNotExist(err) {
		t.Fatalf("probe created busy.lock in a lockless dir: %v", err)
	}
	// The probe leaves a free lock free: a fresh acquire still succeeds.
	g, busy, err := process.TryExclusiveLock(filepath.Join(stateDir, "worktree-locks", "aaa", worktreeLockFile))
	if err != nil || busy {
		t.Fatalf("free lock after probe: busy=%v err=%v", busy, err)
	}
	g.Close()

	f.Close()
	got, err = BusyWorktreeLocks(stateDir)
	if err != nil || got != nil {
		t.Fatalf("after release = %v, %v; want nil, nil", got, err)
	}
}

func TestBusyWorktreeLocksEmptyAndMissingRoot(t *testing.T) {
	stateDir := testsupport.TempDir(t)
	got, err := BusyWorktreeLocks(stateDir)
	if err != nil || got != nil {
		t.Fatalf("missing root = %v, %v; want nil, nil", got, err)
	}
	if err := os.MkdirAll(filepath.Join(stateDir, "worktree-locks"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err = BusyWorktreeLocks(stateDir)
	if err != nil || got != nil {
		t.Fatalf("empty root = %v, %v; want nil, nil", got, err)
	}
}

// An unprobeable lock is liveness unknown: it is an error, never "not busy".
func TestBusyWorktreeLocksUnprobeableIsError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file modes")
	}
	stateDir := testsupport.TempDir(t)
	dir := mkLockDir(t, stateDir, "aaa", true)
	lock := filepath.Join(dir, worktreeLockFile)
	if err := os.Chmod(lock, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lock, 0o600) })
	got, err := BusyWorktreeLocks(stateDir)
	if err == nil {
		t.Fatalf("unprobeable lock = %v, nil; want an error", got)
	}
}
