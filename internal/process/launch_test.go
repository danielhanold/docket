package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
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
	out, err := svc.Launch(LaunchRequest{Root: root, Cwd: testsupport.TempDir(t), Argv: helperArgv(t, mode, extra...)})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	reapSupervisor(out.RunDir)
	// Wait for the detached supervisor to quiesce before this test's fixture
	// dirs are removed out from under it — see quiesceRun. Registered as a
	// testsupport.DrainOnCleanup drain, so the fixture's drain-before-removal
	// contract runs this ahead of any removeAllTolerant pass — the ordering no
	// longer rests on t.Cleanup LIFO reasoning.
	testsupport.DrainOnCleanup(t, func() { quiesceRun(t, out.RunDir) })
	return out
}

// quiesceRun waits for a launched run's detached supervisor to finish every
// same-directory write and release live.lock, so a test can mutate the run dir
// or let testsupport.TempDir(t) teardown remove it with no race against the supervisor.
//
// The supervisor keeps writing manifest/terminal/log records after a fast child
// exits (supervisor.go step 8: terminal.json, then the phase="terminal" manifest
// write, then closeLock LAST). Folded under -race those trailing writes race
// t.TempDir's RemoveAll ("directory not empty" from a just-created atomic temp
// file) and clobber a test's own post-terminal manifest mutation. Because
// closeLock is the supervisor's final act, a free live.lock proves the run dir
// is quiescent — no further writes will land.
//
// A still-HELD lock proves the supervisor is alive and unreaped, so its recorded
// pid is not yet reused: signalling its group can never hit a reused pid. That is
// used only to bound teardown for a run whose child is still alive (no terminal
// and no failure record yet — the supervisor is parked in cmd.Wait, not
// mid-write); a run that has already recorded a verdict is merely finishing its
// post-terminal writes and is waited out, never signalled (a SIGKILL mid-write
// could strand an atomic temp file, which is the very race this closes).
// Best-effort under a generous deadline; a test-support helper only. The
// bounded poll loop is testsupport.WaitQuiesced (30s deadline, 10ms step);
// the domain probe below stays here because it reads unexported records.
func quiesceRun(t *testing.T, runDir string) {
	t.Helper()
	lockPath := filepath.Join(runDir, liveLockFile)
	killed := false
	testsupport.WaitQuiesced(30*time.Second, 10*time.Millisecond, func() bool {
		held, ans := probeFlock(lockPath)
		if !held && ans == probeAbsent {
			return true // supervisor gone; the run dir will take no further writes
		}
		if held && !killed {
			term, _ := readTerminal(runDir)
			fr, _ := readFailureRecord(runDir)
			if term == nil && fr == nil {
				// Child still alive, supervisor parked in Wait (idle, not
				// mid-write): end the group so teardown is bounded.
				if m, err := readManifest(runDir); err == nil && m != nil && m.PGID > 1 {
					_ = signalGroup(m.PGID, syscall.SIGKILL)
					killed = true
				}
			}
		}
		return false
	})
}

// reapSupervisor emulates init reaping the orphaned supervisor. In production
// the launcher process exits right after Launch returns, so the supervisor is
// reparented to init and reaped when it dies — its process group then becomes
// provably absent (groupAlive -> probeAbsent) once torn down, which is what
// Stop's teardown verification (spec: verified group absence) rests on. The
// in-process test harness instead keeps the supervisor as a waitable child of
// the long-lived test process; with no reaper it lingers as a zombie group
// leader after death, so kill(-pgid, 0) returns EPERM (probeUnknown) forever
// and group absence is never observable. A per-supervisor blocking Wait4 —
// targeted at the exact pid, never -1, so it can never steal another child's
// wait status — plays init's role for exactly the supervisors this harness
// spawns.
func reapSupervisor(runDir string) {
	m, err := readManifest(runDir)
	if err != nil || m == nil || m.SupervisorPID <= 1 {
		return
	}
	pid := m.SupervisorPID
	go func() {
		var ws syscall.WaitStatus
		for {
			wpid, werr := syscall.Wait4(pid, &ws, 0, nil)
			if werr != syscall.EINTR || wpid == pid {
				return
			}
		}
	}()
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
	out := launchHelper(t, svc, testsupport.TempDir(t), "sleep")
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
	root := testsupport.TempDir(t)
	exe, _ := os.Executable()
	cmd := exec.Command(exe, "gate-test-helper", "launch", root)
	outBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("launcher subprocess: %v", err)
	}
	runDir := strings.TrimSpace(string(outBytes))
	testsupport.DrainOnCleanup(t, func() { quiesceRun(t, runDir) })
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
	out := launchHelper(t, svc, testsupport.TempDir(t), "emit", "OUT-BYTES", "ERR-BYTES")
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
	out2 := launchHelper(t, svc, testsupport.TempDir(t), "read-stdin")
	if obs := observeUntilTerminal(t, svc, out2.RunDir); obs.State != StatePassed {
		t.Fatalf("stdin not closed: %v", obs.State)
	}
}

// TestLaunchStripsSupervisorEnv — the child sees neither private env var and
// the inherited lock fd is closed (CLOEXEC).
func TestLaunchStripsSupervisorEnv(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, testsupport.TempDir(t), "env-check")
	if st := waitTerminalState(t, out.RunDir, false); st != StatePassed {
		t.Fatalf("supervisor leaked env or fds into the child: %v", st)
	}
}

// TestLaunchFastExit — a command that finishes inside the establishment
// window returns its exact terminal state from Launch itself, or converges to
// it under observation.
func TestLaunchFastExit(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	out, err := svc.Launch(LaunchRequest{Root: root, Cwd: testsupport.TempDir(t), Argv: helperArgv(t, "exit", "0")})
	if err != nil {
		t.Fatal(err)
	}
	testsupport.DrainOnCleanup(t, func() { quiesceRun(t, out.RunDir) })
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
	root := filepath.Join(testsupport.TempDir(t), "root-not-yet")
	_, err := svc.Launch(LaunchRequest{Root: root, Cwd: "relative", Argv: helperArgv(t, "sleep")})
	if err == nil {
		t.Fatal("invalid cwd accepted")
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Fatalf("validation failure created the root anyway")
	}
}
