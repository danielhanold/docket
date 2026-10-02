//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_gatelifecycle.sh (prefix ^TestIntegrationGateLifecycle).
// TestMain stays in the untagged gate_test.go: both builds need its supervisor and
// guardian re-exec routing.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

func TestIntegrationGateLifecycleGateLaunchInvalidInput(t *testing.T) {
	res := GateLaunch("relative-root", "/", []string{"/bin/echo"})
	if res.Result != ResultInvalidInput {
		t.Fatalf("result %s", res.Result)
	}
	if ExitCode(res.Result) != 2 {
		t.Fatalf("exit mapping")
	}
}

// --- change 0490: raw gate.launch takes the worktree lock ---

// TestIntegrationGateLifecycleRawLaunchHoldsWorktreeLock proves a raw launch whose
// cwd sits inside a registered worktree hands the worktree lock to its supervisor:
// while the run lives, the lock is busy, the busy refusal names the raw run as its
// cause, and holder.json names kind=raw, owner=raw, and the run dir.
func TestIntegrationGateLifecycleRawLaunchHoldsWorktreeLock(t *testing.T) {
	requireRealGit(t)
	worktree, gitDir := initGitRepo(t, "")
	runRoot := testsupport.TempDir(t)
	reapRunSupervisors(t, runRoot) // so the cleanup stop proves the group gone promptly
	res := GateLaunch(runRoot, worktree, []string{"/bin/sleep", "60"})
	t.Cleanup(func() { GateStop(res.RunDir, "test cleanup") })
	if res.Result != ResultApplied || res.RunDir == "" {
		t.Fatalf("launch: result=%s reason=%q rundir=%q", res.Result, res.Reason, res.RunDir)
	}
	root, store, ok := resolveWorktreeAdmission(worktree)
	if !ok {
		t.Fatal("resolveWorktreeAdmission: worktree not resolved")
	}
	svc, _, reason := gateService()
	if svc == nil {
		t.Fatalf("gate service: %s", reason)
	}
	lock, err := store.TryWorktreeLock(root, svc)
	if err == nil {
		lock.Release()
		t.Fatal("the worktree lock must be busy while the raw run lives")
	}
	oe, isOwn := gatedrive.AsOwnershipError(err)
	if !isOwn || oe.Kind != gatedrive.ErrWorktreeBusy {
		t.Fatalf("TryWorktreeLock = %v, want worktree-busy", err)
	}
	if got := admissionRefusalCause(err); got != "incumbent-run:"+res.RunID {
		t.Fatalf("busy refusal cause = %q, want incumbent-run:%s", got, res.RunID)
	}
	// holder.json sits beside busy.lock under <common>/docket/worktree-locks/<sha256(root)>/.
	sum := sha256.Sum256([]byte(root))
	buf, err := os.ReadFile(filepath.Join(gitDir, "docket", "worktree-locks", hex.EncodeToString(sum[:]), "holder.json"))
	if err != nil {
		t.Fatalf("holder.json beside the lock: %v", err)
	}
	var note gatedrive.HolderNote
	if err := json.Unmarshal(buf, &note); err != nil {
		t.Fatalf("holder.json: %v", err)
	}
	if note.Kind != "raw" || note.Owner != "raw" || note.RunDir != res.RunDir || note.DriveID != "" {
		t.Fatalf("holder note = %+v, want kind=raw owner=raw run_dir=%s", note, res.RunDir)
	}
}

// TestRaceIntegrationAppConcurrencyGateLaunchSecondRefusedWhileFirstLives proves a second raw launch into a
// worktree whose lock a live supervisor holds is refused worktree-busy — even from a
// DISTINCT run root — with no process spawned, and that the refusal locates the
// incumbent run (a safe locator) without leaking a launch token.
// Race shard (change 0465): two raw launches contend for one worktree lock while the first run is still live.
func TestRaceIntegrationAppConcurrencyGateLaunchSecondRefusedWhileFirstLives(t *testing.T) {
	requireRealGit(t)
	worktree, _ := initGitRepo(t, "")
	// The first run is genuinely LIVE (a long sleep), so its supervisor still holds
	// the worktree lock when the second launch tries it.
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

// TestIntegrationGateLifecycleGateLaunchOutsideGitUnchanged proves a launch whose cwd is outside any git
// worktree keeps its pre-admission contract: no worktree lock is taken, so a second
// launch in the same non-worktree cwd is not refused.
func TestIntegrationGateLifecycleGateLaunchOutsideGitUnchanged(t *testing.T) {
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

// TestIntegrationGateLifecycleStoppedRawRunFreesWorktree proves a raw run's end
// frees its worktree with no release step: once gate.stop has stopped a live raw
// run and its supervisor is gone, the next raw launch into the worktree admits.
// gate.stop only stops — the kernel frees the lock when the supervisor exits.
func TestIntegrationGateLifecycleStoppedRawRunFreesWorktree(t *testing.T) {
	requireRealGit(t)
	worktree, _ := initGitRepo(t, "")
	// The reaper plays init's role for the in-process supervisor, as production's
	// exited launcher leaves it orphaned to init: without it the exited supervisor
	// stays a zombie group leader and the stop waits out its whole TERM bound.
	runRoot := testsupport.TempDir(t)
	reapRunSupervisors(t, runRoot)
	res := GateLaunch(runRoot, worktree, []string{"/bin/sleep", "60"})
	t.Cleanup(func() { GateStop(res.RunDir, "test cleanup") })
	if res.Result != ResultApplied || res.RunDir == "" {
		t.Fatalf("launch: result=%s reason=%q", res.Result, res.Reason)
	}
	if busy := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/echo", "hi"}); busy.Reason != "worktree-busy" {
		if busy.RunDir != "" {
			GateStop(busy.RunDir, "test cleanup")
		}
		t.Fatalf("second launch while the first lives = %s/%q, want worktree-busy", busy.Result, busy.Reason)
	}
	if stop := GateStop(res.RunDir, "test stop"); stop.Result != ResultApplied || stop.State != GateState(process.StateStopped) {
		t.Fatalf("stop of the live raw run = %s/%s (reason %q), want applied/stopped", stop.Result, stop.State, stop.Reason)
	}
	waitGateRunTerminal(t, res.RunDir)
	readmit := waitLaunchAdmitted(t, worktree)
	t.Cleanup(func() { waitGateRunTerminal(t, readmit.RunDir); GateStop(readmit.RunDir, "test cleanup") })
}

// waitLaunchAdmitted launches into worktree until a launch is admitted: a stopped
// run's state turns terminal while its supervisor is still exiting, and the lock
// frees only when the supervisor is gone. A refusal other than worktree-busy fails.
func waitLaunchAdmitted(t *testing.T, worktree string) GateResult {
	t.Helper()
	for i := 0; i < 300; i++ {
		r := GateLaunch(testsupport.TempDir(t), worktree, []string{"/bin/echo", "hi"})
		if r.Result == ResultApplied && r.RunDir != "" {
			return r
		}
		if r.Reason != "worktree-busy" {
			t.Fatalf("readmission: result=%s reason=%q", r.Result, r.Reason)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the worktree never freed after its raw run was stopped")
	return GateResult{}
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
