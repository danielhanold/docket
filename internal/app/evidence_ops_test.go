package app

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// testGateCommand is the resolved finalize.test_command the evidence fixtures
// pin: EvidenceRecord records THIS (the observed gate command), never a value
// carried in the request. A 40-hex feature head every fixture agrees on.
const (
	testGateCommand   = "scripts/run-tests.sh"
	evidenceHead      = "abcdef0000000000000000000000000000000000"
	evidenceOtherHead = "0000000000000000000000000000000000abcdef"
)

// evidencePin builds a main-mode pin whose resolved config carries a non-empty
// finalize.test_command, so EvidenceRecord has an observed gate command to
// record. mainPin already resolves the corpus directories WorkspaceInspect
// reads; only the test command is overlaid.
func evidencePin(t *testing.T) StatusPin {
	t.Helper()
	pin := mainPin(t)
	eff := pin.Config.Effective
	eff.Finalize.TestCommand.Value = testGateCommand
	pin.Config.Effective = eff
	return pin
}

// evidenceDeps wires the read-only planning seams over a fake reader that
// returns an in-progress change (so WorkspaceInspect resolves a target) plus a
// real Git client; the workspace service is the injected fake.
func evidenceDeps(t *testing.T, svc *fakeWorkspaceService) (PlanningDeps, WorkspaceDeps, string) {
	t.Helper()
	reader := &fakeReader{
		pin:    evidencePin(t),
		corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")},
	}
	deps := workspaceDepsFor(t, reader)
	repoDir := newMainModeRepo(t, nil).invocation
	return deps, WorkspaceDeps{Service: svc}, repoDir
}

// --- run-dir fixtures: real run dirs written by the process record writer ----

// passedRunDir launches a trivially-passing gate through the landed supervisor
// and polls it to a durable terminal `passed` record, returning the run dir.
func passedRunDir(t *testing.T) string {
	t.Helper()
	return runToTerminal(t, []string{"/usr/bin/true"}, "passed")
}

// runToTerminal launches argv through GateLaunch and polls GateObserve until the
// run reaches the wanted terminal state, returning the run dir.
func runToTerminal(t *testing.T, argv []string, wantState string) string {
	t.Helper()
	root := t.TempDir()
	res := GateLaunch(root, t.TempDir(), argv)
	if res.RunDir == "" {
		t.Fatalf("launch produced no run dir: result=%s reason=%s", res.Result, res.Reason)
	}
	for i := 0; i < 300; i++ {
		obs := GateObserve(res.RunDir)
		if string(obs.State) == wantState {
			// GateObserve reports the terminal state as soon as the supervisor
			// writes terminalFile, but the supervisor then does an atomic
			// manifestFile phase-rewrite (a temp file inside the run dir) and
			// "releases the lock LAST" (internal/process/supervisor.go). Return
			// before that and the supervisor's temp-file write races the
			// t.TempDir() RemoveAll, which fails with "directory not empty".
			// Wait for the lock to free so the run dir is quiescent at cleanup.
			waitSupervisorGone(t, res.RunDir)
			return res.RunDir
		}
		if obs.State != "running" {
			t.Fatalf("run reached %q, wanted %q (reason %q)", obs.State, wantState, obs.Reason)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run never reached %q", wantState)
	return ""
}

// waitSupervisorGone blocks until the detached supervisor has released the
// run's "live.lock" (internal/process liveLockFile) — its final act, performed
// after every write to the run dir. Acquiring the flock non-blocking succeeds
// only once the supervisor is done, guaranteeing no concurrent write races a
// subsequent RemoveAll of the run dir.
func waitSupervisorGone(t *testing.T, runDir string) {
	t.Helper()
	lockPath := filepath.Join(runDir, "live.lock")
	for i := 0; i < 300; i++ {
		f, err := os.OpenFile(lockPath, os.O_RDWR, 0)
		if err != nil {
			// No lock file means no supervisor holds the run dir.
			return
		}
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			f.Close()
			return
		}
		f.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("supervisor never released %s", lockPath)
}

// runningRunDir launches a long-lived gate and returns while it is still
// running, registering a cleanup that stops it.
func runningRunDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	res := GateLaunch(root, t.TempDir(), []string{"/bin/sleep", "60"})
	if res.RunDir == "" {
		t.Fatalf("launch produced no run dir: %s", res.Reason)
	}
	t.Cleanup(func() { GateStop(res.RunDir, "test cleanup") })
	// The launch handshake already established a running group.
	if res.State != "running" {
		// Give the poll one chance in case the handle raced the observe.
		if obs := GateObserve(res.RunDir); obs.State != "running" {
			t.Fatalf("sleep run not running: %q", obs.State)
		}
	}
	return res.RunDir
}

// --- record: a passed run at the matching head yields evidence ---------------

// TestEvidenceRecordFromPassedRun: a green terminal record plus a feature head
// matching the request produces an immutable record carrying the OBSERVED gate
// command (never a request field) and the exact head; the rendered block
// round-trips through evidence.Extract.
func TestEvidenceRecordFromPassedRun(t *testing.T) {
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(evidenceHead)},
	}
	deps, wdeps, repoDir := evidenceDeps(t, svc)
	runDir := passedRunDir(t)

	res := EvidenceRecord(context.Background(), deps, wdeps, repoDir,
		EvidenceRecordRequest{ID: 7, RunDir: runDir, Head: evidenceHead})

	if res.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q msg=%q, want applied", res.Result, res.Reason, res.Message)
	}
	if res.Command != testGateCommand {
		t.Errorf("command=%q, want the observed gate command %q", res.Command, testGateCommand)
	}
	if res.Head != evidenceHead {
		t.Errorf("head=%q, want %q", res.Head, evidenceHead)
	}
	if res.Outcome != string(evidence.ResultGreen) {
		t.Errorf("outcome=%q, want green", res.Outcome)
	}
	if res.Block == "" {
		t.Fatal("no rendered evidence block")
	}
	rec, err := evidence.Extract([]byte(res.Block))
	if err != nil {
		t.Fatalf("block did not round-trip through Extract: %v", err)
	}
	if rec.Command != testGateCommand || rec.Head != evidenceHead || rec.Result != evidence.ResultGreen {
		t.Errorf("extracted record = %+v, want command/head to match and green", rec)
	}
}

// --- record: every non-passed / mismatched / probe-error path refuses --------

// TestEvidenceRecordRefusals: failed, still-running, vanished, malformed, and
// head-mismatch runs each refuse with a DISTINCT stable reason and never render
// a block. A malformed or unreadable run dir is a probe error — its own typed
// failure, never folded into the clean "vanished" absence
// (probe-error-is-not-clean-absence). The whole table is the mutation guard:
// strip the passed-only gate and the non-passed rows would produce a block.
func TestEvidenceRecordRefusals(t *testing.T) {
	cases := []struct {
		name       string
		runDir     func(t *testing.T) string
		reqHead    string
		wantReason string
		reached    bool // whether workspace Inspect should have been reached
	}{
		{
			name:       "failed run",
			runDir:     func(t *testing.T) string { return runToTerminal(t, []string{"/bin/sh", "-c", "exit 3"}, "failed") },
			reqHead:    evidenceHead,
			wantReason: ReasonEvidenceGateFailed,
		},
		{
			name:       "still-running lock",
			runDir:     runningRunDir,
			reqHead:    evidenceHead,
			wantReason: ReasonEvidenceGateRunning,
		},
		{
			name: "vanished dir",
			runDir: func(t *testing.T) string {
				d := passedRunDir(t)
				if err := os.Remove(filepath.Join(d, "terminal.json")); err != nil {
					t.Fatalf("removing terminal record: %v", err)
				}
				return d
			},
			reqHead:    evidenceHead,
			wantReason: ReasonEvidenceGateVanished,
		},
		{
			name: "malformed terminal.json",
			runDir: func(t *testing.T) string {
				d := passedRunDir(t)
				if err := os.WriteFile(filepath.Join(d, "terminal.json"), []byte("{not json"), 0o600); err != nil {
					t.Fatalf("corrupting terminal record: %v", err)
				}
				return d
			},
			reqHead:    evidenceHead,
			wantReason: ReasonEvidenceProbeMalformed,
		},
		{
			name:       "head mismatch",
			runDir:     passedRunDir,
			reqHead:    evidenceOtherHead,
			wantReason: ReasonEvidenceHeadMismatch,
			reached:    true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := &fakeWorkspaceService{
				inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(evidenceHead)},
			}
			deps, wdeps, repoDir := evidenceDeps(t, svc)
			runDir := c.runDir(t)

			res := EvidenceRecord(context.Background(), deps, wdeps, repoDir,
				EvidenceRecordRequest{ID: 7, RunDir: runDir, Head: c.reqHead})

			if res.Result == ResultApplied {
				t.Fatalf("a %s run produced a record: %+v", c.name, res)
			}
			if res.Reason != c.wantReason {
				t.Fatalf("reason=%q, want %q (result %q)", res.Reason, c.wantReason, res.Result)
			}
			if res.Block != "" {
				t.Errorf("a refusal rendered a block: %q", res.Block)
			}
			if !c.reached && len(svc.inspectCalls) != 0 {
				t.Errorf("workspace Inspect reached on a pre-inspect refusal (%d calls)", len(svc.inspectCalls))
			}
		})
	}
}

// TestEvidenceRecordUnreadableRunDir: a run dir the process cannot read is a
// probe error (its own external failure), never a silent "no evidence".
func TestEvidenceRecordUnreadableRunDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	svc := &fakeWorkspaceService{}
	deps, wdeps, repoDir := evidenceDeps(t, svc)
	runDir := passedRunDir(t)
	if err := os.Chmod(runDir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(runDir, 0o755) })

	res := EvidenceRecord(context.Background(), deps, wdeps, repoDir,
		EvidenceRecordRequest{ID: 7, RunDir: runDir, Head: evidenceHead})

	if res.Result == ResultApplied {
		t.Fatalf("an unreadable run dir produced a record: %+v", res)
	}
	if res.Reason != ReasonEvidenceProbeUnreadable {
		t.Fatalf("reason=%q, want %q", res.Reason, ReasonEvidenceProbeUnreadable)
	}
	if len(svc.inspectCalls) != 0 {
		t.Errorf("workspace Inspect reached on a probe error (%d calls)", len(svc.inspectCalls))
	}
}

// TestEvidenceRecordUnconfiguredGate: a passed run but no resolved
// finalize.test_command has no observed gate command to record, so the
// operation refuses (unsupported-config) rather than fabricate an empty command.
func TestEvidenceRecordUnconfiguredGate(t *testing.T) {
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(evidenceHead)},
	}
	reader := &fakeReader{
		pin:    mainPin(t), // built-in config: test_command resolves to unset ("")
		corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")},
	}
	deps := workspaceDepsFor(t, reader)
	repoDir := newMainModeRepo(t, nil).invocation
	runDir := passedRunDir(t)

	res := EvidenceRecord(context.Background(), deps, WorkspaceDeps{Service: svc}, repoDir,
		EvidenceRecordRequest{ID: 7, RunDir: runDir, Head: evidenceHead})

	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result=%q reason=%q, want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if res.Block != "" {
		t.Errorf("refusal rendered a block: %q", res.Block)
	}
}

// --- verify: the head pin is the invalidate-on-fix property ------------------

// TestEvidenceVerifyHeadPin: verify is green for the exact recorded head and red
// once the head changes — the property that any fix (which moves HEAD)
// invalidates the prior evidence.
func TestEvidenceVerifyHeadPin(t *testing.T) {
	rec, err := evidence.NewRecord(testGateCommand, evidenceHead, time.Now())
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	block := []byte(evidence.Render(rec))

	ok := EvidenceVerify(EvidenceVerifyRequest{RecordFile: block, Head: evidenceHead})
	if ok.Result != ResultApplied || ok.Verdict != string(evidence.VerdictVerified) {
		t.Fatalf("exact head: result=%q verdict=%q, want applied/verified", ok.Result, ok.Verdict)
	}

	stale := EvidenceVerify(EvidenceVerifyRequest{RecordFile: block, Head: evidenceOtherHead})
	if stale.Result == ResultApplied || stale.Verdict != string(evidence.VerdictStale) {
		t.Fatalf("changed head: result=%q verdict=%q, want a non-applied stale verdict", stale.Result, stale.Verdict)
	}
}

// TestEvidenceVerifyMissingAndMalformed: a body with no block is missing; a body
// whose block does not parse is malformed — each a distinct non-applied verdict.
func TestEvidenceVerifyMissingAndMalformed(t *testing.T) {
	missing := EvidenceVerify(EvidenceVerifyRequest{RecordFile: []byte("# just prose\n"), Head: evidenceHead})
	if missing.Result == ResultApplied || missing.Verdict != string(evidence.VerdictMissing) {
		t.Fatalf("missing: result=%q verdict=%q", missing.Result, missing.Verdict)
	}
	malformed := EvidenceVerify(EvidenceVerifyRequest{
		RecordFile: []byte("<!-- docket:build-evidence:start -->\ngarbage\n<!-- docket:build-evidence:end -->\n"),
		Head:       evidenceHead,
	})
	if malformed.Result == ResultApplied || malformed.Verdict != string(evidence.VerdictMalformed) {
		t.Fatalf("malformed: result=%q verdict=%q", malformed.Result, malformed.Verdict)
	}
}
