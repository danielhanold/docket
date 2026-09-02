package process

import (
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

func TestIdentityConjunctionRejectsOwnGroup(t *testing.T) {
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
	if err := identityConjunction(m, pgid); err == nil {
		t.Fatal("observer's own group passed the conjunction")
	}
}
