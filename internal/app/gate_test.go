package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"strings"
	"testing"
)

// nogitPkg and nogitShardGlob name this package to the shared test guards
// (testsupport.InstallNoGitGuard and testsupport.RefuseUnfilteredIntegrationRun):
// their diagnostics and their remedy text.
const (
	nogitPkg       = "internal/app"
	nogitShardGlob = "tests/test_go_integration_app_*.sh"
)

// TestMain routes the supervisor re-exec role of the app test binary: a real
// GateLaunch re-executes this binary with the private supervisor env var set,
// and it must become the supervisor rather than re-running the test suite.
// Ordinary `go test` runs set neither and fall through to m.Run.
// Ordinary runs then install the default-build no-real-git guard (change 0465,
// testsupport.InstallNoGitGuard since change 0466) around m.Run.
// Integration-tagged runs first pass the unfiltered-run guard (change 0479).
func TestMain(m *testing.M) {
	if process.SupervisorRequested() {
		os.Exit(process.RunSupervisorFromEnv())
	}
	// Route the death-guardian re-exec role: an agent_guardian_integration_test.go real-process
	// test re-execs THIS binary as a detached guardian, which must run the guardian
	// lifetime rather than re-running the suite (change 0375 Task 13).
	if GuardianRequested() {
		os.Exit(RunAgentGuardianFromEnv())
	}
	// Change 0479: the integration-tagged build refuses an unfiltered run at go
	// test's default 10m timeout (the whole corpus outlasts it) and names the
	// supported forms; every other build gets testsupport's no-op twin. It sits
	// AFTER the supervisor and guardian re-exec routing (re-exec'd children never
	// parse test flags) and before m.Run.
	if err := testsupport.RefuseUnfilteredIntegrationRun(nogitPkg, nogitShardGlob); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Change 0465 (hoisted by change 0466): the default build installs the no-real-git
	// guard (testsupport.InstallNoGitGuard) AFTER the re-exec routing above, so the
	// supervisor and guardian roles behave exactly as before; tagged builds get
	// testsupport's no-op twin. Its proving tests are in nogit_guard_test.go.
	finish, err := testsupport.InstallNoGitGuard(nogitPkg, nogitShardGlob)
	if err != nil {
		// The library returns the setup failure; this TestMain ends the process.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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

// TestGateLaunchRefusalCauseFromSnapshot proves a worktree-busy refusal's cause is
// derived from the refusal's own holder snapshot (the live holder TryWorktreeLock
// confirmed), never a post-refusal re-read: a raw holder yields its run locator, a
// drive holder its drive locator, and a holder-unknown refusal yields "".
func TestGateLaunchRefusalCauseFromSnapshot(t *testing.T) {
	raw := &gatedrive.OwnershipError{Kind: gatedrive.ErrWorktreeBusy, Op: "worktree-admission",
		Incumbent: &gatedrive.IncumbentSnapshot{Kind: "raw", RawRunID: "0123456789abcdef0123456789abcdef",
			RawRunDir: "/runs/0123456789abcdef0123456789abcdef", Owner: "raw"}}
	if got := admissionRefusalCause(raw); got != "incumbent-run:0123456789abcdef0123456789abcdef" {
		t.Fatalf("raw holder cause = %q", got)
	}
	drive := &gatedrive.OwnershipError{Kind: gatedrive.ErrWorktreeBusy, Op: "worktree-admission",
		Incumbent: &gatedrive.IncumbentSnapshot{Kind: "drive", DriveID: "0490aaaaaaaaaaaaaaaaaaaaaaaaaa01",
			RawRunID: "fedcba9876543210fedcba9876543210", ChangeID: "490", Owner: "build"}}
	if got := admissionRefusalCause(drive); got != "incumbent-drive:0490aaaaaaaaaaaaaaaaaaaaaaaaaa01" {
		t.Fatalf("drive holder cause = %q", got)
	}
	bare := &gatedrive.OwnershipError{Kind: gatedrive.ErrWorktreeBusy, Op: "worktree-admission"}
	if got := admissionRefusalCause(bare); got != "" {
		t.Fatalf("holder-unknown cause = %q, want empty", got)
	}
	if got := admissionRefusalCause(errors.New("io")); got != "" {
		t.Fatalf("non-ownership cause = %q, want empty", got)
	}
}
