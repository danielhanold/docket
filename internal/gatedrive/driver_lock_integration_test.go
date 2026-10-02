//go:build integration

package gatedrive

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationGatedriveKilledSupervisorFreesWorktreeForNextStart (L1,
// change 0490): the worktree lock lives in the gate supervisor, so a supervisor
// killed with SIGKILL frees the worktree by the kernel alone. Against the real
// process supervisor and a real git worktree: while the first drive's supervisor
// runs, a second start is refused worktree-busy naming the live holder; once the
// supervisor is killed and gone, the very next start is admitted with no
// recovery call in between. The killed supervisor's suite survives it (the
// accepted teardown gap, change 0492), so cleanup ends its process group.
func TestIntegrationGatedriveKilledSupervisorFreesWorktreeForNextStart(t *testing.T) {
	skipUnlessSupported(t)
	svc := mustService(t)
	repo := testsupport.TempDir(t)
	gitInit(t, repo)
	writeFile(t, repo, "a.txt", "a\n")
	gitAdd(t, repo, "a.txt")
	gitCommit(t, repo, "init")
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	store := OpenStore(testsupport.TempDir(t))
	// The reaper is registered FIRST so it outlives the cleanup stop: the next
	// start's run is still live at cleanup, and Stop proves its teardown only once
	// the exited supervisor is reaped.
	reapSupervisors(t, runRoot)
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })

	d := NewDriver(store, systemClock{}, svc, realGit{})
	d.slice = intSlice
	d.pollInterval = intPoll
	d.sleep = time.Sleep

	req := intStartRequest(mustExe(t), runRoot, repo, "sleep-forever", "")
	req.Owner = "build"
	first, err := d.Start(req)
	if err != nil || first.Outcome != WAITING {
		t.Fatalf("first start = %s (%v), want WAITING", first.Outcome, err)
	}
	runDir := soleRunDir(t, runRoot)
	id := readManifestIdentity(t, runDir)
	t.Cleanup(func() { _ = syscall.Kill(-id.PGID, syscall.SIGKILL) })

	// The live supervisor holds the worktree: a second start is refused, and the
	// refusal names the running holder from its validated note.
	second := req
	second.ChangeID = "0343"
	_, err = d.Start(second)
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrWorktreeBusy {
		t.Fatalf("a start while the supervisor runs = %v, want worktree-busy", err)
	}
	if oe.Incumbent == nil || oe.Incumbent.DriveID != first.DriveID || oe.Incumbent.RawRunDir != runDir {
		t.Fatalf("busy refusal must name the running holder %s (%s), got %+v", first.DriveID, runDir, oe.Incumbent)
	}

	// SIGKILL only the supervisor, then wait until it is gone — reaped, so every
	// descriptor it held is closed. No gatedrive or process recovery call runs.
	if err := syscall.Kill(id.SupervisorPID, syscall.SIGKILL); err != nil {
		t.Fatalf("kill supervisor: %v", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for pidAlive(id.SupervisorPID) {
		if time.Now().After(deadline) {
			t.Fatalf("the killed supervisor %d never went away", id.SupervisorPID)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if st := observeState(t, svc, runDir); st == process.StateRunning {
		t.Fatalf("a killed supervisor's run still observes running")
	}

	next, err := d.Start(second)
	if err != nil {
		t.Fatalf("the next start after the supervisor died must be admitted with no recovery step: %v", err)
	}
	if next.Outcome != WAITING || next.DriveID == first.DriveID {
		t.Fatalf("next start = %+v, want a NEW WAITING drive", next)
	}
}
