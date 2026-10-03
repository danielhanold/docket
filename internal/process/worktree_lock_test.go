package process

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// freeLock reports whether the worktree lock at path is free right now. It
// takes the lock and closes it again on success, so it leaves no hold behind.
func freeLock(t *testing.T, path string) bool {
	t.Helper()
	f, busy, err := TryExclusiveLock(path)
	if err != nil {
		t.Fatalf("TryExclusiveLock(%s): %v", path, err)
	}
	if busy {
		return false
	}
	f.Close()
	return true
}

func TestTryExclusiveLockBusyIsTyped(t *testing.T) {
	path := filepath.Join(testsupport.TempDir(t), "busy.lock")
	f, busy, err := TryExclusiveLock(path)
	if err != nil || busy || f == nil {
		t.Fatalf("first acquire: f=%v busy=%v err=%v", f, busy, err)
	}
	defer f.Close()
	g, busy, err := TryExclusiveLock(path)
	if err != nil || !busy || g != nil {
		t.Fatalf("held lock must answer busy with no file and no error: g=%v busy=%v err=%v", g, busy, err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("lock file mode %o, want 600", fi.Mode().Perm())
	}
}

func TestTryExclusiveLockIOErrorIsNeitherFreeNorBusy(t *testing.T) {
	plain := filepath.Join(testsupport.TempDir(t), "plain")
	if err := os.WriteFile(plain, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f, busy, err := TryExclusiveLock(filepath.Join(plain, "busy.lock")) // parent is a file
	if err == nil || busy || f != nil {
		t.Fatalf("unopenable lock must be an error: f=%v busy=%v err=%v", f, busy, err)
	}
	if fl, ok := AsFailure(err); !ok || fl.Class != FailExternal {
		t.Fatalf("unopenable lock must be a FailExternal failure, got %v", err)
	}
}

// L2 (in-process half): a launch that fails before spawn frees the worktree.
func TestLaunchValidationFailureFreesWorktreeLock(t *testing.T) {
	svc := newTestService(t)
	lockPath := filepath.Join(testsupport.TempDir(t), "busy.lock")
	wl, _, err := TryExclusiveLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	_, lerr := svc.Launch(LaunchRequest{Root: "relative-root", Cwd: testsupport.TempDir(t), Argv: helperArgv(t, "exit", "0"), WorktreeLock: wl})
	if lerr == nil {
		t.Fatal("relative root must fail validation")
	}
	if _, serr := wl.Stat(); !errors.Is(serr, os.ErrClosed) {
		t.Fatalf("Launch must close the caller's copy on a validation failure, Stat err = %v", serr)
	}
	if !freeLock(t, lockPath) {
		t.Fatal("worktree lock still held after a pre-spawn launch failure")
	}
}

// L2 (process half): a launcher that dies after taking the lock and before
// spawning frees the worktree — the kernel releases on exit.
func TestWorktreeLockFreedWhenLauncherDiesBeforeSpawn(t *testing.T) {
	lockPath := filepath.Join(testsupport.TempDir(t), "busy.lock")
	exe, _ := os.Executable()
	if out, err := exec.Command(exe, "gate-test-helper", "lock-and-exit", lockPath).CombinedOutput(); err != nil {
		t.Fatalf("lock-and-exit helper: %v %s", err, out)
	}
	if !freeLock(t, lockPath) {
		t.Fatal("a dead launcher still holds the worktree lock")
	}
}

// Handoff: after Launch the supervisor alone holds the lock; the caller copy is closed.
func TestLaunchHandsWorktreeLockToSupervisor(t *testing.T) {
	svc := newTestService(t)
	lockPath := filepath.Join(testsupport.TempDir(t), "busy.lock")
	wl, _, _ := TryExclusiveLock(lockPath)
	out := launchHelperReq(t, svc, LaunchRequest{Root: testsupport.TempDir(t), Cwd: testsupport.TempDir(t), Argv: helperArgv(t, "sleep"), WorktreeLock: wl})
	if _, serr := wl.Stat(); !errors.Is(serr, os.ErrClosed) {
		t.Fatalf("Launch must close the caller's copy after spawn, Stat err = %v", serr)
	}
	if freeLock(t, lockPath) {
		t.Fatal("worktree lock not held while the supervisor runs")
	}
	killRun(t, out.RunDir)
	waitFor(t, "worktree lock release after the gate ends", 30*time.Second, func() bool { return freeLock(t, lockPath) })
}

// L1 (process half): a supervisor killed with SIGKILL frees the worktree. Its
// suite may survive; the census reports that as tree-survives and never stops it (change 0492) —
// the test ends the group itself.
func TestWorktreeLockFreedWhenSupervisorKilled(t *testing.T) {
	svc := newTestService(t)
	lockPath := filepath.Join(testsupport.TempDir(t), "busy.lock")
	wl, _, _ := TryExclusiveLock(lockPath)
	out := launchHelperReq(t, svc, LaunchRequest{Root: testsupport.TempDir(t), Cwd: testsupport.TempDir(t), Argv: helperArgv(t, "sleep"), WorktreeLock: wl})
	m, err := readManifest(out.RunDir)
	if err != nil || m == nil || m.SupervisorPID <= 1 {
		t.Fatalf("manifest: %v %v", m, err)
	}
	defer signalGroup(m.PGID, syscall.SIGKILL)
	if err := syscall.Kill(m.SupervisorPID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "worktree lock release after SIGKILL", 30*time.Second, func() bool { return freeLock(t, lockPath) })
}

// L6: the supervised command never inherits the worktree lock descriptor.
// Mutation check: deleting syscall.CloseOnExec(supervisorWorktreeLockFD) must turn this red.
func TestSupervisedCommandNeverInheritsWorktreeLock(t *testing.T) {
	svc := newTestService(t)
	lockPath := filepath.Join(testsupport.TempDir(t), "busy.lock")
	wl, _, _ := TryExclusiveLock(lockPath)
	out := launchHelperReq(t, svc, LaunchRequest{Root: testsupport.TempDir(t), Cwd: testsupport.TempDir(t),
		Argv: helperArgv(t, "env-check-worktree-lock", lockPath), WorktreeLock: wl})
	if st := waitTerminalState(t, out.RunDir, false); st != StatePassed {
		t.Fatalf("supervised command inherited the worktree lock or its env var: %v", st)
	}
}

// L7 (behavior): whoever finds the worktree free also finds the terminal record
// durable and live.lock free.
func TestWorktreeLockReleasedAfterTerminalAndLiveLock(t *testing.T) {
	svc := newTestService(t)
	for i := 0; i < 10; i++ {
		lockPath := filepath.Join(testsupport.TempDir(t), "busy.lock")
		wl, _, _ := TryExclusiveLock(lockPath)
		out := launchHelperReq(t, svc, LaunchRequest{Root: testsupport.TempDir(t), Cwd: testsupport.TempDir(t), Argv: helperArgv(t, "exit", "0"), WorktreeLock: wl})
		waitFor(t, "worktree lock free", 30*time.Second, func() bool { return freeLock(t, lockPath) })
		if term, err := readTerminal(out.RunDir); err != nil || term == nil {
			t.Fatalf("iteration %d: worktree free before the terminal record was durable (%v)", i, err)
		}
		if held, ans := probeFlock(filepath.Join(out.RunDir, liveLockFile)); held || ans != probeAbsent {
			t.Fatalf("iteration %d: worktree free while live.lock still held", i)
		}
	}
}

// L7 (shape): in every block of RunSupervisorFromEnv that calls
// closeWorktreeLock(), closeLock() is called earlier in the same block. Mutation
// check: swapping the two calls in either the terminal path or writeFailure must turn this red.
func TestSupervisorClosesWorktreeLockAfterLiveLock(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "supervisor.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	blocks := 0
	ast.Inspect(f, func(n ast.Node) bool {
		b, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		sawLive, sawWT := false, false
		for _, s := range b.List {
			es, ok := s.(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := es.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok {
				continue
			}
			switch id.Name {
			case "closeLock":
				sawLive = true
			case "closeWorktreeLock":
				sawWT = true
				if !sawLive {
					t.Errorf("closeWorktreeLock() called before closeLock() in a block of supervisor.go")
				}
			}
		}
		if sawWT {
			blocks++
		}
		return true
	})
	if blocks < 2 {
		t.Fatalf("expected closeWorktreeLock() in at least 2 blocks (writeFailure and the terminal path), found %d", blocks)
	}
}
