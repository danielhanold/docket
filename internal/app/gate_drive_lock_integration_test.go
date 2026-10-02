//go:build integration

package app

// End-to-end tests for build-start admission on the worktree lock (change 0490)
// through the application gate-drive seam: the REAL driver, the REAL process
// supervisor, and a real temp git worktree. A busy worktree refuses a build start
// before anything is created or charged, and a busy refusal names a running
// holder. The unit-level refusal mapping is pinned in gate_drive_test.go.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

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

// reapRunSupervisors plays init's role for supervisors THIS test process spawned
// in-process: it Wait4s (WNOHANG) the exact supervisor pids recorded in the run
// manifests under runRoot until cleanup, so an exited supervisor is reaped and a
// stop can prove its process group absent instead of waiting out its bound on a
// zombie group leader. It never waits on -1, so it steals no other child's
// status. (It mirrors gatedrive's reapSupervisors.)
func reapRunSupervisors(t *testing.T, runRoot string) {
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
			for _, name := range mustDirEntries(runRoot) {
				raw, err := os.ReadFile(filepath.Join(runRoot, name, "manifest.json"))
				if err != nil {
					continue
				}
				var m struct {
					SupervisorPID int `json:"supervisor_pid"`
				}
				if json.Unmarshal(raw, &m) != nil || m.SupervisorPID <= 1 {
					continue
				}
				var ws syscall.WaitStatus
				_, _ = syscall.Wait4(m.SupervisorPID, &ws, syscall.WNOHANG, nil)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	t.Cleanup(func() { close(done); wg.Wait() })
}

// buildLockFixture is a real repo with a build-owned gate-drive service rooted at
// its Git common dir, a run root whose runs are stopped at cleanup, and the
// canonical worktree root the lock keys on.
func buildLockFixture(t *testing.T, command string) (svc *GateDriveService, worktree, gitDir, root, runRoot string) {
	t.Helper()
	requireRealGit(t)
	requireProcessSupervisor(t)
	worktree, gitDir = initGitRepo(t, "")
	runRoot = filepath.Join(testsupport.TempDir(t), "runs")
	svc, res, reason := NewBuildGateDriveService(gitDir, guardianExecutable(t), buildEffWithMaxAttempts(command, 4))
	if svc == nil {
		t.Fatalf("build gate-drive service was nil: %s %s", res, reason)
	}
	t.Cleanup(func() {
		for _, e := range mustDirEntries(runRoot) {
			GateStop(filepath.Join(runRoot, e), "test cleanup")
		}
	})
	wt, err := newGitClient(t).DiscoverWorktree(context.Background(), gitcli.DiscoverOptions{InvocationPath: worktree})
	if err != nil {
		t.Fatalf("DiscoverWorktree: %v", err)
	}
	return svc, worktree, gitDir, wt.Root, runRoot
}

func buildLockStartReq(worktree, runRoot, changeID string) GateDriveStartRequest {
	return GateDriveStartRequest{
		RepoDir: worktree, Worktree: worktree,
		ChangeID: changeID, Phase: "build",
		Branch: "fix/x", Ref: "refs/heads/fix/x", Cwd: worktree,
		RunRoot: runRoot,
	}
}

// TestIntegrationBuildStartBusyWorktreeCreatesNoDriveAndChargesNothing (L5): a
// build start into a worktree whose lock another gate holds is refused
// worktree-busy at the worktree-admission stage, with no drive created, no run
// launched, and no suite attempt charged. Once the holder lets go, the next
// start is admitted with no recovery step and charges exactly one attempt.
func TestIntegrationBuildStartBusyWorktreeCreatesNoDriveAndChargesNothing(t *testing.T) {
	svc, worktree, gitDir, root, runRoot := buildLockFixture(t, "/bin/echo hi")
	store := gatedrive.OpenStore(gitDir)
	held, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatalf("hold the worktree lock: %v", err)
	}
	defer held.Release()

	req := buildLockStartReq(worktree, runRoot, "0490")
	got := svc.Start(req)
	if got.Result == ResultApplied || got.Drive != nil {
		t.Fatalf("a busy worktree must refuse the build start, got result=%s drive=%v", got.Result, got.Drive)
	}
	if got.Reason != string(gatedrive.ErrWorktreeBusy) || got.Stage != stageWorktreeAdmission {
		t.Fatalf("refusal = reason %q stage %q, want worktree-busy at %s", got.Reason, got.Stage, stageWorktreeAdmission)
	}
	if n := len(mustDirEntries(filepath.Join(gitDir, "docket", "gate-drives", "v2"))); n != 0 {
		t.Fatalf("a busy refusal must create no drive, found %d", n)
	}
	if n := countRunDirs(t, runRoot); n != 0 {
		t.Fatalf("a busy refusal must launch nothing, found %d run dirs", n)
	}
	key := gatedrive.SuiteBudgetKey{RepoIdentity: worktree, ChangeID: "0490", Phase: "build"}
	if used, _, err := store.SuiteBudgetUsage(key); err != nil || used != 0 {
		t.Fatalf("a busy refusal must charge nothing, got used=%d err=%v", used, err)
	}

	held.Release()
	next := svc.Start(req)
	if next.Result != ResultApplied || next.Drive == nil {
		t.Fatalf("once the holder lets go the next start must be admitted, got result=%s reason=%q", next.Result, next.Reason)
	}
	if next.Drive.Outcome == gatedrive.WAITING {
		driveDoneOrFail(t, svc, next.Drive.DriveID, next.Drive.Generation)
	}
	if n := countRunDirs(t, runRoot); n != 1 {
		t.Fatalf("the admitted start must launch exactly one run, found %d", n)
	}
	if used, _, err := store.SuiteBudgetUsage(key); err != nil || used != 1 {
		t.Fatalf("the admitted start must charge exactly one attempt, got used=%d err=%v", used, err)
	}
}

// TestIntegrationBuildStartBusyRefusalNamesRunningHolder (L5): while one build
// start's suite runs, a second build start into the same worktree is refused
// worktree-busy with the running holder's locator (incumbent-drive:<id>) and a
// remedy naming its change and run.cancel. The first start blocks inside its
// slice while its suite runs, so it is driven on a goroutine and released by
// stopping its run.
func TestIntegrationBuildStartBusyRefusalNamesRunningHolder(t *testing.T) {
	svc, worktree, gitDir, _, runRoot := buildLockFixture(t, "sleep 60")
	reapRunSupervisors(t, runRoot)

	firstDone := make(chan GateDriveResult, 1)
	go func() { firstDone <- svc.Start(buildLockStartReq(worktree, runRoot, "0490")) }()

	// Wait until the first start launched its run and recorded the holder note.
	lockRoot := filepath.Join(gitDir, "docket", "worktree-locks")
	var runDir string
	deadline := time.Now().Add(30 * time.Second)
	for {
		var dirs []string
		for _, e := range mustDirEntries(runRoot) {
			if gateRunIDRe.MatchString(e) {
				dirs = append(dirs, e)
			}
		}
		keys := mustDirEntries(lockRoot)
		if len(dirs) == 1 && len(keys) == 1 {
			if _, err := os.Stat(filepath.Join(lockRoot, keys[0], "holder.json")); err == nil {
				runDir = filepath.Join(runRoot, dirs[0])
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the first start never launched and recorded its holder (runs=%v locks=%v)", dirs, keys)
		}
		time.Sleep(10 * time.Millisecond)
	}
	drives := mustDirEntries(filepath.Join(gitDir, "docket", "gate-drives", "v2"))
	if len(drives) != 1 {
		t.Fatalf("want exactly the first drive, got %v", drives)
	}
	firstID := drives[0]

	second := svc.Start(buildLockStartReq(worktree, runRoot, "0491"))
	stopped := GateStop(runDir, "test: end the holder")
	first := <-firstDone

	if second.Result == ResultApplied || second.Reason != string(gatedrive.ErrWorktreeBusy) {
		t.Fatalf("a start while the first suite runs = result %s reason %q, want worktree-busy", second.Result, second.Reason)
	}
	if second.Stage != stageWorktreeAdmission || second.Locator != "incumbent-drive:"+firstID {
		t.Fatalf("refusal stage/locator = %q/%q, want %s/incumbent-drive:%s", second.Stage, second.Locator, stageWorktreeAdmission, firstID)
	}
	if !strings.Contains(second.Message, "change 0490") || !strings.Contains(second.Message, "run.cancel") {
		t.Fatalf("refusal remedy must name the holder's change and run.cancel, got %q", second.Message)
	}
	if n := len(mustDirEntries(filepath.Join(gitDir, "docket", "gate-drives", "v2"))); n != 1 {
		t.Fatalf("the refused start must create no drive, found %d", n)
	}
	if first.Drive == nil || first.Drive.DriveID != firstID {
		t.Fatalf("the first start must return its own drive once its run is stopped: %+v (stop=%s %s)", first, stopped.Result, stopped.Reason)
	}
}
