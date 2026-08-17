package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewService(exe)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func launchHelper(t *testing.T, svc *Service, root string, mode string, extra ...string) *LaunchOutcome {
	t.Helper()
	out, err := svc.Launch(LaunchRequest{Root: root, Cwd: t.TempDir(), Argv: helperArgv(t, mode, extra...)})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	return out
}

// waitTerminalState polls the durable terminal record (Observe/Stop belong to
// later tasks) and maps it through terminalState. Correctness rests on the
// terminal.json transition, never on the interval.
func waitTerminalState(t *testing.T, runDir string, stopIntent bool) State {
	t.Helper()
	var st State
	waitFor(t, "terminal record", 30*time.Second, func() bool {
		term, err := readTerminal(runDir)
		if err != nil {
			t.Fatalf("readTerminal: %v", err)
		}
		if term == nil {
			return false
		}
		st = terminalState(term, stopIntent)
		return true
	})
	return st
}

// killRun tears a live run down for test hygiene: re-read the manifest and
// SIGKILL the whole recorded group (Stop belongs to Task 9).
func killRun(t *testing.T, runDir string) {
	t.Helper()
	m, err := readManifest(runDir)
	if err != nil || m == nil || m.PGID <= 1 {
		return
	}
	_ = signalGroup(m.PGID, syscall.SIGKILL)
}

// TestLaunchEstablishesAddressableSession — the handshake returns only
// after a live pid==pgid==sid supervisor exists, distinct from ours.
func TestLaunchEstablishesAddressableSession(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "sleep")
	if out.State != StateRunning {
		t.Fatalf("state = %v", out.State)
	}
	m, err := readManifest(out.RunDir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Phase != "running" {
		t.Fatalf("phase = %q", m.Phase)
	}
	if m.SupervisorPID != m.PGID || m.PGID != m.SID {
		t.Fatalf("not a session leader: %+v", m)
	}
	self, _ := syscall.Getpgid(0)
	if m.PGID == self {
		t.Fatalf("supervisor in the launcher's own group")
	}
	// Live facts agree with the record.
	if err := identityConjunction(m, self); err != nil {
		t.Fatalf("conjunction on a live run: %v", err)
	}
	// Modes: run dir 0700, records 0600.
	di, _ := os.Stat(out.RunDir)
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("run dir mode %o", di.Mode().Perm())
	}
	// Teardown for hygiene.
	killRun(t, out.RunDir)
}

// TestGateSurvivesLauncherExit — the launcher is a REAL separate process that
// exits the moment it has published the run. Its process group is gone, yet
// the gate (its own Setsid session) must still be live and addressable.
func TestGateSurvivesLauncherExit(t *testing.T) {
	root := t.TempDir()
	exe, _ := os.Executable()
	cmd := exec.Command(exe, "gate-test-helper", "launch", root)
	outBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("launcher subprocess: %v", err)
	}
	runDir := strings.TrimSpace(string(outBytes))
	// The launcher subprocess has fully exited (cmd.Output returned and reaped
	// it); its process group is gone. The gate must still be live.
	m, err := readManifest(runDir)
	if err != nil || m == nil {
		t.Fatalf("manifest after launcher death: %v %v", m, err)
	}
	if m.Phase != "running" {
		t.Fatalf("gate did not survive the launcher: phase = %q", m.Phase)
	}
	if term, err := readTerminal(runDir); err != nil || term != nil {
		t.Fatalf("gate already terminal after launcher exit: %v %v", term, err)
	}
	if got := groupAlive(m.PGID); got != probeLive {
		t.Fatalf("supervised group not live after launcher exit: %v", got)
	}
	self, _ := syscall.Getpgid(0)
	if err := identityConjunction(m, self); err != nil {
		t.Fatalf("survived run is not addressable: %v", err)
	}
	killRun(t, runDir)
}

// TestLaunchStreamsAndStdin — stdout/stderr byte-exact separate durable
// files; child stdin is at EOF.
func TestLaunchStreamsAndStdin(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "emit", "OUT-BYTES", "ERR-BYTES")
	if obs := observeUntilTerminal(t, svc, out.RunDir); obs.State != StatePassed {
		t.Fatalf("emit helper: %v", obs.State)
	}
	so, _ := os.ReadFile(filepath.Join(out.RunDir, stdoutLogFile))
	se, _ := os.ReadFile(filepath.Join(out.RunDir, stderrLogFile))
	if string(so) != "OUT-BYTES" || string(se) != "ERR-BYTES" {
		t.Fatalf("streams not byte-exact/separate: out=%q err=%q", so, se)
	}
	fi, _ := os.Stat(filepath.Join(out.RunDir, stdoutLogFile))
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("stdout.log mode %o", fi.Mode().Perm())
	}
	out2 := launchHelper(t, svc, t.TempDir(), "read-stdin")
	if obs := observeUntilTerminal(t, svc, out2.RunDir); obs.State != StatePassed {
		t.Fatalf("stdin not closed: %v", obs.State)
	}
}

// TestLaunchStripsSupervisorEnv — the child sees neither private env var and
// the inherited lock fd is closed (CLOEXEC).
func TestLaunchStripsSupervisorEnv(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "env-check")
	if st := waitTerminalState(t, out.RunDir, false); st != StatePassed {
		t.Fatalf("supervisor leaked env or fds into the child: %v", st)
	}
}

// TestLaunchFastExit — a command that finishes inside the establishment
// window returns its exact terminal state from Launch itself, or converges to
// it under observation.
func TestLaunchFastExit(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	out, err := svc.Launch(LaunchRequest{Root: root, Cwd: t.TempDir(), Argv: helperArgv(t, "exit", "0")})
	if err != nil {
		t.Fatal(err)
	}
	if out.State == StateRunning {
		// Racing running->terminal is legal; observe must converge.
		if obs := observeUntilTerminal(t, svc, out.RunDir); obs.State != StatePassed {
			t.Fatalf("fast exit converged to %v", obs.State)
		}
	} else if out.State != StatePassed {
		t.Fatalf("fast exit state %v", out.State)
	}
}

func TestLaunchRejectsBeforeCreating(t *testing.T) {
	svc := newTestService(t)
	root := filepath.Join(t.TempDir(), "root-not-yet")
	_, err := svc.Launch(LaunchRequest{Root: root, Cwd: "relative", Argv: helperArgv(t, "sleep")})
	if err == nil {
		t.Fatal("invalid cwd accepted")
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Fatalf("validation failure created the root anyway")
	}
}
