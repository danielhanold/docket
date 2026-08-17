package process

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// startSleeper starts a real child that blocks until killed; returns pid.
func startSleeper(t *testing.T) (*exec.Cmd, int) {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "300")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	return cmd, cmd.Process.Pid
}

func TestProcessAliveThreeWay(t *testing.T) {
	cmd, pid := startSleeper(t)
	if got := processAlive(pid); got != probeLive {
		t.Fatalf("live pid probed %v", got)
	}
	cmd.Process.Kill()
	cmd.Wait() // reap: an unreaped zombie still probes live
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) != probeAbsent {
		if time.Now().After(deadline) {
			t.Fatalf("reaped pid never probed absent")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// pid 1 belongs to root: kill(1, 0) is EPERM for a non-root test run —
	// the canonical "unknown, not absent" case.
	if got := processAlive(1); got == probeAbsent {
		t.Fatalf("EPERM collapsed into clean absence")
	}
}

func TestGetSIDAndPGIDReadLiveFacts(t *testing.T) {
	_, pid := startSleeper(t)
	pgid, ans := getPGID(pid)
	if ans != probeLive || pgid <= 0 {
		t.Fatalf("getPGID: %d %v", pgid, ans)
	}
	sid, ans := getSID(pid)
	if ans != probeLive || sid <= 0 {
		t.Fatalf("getSID: %d %v", sid, ans)
	}
}

// TestLivenessPrimitivesFailClosedOnNonRealPGID pins the fail-closed guard on
// non-real group ids. Before the guard, groupAlive(0) issued kill(-0, 0) ==
// kill(0, 0), which addresses the caller's OWN process group and returns nil ->
// probeLive, so a leaked PGID:0 slot falsely probed live. pgid 1 is init.
func TestLivenessPrimitivesFailClosedOnNonRealPGID(t *testing.T) {
	for _, pgid := range []int{0, 1} {
		if got := groupAlive(pgid); got != probeUnknown {
			t.Fatalf("groupAlive(%d) = %v, want probeUnknown (must never probe live/absent)", pgid, got)
		}
	}
	// getPGID must not hand back a live/absent answer for a non-real input pid
	// (0 is "self" to Getpgid; 1 is init).
	for _, pid := range []int{0, 1} {
		if _, got := getPGID(pid); got != probeUnknown {
			t.Fatalf("getPGID(%d) = %v, want probeUnknown", pid, got)
		}
	}
	// signalGroup must refuse a non-real group with a typed failure and deliver
	// nothing, rather than SIGKILLing the caller's own group.
	err := signalGroup(0, syscall.SIGKILL)
	if err == nil {
		t.Fatal("signalGroup(0, SIGKILL) returned nil — would signal the caller's own group")
	}
	f, ok := AsFailure(err)
	if !ok || f.Class != FailInvalidState {
		t.Fatalf("signalGroup(0, …) error = %v (%T), want *Failure class %q", err, err, FailInvalidState)
	}
}

func TestSessionAttrsCreateNewSession(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "300")
	cmd.SysProcAttr = sessionAttrs()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	pid := cmd.Process.Pid
	pgid, _ := getPGID(pid)
	sid, _ := getSID(pid)
	if pgid != pid || sid != pid {
		t.Fatalf("want pid==pgid==sid, got pid=%d pgid=%d sid=%d", pid, pgid, sid)
	}
	self, _ := syscall.Getpgid(0)
	if pgid == self {
		t.Fatalf("child shares the test's own group")
	}
}
