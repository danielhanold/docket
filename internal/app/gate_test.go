package app

import (
	"encoding/json"
	"errors"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"strings"
	"testing"
)

// TestMain routes the supervisor re-exec role of the app test binary: a real
// GateLaunch re-executes this binary with the private supervisor env var set,
// and it must become the supervisor rather than re-running the test suite.
// Ordinary `go test` runs set neither and fall through to m.Run.
// Ordinary runs then install the default-build no-real-git guard (change 0465) around m.Run.
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
	// Change 0465: the default build installs the no-real-git guard (nogit_guard_test.go)
	// AFTER the re-exec routing above, so the supervisor and guardian roles behave
	// exactly as before; tagged builds get the no-op twin (nogit_guard_off_test.go).
	finish := installNoGitGuard()
	os.Exit(finish(m.Run()))
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
