package process

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func syscall_Getpid() int { return syscall.Getpid() }

func TestFlockLifecycle(t *testing.T) {
	path := filepath.Join(testsupport.TempDir(t), liveLockFile)
	f, err := acquireFlock(path)
	if err != nil {
		t.Fatal(err)
	}
	if held, ans := probeFlock(path); !held || ans != probeLive {
		t.Fatalf("held lock probed %v %v", held, ans)
	}
	if _, err := acquireFlock(path); err == nil {
		t.Fatal("second acquisition of a held lock succeeded")
	} else if fl, _ := AsFailure(err); fl == nil || fl.Class != FailBlocked {
		t.Fatalf("contended class = %v", err)
	}
	f.Close() // kernel releases on close
	if held, ans := probeFlock(path); held || ans != probeAbsent {
		t.Fatalf("released lock probed %v %v", held, ans)
	}
	// A missing lock file is clean absence, not an error.
	if held, ans := probeFlock(filepath.Join(testsupport.TempDir(t), "never")); held || ans != probeAbsent {
		t.Fatalf("missing lock file probed %v %v", held, ans)
	}
}

func TestIdentityConditionsRejectOwnGroup(t *testing.T) {
	// A manifest describing the OBSERVER's own group must never pass —
	// clause 5 exists so stop cannot signal itself.
	self := syscall_Getpid()
	pgid, _ := getPGID(self)
	sid, _ := getSID(self)
	dir := testsupport.TempDir(t)
	f, err := acquireFlock(filepath.Join(dir, liveLockFile))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m := &manifestRecord{Schema: recordSchema, RunID: "aa", Token: "bb", RunDir: dir,
		SupervisorPID: self, PGID: pgid, SID: sid}
	if err := identityConditions(m, pgid); err == nil {
		t.Fatal("observer's own group passed the conditions")
	}
}

// TestProbeLockTypedAnswers (change 0494): ProbeLock is the exported, typed,
// never-creating, never-waiting probe the publish journal classifier uses.
func TestProbeLockTypedAnswers(t *testing.T) {
	if LockProbe(0) != LockProbeUnknown {
		t.Fatal("the zero LockProbe must be LockProbeUnknown, so a default is never free")
	}
	dir := testsupport.TempDir(t)
	path := filepath.Join(dir, "publish-x.lock")

	if got := ProbeLock(path); got != LockProbeMissing {
		t.Fatalf("missing file probed %v, want LockProbeMissing", got)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ProbeLock must never create the lock file, stat err = %v", err)
	}

	f, busy, err := TryExclusiveLock(path)
	if err != nil || busy {
		t.Fatalf("TryExclusiveLock: busy=%v err=%v", busy, err)
	}
	if got := ProbeLock(path); got != LockProbeHeld {
		t.Fatalf("held lock probed %v, want LockProbeHeld", got)
	}
	f.Close() // the kernel releases on close
	if got := ProbeLock(path); got != LockProbeFree {
		t.Fatalf("released lock probed %v, want LockProbeFree", got)
	}
	// The probe leaves no hold of its own behind.
	g, busy, err := TryExclusiveLock(path)
	if err != nil || busy {
		t.Fatalf("lock not free after a Free probe: busy=%v err=%v", busy, err)
	}
	g.Close()

	// A directory at the lock path cannot be proven free, held, or missing.
	d := filepath.Join(dir, "is-a-dir.lock")
	if err := os.Mkdir(d, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := ProbeLock(d); got != LockProbeUnknown {
		t.Fatalf("directory probed %v, want LockProbeUnknown", got)
	}
}
