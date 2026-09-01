package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// testGateCommand is the resolved finalize.test_command the evidence fixtures
// pin: EvidenceRecord records THIS (the observed gate command), never a value
// carried in the request. A 40-hex feature head every fixture agrees on.
const (
	testGateCommand   = "go test ./..."
	evidenceHead      = "abcdef0000000000000000000000000000000000"
	evidenceOtherHead = "0000000000000000000000000000000000abcdef"
)

// evidencePin builds a main-mode pin whose resolved config carries a non-empty
// build.test_command (a local build gate) AND a finalize.test_command, so
// EvidenceRecord — which is build-owned (change 0374) — has an observed gate
// command to record. mainPin already resolves the corpus directories
// WorkspaceInspect reads; only the gate policy is overlaid.
func evidencePin(t *testing.T) StatusPin {
	t.Helper()
	pin := mainPin(t)
	eff := pin.Config.Effective
	eff.Build.Gate.Value = "local"
	eff.Build.TestCommand.Value = testGateCommand
	eff.Finalize.TestCommand.Value = testGateCommand
	pin.Config.Effective = eff
	return pin
}

// evidencePinWithYAML overlays a repository-layer .docket.yml onto the evidence
// pin, preserving mainPin's corpus directories. It is how the build-owned
// EvidenceRecord fixtures declare divergent build/finalize gate policy.
func evidencePinWithYAML(t *testing.T, yaml string) StatusPin {
	t.Helper()
	snap, _, err := config.Resolve(
		[]config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: []byte(yaml)}},
		config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("resolve config overlay: %v", err)
	}
	pin := mainPin(t)
	eff := pin.Config.Effective
	eff.Build = snap.Effective.Build
	eff.Finalize = snap.Effective.Finalize
	pin.Config.Effective = eff
	return pin
}

// evidenceDepsWithConfig mirrors evidenceDeps but overlays a repository-layer
// YAML config so a test can declare build.gate/build.test_command divergently.
func evidenceDepsWithConfig(t *testing.T, svc *fakeWorkspaceService, yaml string) (PlanningDeps, WorkspaceDeps, string) {
	t.Helper()
	reader := &fakeReader{
		pin:    evidencePinWithYAML(t, yaml),
		corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")},
	}
	deps := workspaceDepsFor(t, reader)
	repoDir := newWorkingRepo(t, nil).invocation
	return deps, WorkspaceDeps{Service: svc}, repoDir
}

// currentFeatureHead is the head every evidence fixture's workspace inspection
// reports; a request carrying it passes the head-equality precondition.
func currentFeatureHead(t *testing.T) string {
	t.Helper()
	return evidenceHead
}

// readyWorkspace returns a fake workspace service inspecting to the current
// feature head, so EvidenceRecord's head-equality check is satisfied.
func readyWorkspace() *fakeWorkspaceService {
	return &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(evidenceHead)},
	}
}

// evidenceRecordWithConfig runs EvidenceRecord under the given repository YAML
// overlay and request (no run is observed unless req.RunDir is set — the gate-off
// and unconfigured paths never reach observation).
func evidenceRecordWithConfig(t *testing.T, yaml string, req EvidenceRecordRequest) EvidenceOpResult {
	t.Helper()
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), yaml)
	return EvidenceRecord(context.Background(), deps, wdeps, repoDir, req)
}

// evidenceRecordPassedRun runs EvidenceRecord over a real passed gate run dir
// under the given repository YAML overlay, at the current feature head.
func evidenceRecordPassedRun(t *testing.T, yaml string) EvidenceOpResult {
	t.Helper()
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), yaml)
	runDir := passedRunDir(t)
	return EvidenceRecord(context.Background(), deps, wdeps, repoDir,
		EvidenceRecordRequest{ID: 7, RunDir: runDir, Head: evidenceHead})
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
	repoDir := newWorkingRepo(t, nil).invocation
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

// --- record: every non-passed / mismatched / probe-error path refuses --------

// --- record: build-owned gate policy (change 0374) ---------------------------

// TestEvidenceRecordBuildGateOffMintsSkipped: build.gate: off is an explicit
// no-gate policy. No run is observed (RunDir empty), the head must still be the
// current feature head, and the block is the truthful skipped record.
func TestEvidenceRecordBuildGateOffMintsSkipped(t *testing.T) {
	res := evidenceRecordWithConfig(t, "build:\n  gate: \"off\"\n",
		EvidenceRecordRequest{ID: 7, Head: currentFeatureHead(t)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %v (%s), want applied", res.Result, res.Reason)
	}
	if res.Outcome != "skipped" || !strings.Contains(res.Block, "build-gate-off") {
		t.Errorf("outcome/block = %q/%q, want skipped/build-gate-off", res.Outcome, res.Block)
	}
	if res.Command != "" {
		t.Errorf("a skipped record carries no command, got %q", res.Command)
	}
}

// TestEvidenceRecordUnconfiguredBuildCommandIsTypedSetupRefusal: a local build
// gate with no build.test_command is a typed setup refusal that names the
// remedy command — never a fabricated empty command.
func TestEvidenceRecordUnconfiguredBuildCommandIsTypedSetupRefusal(t *testing.T) {
	res := evidenceRecordWithConfig(t, "build:\n  gate: local\n",
		EvidenceRecordRequest{ID: 7, Head: currentFeatureHead(t), RunDir: t.TempDir()})
	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result/reason = %v/%s, want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if !strings.Contains(res.Message, "docket repository configure-tests") {
		t.Errorf("message %q must name the setup remedy", res.Message)
	}
}

// TestEvidenceRecordRecordsBuildCommandNotFinalize: the divergent-command
// fixture. EvidenceRecord is build-owned, so the recorded command is
// build.test_command — swapping the source to finalize.test_command reddens
// this test (the guard the divergent fixture exists for).
func TestEvidenceRecordRecordsBuildCommandNotFinalize(t *testing.T) {
	res := evidenceRecordPassedRun(t, "build:\n  gate: local\n  test_command: go test ./build-only\nfinalize:\n  test_command: make finalize-only\n")
	if res.Result != ResultApplied {
		t.Fatalf("result = %v (%s: %s), want applied", res.Result, res.Reason, res.Message)
	}
	if res.Command != "go test ./build-only" {
		t.Errorf("recorded command = %q; evidence must record build.test_command", res.Command)
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

// TestEvidenceVerifySkippedAtExactHead: a truthful skipped (build-gate-off)
// record at the exact head verifies as applied — it must not be mistaken for
// malformed input. This mirrors the four publication gates that accept
// VerdictSkipped.
func TestEvidenceVerifySkippedAtExactHead(t *testing.T) {
	rec, err := evidence.NewSkippedRecord(evidenceHead, time.Now())
	if err != nil {
		t.Fatalf("NewSkippedRecord: %v", err)
	}
	block := []byte(evidence.Render(rec))

	res := EvidenceVerify(EvidenceVerifyRequest{RecordFile: block, Head: evidenceHead})
	if res.Result != ResultApplied || res.Verdict != string(evidence.VerdictSkipped) {
		t.Fatalf("skipped at exact head: result=%q verdict=%q reason=%q, want applied/skipped",
			res.Result, res.Verdict, res.Reason)
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
