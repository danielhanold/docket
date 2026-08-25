// Process-integration tests: the gate driver driven against the REAL native
// process supervisor (internal/process.Service), not a scripted double.
//
// Every other gatedrive test injects a fakeProc so the state machine is exercised
// deterministically. This file proves the composition itself: a *process.Service
// satisfies the driver's ProcessSeam as-is (Launch/Observe/Stop), so the driver
// drives a real detached supervisor and a real child process that spans several
// injected-short slices. The invariants proven here are the ones a double cannot
// vouch for (spec "Verification strategy → Process integration tests"):
//
//   - each driver invocation returns within its (short, test-injected) slice
//     bound rather than blocking for the whole observation budget;
//   - the supervisor and its child keep one stable identity across invocations,
//     and no invocation duplicates the child;
//   - ending a driver invocation (here: a separate CLI-shaped subprocess that
//     exits) neither kills nor duplicates the detached child;
//   - a FRESH driver process resumes the drive purely from the durable record and
//     consumes the exact terminal status the child produced while no driver was
//     watching;
//   - a process-tree death permits at most one non-overlapping relaunch;
//   - deadline expiry stops the whole owned process tree;
//   - durable logs and the passed-run raw directory remain usable for evidence.
//
// The identity oracle is ALWAYS the native receipt — the supervisor pid / pgid /
// sid recorded in the run's manifest.json and the exact kind/exit/signal in its
// terminal.json — NEVER process-name matching (spec: "It never uses
// process-name matching as its oracle.").
//
// This file also owns the package's TestMain, which routes the test binary's
// three re-exec roles: the native supervisor (env-driven, via
// process.RunSupervisorFromEnv), the purpose-built child command (argv marker
// "gatedrive-int-child"), and — falling through — the ordinary test run. It
// mirrors internal/process's own main_test.go, which is the established pattern
// for driving the real supervisor from a Go test.
package gatedrive

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// TestMain routes the three re-exec roles of the gatedrive test binary. go test
// itself sets neither the supervisor env nor the child argv marker, so an
// ordinary run falls through to m.Run.
func TestMain(m *testing.M) {
	if process.SupervisorRequested() {
		os.Exit(process.RunSupervisorFromEnv())
	}
	if len(os.Args) > 1 && os.Args[1] == intChildMarker {
		os.Exit(runIntChild(os.Args[2:]))
	}
	os.Exit(m.Run())
}

// intChildMarker is the argv[1] sentinel the supervisor-spawned child (and the
// CLI-shaped drive-start subprocess) carry so TestMain routes them to
// runIntChild instead of the test suite.
const intChildMarker = "gatedrive-int-child"

// intChildStdout is a recognizable marker the pass child writes to stdout, so a
// test can prove the passed run's durable stdout log holds the child's bytes.
const intChildStdout = "GATEDRIVE-INT-CHILD-STDOUT"

// runIntChild is the purpose-built child command for the integration tests.
// Modes:
//
//	pass-after <ms>     sleep ms, write the stdout marker, exit 0 (a clean pass
//	                    that spans several short slices)
//	sleep-forever       block ~forever with DEFAULT signal disposition, so a
//	                    group-directed SIGTERM from Stop ends it the way a real
//	                    supervised command dies (never caught → never a fabricated
//	                    exit record)
//	selfkill-after <ms> sleep ms, then SIGKILL only this process — a genuine
//	                    signaled death (kind=signal) with no stop intent, the
//	                    input the single-relaunch path keys on
//	drive-start <gitCommonDir> <runRoot> <childMs>
//	                    start a real drive against the real supervisor, print
//	                    "<driveID> <ownerGen> <runDir>", and exit — the separate
//	                    CLI-shaped process whose exit must not disturb the child
func runIntChild(args []string) int {
	if len(args) == 0 {
		return 90
	}
	switch args[0] {
	case "pass-after":
		ms, _ := strconv.Atoi(args[1])
		time.Sleep(time.Duration(ms) * time.Millisecond)
		fmt.Fprint(os.Stdout, intChildStdout)
		return 0
	case "sleep-forever":
		// A bare select{} trips Go's all-goroutines-asleep deadlock detector; a
		// parked timer does not, so time.Sleep keeps the runtime alive with the
		// DEFAULT SIGTERM disposition, exactly as internal/process's own "sleep"
		// helper does.
		time.Sleep(time.Hour)
		return 0
	case "selfkill-after":
		ms, _ := strconv.Atoi(args[1])
		time.Sleep(time.Duration(ms) * time.Millisecond)
		// Kill only this process (not the group) so the supervisor survives to
		// record a signaled terminal, which is what StateSignaled requires.
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGKILL)
		time.Sleep(time.Hour) // unreachable once the signal lands
		return 0
	case "drive-start":
		return runDriveStartChild(args[1:])
	}
	return 92
}

// runDriveStartChild is the CLI-shaped launcher: a separate process that starts
// a drive against the real supervisor, prints the opaque continuation another
// process needs to resume it, and exits. Its exit is the "terminating a driver
// CLI invocation" event whose harmlessness the resume test proves.
func runDriveStartChild(args []string) int {
	if len(args) != 3 {
		return 80
	}
	gitCommonDir, runRoot := args[0], args[1]
	exe, err := os.Executable()
	if err != nil {
		return 81
	}
	svc, err := process.NewService(exe)
	if err != nil {
		return 82
	}
	store := OpenStore(gitCommonDir)
	d := newIntDriver(store, svc)
	doc, err := d.Start(intStartRequest(exe, runRoot, gitCommonDir, "pass-after", args[2]))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 83
	}
	if doc.Outcome != WAITING {
		fmt.Fprintf(os.Stderr, "start outcome %s\n", doc.Outcome)
		return 84
	}
	rec, err := store.Load(doc.DriveID)
	if err != nil {
		return 85
	}
	// One line the parent parses; nothing else reaches this process's stdout (the
	// supervisor and child write to their own durable log files).
	fmt.Println(doc.DriveID, doc.Generation, rec.RawRunDir)
	return 0
}

// ---------------------------------------------------------------------------
// Fixtures shared by the integration tests.
// ---------------------------------------------------------------------------

// intSlice / intPoll are the test-injected short slice and poll interval. They
// are the whole point of the integration suite: with a 30-minute budget, an
// invocation that returns in tens of milliseconds proves the call is
// slice-bounded, not budget-bounded.
const (
	intSlice = 40 * time.Millisecond
	intPoll  = 5 * time.Millisecond
	// sliceCeiling is the generous upper bound one slice-bounded invocation may
	// take under CI load. It is far below the 30-minute budget, so staying under
	// it is real proof of slice-bounding, not an exact-timing assertion.
	sliceCeiling = 5 * time.Second
)

var intRunIDRe = regexp.MustCompile("^[0-9a-f]{32}$")

// skipUnlessSupported skips on a platform where the native supervisor is not
// built (internal/process gates Launch on darwin/linux only). It is the one
// platform guard this file uses; the process package itself carries no
// testing.Short guard, and neither does this suite.
func skipUnlessSupported(t *testing.T) {
	t.Helper()
	switch runtime.GOOS {
	case "darwin", "linux":
	default:
		t.Skipf("native process supervisor is unsupported on %s", runtime.GOOS)
	}
}

// newIntDriver wires a driver over the REAL process service with the injected
// short slice and a real sleep/clock, so slices are bounded by real wall-clock
// time against a live child.
func newIntDriver(store *Store, svc *process.Service) *Driver {
	d := NewDriver(store, systemClock{}, svc, stableGit())
	d.slice = intSlice
	d.pollInterval = intPoll
	d.sleep = time.Sleep
	return d
}

// mustExe returns the test binary path — the executable the real service
// re-execs as the supervisor and as the child command.
func mustExe(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return exe
}

// mustService builds a real process.Service around the test binary.
func mustService(t *testing.T) *process.Service {
	t.Helper()
	svc, err := process.NewService(mustExe(t))
	if err != nil {
		t.Fatalf("process.NewService: %v", err)
	}
	return svc
}

// intStartRequest builds a well-formed StartRequest whose raw launch runs the
// integration child helper through the real supervisor. The command is the test
// binary re-invoked with the child marker; the fingerprint rides on stableGit so
// it is deterministic and reproducible across processes without a real repo
// (repository-dimension drift is Task 8's subject, not this file's).
func intStartRequest(exe, runRoot, cwd, childMode, childArg string) StartRequest {
	argv := []string{exe, intChildMarker, childMode}
	if childArg != "" {
		argv = append(argv, childArg)
	}
	return StartRequest{
		RepoDir:             "/repo",
		Worktree:            "/repo",
		ChangeID:            "0342",
		TaskID:              "task-7",
		Phase:               "build",
		Branch:              "feat/x",
		Ref:                 "refs/heads/feat/x",
		Command:             argv,
		Cwd:                 cwd,
		ConfigProvenance:    "config:finalize.test_command",
		Budget:              30 * time.Minute,
		EnvHash:             "envhash",
		RunRoot:             runRoot,
		IdempotentSuiteGate: true,
	}
}

// ---------------------------------------------------------------------------
// Native-receipt oracle helpers. Identity comes from the manifest, terminal
// status from terminal.json — never from a process name.
// ---------------------------------------------------------------------------

// manifestIdentity is the subset of the native manifest receipt that identifies
// the supervised tree: the supervisor pid, its process-group id, and its
// session id. These are the driver-independent identity the tests compare across
// invocations.
type manifestIdentity struct {
	SupervisorPID int `json:"supervisor_pid"`
	PGID          int `json:"pgid"`
	SID           int `json:"sid"`
}

func readManifestIdentity(t *testing.T, runDir string) manifestIdentity {
	t.Helper()
	var m manifestIdentity
	readRunJSON(t, runDir, "manifest.json", &m)
	if m.SupervisorPID <= 1 || m.PGID != m.SupervisorPID || m.SID != m.SupervisorPID {
		t.Fatalf("manifest identity is not a well-formed session leader: %+v", m)
	}
	return m
}

// terminalReceipt is the exact decoded child wait status from the native
// terminal record — the oracle for "consumed the exact terminal status".
type terminalReceipt struct {
	Kind     string `json:"kind"`
	ExitCode int    `json:"exit_code"`
	Signal   int    `json:"signal"`
}

func readTerminalReceipt(t *testing.T, runDir string) terminalReceipt {
	t.Helper()
	var term terminalReceipt
	readRunJSON(t, runDir, "terminal.json", &term)
	return term
}

func readRunJSON(t *testing.T, runDir, name string, v any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(runDir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decoding %s: %v", name, err)
	}
}

// pidAlive probes whether a pid is live via signal 0 — ESRCH means gone, EPERM
// means alive-but-not-ours. It never matches on a process name.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return err == syscall.EPERM
}

// observeState returns the native supervisor's read-only verdict for a run — the
// durable-receipt oracle for "the tree is stopped / dead", independent of any
// process-name match and of reaping timing (Observe reads the terminal record
// and the live-lock probe, never a command line).
func observeState(t *testing.T, svc *process.Service, runDir string) process.State {
	t.Helper()
	obs, err := svc.Observe(runDir)
	if err != nil {
		t.Fatalf("observe %s: %v", runDir, err)
	}
	return obs.State
}

// reapSupervisors emulates init reaping the orphaned supervisors of an
// IN-PROCESS launcher. In production the launcher exits right after Launch, so
// each supervisor is reparented to init and reaped when it dies — its process
// group then becomes provably absent, which is what the native Stop's teardown
// verification rests on (spec: verified group absence). An in-process test keeps
// every supervisor a waitable child of the long-lived test process; with no
// reaper it lingers as a zombie group leader after death, so kill(-pgid, 0)
// returns EPERM forever and group absence — hence a driver-internal Stop's
// teardown — is never observable. This background sweep plays init's role by
// Wait4-ing the EXACT supervisor pids under runRoot (never -1, so it can never
// steal the drive-start subprocess's own wait status); WNOHANG makes each probe a
// no-op while a supervisor is alive and a reap once it is a zombie. It mirrors
// internal/process's reapSupervisor, which launchHelper installs for the same
// reason. Supervisors this process did not spawn (the resume test's, owned by the
// exited subprocess and reparented to init) return ECHILD and are left to init.
func reapSupervisors(t *testing.T, runRoot string) {
	t.Helper()
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			for _, pid := range supervisorPIDsUnder(runRoot) {
				var ws syscall.WaitStatus
				_, _ = syscall.Wait4(pid, &ws, syscall.WNOHANG, nil)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	t.Cleanup(func() { close(done); wg.Wait() })
}

// supervisorPIDsUnder reads the recorded supervisor pid from each run manifest
// under root. It is the reaper's non-fatal scanner, safe to call from the reaper
// goroutine (it never touches *testing.T) and tolerant of a not-yet-created root,
// a half-written run dir, or a manifest mid-rename.
func supervisorPIDsUnder(root string) []int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		if !e.IsDir() || !intRunIDRe.MatchString(e.Name()) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name(), "manifest.json"))
		if err != nil {
			continue
		}
		var m manifestIdentity
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if m.SupervisorPID > 1 {
			pids = append(pids, m.SupervisorPID)
		}
	}
	return pids
}

// runDirsUnder returns every run directory (32-hex name) under root.
func runDirsUnder(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading run root %s: %v", root, err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && intRunIDRe.MatchString(e.Name()) {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	return dirs
}

// soleRunDir asserts exactly one run directory exists under root and returns it,
// so a test proves the child was never duplicated.
func soleRunDir(t *testing.T, root string) string {
	t.Helper()
	dirs := runDirsUnder(t, root)
	if len(dirs) != 1 {
		t.Fatalf("want exactly one run dir under %s, got %d: %v", root, len(dirs), dirs)
	}
	return dirs[0]
}

// stopAllRuns is a defensive cleanup: best-effort stop of every run under root so
// a failed test never leaks a detached child. A run our own process group
// launched is ownership-provable, so Stop can end it; errors are ignored.
func stopAllRuns(t *testing.T, svc *process.Service, root string) {
	t.Helper()
	for _, dir := range runDirsUnder(t, root) {
		_, _ = svc.Stop(dir, "gatedrive-int-cleanup")
	}
}

// advanceUntilTerminal advances the drive with a fixed owner generation until a
// terminal outcome, asserting every invocation is slice-bounded, and returns the
// terminal document plus how many WAITING slices it took.
func advanceUntilTerminal(t *testing.T, d *Driver, id, gen string) (DriveDoc, int) {
	t.Helper()
	end := time.Now().Add(30 * time.Second)
	waiting := 0
	for {
		start := time.Now()
		doc, err := d.Advance(id, gen)
		if err != nil {
			t.Fatalf("advance: %v", err)
		}
		if el := time.Since(start); el > sliceCeiling {
			t.Fatalf("advance invocation took %v — not slice-bounded (budget is 30m)", el)
		}
		if doc.Outcome != WAITING {
			return doc, waiting
		}
		waiting++
		if time.Now().After(end) {
			t.Fatalf("drive never left WAITING within 30s")
		}
	}
}

// ---------------------------------------------------------------------------
// Tests.
// ---------------------------------------------------------------------------

// TestIntegrationDriverSlicesAcrossLiveChildThenPasses drives a real child that
// outlives several short slices. It proves the slice bound holds on every
// invocation, the supervised identity is stable across invocations, the child is
// never duplicated, and the eventual pass exposes a usable raw run directory with
// durable logs and the exact terminal receipt.
func TestIntegrationDriverSlicesAcrossLiveChildThenPasses(t *testing.T) {
	skipUnlessSupported(t)
	svc := mustService(t)
	runRoot := filepath.Join(t.TempDir(), "runs")
	store := OpenStore(t.TempDir())
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	reapSupervisors(t, runRoot)
	d := newIntDriver(store, svc)

	// A child that stays alive well past a handful of 40ms slices.
	start := time.Now()
	doc, err := d.Start(intStartRequest(mustExe(t), runRoot, t.TempDir(), "pass-after", "600"))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if el := time.Since(start); el > sliceCeiling {
		t.Fatalf("start invocation took %v — not slice-bounded", el)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice over a live child: outcome %s (cause %q), want WAITING", doc.Outcome, doc.Cause)
	}

	runDir := soleRunDir(t, runRoot)
	idFirst := readManifestIdentity(t, runDir)

	// Every subsequent invocation is slice-bounded; the child spans several of
	// them before it exits and the next invocation reads the durable pass.
	term, waiting := advanceUntilTerminal(t, d, doc.DriveID, doc.Generation)
	if term.Outcome != PASSED {
		t.Fatalf("terminal outcome %s (cause %q), want PASSED", term.Outcome, term.Cause)
	}
	if waiting < 2 {
		t.Fatalf("child did not span multiple slices: only %d WAITING invocations", waiting)
	}

	// Identity stable across every invocation, and the child was never
	// duplicated: still exactly one run dir, same supervisor pid/pgid/sid.
	if got := soleRunDir(t, runRoot); got != runDir {
		t.Fatalf("run dir changed across invocations: %s -> %s", runDir, got)
	}
	if idLast := readManifestIdentity(t, runDir); idLast != idFirst {
		t.Fatalf("supervised identity drifted across invocations: %+v -> %+v", idFirst, idLast)
	}

	// A pass exposes the exact raw run dir for evidence, and the native receipt is
	// a clean exit 0 (oracle: terminal.json, never a process name).
	if term.RawRunDir != runDir {
		t.Fatalf("PASSED raw run dir %q != %q", term.RawRunDir, runDir)
	}
	if rc := readTerminalReceipt(t, runDir); rc.Kind != "exit" || rc.ExitCode != 0 {
		t.Fatalf("terminal receipt %+v, want a clean exit 0", rc)
	}

	// Durable logs remain usable: the child's stdout bytes are in the run's
	// stdout log.
	so, err := os.ReadFile(filepath.Join(runDir, "stdout.log"))
	if err != nil {
		t.Fatalf("reading stdout.log: %v", err)
	}
	if !strings.Contains(string(so), intChildStdout) {
		t.Fatalf("passed run's stdout log missing the child marker: %q", string(so))
	}
}

// TestIntegrationFreshProcessResumesAndChildSurvives runs the drive-start in a
// SEPARATE CLI-shaped process that exits the moment it has launched the child.
// It proves that ending that invocation neither kills nor duplicates the detached
// child, and that a fresh driver process resumes the drive purely from the
// durable record and consumes the exact terminal status the child produced while
// no driver was watching — with the supervised identity stable across the process
// boundary.
func TestIntegrationFreshProcessResumesAndChildSurvives(t *testing.T) {
	skipUnlessSupported(t)
	svc := mustService(t)
	gitCommon := t.TempDir()
	runRoot := filepath.Join(t.TempDir(), "runs")
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	reapSupervisors(t, runRoot)

	// The separate launcher process starts the drive and exits. A child that
	// stays alive long enough for us to observe it surviving the launcher's death.
	out, err := exec.Command(mustExe(t), intChildMarker, "drive-start", gitCommon, runRoot, "1500").Output()
	if err != nil {
		t.Fatalf("drive-start subprocess: %v", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) != 3 {
		t.Fatalf("drive-start printed %q, want '<id> <gen> <runDir>'", string(out))
	}
	driveID, ownerGen, runDir := fields[0], fields[1], fields[2]

	// The launcher subprocess has fully exited; its child must still be live and
	// unduplicated (oracle: the manifest's recorded supervisor pid).
	if got := soleRunDir(t, runRoot); got != runDir {
		t.Fatalf("resume run dir %q != sole run dir %q", runDir, got)
	}
	idBefore := readManifestIdentity(t, runDir)
	if !pidAlive(idBefore.SupervisorPID) {
		t.Fatalf("child did not survive the launcher's exit: supervisor pid %d gone", idBefore.SupervisorPID)
	}

	// A FRESH driver process (this test) resumes from disk alone and drives the
	// same suite to its durable terminal.
	store := OpenStore(gitCommon)
	d := newIntDriver(store, svc)
	term, _ := advanceUntilTerminal(t, d, driveID, ownerGen)
	if term.Outcome != PASSED {
		t.Fatalf("resumed drive terminal %s (cause %q), want PASSED", term.Outcome, term.Cause)
	}

	// Same identity across the process boundary, still one run dir, exact receipt.
	if got := soleRunDir(t, runRoot); got != runDir {
		t.Fatalf("run dir changed after resume: %s -> %s", runDir, got)
	}
	if idAfter := readManifestIdentity(t, runDir); idAfter != idBefore {
		t.Fatalf("identity drifted across the process boundary: %+v -> %+v", idBefore, idAfter)
	}
	if rc := readTerminalReceipt(t, runDir); rc.Kind != "exit" || rc.ExitCode != 0 {
		t.Fatalf("resumed terminal receipt %+v, want a clean exit 0", rc)
	}
}

// TestIntegrationDeadlineExpiryStopsOwnedTree proves that when the fixed deadline
// has already passed at the first observation (a zero budget), the driver takes
// exactly one observation of the live tree, stops the whole owned tree, and
// returns HALTED — never a fabricated verdict — leaving the child's process group
// dead.
func TestIntegrationDeadlineExpiryStopsOwnedTree(t *testing.T) {
	skipUnlessSupported(t)
	svc := mustService(t)
	runRoot := filepath.Join(t.TempDir(), "runs")
	store := OpenStore(t.TempDir())
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	reapSupervisors(t, runRoot)
	d := newIntDriver(store, svc)

	// A child that would otherwise run forever, so the stop is what ends it.
	req := intStartRequest(mustExe(t), runRoot, t.TempDir(), "sleep-forever", "")
	req.Budget = 0 // deadline == start: expired at the very first observation.

	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("zero-budget start outcome %s (cause %q), want HALTED", doc.Outcome, doc.Cause)
	}
	if doc.Cause != "deadline-expired" {
		t.Fatalf("halt cause %q, want deadline-expired", doc.Cause)
	}
	// The whole owned tree was stopped, not left running: the native durable
	// receipt reads StateStopped — a group-directed stop this drive initiated,
	// distinct from an unrequested signal death (oracle: terminal.json + stop
	// intent via Observe, never a process name). One run dir: the child was never
	// duplicated.
	runDir := soleRunDir(t, runRoot)
	if st := observeState(t, svc, runDir); st != process.StateStopped {
		t.Fatalf("owned tree state after deadline stop = %v, want stopped", st)
	}
	if rc := readTerminalReceipt(t, runDir); rc.Kind != "signal" {
		t.Fatalf("deadline-stop terminal receipt %+v, want a signal death", rc)
	}
}

// TestIntegrationProcessDeathPermitsAtMostOneRelaunch drives a child that dies by
// a signal with no stop intent (a genuine tree death). The single-relaunch policy
// admits exactly one non-overlapping second raw run under the original deadline;
// when that one also dies, the driver HALTs with relaunch-exhausted rather than
// launching a third. Both raw runs' groups are dead, and the two runs are
// distinct — the first proven gone before the second launched.
func TestIntegrationProcessDeathPermitsAtMostOneRelaunch(t *testing.T) {
	skipUnlessSupported(t)
	svc := mustService(t)
	runRoot := filepath.Join(t.TempDir(), "runs")
	store := OpenStore(t.TempDir())
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	reapSupervisors(t, runRoot)
	d := newIntDriver(store, svc)

	// A child that lives long enough to establish "running", then SIGKILLs itself.
	doc, err := d.Start(intStartRequest(mustExe(t), runRoot, t.TempDir(), "selfkill-after", "120"))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice over a live child: outcome %s (cause %q), want WAITING", doc.Outcome, doc.Cause)
	}

	term, _ := advanceUntilTerminal(t, d, doc.DriveID, doc.Generation)
	if term.Outcome != HALTED {
		t.Fatalf("terminal outcome %s (cause %q), want HALTED", term.Outcome, term.Cause)
	}
	if term.Cause != "relaunch-exhausted" {
		t.Fatalf("halt cause %q, want relaunch-exhausted", term.Cause)
	}

	rec, err := store.Load(doc.DriveID)
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	if rec.RelaunchCount != 1 {
		t.Fatalf("relaunch count %d, want exactly 1 (at most one relaunch)", rec.RelaunchCount)
	}
	if rec.Attempt != 2 {
		t.Fatalf("attempt %d, want 2 after the single relaunch", rec.Attempt)
	}
	// Two distinct, non-overlapping raw runs: the first is preserved as the prior
	// attempt, the second is the current one, and both trees are dead.
	if rec.PriorRawRunDir == "" || rec.PriorRawRunDir == rec.RawRunDir {
		t.Fatalf("relaunch did not record a distinct prior run: prior=%q current=%q", rec.PriorRawRunDir, rec.RawRunDir)
	}
	first := readManifestIdentity(t, rec.PriorRawRunDir)
	second := readManifestIdentity(t, rec.RawRunDir)
	if first.SupervisorPID == second.SupervisorPID {
		t.Fatalf("two attempts share a supervisor pid %d — not distinct runs", first.SupervisorPID)
	}
	// Neither tree is still running: both died a genuine signal death (the child
	// SIGKILLed itself, no stop intent), which the native receipt records as
	// StateSignaled (oracle: terminal.json via Observe, never a process name).
	if st := observeState(t, svc, rec.PriorRawRunDir); st != process.StateSignaled {
		t.Fatalf("first (relaunched-away) tree state = %v, want signaled", st)
	}
	if st := observeState(t, svc, rec.RawRunDir); st != process.StateSignaled {
		t.Fatalf("second (exhausting) tree state = %v, want signaled", st)
	}
	if got := len(runDirsUnder(t, runRoot)); got != 2 {
		t.Fatalf("want exactly two raw run dirs after one relaunch, got %d", got)
	}
}
