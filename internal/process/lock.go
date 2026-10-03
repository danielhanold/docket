package process

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// acquireFlock opens (creating if needed) path and takes an exclusive,
// non-blocking kernel advisory lock (flock) on it, enforcing 0600 with an
// explicit chmod because the create-time mode is umask-masked. The returned
// *os.File owns the lock for its lifetime; closing it releases the lock. A
// lock already held by any other open descriptor returns FailBlocked without
// blocking.
func acquireFlock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, failf(FailExternal, "acquire-lock", "opening %s: %v", filepath.Base(path), err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, failf(FailExternal, "acquire-lock", "chmod %s: %v", filepath.Base(path), err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, failf(FailBlocked, "acquire-lock", "lock held")
		}
		return nil, failf(FailExternal, "acquire-lock", "flock %s: %v", filepath.Base(path), err)
	}
	return f, nil
}

// TryExclusiveLock is the exported non-blocking acquire the worktree lock uses
// (change 0490). It is acquireFlock with the busy case lifted into a typed
// result: busy is true (and f nil, err nil) exactly when another open file
// description holds the lock; every other failure is an error and is never
// read as either "free" or "busy". The returned file owns the lock; only
// closing it (or the process exiting) releases it — callers never LOCK_UN it.
func TryExclusiveLock(path string) (f *os.File, busy bool, err error) {
	f, err = acquireFlock(path)
	if err == nil {
		return f, false, nil
	}
	if fl, ok := AsFailure(err); ok && fl.Class == FailBlocked {
		return nil, true, nil
	}
	return nil, false, err
}

// LockProbe is ProbeLock's typed answer (change 0494). The zero value is
// LockProbeUnknown, so an unset or defaulted answer is never read as free.
type LockProbe int

const (
	// LockProbeUnknown: the probe could not decide (an open or flock error other
	// than not-exist / would-block). Never read as free and never as missing.
	LockProbeUnknown LockProbe = iota
	// LockProbeHeld: another open file description holds the lock right now.
	LockProbeHeld
	// LockProbeFree: the lock file exists and nobody holds it.
	LockProbeFree
	// LockProbeMissing: no file exists at the path.
	LockProbeMissing
)

// ProbeLock reports, without creating the file and without waiting, whether the
// advisory lock at path is held. It opens WITHOUT O_CREATE and tries
// LOCK_EX|LOCK_NB on a fresh descriptor. Acquiring proves no holder; the probe
// then releases by closing (never LOCK_UN), leaving the lock exactly as it found
// it. EWOULDBLOCK proves a live holder. A missing file is LockProbeMissing, and
// any other error is LockProbeUnknown.
func ProbeLock(path string) LockProbe {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return LockProbeMissing
		}
		return LockProbeUnknown
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return LockProbeHeld
		}
		return LockProbeUnknown
	}
	return LockProbeFree
}

// probeFlock reports whether path's advisory lock is currently held by a live
// holder, plus the three-way answer. It tries LOCK_EX|LOCK_NB on a fresh
// descriptor: acquiring proves no holder (supervisor gone cleanly), so it
// immediately closes, leaving no lock of its own -> (false,
// probeAbsent); EWOULDBLOCK proves a live holder -> (true, probeLive); a
// missing file is clean absence -> (false, probeAbsent); any other error is
// unknown, never mistaken for absence -> (false, probeUnknown).
func probeFlock(path string) (held bool, answer probeAnswer) {
	switch ProbeLock(path) {
	case LockProbeHeld:
		return true, probeLive
	case LockProbeFree, LockProbeMissing:
		return false, probeAbsent
	default:
		return false, probeUnknown
	}
}

// identityConditions proves clauses 3-5 of the spec's ownership
// conditions for a live run: lock held by a live supervisor whose pid is
// >1 and still equals its live pgid and sid, and whose group is not the
// observer's own. Any unprovable read is FailBlocked — never treated as
// absence, never permission to signal.
func identityConditions(m *manifestRecord, selfPGID int) error {
	held, ans := probeFlock(filepath.Join(m.RunDir, liveLockFile))
	if ans == probeUnknown {
		return failf(FailBlocked, "identity", "live lock unprobeable")
	}
	if !held {
		return failf(FailBlocked, "identity", "live lock not held")
	}
	if m.SupervisorPID <= 1 {
		return failf(FailBlocked, "identity", "recorded pid %d is not a valid supervisor", m.SupervisorPID)
	}
	pgid, pans := getPGID(m.SupervisorPID)
	sid, sans := getSID(m.SupervisorPID)
	if pans != probeLive || sans != probeLive {
		return failf(FailBlocked, "identity", "supervisor process facts unprovable")
	}
	if pgid != m.PGID || sid != m.SID || m.PGID != m.SupervisorPID || m.SID != m.SupervisorPID {
		return failf(FailBlocked, "identity", "recorded identity no longer matches live process facts")
	}
	if m.PGID == selfPGID {
		return failf(FailBlocked, "identity", "recorded group is the observer's own")
	}
	return nil
}
