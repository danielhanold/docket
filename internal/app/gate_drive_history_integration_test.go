//go:build integration

package app

// End-to-end acceptance tests for the legacy-history admission composition through
// the application gate-drive seam (change 0428, Task 10). Criterion 1 drives the
// REAL driver + REAL process supervisor + a real temp git worktree through
// GateDriveService; Criterion 7's non-inventory refusal is a deterministic
// fake-engine mapping check. The unit-level mapping of an inventory refusal's
// stage/locator/summary is pinned separately in gate_drive_test.go (Task 6).

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/testsupport"
)

// Exercise the real slot left by a completed drive, cancellation, a resume that
// never launched, and another resume. The slot can lag multiple epochs.
func TestIntegrationBuildStartAfterCancelledResumeChain(t *testing.T) {
	requireProcessSupervisor(t)
	primary, common := initGitRepo(t, "")
	runGit(t, primary, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "base")
	worktree := filepath.Join(testsupport.TempDir(t), "feature")
	runGit(t, primary, "worktree", "add", "-b", "fix/replacement", worktree, "HEAD")
	store := gatedrive.OpenStore(common)
	key, err := MintGateRecord(worktree, GateRecord{Target: gateBeforeStoredTarget, AttemptLimit: 2, Retry: RetryUnused, Disposition: "gate-armed", AttributedID: 5, ScopeID: "outer", ParentCap: "parent"})
	if err != nil {
		t.Fatal(err)
	}
	ep, err := MintEpochRecord(worktree, key, "5")
	if err != nil {
		t.Fatal(err)
	}
	if err := bindEpochWorktree(worktree, key, worktree); err != nil {
		t.Fatal(err)
	}
	svc, res, reason := NewBuildGateDriveService(common, guardianExecutable(t), buildEffWithMaxAttempts("/bin/echo hi", 4))
	if svc == nil {
		t.Fatalf("service: %s %s", res, reason)
	}
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	start := func(epoch string) GateDriveResult {
		scope := svc.PrepareScope(gatedrive.ScopeRequest{RepoIdentity: common, Worktree: worktree, ChangeID: "5", TaskID: "task", Phase: "build", Branch: "fix/replacement", RunEpochID: epoch, RunRoot: runRoot})
		if scope.ScopeID == "" {
			t.Fatalf("scope: %+v", scope)
		}
		return svc.Start(GateDriveStartRequest{RepoDir: common, Worktree: worktree, ChangeID: "5", TaskID: "task", Phase: "build", Branch: "fix/replacement", Ref: "HEAD", Cwd: worktree, RunRoot: runRoot, ScopeID: scope.ScopeID, ChildCapability: scope.ChildCapability, RunEpochID: epoch})
	}
	first := start(ep.EpochID)
	if first.Result != ResultApplied || first.Drive == nil || first.Drive.Outcome != gatedrive.PASSED {
		t.Fatalf("first start: %+v", first)
	}
	for i := 0; i < 2; i++ {
		cancel := runCancel(cancelSeams{store: store, stopper: &fakeCancelStopper{}}, worktree, key, ep.EpochID, "test human-authorized resume")
		if cancel.Disposition != CancelDispositionCancelled {
			t.Fatalf("cancel: %+v", cancel)
		}
		deps, wdeps := resumeEpochDeps(t)
		wdeps.Service = resumeInspectService(worktree)
		armed := RunGateBefore(context.Background(), deps, wdeps, GateScopeDeps{Prepare: store.PrepareScope}, worktree, "implement-next", 5)
		if !armed.Armed {
			t.Fatalf("resume: %s", armed.HumanText())
		}
		key = armed.Key
		ep, _, err = LoadEpochRecord(worktree, key)
		if err != nil {
			t.Fatal(err)
		}
	}
	missingScope := svc.Start(GateDriveStartRequest{RepoDir: common, Worktree: worktree, ChangeID: "5", Phase: "build", Branch: "fix/replacement", Ref: "HEAD", Cwd: worktree, RunRoot: runRoot, RunEpochID: ep.EpochID})
	if missingScope.Reason != "epoch-scope-required" || missingScope.Message == "" || missingScope.Drive != nil {
		t.Fatalf("scope omission must explain the remedy without a drive: %+v", missingScope)
	}
	budgetKey := gatedrive.SuiteBudgetKey{RepoIdentity: common, ChangeID: "5", Phase: "build"}
	if used, limit, err := store.SuiteBudgetUsage(budgetKey); err != nil || used != 1 || limit != 4 {
		t.Fatalf("refusal charged or reset budget: %d/%d %v", used, limit, err)
	}
	last := start(ep.EpochID)
	if last.Result != ResultApplied || last.Drive == nil || last.Drive.Outcome != gatedrive.PASSED {
		t.Fatalf("replacement must launch over released ancestor slot: %+v", last)
	}
	if n := countRunDirs(t, runRoot); n != 2 {
		t.Fatalf("launched %d runs, want exactly original and replacement", n)
	}
	if used, limit, err := store.SuiteBudgetUsage(budgetKey); err != nil || used != 2 || limit != 4 {
		t.Fatalf("replacement budget: %d/%d %v", used, limit, err)
	}
}

// requireProcessSupervisor skips on a platform where the native process supervisor
// is not built (internal/process gates Launch on darwin/linux only), mirroring the
// gatedrive integration suite's one platform guard.
func requireProcessSupervisor(t *testing.T) {
	t.Helper()
	switch runtime.GOOS {
	case "darwin", "linux":
	default:
		t.Skipf("native process supervisor is unsupported on %s", runtime.GOOS)
	}
}

var gateRunIDRe = regexp.MustCompile("^[0-9a-f]{32}$")

// countRunDirs returns the number of native run directories (32-hex names) under
// root — the proof of how many raw runs a drive launched.
func countRunDirs(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("reading run root %s: %v", root, err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() && gateRunIDRe.MatchString(e.Name()) {
			n++
		}
	}
	return n
}

// seedLegacyV2Passed installs the frozen v2 PASSED fixture (a completed pre-0375
// drive whose worktree is removed) as drive id's record under the store rooted at
// gitDir, so a first-admission census over gitDir actually assesses one legacy
// record. The fixture is the sibling gatedrive package's frozen testdata — the same
// bytes Task 2's reader tests validate — read relative to this package's source dir
// (go test runs with cwd = the package directory).
func seedLegacyV2Passed(t *testing.T, gitDir, id string) {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("..", "gatedrive", "testdata", "legacy-v2", "passed.json"))
	if err != nil {
		t.Fatalf("read frozen v2 PASSED fixture: %v", err)
	}
	dir := filepath.Join(gitDir, "docket", "gate-drives", "v1", id) // gatedrive.OpenStore layout
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "record.json"), buf, 0o600); err != nil {
		t.Fatalf("seed record: %v", err)
	}
}

// TestIntegrationBuildStartAdmitsOverLegacyPassedHistoryOneLaunch is Criterion 1: over a repo
// whose gate-drive store carries a completed pre-0375 (schema-2) PASSED drive bound
// to a REMOVED worktree, one ordinary build-owned scoped Start through the REAL
// GateDriveService (real driver, real process supervisor, fake-fast /bin/echo suite
// command) admits normally — it reaches launch with the legacy history recognised
// nonblocking (Checked 1, none recovered, none retained), launches EXACTLY ONE raw
// run, and charges EXACTLY ONE full-suite attempt. No prior manual cleanup and no
// second start are required for the completed history to be non-blocking.
func TestIntegrationBuildStartAdmitsOverLegacyPassedHistoryOneLaunch(t *testing.T) {
	requireRealGit(t)
	requireProcessSupervisor(t)
	worktree, gitDir := initGitRepo(t, "")
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")

	seedLegacyV2Passed(t, gitDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01")

	svc, res, reason := NewBuildGateDriveService(gitDir, guardianExecutable(t), buildEffWithMaxAttempts("/bin/echo hi", 4))
	if svc == nil {
		t.Fatalf("build gate-drive service was nil: %s %s", res, reason)
	}
	t.Cleanup(func() {
		for i := 0; i < 4; i++ {
			for _, e := range mustDirEntries(runRoot) {
				GateStop(filepath.Join(runRoot, e), "test cleanup")
			}
		}
	})

	scope := svc.PrepareScope(gatedrive.ScopeRequest{
		RepoIdentity: worktree, Worktree: worktree,
		ChangeID: "0428", TaskID: "task-10", Phase: "build", Branch: "fix/x",
	})
	if scope.ScopeID == "" || scope.ChildCapability == "" {
		t.Fatalf("PrepareScope: %s (%s)", scope.Result, scope.Reason)
	}

	req := GateDriveStartRequest{
		RepoDir: worktree, Worktree: worktree,
		ChangeID: "0428", TaskID: "task-10", Phase: "build",
		Branch: "fix/x", Ref: "refs/heads/fix/x", Cwd: worktree,
		RunRoot:             runRoot,
		ScopeID:             scope.ScopeID,
		ChildCapability:     scope.ChildCapability,
		IdempotentSuiteGate: true,
	}
	got := svc.Start(req)
	if got.Result != ResultApplied || got.Drive == nil {
		t.Fatalf("scoped build start over legacy PASSED history must apply, got result=%s reason=%q drive=%v", got.Result, got.Reason, got.Drive)
	}

	// The completed legacy history was assessed and recognised nonblocking on the
	// START document: one checked, none recovered, none retained.
	sum := got.Drive.LegacyHistory
	if sum == nil {
		t.Fatalf("start document must carry the legacy-history summary when a legacy record was assessed")
	}
	if sum.Checked != 1 || len(sum.Recovered) != 0 || len(sum.Retained) != 0 {
		t.Fatalf("legacy summary = checked %d recovered %d retained %d, want checked 1 / 0 / 0", sum.Checked, len(sum.Recovered), len(sum.Retained))
	}

	// EXACTLY ONE raw run launched under the run root: the completed history added
	// no cleanup run and no second start. (Drive a WAITING launch to its terminal so
	// the assertion is stable and no child is left live.)
	if got.Drive.Outcome == gatedrive.WAITING {
		driveDoneOrFail(t, svc, got.Drive.DriveID, got.Drive.Generation)
	}
	if n := countRunDirs(t, runRoot); n != 1 {
		t.Fatalf("want EXACTLY one raw run dir under the run root, got %d", n)
	}

	// EXACTLY ONE full-suite attempt charged for this build phase.
	store := gatedrive.OpenStore(gitDir)
	used, limit, err := store.SuiteBudgetUsage(gatedrive.SuiteBudgetKey{RepoIdentity: worktree, ChangeID: "0428", Phase: "build"})
	if err != nil {
		t.Fatalf("SuiteBudgetUsage: %v", err)
	}
	if used != 1 || limit != 4 {
		t.Fatalf("suite-attempt usage = (%d,%d), want (1,4)", used, limit)
	}
}

// mustDirEntries lists the immediate entry names under dir (empty if absent); the
// run-root stop-cleanup uses it to end any run a WAITING drive left live.
func mustDirEntries(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// driveDoneOrFail advances a WAITING drive through the service until it leaves
// WAITING or a generous deadline elapses, so a fast /bin/echo suite reaches its
// terminal without leaving a live child.
func driveDoneOrFail(t *testing.T, svc *GateDriveService, id, gen string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		got := svc.Advance(id, gen)
		if got.Drive == nil {
			t.Fatalf("advance produced no drive document: result=%s reason=%q", got.Result, got.Reason)
		}
		if got.Drive.Outcome != gatedrive.WAITING {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("drive never left WAITING within 30s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestIntegrationBuildStartBusySlotRefusalCarriesNoInventoryStageLocator is Criterion 7's
// non-inventory negative space, exercised end-to-end through Start: a worktree-busy
// admission refusal (a NON-inventory ownership error) carries no legacy-inventory
// stage, locator, or summary, and keeps its existing slot-recovery next-action
// message. It complements the inventory-refusal mapping pinned in gate_drive_test.go.
func TestIntegrationBuildStartBusySlotRefusalCarriesNoInventoryStageLocator(t *testing.T) {
	svc, eng, _ := newBudgetTestBuildService(t, 4)
	eng.admitErr = &gatedrive.OwnershipError{Kind: gatedrive.ErrWorktreeBusy, Op: "reserve-worktree-execution"}

	got := svc.Start(buildStartReq("0428"))
	if got.Result == ResultApplied || got.Drive != nil {
		t.Fatalf("a worktree-busy admission must refuse, got result=%s drive=%v", got.Result, got.Drive)
	}
	if got.Reason != string(gatedrive.ErrWorktreeBusy) {
		t.Fatalf("refusal reason = %q, want %q", got.Reason, gatedrive.ErrWorktreeBusy)
	}
	if got.Stage != "" || got.Locator != "" || got.LegacyHistory != nil {
		t.Fatalf("a non-inventory busy-slot refusal must carry no stage/locator/legacy, got %q/%q/%v", got.Stage, got.Locator, got.LegacyHistory)
	}
	if got.Message != ownershipNextAction(gatedrive.ErrWorktreeBusy) {
		t.Fatalf("busy-slot refusal must keep its slot-recovery next-action message, got %q", got.Message)
	}
}
