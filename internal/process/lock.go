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

// probeFlock reports whether path's advisory lock is currently held by a live
// holder, plus the three-way answer. It tries LOCK_EX|LOCK_NB on a fresh
// descriptor: acquiring proves no holder (supervisor gone cleanly), so it
// immediately unlocks and closes, leaving no lock of its own -> (false,
// probeAbsent); EWOULDBLOCK proves a live holder -> (true, probeLive); a
// missing file is clean absence -> (false, probeAbsent); any other error is
// unknown, never mistaken for absence -> (false, probeUnknown).
func probeFlock(path string) (held bool, answer probeAnswer) {
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, probeAbsent
		}
		return false, probeUnknown
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return true, probeLive
		}
		return false, probeUnknown
	}
	// Acquired it: there is no live holder. Release immediately so this probe
	// leaves the lock exactly as it found it.
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false, probeAbsent
}

// identityConjunction proves clauses 3-5 of the spec's ownership
// conjunction for a live run: lock held by a live supervisor whose pid is
// >1 and still equals its live pgid and sid, and whose group is not the
// observer's own. Any unprovable read is FailBlocked — never treated as
// absence, never permission to signal.
func identityConjunction(m *manifestRecord, selfPGID int) error {
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
