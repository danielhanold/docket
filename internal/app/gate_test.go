package app

import (
	"encoding/json"
	"errors"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain routes the supervisor re-exec role of the app test binary: a real
// GateLaunch re-executes this binary with the private supervisor env var set,
// and it must become the supervisor rather than re-running the test suite.
// Ordinary `go test` runs set neither and fall through to m.Run.
func TestMain(m *testing.M) {
	if process.SupervisorRequested() {
		os.Exit(process.RunSupervisorFromEnv())
	}
	// Route the death-guardian re-exec role: an agent_guardian_test.go real-process
	// test re-execs THIS binary as a detached guardian, which must run the guardian
	// lifetime rather than re-running the suite (change 0375 Task 13).
	if GuardianRequested() {
		os.Exit(RunAgentGuardianFromEnv())
	}
	os.Exit(m.Run())
}

func TestMapObservationTable(t *testing.T) {
	cases := map[process.State]Result{
		process.StateRunning:  ResultApplied,
		process.StatePassed:   ResultApplied,
		process.StateFailed:   ResultGateFailed,
		process.StateSignaled: ResultInterrupted,
		process.StateStopped:  ResultInterrupted,
		process.StateVanished: ResultInterrupted,
	}
	for st, want := range cases {
		if got := mapObservation(st); got != want {
			t.Errorf("%s -> %s, want %s", st, got, want)
		}
	}
}

func TestGateLaunchInvalidInput(t *testing.T) {
	res := GateLaunch("relative-root", "/", []string{"/bin/echo"})
	if res.Result != ResultInvalidInput {
		t.Fatalf("result %s", res.Result)
	}
	if ExitCode(res.Result) != 2 {
		t.Fatalf("exit mapping")
	}
}

func TestGateRecoverNormalizesEmptyEntries(t *testing.T) {
	res := GateRecover(testsupport.TempDir(t))
	if res.Result != ResultNoOp {
		t.Fatalf("clean scan result %s", res.Result)
	}
	buf, _ := json.Marshal(res)
	if !strings.Contains(string(buf), `"recovery":[]`) {
		t.Fatalf("nil collection leaked as absent: %s", buf)
	}
}

func TestGateResultHumanTextStable(t *testing.T) {
	code := 7
	r := GateResult{Envelope: NewEnvelope("gate.observe", ResultGateFailed),
		RunID: "aa", RunDir: "/r/aa", State: "failed", ExitCode: &code,
		StdoutLog: "/r/aa/stdout.log", StderrLog: "/r/aa/stderr.log"}
	want := "state: failed\nrun_id: aa\nrun_dir: /r/aa\nexit_code: 7\nstdout_log: /r/aa/stdout.log\nstderr_log: /r/aa/stderr.log"
	if r.HumanText() != want {
		t.Fatalf("HumanText:\n got %q\nwant %q", r.HumanText(), want)
	}
}

// --- change 0375: raw gate.launch admits through the worktree execution slot ---

// TestGateLaunchInsideWorktreeReservesSlot proves a raw launch whose cwd sits
// inside a registered worktree acquires the durable execution slot: after a PASSED
// start the slot is executing, carries this launch's raw run identity, is Kind
// "raw", and holds no drive id.
func TestGateLaunchInsideWorktreeReservesSlot(t *testing.T) {
	requireRealGit(t)
	worktree, gitDir := initGitRepo(t, "")
	// A raw slot holds the worktree until a GateStop-proven teardown releases it,
	// so even a fast command keeps the slot "executing" for the assertions below.
	res := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/echo", "hi"})
	t.Cleanup(func() { GateStop(res.RunDir, "test cleanup") })
	if res.Result != ResultApplied || res.RunDir == "" {
		t.Fatalf("launch: result=%s reason=%q rundir=%q", res.Result, res.Reason, res.RunDir)
	}
	slot, _, err := gatedrive.OpenStore(gitDir).LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if got := string(slot.State); got != "executing" {
		t.Fatalf("slot state = %q, want executing", got)
	}
	if slot.Kind != "raw" {
		t.Fatalf("slot kind = %q, want raw", slot.Kind)
	}
	if slot.RawRunDir != res.RunDir || slot.RawRunID != res.RunID {
		t.Fatalf("slot raw run = (%q,%q), want (%q,%q)", slot.RawRunID, slot.RawRunDir, res.RunID, res.RunDir)
	}
	if slot.DriveID != "" {
		t.Fatalf("raw slot carries a drive id %q", slot.DriveID)
	}
}

// TestGateLaunchSecondRefusedWhileFirstLives proves a second raw launch into a
// worktree whose slot is live is refused worktree-busy — even from a DISTINCT run
// root — with no process spawned, and that the refusal locates the incumbent run
// (a safe locator) without leaking a reservation token.
func TestGateLaunchSecondRefusedWhileFirstLives(t *testing.T) {
	requireRealGit(t)
	worktree, _ := initGitRepo(t, "")
	// The first run is genuinely LIVE (a long sleep), so the admission-boundary
	// finished-incumbent reconciliation (change 0446 spec §3) has no teardown proof
	// and the slot still blocks a second admission.
	first := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/sleep", "60"})
	t.Cleanup(func() { GateStop(first.RunDir, "test cleanup") })
	if first.Result != ResultApplied || first.RunID == "" {
		t.Fatalf("first launch: result=%s reason=%q", first.Result, first.Reason)
	}
	second := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/echo", "hi"})
	if second.RunDir != "" {
		GateStop(second.RunDir, "test cleanup") // never expected; avoid leaking a process
		t.Fatalf("refused launch produced a run handle: %+v", second)
	}
	if second.Result != ResultBlocked {
		t.Fatalf("second launch result = %s (reason %q), want blocked", second.Result, second.Reason)
	}
	if second.Reason != "worktree-busy" {
		t.Fatalf("second launch reason = %q, want worktree-busy", second.Reason)
	}
	if !strings.Contains(second.Cause, first.RunID) {
		t.Fatalf("refusal cause %q does not locate the incumbent run %q", second.Cause, first.RunID)
	}
}

// TestGateLaunchSettlesFinishedRawIncumbent (change 0446 spec §3): a COMPLETED raw
// run whose slot was never stopped no longer blocks the worktree. The next raw
// launch's normal admission proves the incumbent torn down through the process
// predicate, settles its slot, and admits — with no manual GateStop and no second
// launch attempt.
func TestGateLaunchSettlesFinishedRawIncumbent(t *testing.T) {
	requireRealGit(t)
	worktree, gitDir := initGitRepo(t, "")
	first := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/echo", "hi"})
	if first.Result != ResultApplied || first.RunDir == "" {
		t.Fatalf("first launch: result=%s reason=%q", first.Result, first.Reason)
	}
	waitRawRunTornDown(t, first.RunDir)
	if slot, _, err := gatedrive.OpenStore(gitDir).LoadWorktreeExecution(worktree); err != nil || string(slot.State) != "executing" {
		t.Fatalf("a completed raw run keeps its slot occupied until settled: state=%q err=%v", string(slot.State), err)
	}

	second := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/echo", "hi"})
	if second.RunDir != "" {
		t.Cleanup(func() { waitGateRunTerminal(t, second.RunDir); GateStop(second.RunDir, "test cleanup") })
	}
	if second.Result != ResultApplied || second.RunDir == "" || second.RunDir == first.RunDir {
		t.Fatalf("a proven-finished raw incumbent must not block the next launch: result=%s reason=%q cause=%q",
			second.Result, second.Reason, second.Cause)
	}
	slot, _, err := gatedrive.OpenStore(gitDir).LoadWorktreeExecution(worktree)
	if err != nil || slot.RawRunDir != second.RunDir {
		t.Fatalf("the slot must now hold the second run, got %q err=%v", slot.RawRunDir, err)
	}
}

// waitRawRunTornDown polls the process predicate reconciliation consults until the
// run's supervisor has released it with a durable terminal record, so a following
// admission deterministically sees positive teardown proof.
func waitRawRunTornDown(t *testing.T, runDir string) {
	t.Helper()
	svc, _, reason := gateService()
	if svc == nil {
		t.Fatalf("gate service: %s", reason)
	}
	for i := 0; i < 300; i++ {
		if e, err := svc.ClassifyRun(runDir, false); err == nil && e.Disposition == "terminal" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("run never reached a torn-down terminal disposition")
}

// TestGateLaunchOutsideGitUnchanged proves a launch whose cwd is outside any git
// worktree keeps its pre-admission contract: no slot is reserved, so a second
// launch in the same non-worktree cwd is not refused.
func TestGateLaunchOutsideGitUnchanged(t *testing.T) {
	cwd := testsupport.TempDir(t) // not a git worktree
	first := GateLaunch(testsupport.TempDir(t), cwd, []string{"/bin/echo", "hi"})
	if first.Result != ResultApplied || first.RunDir == "" {
		t.Fatalf("first launch outside git: result=%s reason=%q", first.Result, first.Reason)
	}
	second := GateLaunch(testsupport.TempDir(t), cwd, []string{"/bin/echo", "hi"})
	if second.Result != ResultApplied || second.RunDir == "" {
		t.Fatalf("second launch outside git: result=%s reason=%q", second.Result, second.Reason)
	}
}

// TestGateStopReleasesRawSlot proves GateStop's PROVEN teardown releases the raw
// slot the run held, so the worktree readmits. The teardown proof is the run's own
// terminal state: the launched command runs to completion (a no-op stop then
// observes it passed), which is the deterministic proof of a gone process group.
// A stop of a still-LIVE run cannot prove teardown in this supervisor-as-test-binary
// harness (the group TERM frees the live lock before a terminal record lands, so
// Stop is blocked — pre-existing behavior), and the fail-closed release correctly
// leaves such a slot untouched; this test pins the provable path Task 7 adds.
func TestGateStopReleasesRawSlot(t *testing.T) {
	requireRealGit(t)
	worktree, gitDir := initGitRepo(t, "")
	store := gatedrive.OpenStore(gitDir)

	res := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/echo", "hi"})
	if res.Result != ResultApplied || res.RunDir == "" {
		t.Fatalf("launch: result=%s reason=%q", res.Result, res.Reason)
	}
	// The slot is reserved and confirmed even for a fast command.
	if slot, _, err := store.LoadWorktreeExecution(worktree); err != nil || string(slot.State) != "executing" {
		t.Fatalf("pre-stop slot state=%q err=%v", string(slot.State), err)
	}
	// Let the run reach a terminal state so the stop's teardown is provable.
	waitGateRunTerminal(t, res.RunDir)

	stop := GateStop(res.RunDir, "test cleanup")
	if stop.Result != ResultNoOp {
		t.Fatalf("stop of a terminal run result = %s (%s), want no-op", stop.Result, stop.Reason)
	}
	slot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("post-stop LoadWorktreeExecution: %v", err)
	}
	if got := string(slot.State); got != "released" {
		t.Fatalf("post-stop slot state = %q, want released", got)
	}
	readmit := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/echo", "hi"})
	if readmit.Result != ResultApplied {
		t.Fatalf("readmission after release: result=%s reason=%q", readmit.Result, readmit.Reason)
	}
}

// waitGateRunTerminal polls GateObserve until the run leaves the running state or a
// generous deadline elapses, so a following stop is a proven-terminal no-op.
func waitGateRunTerminal(t *testing.T, runDir string) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if GateObserve(runDir).State != "running" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("run never became terminal")
}

// TestGateLaunchRefusalCauseFromSnapshot proves the admission-refusal cause is
// derived from the refusal's own incumbent snapshot, not a post-refusal re-read:
// a snapshot-bearing error yields its locator; a snapshot-free error yields "".
func TestGateLaunchRefusalCauseFromSnapshot(t *testing.T) {
	withInc := &gatedrive.OwnershipError{Kind: gatedrive.ErrWorktreeBusy, Op: "reserve-worktree-execution",
		Incumbent: &gatedrive.IncumbentSnapshot{Kind: "raw", RawRunID: "0123456789abcdef0123456789abcdef"}}
	if got := admissionRefusalCause(withInc); got != "incumbent-run:0123456789abcdef0123456789abcdef" {
		t.Fatalf("cause = %q", got)
	}
	bare := &gatedrive.OwnershipError{Kind: gatedrive.ErrWorktreeBusy, Op: "reserve-worktree-execution"}
	if got := admissionRefusalCause(bare); got != "" {
		t.Fatalf("snapshot-free cause = %q, want empty", got)
	}
	if got := admissionRefusalCause(errors.New("io")); got != "" {
		t.Fatalf("non-ownership cause = %q, want empty", got)
	}
}

// TestGateLaunchLegacyInventoryRefusalNamesMatchedDrive (change 0446 spec §6): a
// raw launch refused by the first-admission legacy inventory — a nonterminal
// historical drive bound to THIS worktree — carries the drive's locator as its
// Cause and the inventory summary whose finding names the matched worktree,
// instead of a bare unresolved-execution with an empty cause. A second, unrelated
// worktree of the same repository is not vetoed by that record.
func TestGateLaunchLegacyInventoryRefusalNamesMatchedDrive(t *testing.T) {
	requireRealGit(t)
	worktree, gitDir := initGitRepo(t, "")
	const id = "0446bbbbbbbbbbbbbbbbbbbbbbbbbb01"
	dir := filepath.Join(gitDir, "docket", "gate-drives", "v1", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	record := `{"generation":"g","record":{"schema_version":2,"repo_identity":"` + gitDir +
		`","worktree_path":"` + worktree + `","started_at":"2026-08-01T14:00:00Z","last_outcome":"WAITING"}}`
	if err := os.WriteFile(filepath.Join(dir, "record.json"), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}

	res := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/echo", "hi"})
	if res.RunDir != "" {
		GateStop(res.RunDir, "test cleanup")
		t.Fatalf("refused launch produced a run handle: %+v", res)
	}
	if res.Result != ResultBlocked || res.Reason != string(gatedrive.ErrUnresolvedExecution) {
		t.Fatalf("result/reason = %s/%q, want blocked/unresolved-execution", res.Result, res.Reason)
	}
	if res.Cause != "inventory-legacy-drive-"+id {
		t.Fatalf("raw refusal cause = %q, want the matched drive locator", res.Cause)
	}
	if res.LegacyHistory == nil || len(res.LegacyHistory.Retained) != 1 ||
		res.LegacyHistory.Retained[0].DriveID != id || res.LegacyHistory.Retained[0].Worktree != worktree {
		t.Fatalf("raw refusal must carry the matched finding naming its worktree, got %+v", res.LegacyHistory)
	}
	if !strings.Contains(res.HumanText(), "cause: inventory-legacy-drive-"+id) {
		t.Fatalf("human text must render the locator:\n%s", res.HumanText())
	}

	// The same record never vetoes a different worktree of the same repository.
	other := filepath.Join(testsupport.TempDir(t), "other")
	runGit(t, worktree, "commit", "--allow-empty", "-m", "base")
	runGit(t, worktree, "worktree", "add", other)
	ok := GateLaunch(testsupport.TempDir(t), other, []string{"/bin/echo", "hi"})
	if ok.RunDir != "" {
		t.Cleanup(func() { waitGateRunTerminal(t, ok.RunDir); GateStop(ok.RunDir, "test cleanup") })
	}
	if ok.Result != ResultApplied {
		t.Fatalf("an unrelated worktree must admit, got %s (%q, cause %q)", ok.Result, ok.Reason, ok.Cause)
	}
}
