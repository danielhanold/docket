package workspace

// This file owns cross-process serialization for workspace state via flock,
// following internal/install/lock.go's reasoning exactly (reference only — this
// package must not import that one). flock's lock lives on the open file
// description, so the kernel drops it when the last descriptor closes, including
// the implicit close of a process that died however it died. A crash therefore
// costs the next run nothing, and liveness is "the lock is held right now", never
// a PID or a timestamp. The lock FILES are deliberately left on disk on release —
// they are mutexes, not flags, and their mere presence means nothing.
//
// Two locks keep first-publication and per-workspace operations exclusive without
// coupling: the registry lock (one per workspaces root) covers only the brief
// first publication of a manifest, and each workspace's own operation lock
// serializes prepare, inspect-state refresh, publication, and cleanup for that
// one workspace. No lock is ever held while an agent builds.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const (
	// registryLockName is the workspaces-root mutex serializing first manifest
	// publication.
	registryLockName = "registry.lock"
	// operationLockName is a workspace's per-directory operation mutex.
	operationLockName = "operation.lock"
)

// errLockHeld is the sentinel a non-blocking acquire returns when another open
// file description already holds the flock. It is a contention signal, not a
// defect.
var errLockHeld = errors.New("workspace: lock is held by another process")

// fileLock is a held flock on an open file description. A nil pointer or a nil
// file both mean "not held".
type fileLock struct{ f *os.File }

// acquireFlock opens (creating if needed) the file at path and takes an exclusive
// flock. When block is false it uses LOCK_NB and maps the would-block error onto
// errLockHeld; when block is true it waits. The lock file's mode is forced to
// workspaceFileMode with an explicit Chmod because O_CREATE's perm is umask-masked.
func acquireFlock(path string, block bool) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, workspaceFileMode)
	if err != nil {
		return nil, fmt.Errorf("workspace: opening lock %s: %w", path, err)
	}
	if err := os.Chmod(path, workspaceFileMode); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("workspace: setting mode on %s: %w", path, err)
	}
	how := syscall.LOCK_EX
	if !block {
		how |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		_ = f.Close()
		if !block && errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%w: %s", errLockHeld, path)
		}
		return nil, fmt.Errorf("workspace: locking %s: %w", path, err)
	}
	return &fileLock{f: f}, nil
}

// release drops the lock and closes its descriptor. Closing alone would release
// the flock, but the explicit unlock states the intent, and niling the file makes
// a double release a no-op rather than acting on a descriptor number the kernel
// may since have reused.
func (l *fileLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
	l.f = nil
}

// acquireRegistryLock takes the workspaces-root registry lock, blocking until it
// is available. The root is created at 0700 first so the lock file has a home.
// The returned release drops the lock; the critical section it guards is only the
// first publication of a manifest, never mutation work.
func acquireRegistryLock(root string) (func(), error) {
	if err := ensureDir(root); err != nil {
		return nil, err
	}
	lk, err := acquireFlock(filepath.Join(root, registryLockName), true)
	if err != nil {
		return nil, err
	}
	return lk.release, nil
}

// acquireOperationLock takes the per-workspace operation lock, blocking until it
// is available. The workspace directory is created at 0700 first. The returned
// release drops the lock. This serializes prepare, inspect-state refresh,
// publication, and cleanup for one workspace.
func acquireOperationLock(dir string) (func(), error) {
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	lk, err := acquireFlock(filepath.Join(dir, operationLockName), true)
	if err != nil {
		return nil, err
	}
	return lk.release, nil
}

// tryOperationLock probes the per-workspace operation lock without blocking. When
// another process holds it, it returns (nil, true, nil). When it is free, it
// acquires it and returns (release, false, nil). A genuine acquisition or probe
// error other than contention is returned as err with held false.
func tryOperationLock(dir string) (func(), bool, error) {
	if err := ensureDir(dir); err != nil {
		return nil, false, err
	}
	lk, err := acquireFlock(filepath.Join(dir, operationLockName), false)
	if err != nil {
		if errors.Is(err, errLockHeld) {
			return nil, true, nil
		}
		return nil, false, err
	}
	return lk.release, false, nil
}
