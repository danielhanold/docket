package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// lockRoots is an installation universe for the lock's own unit tests: a fake
// home whose data root nothing has created yet, which is the state a first
// install meets.
func lockRoots(t *testing.T) UserRoots {
	t.Helper()
	roots, err := ResolveRoots(fixedHome(cleanTempDir(t)), fakeEnv(nil))
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}
	return roots
}

func TestInstallLockExcludesASecondHolder(t *testing.T) {
	roots := lockRoots(t)

	first, err := acquireInstallLock(roots)
	if err != nil {
		t.Fatalf("acquiring the lock: %v", err)
	}
	if !first.held() {
		t.Fatalf("a successful acquisition reports the lock unheld")
	}
	if _, err := os.Lstat(roots.LockPath()); err != nil {
		t.Fatalf("the lock file was not created: %v", err)
	}

	// flock ownership belongs to the open file description, not to the process,
	// so a second acquisition contends here exactly as another process's would.
	second, err := acquireInstallLock(roots)
	if !errors.Is(err, ErrInstallLocked) {
		t.Fatalf("second acquisition = (%v, %v), want ErrInstallLocked", second, err)
	}
	if second.held() {
		t.Fatalf("a refused acquisition handed back a held lock")
	}

	first.release()
	if first.held() {
		t.Errorf("a released lock still reports itself held")
	}
	// The file survives the release on purpose: it is a mutex, not a flag.
	if _, err := os.Lstat(roots.LockPath()); err != nil {
		t.Errorf("the lock file vanished on release: %v", err)
	}

	third, err := acquireInstallLock(roots)
	if err != nil {
		t.Fatalf("acquiring after a release: %v", err)
	}
	third.release()
	// A second release is a no-op rather than a second close of a descriptor
	// number the runtime may since have reissued.
	third.release()
}

// A process killed while holding the lock leaves the file behind and takes the
// flock with it. If the file's existence were the lock, that crash would refuse
// every installation from then on, and the cure would be a user deleting a file
// docket never told them about.
func TestInstallLockIgnoresAStaleLockFile(t *testing.T) {
	roots := lockRoots(t)
	if err := os.MkdirAll(roots.DataRoot, 0o700); err != nil {
		t.Fatalf("creating the data root: %v", err)
	}
	if err := os.WriteFile(roots.LockPath(), []byte("left behind by a killed run\n"), 0o600); err != nil {
		t.Fatalf("planting a stale lock file: %v", err)
	}

	lk, err := acquireInstallLock(roots)
	if err != nil {
		t.Fatalf("acquiring over a stale lock file: %v", err)
	}
	lk.release()
}

func TestInstallLockRefusesRootlessOptions(t *testing.T) {
	if _, err := acquireInstallLock(UserRoots{DataRoot: "relative/data"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
	if got := lockReason(ErrInvalidInput); got != ReasonInvalidOptions {
		t.Errorf("lockReason(ErrInvalidInput) = %q, want %q", got, ReasonInvalidOptions)
	}
	if got := lockReason(ErrInstallLocked); got != ReasonInstallInProgress {
		t.Errorf("lockReason(ErrInstallLocked) = %q, want %q", got, ReasonInstallInProgress)
	}
	if got := lockReason(errors.New("disk on fire")); got != ReasonFilesystemFailed {
		t.Errorf("lockReason(other) = %q, want %q", got, ReasonFilesystemFailed)
	}
}

// Recovery is only safe because it is unreachable without the lock: a journal
// found while holding the lock cannot belong to a live run. The guard makes
// that a property of the function rather than of its callers' good manners.
func TestRecoverPendingRequiresTheLock(t *testing.T) {
	roots := lockRoots(t)
	target := filepath.Join(roots.Home, "target.md")
	if err := os.WriteFile(target, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("writing the target: %v", err)
	}
	insp := Inspection{
		Target:      Target{Path: target, Kind: KindFile, Content: []byte("new\n"), Role: "agent"},
		Disposition: DispositionUpdate,
	}
	txn, err := BeginTxn(RealFS{}, roots, []Inspection{insp})
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	// The owning run has applied its step and has not committed yet: exactly the
	// window in which another run's rollback would destroy work in flight.
	if err := os.WriteFile(target, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("applying the step: %v", err)
	}

	o := Options{Roots: roots, FS: RealFS{}}
	if _, err := recoverPending(o, Outcome{}, nil); err == nil {
		t.Fatalf("recovery ran without the installation lock")
	}
	if _, err := recoverPending(o, Outcome{}, &installLock{}); err == nil {
		t.Fatalf("recovery ran with a released lock")
	}
	if id, found, err := DetectRecovery(roots); err != nil || !found || id != txn.ID() {
		t.Fatalf("the journal was rolled back anyway: found=%v id=%q err=%v", found, id, err)
	}
	if body, err := os.ReadFile(target); err != nil || string(body) != "new\n" {
		t.Fatalf("the applied step was undone without the lock: %q (%v)", body, err)
	}

	lk, err := acquireInstallLock(roots)
	if err != nil {
		t.Fatalf("acquiring the lock: %v", err)
	}
	defer lk.release()
	out, err := recoverPending(o, Outcome{}, lk)
	if err != nil {
		t.Fatalf("recovery under the lock: %v", err)
	}
	if !out.Applied {
		t.Errorf("a recovery that ran reported no applied work")
	}
	if body, err := os.ReadFile(target); err != nil || string(body) != "original\n" {
		t.Errorf("recovery under the lock did not restore the pre-image: %q (%v)", body, err)
	}
	if _, found, err := DetectRecovery(roots); err != nil || found {
		t.Errorf("the journal survived recovery: found=%v err=%v", found, err)
	}
}
