package suiterunner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// pidAlive reports whether pid still exists (signal 0 probes without delivering).
// ESRCH means gone; any other outcome (nil, EPERM) means the process — or its
// unreaped zombie — is still present. These are the test's own descendants, so
// EPERM does not arise in practice.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return !errors.Is(err, syscall.ESRCH)
}

// waitForFile blocks until path exists or the deadline elapses, polling at a
// fixed small interval. It is a readiness gate, never a verdict: the assertions
// that follow are what pass or fail the test.
func waitForFile(t *testing.T, path string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", within, path)
}

// waitPidGone deadline-polls until pid is gone, failing the test if it survives.
func waitPidGone(t *testing.T, label string, pid int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s (pid %d) still alive after %s — signal did not reach it", label, pid, within)
}

func readPidFile(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pid file %s: %v", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("parse pid %q from %s: %v", b, path, err)
	}
	return pid
}

// TestSignalReachesProcessGroupIncludingGrandchildren is the named mutation
// guard for pgid-vs-pid forwarding. The target backgrounds a grandchild that
// shares the target's process group but has its own pid. The handler forwards
// SIGTERM to the registered process GROUP (kill(-pgid)); a mutation to
// kill(+pgid) would signal only the group leader, leaving the grandchild alive
// and this test red.
func TestSignalReachesProcessGroupIncludingGrandchildren(t *testing.T) {
	scripts := testsupport.TempDir(t)
	work := testsupport.TempDir(t)
	pids := testsupport.TempDir(t)

	// Grandchild (a plain background sleep, its own pid) writes grand.pid; the
	// target (group leader) writes self.pid, signals readiness, then blocks. Both
	// live in one process group. grand.pid is the mutation-sensitive process:
	// only kill(-pgid) reaches it.
	body := strings.Join([]string{
		`sleep 30 &`,
		`echo $! > "` + filepath.Join(pids, "grand.pid") + `"`,
		`echo $$ > "` + filepath.Join(pids, "self.pid") + `"`,
		`: > "` + filepath.Join(pids, "ready") + `"`,
		`sleep 30`,
	}, "\n") + "\n"
	tgt := writeScript(t, scripts, "grp", body)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg := newProcRegistry()
	// killAfter high so the FIRST forwarded signal — not the SIGKILL escalation —
	// is what must reach the whole group for this test to pass.
	fired, stop := InstallSignalHandling(cancel, reg, 5*time.Second)
	defer stop()

	done := make(chan struct{})
	go func() {
		_, _ = ExecuteTarget(ctx, bashPath(t), tgt, work, reg, nil)
		close(done)
	}()

	waitForFile(t, filepath.Join(pids, "ready"), 5*time.Second)
	waitForFile(t, filepath.Join(pids, "grand.pid"), 5*time.Second)
	self := readPidFile(t, filepath.Join(pids, "self.pid"))
	grand := readPidFile(t, filepath.Join(pids, "grand.pid"))

	// Drive the handler's real forwarding path by signalling this process.
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("self-signal SIGTERM: %v", err)
	}

	waitPidGone(t, "group leader", self, 5*time.Second)
	waitPidGone(t, "grandchild", grand, 5*time.Second)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteTarget did not return after its process group was killed")
	}
	if sig, ok := fired(); !ok || sig != syscall.SIGTERM {
		t.Fatalf("fired() = (%v, %v), want (SIGTERM, true)", sig, ok)
	}
}

// TestEscalationKillsIgnorers proves the bounded SIGKILL escalation: a target
// that traps and ignores SIGTERM still dies, via the escalation timer, and the
// run does not hang for the full grace default.
func TestEscalationKillsIgnorers(t *testing.T) {
	scripts := testsupport.TempDir(t)
	work := testsupport.TempDir(t)
	pids := testsupport.TempDir(t)

	body := strings.Join([]string{
		`trap '' TERM`,
		`echo $$ > "` + filepath.Join(pids, "self.pid") + `"`,
		`: > "` + filepath.Join(pids, "ready") + `"`,
		`sleep 30`,
	}, "\n") + "\n"
	tgt := writeScript(t, scripts, "ignore", body)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg := newProcRegistry()
	fired, stop := InstallSignalHandling(cancel, reg, 300*time.Millisecond)
	defer stop()

	done := make(chan struct{})
	go func() {
		_, _ = ExecuteTarget(ctx, bashPath(t), tgt, work, reg, nil)
		close(done)
	}()

	waitForFile(t, filepath.Join(pids, "ready"), 5*time.Second)
	self := readPidFile(t, filepath.Join(pids, "self.pid"))

	overall := time.Now()
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("self-signal SIGTERM: %v", err)
	}

	// TERM is trapped/ignored; only the escalation SIGKILL can reap it.
	waitPidGone(t, "signal-ignoring target", self, 4*time.Second)
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("ExecuteTarget did not return after escalation")
	}
	if elapsed := time.Since(overall); elapsed > 4*time.Second {
		t.Fatalf("escalation took %s — expected well under the default grace", elapsed)
	}
	if sig, ok := fired(); !ok || sig != syscall.SIGTERM {
		t.Fatalf("fired() = (%v, %v), want (SIGTERM, true)", sig, ok)
	}
}

// TestInterruptedRunCannotPass composes the full interruption lifecycle the way
// Task 7's Run wires it: install handling, schedule, run the lanes. Target 2
// signals the runner mid-run. The already-durable result of target 1 survives,
// target 3 never launches, fired() reports SIGTERM, and the derived exit code is
// 143 — never 0. (Run itself, which reads these same values, is built in Task 7.)
func TestInterruptedRunCannotPass(t *testing.T) {
	scripts := testsupport.TempDir(t)
	work := testsupport.TempDir(t)
	gate := testsupport.TempDir(t)

	// Byte-order names fix the launch order under Jobs=1: aaa, then bbb, then ccc.
	t1 := writeScript(t, scripts, "aaa", "echo 'ok - one'\n")
	t2 := writeScript(t, scripts, "bbb", strings.Join([]string{
		`: > "` + filepath.Join(gate, "t2ready") + `"`,
		`kill -TERM "$RUNNER_PID"`,
		`sleep 30`,
	}, "\n")+"\n")
	t3 := writeScript(t, scripts, "ccc", "echo 'ok - three'\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg := newProcRegistry()
	fired, stop := InstallSignalHandling(cancel, reg, 3*time.Second)
	defer stop()

	cfg := Config{
		Bash:     bashPath(t),
		Jobs:     1,
		Work:     work,
		ExtraEnv: []string{"RUNNER_PID=" + strconv.Itoa(os.Getpid())},
	}
	par, ser := Schedule([]Target{t1, t2, t3})

	unlaunchedCh := make(chan []Target, 1)
	go func() { unlaunchedCh <- runLanes(ctx, cfg, par, ser, reg, nil) }()

	var unlaunched []Target
	select {
	case unlaunched = <-unlaunchedCh:
	case <-time.After(10 * time.Second):
		t.Fatal("runLanes did not return after interruption")
	}

	sig, ok := fired()
	if !ok || sig != syscall.SIGTERM {
		t.Fatalf("fired() = (%v, %v), want (SIGTERM, true)", sig, ok)
	}
	if code := InterruptExitCode(sig, ok); code != 143 {
		t.Fatalf("InterruptExitCode = %d, want 143 (an interrupted run can never exit 0)", code)
	}

	// Target 3 was queued behind the interrupt and must never have launched.
	if !containsBase(unlaunched, "test_ccc.sh") {
		t.Fatalf("unlaunched = %v, want it to include test_ccc.sh", basesOf(unlaunched))
	}
	if _, err := os.Stat(filepath.Join(work, "stat", "test_ccc.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat/test_ccc.json exists or errored oddly (%v) — target 3 must produce no result", err)
	}

	// Target 1 completed before the interrupt; its durable diagnostic survives.
	r, err := ReadResult(filepath.Join(work, "stat", "test_aaa.json"))
	if err != nil {
		t.Fatalf("target 1's durable result should survive interruption: %v", err)
	}
	if r.RC != 0 {
		t.Fatalf("target 1 durable rc = %d, want 0", r.RC)
	}
}

// TestInterruptBetweenSchedulingAndLaunch cancels via a real signal while later
// targets are still queued behind a lane-holding first target. The queued
// targets surface as never-launched with no result files, and the run's derived
// exit code is non-zero.
func TestInterruptBetweenSchedulingAndLaunch(t *testing.T) {
	scripts := testsupport.TempDir(t)
	work := testsupport.TempDir(t)
	gate := testsupport.TempDir(t)

	t1 := writeScript(t, scripts, "aaa", strings.Join([]string{
		`: > "` + filepath.Join(gate, "held") + `"`,
		`sleep 30`,
	}, "\n")+"\n")
	t2 := writeScript(t, scripts, "bbb", "echo 'ok - two'\n")
	t3 := writeScript(t, scripts, "ccc", "echo 'ok - three'\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg := newProcRegistry()
	fired, stop := InstallSignalHandling(cancel, reg, 3*time.Second)
	defer stop()

	cfg := Config{Bash: bashPath(t), Jobs: 1, Work: work}
	par, ser := Schedule([]Target{t1, t2, t3})

	unlaunchedCh := make(chan []Target, 1)
	go func() { unlaunchedCh <- runLanes(ctx, cfg, par, ser, reg, nil) }()

	// Wait until target 1 holds the single lane, then interrupt.
	waitForFile(t, filepath.Join(gate, "held"), 5*time.Second)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("self-signal SIGINT: %v", err)
	}

	var unlaunched []Target
	select {
	case unlaunched = <-unlaunchedCh:
	case <-time.After(10 * time.Second):
		t.Fatal("runLanes did not return after interruption")
	}

	sig, ok := fired()
	if !ok || sig != syscall.SIGINT {
		t.Fatalf("fired() = (%v, %v), want (SIGINT, true)", sig, ok)
	}
	if code := InterruptExitCode(sig, ok); code != 130 {
		t.Fatalf("InterruptExitCode = %d, want 130", code)
	}
	if code := InterruptExitCode(sig, ok); code == 0 {
		t.Fatal("interrupted run derived exit 0")
	}

	for _, base := range []string{"test_bbb.sh", "test_ccc.sh"} {
		if !containsBase(unlaunched, base) {
			t.Fatalf("unlaunched = %v, want it to include %s", basesOf(unlaunched), base)
		}
		stat := filepath.Join(work, "stat", strings.TrimSuffix(base, ".sh")+".json")
		if _, err := os.Stat(stat); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s exists or errored oddly (%v) — queued target must produce no result", stat, err)
		}
	}
}

// TestInterruptExitCodeInvariant pins the exit mapping directly: no fire -> 0
// (tally decides); SIGINT -> 130; SIGTERM -> 143; any other fired signal -> a
// non-zero fail-closed code. Encodes "an interrupted run can never exit 0".
func TestInterruptExitCodeInvariant(t *testing.T) {
	if got := InterruptExitCode(nil, false); got != 0 {
		t.Fatalf("no interrupt -> %d, want 0", got)
	}
	if got := InterruptExitCode(syscall.SIGINT, true); got != 130 {
		t.Fatalf("SIGINT -> %d, want 130", got)
	}
	if got := InterruptExitCode(syscall.SIGTERM, true); got != 143 {
		t.Fatalf("SIGTERM -> %d, want 143", got)
	}
	if got := InterruptExitCode(syscall.SIGHUP, true); got == 0 {
		t.Fatalf("a fired interrupt must never map to 0; got %d", got)
	}
}

func containsBase(ts []Target, base string) bool {
	for _, t := range ts {
		if t.Base == base {
			return true
		}
	}
	return false
}

func basesOf(ts []Target) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Base)
	}
	return out
}
