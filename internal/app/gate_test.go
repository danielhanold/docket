package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// TestMain routes the supervisor re-exec role of the app test binary: a real
// GateLaunch re-executes this binary with the private supervisor env var set,
// and it must become the supervisor rather than re-running the test suite.
// Ordinary `go test` runs set neither and fall through to m.Run.
func TestMain(m *testing.M) {
	if process.SupervisorRequested() {
		os.Exit(process.RunSupervisorFromEnv())
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

func TestGateLaunchObserveEndToEnd(t *testing.T) {
	root := t.TempDir()
	res := GateLaunch(root, t.TempDir(), []string{"/bin/echo", "hello"})
	if res.Operation != "gate.launch" {
		t.Fatalf("operation %q", res.Operation)
	}
	if res.Result != ResultApplied {
		t.Fatalf("launch result %s (%s)", res.Result, res.Reason)
	}
	if res.RunDir == "" || res.RunID == "" {
		t.Fatalf("no handle: %+v", res)
	}
	// Poll observe to terminal; /bin/echo exits 0 fast.
	deadline := 300 // x100ms
	for i := 0; ; i++ {
		obs := GateObserve(res.RunDir)
		if obs.Result == ResultApplied && obs.State == "passed" {
			if obs.ExitCode == nil || *obs.ExitCode != 0 {
				t.Fatalf("exact code: %+v", obs.ExitCode)
			}
			break
		}
		if obs.State != "running" {
			t.Fatalf("unexpected: %+v", obs)
		}
		if i > deadline {
			t.Fatal("echo never became terminal")
		}
		time.Sleep(100 * time.Millisecond)
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
	res := GateRecover(t.TempDir())
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
