package process

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func syscall_SIGKILL() syscall.Signal { return syscall.SIGKILL }

// observeUntilTerminal polls Observe under a generous outer deadline until the
// run leaves StateRunning. Correctness rests on the state transition, never on
// the interval.
func observeUntilTerminal(t *testing.T, svc *Service, runDir string) *Observation {
	t.Helper()
	var obs *Observation
	waitFor(t, "terminal state", 30*time.Second, func() bool {
		o, err := svc.Observe(runDir)
		if err != nil {
			t.Fatalf("Observe: %v", err)
		}
		obs = o
		return o.State != StateRunning
	})
	// A terminal state is decided by terminal.json, which the supervisor writes
	// BEFORE its final phase="terminal" manifest write and lock release. Wait for
	// the supervisor to fully quiesce so a caller that then mutates the run dir
	// (e.g. rewriting the manifest run id) is not clobbered by that trailing
	// write — see quiesceRun.
	quiesceRun(t, runDir)
	return obs
}

func TestObserveRunningThenTerminal(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "sleep")
	obs, err := svc.Observe(out.RunDir)
	if err != nil || obs.State != StateRunning {
		t.Fatalf("running observe: %+v, %v", obs, err)
	}
	if obs.StdoutLog == "" || obs.StderrLog == "" {
		t.Fatalf("log paths missing from observation")
	}
	m, _ := readManifest(out.RunDir)
	signalGroup(m.PGID, syscall_SIGKILL())
	// Supervisor dies with the child under KILL: no terminal record can
	// exist, no stop intent was recorded -> vanished.
	waitFor(t, "vanished", 30*time.Second, func() bool {
		o, oerr := svc.Observe(out.RunDir)
		if oerr != nil {
			return false // lock release can race; keep polling
		}
		return o.State == StateVanished
	})
}

func TestObserveExitStates(t *testing.T) {
	svc := newTestService(t)
	for code, want := range map[int]State{0: StatePassed, 7: StateFailed, 143: StateFailed} {
		out := launchHelper(t, svc, t.TempDir(), "exit", strconv.Itoa(code))
		obs := observeUntilTerminal(t, svc, out.RunDir)
		if obs.State != want {
			t.Fatalf("exit %d observed %v, want %v", code, obs.State, want)
		}
		if want == StateFailed && (obs.Terminal == nil || obs.Terminal.ExitCode != code) {
			t.Fatalf("exact code lost: %+v", obs.Terminal)
		}
	}
}

// TestObserveNeverFabricatesFromMalformedState
func TestObserveNeverFabricatesFromMalformedState(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "exit", "0")
	observeUntilTerminal(t, svc, out.RunDir)
	if err := os.WriteFile(filepath.Join(out.RunDir, terminalFile), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Observe(out.RunDir)
	if err == nil {
		t.Fatal("malformed terminal record produced a verdict")
	}
	if f, _ := AsFailure(err); f == nil || f.Class != FailInvalidState {
		t.Fatalf("malformed class = %v", err)
	}
}

// TestObserveTokenAgreement — a manifest whose run_id disagrees with its
// directory is invalid state, not a report about some other run.
func TestObserveTokenAgreement(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "exit", "0")
	observeUntilTerminal(t, svc, out.RunDir)
	m, _ := readManifest(out.RunDir)
	m.RunID = "00000000000000000000000000000000"
	writeAtomicJSON(filepath.Join(out.RunDir, manifestFile), m)
	if _, err := svc.Observe(out.RunDir); err == nil {
		t.Fatal("identity-mismatched manifest accepted")
	}
}

// TestObservePostProbeTerminalRaceWins pins the load-bearing re-read: a run
// that presents as terminal-less with a cleanly free lock, then has a terminal
// record written in the window between the lock probe and the re-read, must be
// reported by its terminal state — never as vanished. The race window is not
// deterministically forceable from outside, so the observePostProbeHook seam
// injects the terminal write at exactly that instant. This test reddens if the
// post-probe re-read of terminal.json is removed.
func TestObservePostProbeTerminalRaceWins(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	runDir := filepath.Join(root, id)
	if err := ensurePrivateDir(runDir); err != nil {
		t.Fatal(err)
	}
	m := &manifestRecord{
		Schema: recordSchema, RunID: id, Token: "bb", Root: root, RunDir: runDir,
		SupervisorPID: 0, PGID: 0, SID: 0, Phase: "terminal",
		Cwd: root, Argv0: "x", Argc: 1, CreatedAt: "t", UpdatedAt: "t",
	}
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), m); err != nil {
		t.Fatal(err)
	}
	// No live.lock file exists, so probeFlock reports cleanly absent. The hook
	// fires once, between that probe and the re-read, writing the racing
	// terminal record.
	observePostProbeHook = func() {
		writeAtomicJSON(filepath.Join(runDir, terminalFile),
			&terminalRecord{Schema: recordSchema, RunID: id, Kind: "exit", ExitCode: 0, RecordedAt: "t"})
	}
	defer func() { observePostProbeHook = nil }()

	obs, err := svc.Observe(runDir)
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if obs.State != StatePassed {
		t.Fatalf("post-probe terminal race lost: state = %v, want passed", obs.State)
	}
}
