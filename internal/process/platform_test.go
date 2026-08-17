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
