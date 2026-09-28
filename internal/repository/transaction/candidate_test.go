package transaction

import (
	"errors"
	"github.com/danielhanold/docket/internal/testsupport"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// fixedBase is an arbitrary well-formed base commit id stored in the manifest.
// candidate.go treats it as opaque, so the exact value only has to round-trip.
const fixedBase gitcli.ObjectID = "0123456789abcdef0123456789abcdef01234567"

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
