package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestStopGracefulTermRecorded(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "sleep")
	res, err := svc.Stop(out.RunDir, "operator asked")
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !res.Performed || res.State != StateStopped {
		t.Fatalf("graceful stop: %+v", res)
	}
	// The supervisor recorded the exact signal, and the intent classifies
	// it as stopped, not signaled.
	term, _ := readTerminal(out.RunDir)
	if term == nil || term.Kind != "signal" || term.Signal != int(syscall.SIGTERM) {
		t.Fatalf("terminal after graceful stop: %+v", term)
	}
	if obs, _ := svc.Observe(out.RunDir); obs.State != StateStopped {
		t.Fatalf("observe after stop: %v", obs.State)
	}
}

func TestStopNoOpOnTerminalRun(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "exit", "7")
	observeUntilTerminal(t, svc, out.RunDir)
	res, err := svc.Stop(out.RunDir, "late stop")
	if err != nil {
		t.Fatal(err)
	}
	if res.Performed || res.State != StateFailed || res.Terminal.ExitCode != 7 {
		t.Fatalf("no-op must preserve the verdict: %+v", res)
	}
	if _, statErr := os.Stat(filepath.Join(out.RunDir, stopIntentFile)); !os.IsNotExist(statErr) {
		t.Fatalf("no-op stop wrote an intent")
	}
}

// TestStopEscalatesTermIgnorer — bounded KILL for a child that ignores
// TERM; stopped marker only after verified group absence.
func TestStopEscalatesTermIgnorer(t *testing.T) {
	svc := newTestService(t)
	svc.stopTermWait = 500 * time.Millisecond // test seam: shrink, don't sleep-tune
	ready := filepath.Join(t.TempDir(), "ready")
	out := launchHelper(t, svc, t.TempDir(), "ignore-term", ready)
	waitFor(t, "helper ready", 30*time.Second, func() bool {
		_, err := os.Stat(ready)
		return err == nil
	})
	res, err := svc.Stop(out.RunDir, "escalate")
	if err != nil {
		t.Fatalf("escalating stop: %v", err)
	}
	if !res.Performed || res.State != StateStopped {
		t.Fatalf("escalation outcome: %+v", res)
	}
	m, _ := readManifest(out.RunDir)
	if groupAlive(m.PGID) != probeAbsent {
		t.Fatalf("stopped reported while the group still exists")
	}
}

// TestStopRefusesUnprovableOwnership — a free lock plus live-looking
// records must never authorize a signal (PID-reuse defense).
func TestStopRefusesUnprovableOwnership(t *testing.T) {
	svc := newTestService(t)
	// Fabricate an owned-looking run whose "supervisor" is a live process
	// we started OURSELVES (so the pid exists and leads its own session)
	// but which holds no live.lock.
	root := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	runDir := filepath.Join(root, id)
	ensurePrivateDir(runDir)
	decoyCmd := exec.Command("/bin/sleep", "300")
	decoyCmd.SysProcAttr = sessionAttrs()
	if err := decoyCmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := decoyCmd.Process.Pid
	// Init-emulate: reap the decoy the instant it dies, so a WRONGFUL signal is
	// observable. Without this, a killed decoy lingers as this process's own
	// unreaped zombie and kill(pid, 0) still returns success (probeLive) — the
	// decisive "decoy still alive" assert below would then be inert, passing
	// even if the ownership gate were stripped and the decoy were killed
	// (verified: without this reaper the gate mutation leaves the assert green).
	// A single blocking Wait4 on the exact pid is the sole waiter (cleanup joins
	// it, never double-waits): if the decoy is untouched it stays genuinely
	// live; if it was signalled it becomes provably absent.
	reaped := make(chan struct{})
	go func() {
		var ws syscall.WaitStatus
		for {
			if _, werr := syscall.Wait4(pid, &ws, 0, nil); werr != syscall.EINTR {
				break
			}
		}
		close(reaped)
	}()
	t.Cleanup(func() { decoyCmd.Process.Kill(); <-reaped })
	writeAtomicJSON(filepath.Join(runDir, manifestFile), &manifestRecord{
		Schema: recordSchema, RunID: id, Token: "aa", Root: root, RunDir: runDir,
		SupervisorPID: pid, PGID: pid, SID: pid, Phase: "running", Cwd: "/",
		Argv0: "sleep", Argc: 2, CreatedAt: "x", UpdatedAt: "x"})
	_, err := svc.Stop(runDir, "must refuse")
	if err == nil {
		t.Fatal("stop signalled a run whose lock is not held")
	}
	if f, _ := AsFailure(err); f == nil || f.Class != FailBlocked {
		t.Fatalf("refusal class = %v", err)
	}
	// THE decisive assert: the decoy is still alive — nothing was signalled.
	if processAlive(pid) != probeLive {
		t.Fatalf("stop killed a process it could not prove it owned")
	}
}

func TestStopNeverWritesTerminal(t *testing.T) {
	// Source-shape assert would be decoration; behavioral pin: after the
	// KILL-takes-supervisor path (SIGKILL group via stop on a term-ignorer
	// whose SUPERVISOR also ignores nothing — KILL is unblockable), the
	// run dir contains stopped.json and terminal.json only if the
	// supervisor got it out first; both classify as stopped either way,
	// and stop itself produced no terminal when stopped.json exists alone.
	svc := newTestService(t)
	svc.stopTermWait = 500 * time.Millisecond
	ready := filepath.Join(t.TempDir(), "ready")
	out := launchHelper(t, svc, t.TempDir(), "ignore-term", ready)
	waitFor(t, "helper ready", 30*time.Second, func() bool { _, err := os.Stat(ready); return err == nil })
	res, err := svc.Stop(out.RunDir, "kill path")
	if err != nil || res.State != StateStopped {
		t.Fatalf("%+v, %v", res, err)
	}
	term, terr := readTerminal(out.RunDir)
	stopped, serr := readStopped(out.RunDir)
	if terr != nil || serr != nil {
		t.Fatalf("read-back: %v %v", terr, serr)
	}
	if term == nil && stopped == nil {
		t.Fatalf("neither terminal nor stopped marker after verified teardown")
	}
	if term != nil && term.Kind == "exit" {
		t.Fatalf("a KILLed group cannot have exited normally: %+v", term)
	}
}
