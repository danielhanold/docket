<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0490 — Replace the durable worktree admission slot with a supervisor-held kernel lock](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0490-replace-the-durable-worktree-admission-slot-with-a-superviso.md)**
<!-- docket:backlink:end -->
# Replace the Durable Worktree Admission Slot with a Supervisor-Held Kernel Lock — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: this plan is executed by the `docket-build` role (one tier worker per task under the `docket-build-task` contract, one commit per task, then one full-suite gate). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the durable `gate-admission` slot state machine with a per-worktree `flock` that the gate supervisor holds for its whole life, so "busy" means "a live supervisor holds it" and a dead gate frees the worktree with no recovery step; delete the slot, its recovery machinery, the legacy inventory, `gate.history.cleanup`, and every slot reader in the run tracker.

**Architecture:** `internal/process` gains an exported non-blocking lock primitive and accepts an already-locked file on `LaunchRequest`, handing it to the re-exec'd supervisor as one more `ExtraFiles` descriptor that the supervisor marks close-on-exec and closes last. `internal/gatedrive` owns the lock path/key (`<git-common-dir>/docket/worktree-locks/<sha256(root)>/busy.lock`), a diagnostic holder note, and takes the lock at every launch site (drive admission, the single relaunch); `internal/app` takes it for raw `gate.launch`. The run tracker stops reading the slot: the launch census attributes drives by `run_context_hash`, and teardown proof becomes "the supervisor is gone".

**Tech Stack:** Go (stdlib `syscall.Flock`, `os/exec` `ExtraFiles`), the repo's Go suite runner (`go run ./cmd/docket development test`), Markdown skill bodies mirrored into `internal/assets/embedded` by `go generate ./internal/assets`.

**Spec:** `docs/superpowers/specs/2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso-design.md` (on the `docket` metadata branch; read it at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/…` or `git show origin/docket:docs/superpowers/specs/2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso-design.md`). Problem facts and Decisions 1–5 are referenced below by number.

## Global Constraints

- Lock file: `<git-common-dir>/docket/worktree-locks/<key>/busy.lock`; holder note `holder.json` beside it. Directories 0700, files 0600 (explicit chmod — create-time modes are umask-masked). Lock files and their directories are **never deleted**.
- `<key>` = lowercase hex sha256 of the canonical worktree root, where root = `gitcli.DiscoverWorktree(<launch working directory>).Root`. Never key on `--repo-dir`, `os.Getwd()`, a subdirectory, or a run root.
- The worktree lock is only ever **tried** (non-blocking `LOCK_EX|LOCK_NB`), never waited on. Release is **by close only** — no code calls `LOCK_UN` on it; never reuse the workspace/transaction `release()` helpers (they unlock before closing).
- A held lock refuses with reason `worktree-busy`, stage `worktree-admission`. Any other open/lock error is an I/O failure, never read as "free". A refusal is never queued, never stops the holder, charges no suite attempt, and leaves nothing behind.
- `process.Service.Launch` takes ownership of `LaunchRequest.WorktreeLock` and closes the caller's copy on **every** path. The supervisor adopts it only when the launcher says it passed one (a private env var stripped from the child's environment), marks it close-on-exec, and closes it **last**: after the terminal or failure record and after `live.lock`. The supervised command never inherits it.
- `internal/process` stays standard-library-only (`TestImportBoundaryStdlibOnly`).
- Holder note fields: `kind` (`drive`|`raw`), `drive_id`, `run_dir`, `change_id`, `owner` (`build`|`finalize`|`raw`), `written_at`. A failed note write is ignored. A busy refusal prints the holder only when `process.Service.Observe(run_dir)` answers `running`; otherwise "holder unknown". Admission, cancel, and every other decision ignore the note.
- No migration, no schema bump for kept stores. `gate-admission/v2/` stays on disk, inert. Drive records keep their shape; the launch token keeps JSON key `admission_token`.
- `launch-unconfirmed` survives only as a drive HALT cause (`haltReservedRelaunch`, the relaunch launch-error leg) — never as an admission refusal.
- Kept for change 0491 (do not touch): `runLaunchGate` at `Admit` and `StartAdmitted` with its `stale-run-id`/cancelled refusals, every `--run-id` flag, the run id in `run.start`/`run.cancel`, the `completing`/`completed` lifecycle, and the run-tracker prose in `CLAUDE.md`/skills.
- Frozen records are never edited: plans, results, archived changes, specs, Accepted ADRs.
- Cross-references in maintained source anchor on a symbol or a quoted clause, never a line number (`TestCommentAnchorStyle`).
- Integration tests carry their shard's name prefix (`TestIntegrationGatedrive…`, `TestRaceIntegrationGatedrive…`, `TestIntegrationBuildStart…`, `TestIntegrationGateLifecycle…`, `TestIntegrationRunCancel…`, `TestIntegrationRunCompletion…`, `TestIntegrationRunStart…`, `TestIntegrationRunFence…`) and `//go:build integration`. Default-corpus tests in `internal/gatedrive` must never start real git (no-git guard).
- Every manual re-run and mutation probe uses `-count=1` (Go's result cache serves stale greens).
- Never restore deleted code or prose to keep a test green; decide each test by what it guards (learning: test-premise-deleted-not-regated).
- Whole-suite gate: `go run ./cmd/docket development test` (`build.test_command`).

## Review Focus

1. **A HALTED drive whose supervisor is still running** (`deadline-expired-stop-unproven`, `uncertain-ownership`, a drifted pass whose stop failed) must be stopped by `run.cancel`, and `run.cancel` must not report `cancelled` while it runs. The slot used to carry this; the census now must. Pinned in Task 3 (`TestCensusStopsLiveSupervisorOfHaltedDrive`).
2. **A run dir that no longer exists** (its run root was removed after the terminal) is clean absence — torn down — while an `Observe` error on a run dir that does exist keeps cancel pending (probe error is not clean absence). Pinned in Task 3 (`TestCensusRunDirAbsentIsTornDownButProbeErrorIsPending`).
3. **Spelling variants of one worktree** — a subdirectory `--cwd`, a symlinked root, `/tmp` vs `/private/tmp`, a case variant on a case-insensitive volume — must map to one lock. Pinned in Task 2 (`TestIntegrationGatedriveWorktreeLockKeyIsCanonical`).
4. **Cancel then restart**: after `run.cancel` stops a run whose drive is running, the very next gate start in that worktree is admitted with no recovery step. Pinned in Task 6 (`TestIntegrationRunCancelFreesWorktreeForNextStart`).
5. **A launch that fails before spawn** (validation error, supervisor spawn error) after the caller took the lock frees the worktree immediately, in-process. Pinned in Task 1 (`TestLaunchValidationFailureFreesWorktreeLock`).

## Plan-level decisions (read before any task)

These resolve points the spec leaves to implementation. A reviewer should check them against the spec.

- **The census sweeps HALTED drives too.** Decision 4 says teardown proof is "the supervisor is gone" for each run dir of an attributed drive. A HALTED drive can still have a live supervisor, and with the slot gone nothing else would stop it on cancel. So the census checks the run dirs of every attributed **nonterminal or HALTED** drive; PASSED/FAILED are settled by their verdict (the supervisor wrote it before exiting).
- **`ErrUnresolvedLaunchTransition` is kept.** The spec lists it for deletion, but two non-slot emitters remain in `revalidateAdmittedLaunch` (a busy launch claim; a drive settled terminal before launch). Deleting it would force a new kind for those legs. Only its slot-side emitters (`ConfirmWorktreeExecution`, `verifyAdmittedSlot`) go; update its doc comment.
- **`CauseRunLinkLost` is deleted** (Task 7) along with its glossary entry: its only emitter, `resolveDriveRun`, goes in Task 4.
- **The raw locator stays today's format**: `incumbent-run:<run-id>`, where `<run-id>` is `filepath.Base(holder.run_dir)` validated by `rawRunIDShape`. The change id appears in the remedy message, not the locator.
- **The worktree lock is keyed on the drive's `Cwd`** (the launch working directory, which defaults to `--repo-dir`), resolved through a new `GitSeam.WorktreeRoot`.
- **The holder note's `owner` survives a relaunch** by reading the previous note: when its `drive_id` matches, the relaunch keeps its `owner`; otherwise `owner` is empty. Drive records stay unchanged.
- **`stop-unproven` is gone for participants too**: `appGateStopper` and `appGateObserver` count any non-`running` state as torn down (Task 6).
- **Recording the ADR is not a build task.** See *Hand-off to implement-next Step 6* at the end.

## File map

| File | Responsibility | Tasks |
|---|---|---|
| `internal/process/lock.go` | `TryExclusiveLock` exported primitive | 1 |
| `internal/process/paths.go` | `LaunchRequest.WorktreeLock` | 1 |
| `internal/process/launch.go` | hand the lock to the supervisor, close the caller copy on every path | 1 |
| `internal/process/supervisor.go` | adopt fd 5 when told, close-on-exec, close last | 1 |
| `internal/process/worktree_lock_test.go` (new), `main_test.go` | L1, L2, L6, L7 tests + helper modes | 1 |
| `internal/gatedrive/worktree_lock.go` (new) | lock path/key, `TryWorktreeLock`, holder note, live-holder diagnosis | 2 |
| `internal/gatedrive/fingerprint.go` | `GitSeam.WorktreeRoot` + `realGit` impl | 2 |
| `internal/gatedrive/reconcile.go` | census by run context, teardown proof | 3 |
| `internal/gatedrive/driver.go`, `drive.go`, `ownership.go` | admission/launch/relaunch on the lock | 4 |
| `internal/gatedrive/launch_sites_guard_test.go` | narrowed run-gate guard | 4 |
| `internal/app/gate_drive.go` | build start order, busy diagnosis | 4 |
| `internal/app/gate.go` | raw `gate.launch`/`gate.stop` on the lock | 5 |
| `internal/repoguard/gatelaunch_admission_test.go` | every `Launch` site hands over a lock | 5 |
| `internal/app/runtracker_{cancel,complete,start,fence}.go` | run tracker without the slot | 3, 6 |
| `internal/gatedrive/{admission,admission_retire,incumbent}.go`, `history.go` | deleted / trimmed | 7 |
| `internal/app/gate_history.go`, `internal/cli/gate.go`, `install.go`, `app/schema_registry.go` | `gate.history.cleanup` removed | 7 |
| skills, glossary, concepts doc, embedded mirror, budgets | prose | 8 |
| `tests/runtime-budgets.tsv` | re-measured rows | 9 |

## Task order and why

1 → 2 are additive. 3 moves the census off the slot **before** 4 deletes `resolveDriveRun` (the census uses it). 4 switches drives while the raw launch still uses the slot; `TestGateLaunchAdmissionCoverage` stays green because gatedrive is an interior package to it. 5 switches the raw launch and rewrites that guard in the same commit — the first point where every site hands over a lock. 6 removes the run tracker's slot readers. 7 deletes the slot store once nothing reads it. 8 is prose. 9 re-measures budgets. **Between Tasks 4 and 5 drives and raw launches do not exclude each other**; that window never ships (single full-suite gate at the end). Do not add tests that depend on it.

---

### Task 1: Process — hand a worktree lock to the supervisor, held for its whole life

**Tier: premium.** Risk: descriptor inheritance and release order in a re-exec'd session leader; a leaked descriptor pins a worktree forever, and a wrong close order breaks ADR-0095's "free lock implies durable terminal record". Correctable, but subtle, and every later task rests on it.

**Files:**
- Modify: `internal/process/lock.go` (add `TryExclusiveLock`)
- Modify: `internal/process/paths.go` (`LaunchRequest.WorktreeLock`)
- Modify: `internal/process/launch.go` (`Launch`, `spawnSupervisor`)
- Modify: `internal/process/supervisor.go` (env/fd contract, `RunSupervisorFromEnv`)
- Modify: `internal/process/main_test.go` (`runTestHelper` modes)
- Create: `internal/process/worktree_lock_test.go`

**Interfaces:**
- Produces: `func TryExclusiveLock(path string) (f *os.File, busy bool, err error)` — creates the file 0600 if absent; `busy=true, f=nil, err=nil` exactly when another open description holds it; any other failure is `err != nil` (a `*Failure` of class `FailExternal`), never `busy`.
- Produces: `LaunchRequest.WorktreeLock *os.File` — optional; `Launch` closes it on every return path.
- Produces (unexported): `supervisorWorktreeLockEnv = "DOCKET_GATE_SUPERVISOR_WORKTREE_LOCK"`, `supervisorWorktreeLockFD = 5`.

- [ ] **Step 1: Write the failing tests** in `internal/process/worktree_lock_test.go`:

```go
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
// suite may survive (accepted gap, change 0492) — the test ends the group itself.
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
```

Add a `launchHelperReq` helper next to `launchHelper` in `launch_test.go` (same body as `launchHelper` but taking a full `LaunchRequest`, still calling `reapSupervisor` and `testsupport.DrainOnCleanup(t, func() { quiesceRun(t, out.RunDir) })`).

Add two modes to `runTestHelper` in `main_test.go` (and to its doc comment):

```go
	case "lock-and-exit":
		f, busy, err := TryExclusiveLock(args[1])
		if err != nil || busy {
			return 97
		}
		_ = f // exit without closing: the kernel must release it
		return 0
	case "env-check-worktree-lock":
		if os.Getenv("DOCKET_GATE_SUPERVISOR_WORKTREE_LOCK") != "" {
			return 6
		}
		want, err := os.Stat(args[1])
		if err != nil {
			return 98
		}
		if got, serr := os.NewFile(5, "probe").Stat(); serr == nil && os.SameFile(want, got) {
			return 7 // the worktree lock fd leaked past CLOEXEC into the child
		}
		return 0
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/process -run 'WorktreeLock|TryExclusiveLock|SupervisedCommandNeverInherits|SupervisorClosesWorktreeLock' -count=1`
Expected: compile failure (`TryExclusiveLock`, `WorktreeLock` undefined).

- [ ] **Step 3: Implement**

`lock.go`:

```go
// TryExclusiveLock is the exported non-blocking acquire the worktree lock uses
// (change 0490). It is acquireFlock with the busy case lifted into a typed
// result: busy is true (and f nil, err nil) exactly when another open file
// description holds the lock; every other failure is an error and is never
// read as either "free" or "busy". The returned file owns the lock; only
// closing it (or the process exiting) releases it.
func TryExclusiveLock(path string) (f *os.File, busy bool, err error) {
	f, err = acquireFlock(path)
	if err == nil {
		return f, false, nil
	}
	if fl, ok := AsFailure(err); ok && fl.Class == FailBlocked {
		return nil, true, nil
	}
	return nil, false, err
}
```

`paths.go` — add to `LaunchRequest`:

```go
	// WorktreeLock, when non-nil, is an already-flocked worktree lock the caller
	// hands over (change 0490). Launch takes ownership on EVERY path: it passes
	// the descriptor to the supervisor and closes the caller's copy, so a launch
	// that fails before spawn frees the worktree and one that spawned leaves the
	// supervisor as the only holder.
	WorktreeLock *os.File
```

`launch.go` `Launch` — first lines, before validation:

```go
	wl := req.WorktreeLock
	defer func() {
		if wl != nil {
			wl.Close()
		}
	}()
```

and immediately after a successful `spawnSupervisor` (next to `lockFile.Close()`):

```go
	if wl != nil {
		wl.Close() // the supervisor's inherited copy now holds the worktree lock alone
		wl = nil
	}
```

`spawnSupervisor`: after building `cmd.Env` and `cmd.ExtraFiles = []*os.File{lockFile, pipeW}`:

```go
	if req.WorktreeLock != nil {
		// ExtraFiles slot 2 -> fd 5 (worktree lock); the env flag tells the
		// supervisor to adopt it. Without the flag fd 5 is never touched.
		cmd.ExtraFiles = append(cmd.ExtraFiles, req.WorktreeLock)
		cmd.Env = append(cmd.Env, supervisorWorktreeLockEnv+"=1")
	}
```

`supervisor.go`: extend the const block and its comment with `supervisorWorktreeLockEnv = "DOCKET_GATE_SUPERVISOR_WORKTREE_LOCK"` and `supervisorWorktreeLockFD = 5`. In step (1), before step (2) opens any file (so fd 5 is still the inherited one):

```go
	var worktreeLock *os.File
	if os.Getenv(supervisorWorktreeLockEnv) != "" {
		worktreeLock = os.NewFile(uintptr(supervisorWorktreeLockFD), "worktree.lock")
		syscall.CloseOnExec(supervisorWorktreeLockFD)
	}
	// closeWorktreeLock releases the worktree lock by close only (never LOCK_UN).
	// It is called LAST on every exit path, after closeLock, so an observer that
	// finds the worktree free also finds live.lock free and the record durable.
	closeWorktreeLock := func() {
		if worktreeLock != nil {
			worktreeLock.Close()
			worktreeLock = nil
		}
	}
```

In `writeFailure`: `handshake(...); closePipe(); closeLock(); closeWorktreeLock(); return 1`. At the terminal path end: `closePipe(); closeLock(); closeWorktreeLock(); return 0`. Strip the new var from the child: `cmd.Env = envWithout(os.Environ(), supervisorRunDirEnv, supervisorArgvEnv, supervisorWorktreeLockEnv)`. Do **not** log to `supervisor.log` after `closeLock()` (`quiesceRun` relies on a free `live.lock` meaning no more run-dir writes).

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/process -count=1`
Expected: PASS (whole package, including `TestLaunchStripsSupervisorEnv`, `TestImportBoundaryStdlibOnly`).

- [ ] **Step 5: Mutation-check the two guards**

Delete the `syscall.CloseOnExec(supervisorWorktreeLockFD)` line, run `go test ./internal/process -run TestSupervisedCommandNeverInheritsWorktreeLock -count=1` → FAIL; restore from a backup copy (never `git checkout --`, which discards the uncommitted work). Swap `closeLock(); closeWorktreeLock()` in the terminal path, run `-run TestSupervisorClosesWorktreeLockAfterLiveLock -count=1` → FAIL; restore. Record both results in the commit body.

- [ ] **Step 6: Commit**

```bash
git add internal/process/lock.go internal/process/paths.go internal/process/launch.go internal/process/supervisor.go internal/process/main_test.go internal/process/launch_test.go internal/process/worktree_lock_test.go
git commit -m "feat(process): hand a worktree lock to the gate supervisor, released last (0490)"
```

---

### Task 2: Gatedrive — the worktree lock, its key, and the holder note

**Tier: standard.** Risk: path canonicalization (a wrong key lets one worktree buy two locks); the diagnosis must never print a stale holder. Additive — nothing calls it yet.

**Files:**
- Create: `internal/gatedrive/worktree_lock.go`
- Modify: `internal/gatedrive/store.go` (`Store.lockRoot`, set in `OpenStore`, doc comment)
- Modify: `internal/gatedrive/ownership.go` (`IncumbentSnapshot` gains `ChangeID`, `Owner`; keep the old fields for now — Task 7 trims them)
- Modify: `internal/gatedrive/fingerprint.go` (`GitSeam.WorktreeRoot`, `realGit.WorktreeRoot`)
- Modify: `internal/gatedrive/driver_test.go` (`fakeGit.WorktreeRoot`), `internal/gatedrive/driver_concurrency_test.go` (`barrierGit.WorktreeRoot`)
- Create: `internal/gatedrive/worktree_lock_test.go` (default corpus, no git)
- Create: `internal/gatedrive/worktree_lock_integration_test.go` (`//go:build integration`, real git)

**Interfaces:**
- Consumes: `process.TryExclusiveLock` (Task 1).
- Produces:
  - `type HolderNote struct { Kind, DriveID, RunDir, ChangeID, Owner string; WrittenAt time.Time }` with JSON keys `kind`, `drive_id`, `run_dir`, `change_id`, `owner`, `written_at`.
  - `type HolderObserver interface { Observe(runDir string) (*process.Observation, error) }` (both `ProcessSeam` and `*process.Service` satisfy it).
  - `type WorktreeLock struct` with `func (l *WorktreeLock) TakeFile() *os.File`, `func (l *WorktreeLock) Release()`, `func (l *WorktreeLock) WriteHolder(n HolderNote)`, `func (l *WorktreeLock) PriorHolder() (HolderNote, bool)`.
  - `func (s *Store) TryWorktreeLock(canonicalRoot string, obs HolderObserver) (*WorktreeLock, error)` — busy → `*OwnershipError{Kind: ErrWorktreeBusy, Op: "worktree-admission", Incumbent: <validated holder or nil>}`; other failure → `*StoreError{Kind: ErrIO, Op: "worktree-admission"}`; a non-absolute root → `ErrInvalidID`.
  - `const CauseWorktreeBusy = "worktree-busy"` is **not** added here (Task 4).
  - `GitSeam.WorktreeRoot(dir string) (string, error)` — the canonical root of the worktree containing `dir`.

- [ ] **Step 1: Write the failing unit tests** (`worktree_lock_test.go`, default corpus — use a fake observer, never real git):

```go
package gatedrive

// fakeObserver answers Observe from a map; a missing dir is an error.
type fakeObserver struct{ states map[string]process.State }

func (f fakeObserver) Observe(runDir string) (*process.Observation, error) {
	st, ok := f.states[runDir]
	if !ok {
		return nil, fmt.Errorf("unobservable")
	}
	return &process.Observation{RunDir: runDir, State: st}, nil
}

func TestTryWorktreeLockRefusesSecondHolder(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	root := testsupport.TempDir(t)
	first, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	_, err = store.TryWorktreeLock(root, nil)
	if !isOwnershipKind(err, ErrWorktreeBusy) {
		t.Fatalf("second acquire = %v, want worktree-busy", err)
	}
	if oe, _ := AsOwnershipError(err); oe.Op != "worktree-admission" {
		t.Fatalf("op = %q", oe.Op)
	}
	first.Release()
	again, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatalf("released lock must readmit with no recovery step: %v", err)
	}
	again.Release()
}

// L8: the holder is printed only while its run is running.
func TestBusyRefusalNamesHolderOnlyWhileRunning(t *testing.T) {
	for _, tc := range []struct {
		name  string
		note  *HolderNote
		raw   []byte
		state process.State
		want  bool
	}{
		{"running drive", &HolderNote{Kind: "drive", DriveID: strings.Repeat("a", 32), RunDir: "/runs/0123456789abcdef0123456789abcdef", ChangeID: "490", Owner: "build"}, nil, process.StateRunning, true},
		{"stale note, run passed", &HolderNote{Kind: "drive", DriveID: strings.Repeat("a", 32), RunDir: "/runs/0123456789abcdef0123456789abcdef"}, nil, process.StatePassed, false},
		{"stale note, run vanished", &HolderNote{Kind: "raw", RunDir: "/runs/0123456789abcdef0123456789abcdef", Owner: "raw"}, nil, process.StateVanished, false},
		{"missing note", nil, nil, "", false},
		{"unreadable note", nil, []byte("{not json"), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := OpenStore(testsupport.TempDir(t))
			root := testsupport.TempDir(t)
			held, err := store.TryWorktreeLock(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer held.Release()
			if tc.note != nil {
				held.WriteHolder(*tc.note)
			}
			if tc.raw != nil {
				if err := os.WriteFile(filepath.Join(held.dir, worktreeHolderFile), tc.raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			obs := fakeObserver{states: map[string]process.State{}}
			if tc.note != nil && tc.state != "" {
				obs.states[tc.note.RunDir] = tc.state
			}
			_, err = store.TryWorktreeLock(root, obs)
			oe, ok := AsOwnershipError(err)
			if !ok || oe.Kind != ErrWorktreeBusy {
				t.Fatalf("want worktree-busy, got %v", err)
			}
			if got := oe.Incumbent != nil; got != tc.want {
				t.Fatalf("holder named = %v, want %v (%+v)", got, tc.want, oe.Incumbent)
			}
			if tc.want && (oe.Incumbent.DriveID != tc.note.DriveID || oe.Incumbent.RawRunDir != tc.note.RunDir ||
				oe.Incumbent.ChangeID != tc.note.ChangeID || oe.Incumbent.Owner != tc.note.Owner || oe.Incumbent.Kind != tc.note.Kind) {
				t.Fatalf("holder snapshot %+v does not match note %+v", oe.Incumbent, tc.note)
			}
		})
	}
}

func TestTryWorktreeLockIOErrorIsNotFree(t *testing.T) {
	common := testsupport.TempDir(t)
	// Make <common>/docket a regular file so the lock root cannot be created.
	if err := os.WriteFile(filepath.Join(common, "docket"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := OpenStore(common).TryWorktreeLock(testsupport.TempDir(t), nil)
	if se, ok := AsStoreError(err); !ok || se.Kind != ErrIO {
		t.Fatalf("unopenable lock root must be a typed IO error, got %v", err)
	}
}

func TestWorktreeLockFilesArePrivateAndNeverDeleted(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	l, err := store.TryWorktreeLock(testsupport.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	l.WriteHolder(HolderNote{Kind: "raw", RunDir: "/runs/x", Owner: "raw"})
	l.Release()
	for path, want := range map[string]os.FileMode{
		l.dir: 0o700, filepath.Join(l.dir, worktreeLockFile): 0o600, filepath.Join(l.dir, worktreeHolderFile): 0o600,
	} {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s must survive Release: %v", path, err)
		}
		if fi.Mode().Perm() != want {
			t.Fatalf("%s mode %o, want %o", path, fi.Mode().Perm(), want)
		}
	}
}

func TestTryWorktreeLockRefusesRelativeRoot(t *testing.T) {
	_, err := OpenStore(testsupport.TempDir(t)).TryWorktreeLock("relative/root", nil)
	if se, ok := AsStoreError(err); !ok || se.Kind != ErrInvalidID {
		t.Fatalf("relative root = %v, want ErrInvalidID", err)
	}
}
```

- [ ] **Step 2: Write the failing integration test (L3)** in `worktree_lock_integration_test.go` (`//go:build integration`, prefix `TestIntegrationGatedrive`). Reuse the real-git repository fixture the existing `TestIntegrationGatedrive…` tests use (see `supervisor_integration_test.go` / `fingerprint_integration_test.go`). For each spelling pair, take the lock via `realGit{}.WorktreeRoot(a)` and assert `TryWorktreeLock(realGit{}.WorktreeRoot(b))` is `worktree-busy`:
  - the root vs a subdirectory of it (`--cwd` pointing below the root);
  - the root vs a symlink to the root;
  - on darwin, when `filepath.EvalSymlinks("/tmp") != "/tmp"`: a repo created under `/tmp/...` vs the same path spelled `/private/tmp/...` (skip otherwise);
  - a case variant (`strings.ToUpper` of the last path component), **skipped** when `os.Stat` of the variant fails (case-sensitive volume);
  - a negative control: a second, separate repository gets its own lock (acquire succeeds).

Name it `TestIntegrationGatedriveWorktreeLockKeyIsCanonical`.

- [ ] **Step 3: Run to verify they fail**

Run: `go test ./internal/gatedrive -run 'WorktreeLock|BusyRefusalNamesHolder' -count=1` and `go test -tags integration ./internal/gatedrive -run TestIntegrationGatedriveWorktreeLockKeyIsCanonical -count=1`
Expected: compile failure.

- [ ] **Step 4: Implement** `internal/gatedrive/worktree_lock.go`:

```go
// The per-worktree admission lock (change 0490, superseding ADR-0118's durable
// slot). One canonical worktree admits at most one live top-level gate: every
// launch site first takes a NON-BLOCKING exclusive flock on
//
//	<git-common-dir>/docket/worktree-locks/<key>/busy.lock
//
// and hands it to the gate supervisor (process.LaunchRequest.WorktreeLock), which
// holds it for its whole life; the kernel releases it when the supervisor exits
// or dies. "Busy" therefore means "a live supervisor holds it", and a dead gate
// frees the worktree with no recovery step. The lock is only ever tried, never
// waited on; it is released by close only (never LOCK_UN). Lock files and their
// directories are never deleted: unlinking a lock file races a concurrent opener
// into holding a lock on an orphaned inode.
//
// holder.json beside busy.lock is a DIAGNOSTIC only: it names the last holder
// and is printed on a busy refusal only after Observe confirms its run is still
// running. Admission, cancel, and every other decision ignore it.
package gatedrive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

const (
	worktreeLockFile   = "busy.lock"
	worktreeHolderFile = "holder.json"
	opWorktreeAdmission = "worktree-admission"
)

// HolderNote is the diagnostic record of the gate that last took a worktree lock.
type HolderNote struct {
	Kind      string    `json:"kind"` // "drive" | "raw"
	DriveID   string    `json:"drive_id,omitempty"`
	RunDir    string    `json:"run_dir"`
	ChangeID  string    `json:"change_id,omitempty"`
	Owner     string    `json:"owner,omitempty"` // "build" | "finalize" | "raw" | ""
	WrittenAt time.Time `json:"written_at"`
}

// HolderObserver is the read-only run observation the busy diagnosis needs.
type HolderObserver interface {
	Observe(runDir string) (*process.Observation, error)
}

// WorktreeLock is a held worktree lock. Its file travels to process.Launch
// through TakeFile; Release closes it when a caller abandons the admission
// before launching. dir survives the handoff so the holder note can be written
// after the launch.
type WorktreeLock struct {
	file *os.File
	dir  string
}

// worktreeLockKey is the lowercase hex sha256 of a canonical worktree root.
func worktreeLockKey(canonicalRoot string) string {
	sum := sha256.Sum256([]byte(canonicalRoot))
	return hex.EncodeToString(sum[:])
}

// TryWorktreeLock takes the worktree lock for canonicalRoot (a root already
// resolved through gitcli.DiscoverWorktree — every symlink hop resolved), never
// blocking. A held lock is a typed ErrWorktreeBusy whose Incumbent names the
// holder only when obs confirms its run is running; any other failure is a typed
// ErrIO and is never read as free.
func (s *Store) TryWorktreeLock(canonicalRoot string, obs HolderObserver) (*WorktreeLock, error) {
	if !filepath.IsAbs(canonicalRoot) {
		return nil, storeErr(ErrInvalidID, opWorktreeAdmission, nil)
	}
	if err := ensurePrivateDir(s.lockRoot); err != nil {
		return nil, storeErr(ErrIO, opWorktreeAdmission, err)
	}
	dir := filepath.Join(s.lockRoot, worktreeLockKey(canonicalRoot))
	if err := ensurePrivateDir(dir); err != nil {
		return nil, storeErr(ErrIO, opWorktreeAdmission, err)
	}
	f, busy, err := process.TryExclusiveLock(filepath.Join(dir, worktreeLockFile))
	if err != nil {
		return nil, storeErr(ErrIO, opWorktreeAdmission, err)
	}
	if busy {
		oe := ownershipErr(ErrWorktreeBusy, opWorktreeAdmission)
		oe.Incumbent = liveHolder(dir, obs)
		return nil, oe
	}
	return &WorktreeLock{file: f, dir: dir}, nil
}

// TakeFile transfers the locked file to the caller (process.Launch takes
// ownership and closes it on every path). A second call returns nil.
func (l *WorktreeLock) TakeFile() *os.File {
	if l == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f
}

// Release closes a lock that was never handed to Launch. Idempotent and nil-safe.
func (l *WorktreeLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	l.file.Close()
	l.file = nil
}

// WriteHolder records the holder note atomically beside (never over) busy.lock.
// Best effort: a failed write is ignored and never fails the launch.
func (l *WorktreeLock) WriteHolder(n HolderNote) {
	if l == nil {
		return
	}
	if n.WrittenAt.IsZero() {
		n.WrittenAt = time.Now().UTC()
	}
	_ = writeAtomicJSON(filepath.Join(l.dir, worktreeHolderFile), n)
}

// PriorHolder reads the current note, for a relaunch that keeps its owner.
func (l *WorktreeLock) PriorHolder() (HolderNote, bool) {
	if l == nil {
		return HolderNote{}, false
	}
	return readHolderNote(l.dir)
}

func readHolderNote(dir string) (HolderNote, bool) {
	buf, err := os.ReadFile(filepath.Join(dir, worktreeHolderFile))
	if err != nil {
		return HolderNote{}, false
	}
	var n HolderNote
	if json.Unmarshal(buf, &n) != nil {
		return HolderNote{}, false
	}
	return n, true
}

// liveHolder projects the holder note into an IncumbentSnapshot only when obs
// confirms its run is running. A missing/unreadable note, a nil observer, an
// observation error, or any non-running state yields nil ("holder unknown"): a
// stale note from an earlier holder is never shown.
func liveHolder(dir string, obs HolderObserver) *IncumbentSnapshot {
	n, ok := readHolderNote(dir)
	if !ok || obs == nil || n.RunDir == "" {
		return nil
	}
	o, err := obs.Observe(n.RunDir)
	if err != nil || o == nil || o.State != process.StateRunning {
		return nil
	}
	return &IncumbentSnapshot{
		Kind:      n.Kind,
		DriveID:   n.DriveID,
		RawRunID:  filepath.Base(n.RunDir),
		RawRunDir: n.RunDir,
		ChangeID:  n.ChangeID,
		Owner:     n.Owner,
	}
}
```

`store.go`: add `lockRoot string` to `Store`, set `lockRoot: filepath.Join(gitCommonDir, "docket", "worktree-locks")` in `OpenStore`, and add the root to `OpenStore`'s doc comment (leave `admissionRoot` until Task 7).

`ownership.go`: add `ChangeID string` and `Owner string` to `IncumbentSnapshot`, with comments. `Kind` now also takes `"drive"`.

`fingerprint.go`: add to `GitSeam`:

```go
	// WorktreeRoot returns the canonical (every-symlink-hop-resolved) toplevel
	// of the working tree containing dir — gitcli.DiscoverWorktree's Root. The
	// worktree lock is keyed on it (change 0490), never on a caller's spelling.
	WorktreeRoot(dir string) (string, error)
```

and

```go
func (realGit) WorktreeRoot(dir string) (string, error) {
	c, err := gitcli.NewClient()
	if err != nil {
		return "", err
	}
	wt, err := c.DiscoverWorktree(context.Background(), gitcli.DiscoverOptions{InvocationPath: dir})
	if err != nil {
		return "", err
	}
	return wt.Root, nil
}
```

Test fakes (`fakeGit`, `barrierGit`): add a `WorktreeRoot` that returns `filepath.EvalSymlinks(dir)` when that succeeds, else `filepath.Clean(dir)`, plus an optional override error field on `fakeGit` (`rootErr error`) for Task 4's I/O case.

- [ ] **Step 5: Run to verify they pass**

Run: `go test ./internal/gatedrive -count=1` and `go test -tags integration ./internal/gatedrive -run TestIntegrationGatedriveWorktreeLockKeyIsCanonical -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gatedrive/worktree_lock.go internal/gatedrive/worktree_lock_test.go internal/gatedrive/worktree_lock_integration_test.go internal/gatedrive/store.go internal/gatedrive/ownership.go internal/gatedrive/fingerprint.go internal/gatedrive/driver_test.go internal/gatedrive/driver_concurrency_test.go
git commit -m "feat(gatedrive): per-worktree lock keyed on the canonical root, with a validated holder note (0490)"
```

---

### Task 3: The launch census attributes drives by run context, and teardown proof is "the supervisor is gone"

**Tier: premium.** Risk: cancel correctness — a census that under-attributes reports `cancelled` over a live suite; one that over-attributes stops another run's gate. Correctable, but it is the one place cancel now looks.

**Files:**
- Modify: `internal/gatedrive/reconcile.go` (rewrite the census; keep `RunLaunchReport`)
- Modify: `internal/gatedrive/reconcile_test.go` (move from slot-token to context-hash attribution)
- Modify: `internal/app/runtracker_cancel.go` (`runLaunchReconciler`, `appLaunchReconciler`, `reconcileRunTeardown` step (5c), `verifyTerminalRunQuiescence`)
- Modify: `internal/app/runtracker_complete.go` (`runLaunchObserver`, `appLaunchObserver`, `accountCompletionLaunches`, `accountCompletionObligations` gain `repoDir`/`runKey`)
- Modify: the app census tests and fixtures that construct drives for the census (`runtracker_production_census_integration_test.go`, and `newCancelFixture` / `newVerdictCompletionFixture` only so far as their drives must now carry the run's context — keep their slot construction until Task 6)

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `func (d *Driver) ReconcileRunLaunches(runContextHash string) (RunLaunchReport, error)`
  - `func (d *Driver) ObserveRunLaunches(runContextHash string) (RunLaunchReport, error)`
  - app: `runLaunchReconciler.reconcile(contextHash string)`, `runLaunchObserver.observe(contextHash string)`, `func runContextHash(repoDir, runKey string) (string, error)` returning the run-tracker record's `ChildContextHash`.
  - Finding tokens kept: `claim-busy:<id>`, `launch-pending:<id>`, `resolution-unresolved:<id>`, `record-unreadable:<id>`, `history-unattributed:<id>`, `registry-unreadable`, `run-terminal:<id>`, `run-live:<id>`, `replacement-stopped:<id>`. Removed: `linkage-unresolved:`, `slot-unreadable`, `stop-unproven:` (census). New app finding: `run-context-unreadable`.

- [ ] **Step 1: Write the failing gatedrive tests** in `reconcile_test.go`. Replace the slot-token attribution tests (every test that builds a slot with `ReserveWorktreeExecution*`/`ConfirmWorktreeExecution` or asserts `linkage-unresolved`/`slot-unreadable`) with context-hash ones. Use seeded drive records (`seedRecord(t)` + `store.NewDrive`/`NewReservedDrive`) with `RunContextHash: capHash("ctx-A")`, and a scripted fake `ProcessSeam` (`Observe`/`Stop`/`ResolveReservation` per run dir). Required cases (L9, L10, Review Focus 1–2):
  - `TestCensusStopsRunningDriveOfTheRun` — nonterminal drive, `RawRunDir` observes `running`, `Stop` then re-observe `stopped` → `Accounted`, finding `replacement-stopped:<id>`.
  - `TestCensusSupervisorAlreadyGoneIsAccounted` — `RawRunDir` observes `vanished` (and, separately, `signaled`) → accounted with no stop issued.
  - `TestCensusSettlesNeverAttachedFirstLaunch` — reserved drive (`RawRunDir == ""`, `!RelaunchReserved`, `AdmissionToken` set), `ResolveReservation` → `never-launched` → record now HALTED `run-cancelled`; accounted. In observe-only mode the same drive reports `launch-pending:<id>` and is **not** settled (record unchanged).
  - `TestCensusStopsIdentifiedFirstLaunch` — same drive, `ResolveReservation` → `identified` with a run dir that observes `running` → stopped; accounted.
  - `TestCensusPendingOnClaimBusy` — hold `store.tryRelaunchClaim(id)` in the test → `claim-busy:<id>`, not accounted.
  - `TestCensusStopsLiveSupervisorOfHaltedDrive` — HALTED drive (`deadline-expired-stop-unproven`) whose `RawRunDir` observes `running` → cancel mode stops it; observe-only mode reports `run-live:<id>` and is not accounted.
  - `TestCensusChecksPriorRunDir` — nonterminal drive with `PriorRawRunDir` observing `running` and `RawRunDir` observing `running` → both stopped.
  - `TestCensusRunDirAbsentIsTornDownButProbeErrorIsPending` — a `RawRunDir` path that does not exist on disk → accounted with no `Observe` call; an existing dir whose `Observe` errors → `resolution-unresolved:<id>`, not accounted.
  - `TestCensusIgnoresOtherRunsDrives` (L10) — a running drive with `RunContextHash: capHash("ctx-B")` in the same worktree is never observed or stopped when reconciling `ctx-A`.
  - `TestCensusUnreadableRecordIsInformational` — a corrupt record → `history-unattributed:<id>`, still accounted.
  - `TestCensusPassedFailedAreSettled` — PASSED/FAILED drives of the run are accounted with no `Observe`.
  - `TestCensusEmptyContextAccountsVacuously` — `""` → accounted, no registry walk.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/gatedrive -run Census -count=1`
Expected: compile failure (signature) / FAIL.

- [ ] **Step 3: Implement the census** in `reconcile.go`. Replace the file header to describe context-hash attribution (drop the slot and "positive reference" rules). Core:

```go
func (d *Driver) ReconcileRunLaunches(runContextHash string) (RunLaunchReport, error) {
	return d.accountRunLaunches(runContextHash, false)
}

func (d *Driver) ObserveRunLaunches(runContextHash string) (RunLaunchReport, error) {
	return d.accountRunLaunches(runContextHash, true)
}

// accountRunLaunches walks the drive registry in id order and accounts every
// drive whose RunContextHash equals runContextHash — the hash run.start minted
// for the run and every tracked gate.drive.start stores. A drive without a run
// context is never attributed; an unreadable record is informational
// (history-unattributed), because nothing positively names it.
func (d *Driver) accountRunLaunches(runContextHash string, observeOnly bool) (RunLaunchReport, error) {
	report := RunLaunchReport{Accounted: true}
	if runContextHash == "" {
		return report, nil
	}
	entries, err := os.ReadDir(d.store.root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return report, nil
		}
		report.Accounted = false
		report.Findings = append(report.Findings, "registry-unreadable")
		return report, nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || validateID(id) != nil {
			continue
		}
		rec, lerr := d.store.Load(id)
		if lerr != nil {
			if storeErrIs(lerr, ErrNotFound) {
				continue // record-less directory: launched nothing
			}
			if h, herr := d.store.loadHistoricalDrive(id); herr == nil && isTerminalOutcome(h.LastOutcome) {
				continue // settled schema-2 history
			}
			report.Findings = append(report.Findings, "history-unattributed:"+id)
			continue
		}
		if rec.RunContextHash != runContextHash {
			continue
		}
		settled, finding := d.reconcileRunDrive(id, rec, observeOnly)
		if finding != "" {
			report.Findings = append(report.Findings, finding)
		}
		if !settled {
			report.Accounted = false
		}
	}
	return report, nil
}
```

`reconcileRunDrive(id, rec, observeOnly)`:
1. PASSED/FAILED → `(true, "")`.
2. HALTED → `d.proveRunDirsGone(id, rec, observeOnly)` (no claim: a terminal drive launches nothing more).
3. Nonterminal: `tryRelaunchClaim` (busy → `claim-busy:`; error → `resolution-unresolved:`), defer close, re-read (`record-unreadable:` on error); a re-read PASSED/FAILED → settled; a re-read HALTED → `proveRunDirsGone`.
4. `cur.RelaunchReserved` → existing `reconcileReservation(id, cur.RunRoot, cur.RelaunchToken, cur.OwnerGeneration, observeOnly)` (empty token → `resolution-unresolved:`).
5. `cur.RawRunDir == ""` (first launch never attached) → empty `AdmissionToken` is `resolution-unresolved:`; otherwise `reconcileFirstLaunch(id, cur, observeOnly)`: `ResolveReservation(cur.RunRoot, cur.AdmissionToken)`; `never-launched` → observe-only `launch-pending:`, else `settleNeverLaunchedFirstLaunch(id, cur.OwnerGeneration)`; `identified` → `supervisorGone(id, res.RunDir, observeOnly)`; anything else / error → `resolution-unresolved:`.
6. Otherwise → `d.proveRunDirsGone(id, cur, observeOnly)`.

`settleNeverLaunchedFirstLaunch` mirrors `settleNeverLaunchedCancelled` but its CAS guard is `r.RawRunDir == "" && !r.RelaunchReserved` (else `errRelaunchRaceLost`), writing HALTED `run-cancelled`; an already-terminal record is accounted.

`reconcileReservation`'s `identified` arm now calls `supervisorGone(id, res.RunDir, observeOnly)` (replacing `observeIdentifiedRun`/`stopIdentifiedRun`, which are deleted).

```go
// proveRunDirsGone applies the lock model's teardown proof to each of a drive's
// run dirs: the drive is torn down when every recorded supervisor is gone.
func (d *Driver) proveRunDirsGone(id string, rec driveRecord, observeOnly bool) (bool, string) {
	settled, last := true, ""
	for _, dir := range []string{rec.RawRunDir, rec.PriorRawRunDir} {
		if dir == "" {
			continue
		}
		ok, finding := d.supervisorGone(id, dir, observeOnly)
		if finding != "" {
			last = finding
		}
		if !ok {
			return false, finding
		}
	}
	return settled, last
}

// supervisorGone: a run dir that does not exist is clean absence (torn down);
// any Observe state other than running counts as torn down (passed, failed,
// signaled, stopped, vanished); a running supervisor is stopped (cancel mode)
// and must then observe as not running; an observe/stop error is unprovable and
// keeps cancel pending — a probe error is never clean absence.
func (d *Driver) supervisorGone(id, runDir string, observeOnly bool) (bool, string) {
	if _, err := os.Lstat(runDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, ""
		}
		return false, "resolution-unresolved:" + id
	}
	o, err := d.proc.Observe(runDir)
	if err != nil || o == nil {
		return false, "resolution-unresolved:" + id
	}
	if o.State != process.StateRunning {
		return true, "run-terminal:" + id
	}
	if observeOnly {
		return false, "run-live:" + id
	}
	if _, serr := d.proc.Stop(runDir, "gatedrive: run.cancel stopping a cancelled run's gate"); serr != nil {
		return false, "resolution-unresolved:" + id
	}
	o, err = d.proc.Observe(runDir)
	if err != nil || o == nil {
		return false, "resolution-unresolved:" + id
	}
	if o.State == process.StateRunning {
		return false, "run-live:" + id
	}
	return true, "replacement-stopped:" + id
}
```

Delete `censusRefs`, `censusReferences`, `observeIdentifiedRun`, `stopIdentifiedRun`. Do **not** touch `resolveDriveRun` here (the driver still uses it until Task 4). `stopProvesTeardown` stays only if the driver still uses it; otherwise delete it.

- [ ] **Step 4: Rewire the app adapters.** In `runtracker_cancel.go`:

```go
// runContextHash returns the run-tracker record's child_context_hash for runKey —
// the hash every drive started inside that run stores as run_context_hash.
func runContextHash(repoDir, runKey string) (string, error) {
	rec, err := LoadRunTrackerRecord(repoDir, runKey)
	if err != nil {
		return "", err
	}
	return rec.ChildContextHash, nil
}
```

`runLaunchReconciler.reconcile(contextHash string)`; `appLaunchReconciler.reconcile` calls `ReconcileRunLaunches(contextHash)`. In `reconcileRunTeardown` step (5c) resolve `runContextHash(repoDir, runKey)`; an error appends `run-context-unreadable` and sets `accounted=false`. In `verifyTerminalRunQuiescence` use `ep.RunKey` the same way (the census no longer needs `slotEp.Worktree`; keep `resolveTerminalRunSlot` for the slot retirement until Task 6). In `runtracker_complete.go`: `runLaunchObserver.observe(contextHash string)`, `appLaunchObserver.observe` → `ObserveRunLaunches(contextHash)`, and thread `repoDir, runKey` into `accountCompletionObligations` → `accountCompletionLaunches` (an unreadable context is a blocking `run-context-unreadable`). Update the fake reconcilers/observers in app tests to the new signatures.

- [ ] **Step 5: Re-point app census tests.** In every app test whose drives the census must now attribute (`runtracker_production_census_integration_test.go`, and the cancel/verdict fixtures `newCancelFixture` (`runtracker_cancel_helpers_test.go`) and `newVerdictCompletionFixture` (`runtracker_verdict_helpers_test.go`)), start or seed the run's drives with the run's raw run context (`StartRequest.RunContext` / `GateDriveStartRequest.RunContext` — the token `run.start` printed), so their `run_context_hash` equals the record's `child_context_hash`. Leave slot construction in place (Task 6 removes it).

- [ ] **Step 6: Run to verify**

Run: `go test ./internal/gatedrive -count=1 && go test ./internal/app -count=1 && go test -tags integration ./internal/app -run 'TestIntegrationRunCancel|TestIntegrationRunCompletion|TestIntegrationRunStart|TestIntegrationRunVerdict' -count=1`
Expected: PASS. Then mutation-check: make `accountRunLaunches` skip the `RunContextHash` comparison → `TestCensusIgnoresOtherRunsDrives` FAILS; make `reconcileRunDrive` return `(true, "")` for HALTED → `TestCensusStopsLiveSupervisorOfHaltedDrive` FAILS. Restore from a backup copy.

- [ ] **Step 7: Commit**

```bash
git add internal/gatedrive/reconcile.go internal/gatedrive/reconcile_test.go internal/app/runtracker_cancel.go internal/app/runtracker_complete.go <each app test/fixture file you changed>
git commit -m "refactor(runtracker): attribute a run's drives by run context; teardown proof is the supervisor gone (0490)"
```

---

### Task 4: Drive admission, launch, and the single relaunch take the worktree lock

**Tier: max.** Risk: the core admission authority changes at every drive path (Admit, StartAdmitted, abandon, launch failure, relaunch, crash-window recovery), under concurrency; a mistake admits two suites into one worktree or wedges it. Interfaces are consumed by finalize, recertify, and the build controller.

**Files:**
- Modify: `internal/gatedrive/driver.go`, `drive.go` (`CauseWorktreeBusy`), `ownership.go` (comments for `ErrWorktreeBusy`, `ErrUnresolvedLaunchTransition`)
- Modify: `internal/gatedrive/driver_test.go`, `driver_concurrency_test.go`, `driver_faults_test.go`, `driver_runfence_test.go`, `run_launch_gate_test.go`, `launch_sites_guard_test.go`, `supervisor_integration_test.go` (or a new `driver_lock_integration_test.go`)
- Modify: `internal/app/gate_drive.go`, `internal/app/gate_drive_test.go`, `internal/app/finalize_rebase.go` (only if `mapDriveHaltCause` needs a doc touch — no behavior change)
- Delete: `internal/app/gate_drive_history_integration_test.go`
- Create: `internal/app/gate_drive_lock_integration_test.go` (`TestIntegrationBuildStart…`)
- Modify: `tests/test_go_integration_app_buildstart.sh` header (describe the new tests)

**Interfaces:**
- Consumes: `Store.TryWorktreeLock`, `WorktreeLock.{TakeFile,Release,WriteHolder,PriorHolder}`, `GitSeam.WorktreeRoot`, `LaunchRequest.WorktreeLock`.
- Produces:
  - `StartRequest.Owner string` (not persisted; `"build"`, `"finalize"`, or `""`), set by `GateDriveService.startRequest` from `s.owner`.
  - `AdmissionTicket` fields: `id, ownerGen string; rec driveRecord; lock *WorktreeLock; owner string; runID string` (no `token`, no `legacy`).
  - `const CauseWorktreeBusy = "worktree-busy"` in `drive.go`'s exported cause block.
  - `func (rec *driveRecord) launchRequest(token string, worktreeLock *os.File) process.LaunchRequest` — the single request builder for both drive launch sites; its body is the only `process.LaunchRequest{…}` literal in gatedrive and always sets `WorktreeLock:` (Task 5's repoguard guard keys on that).
  - `driveEngine` (app) loses `ReconcileFinishedIncumbent`.

- [ ] **Step 1: Write the failing tests.**
  - `driver_test.go`:
    - `TestAdmitRefusesWorktreeBusyAndCreatesNoDrive` — hold `store.TryWorktreeLock(root)` (root = `fakeGit.WorktreeRoot(req.Cwd)`), `Admit` → `worktree-busy`; the drive registry has no new directory.
    - `TestAbandonAdmissionFreesWorktree` — `Admit`, `AbandonAdmission`, then `TryWorktreeLock` succeeds and the reserved drive is gone.
    - `TestStartAdmittedRunRefusalFreesWorktree` — the run launch gate refuses at `StartAdmitted` → drive HALTED `run-cancelled`, lock free.
    - `TestLaunchFailureFreesWorktree` — fake `Launch` returns an error (and closes `req.WorktreeLock`, like the real one) → drive HALTED `launch-failed`, lock free, no `ResolveReservation`/slot call needed.
    - `TestLaunchHandsLockAndWritesHolder` — the fake captures `req.WorktreeLock != nil` and `req.ReservationToken == <drive's admission_token>` (32 hex, minted by the driver); after Start, `holder.json` names `kind=drive`, the drive id, the run dir, the change id, `owner=build`.
    - `TestAdmitLockIOErrorIsNotFree` — `fakeGit.rootErr` set → `Admit` fails with a typed IO store error, no drive.
    - **L4** `TestRelaunchFindsWorktreeHeldHaltsWorktreeBusy` — idempotent drive, first run observes `vanished`; before `Advance`, the test takes the lock for the root; `Advance` → HALTED `worktree-busy`, `proc.launches()` == 1, the relaunch claim is released (a second `tryRelaunchClaim` succeeds).
    - `TestRelaunchRewritesHolderKeepingOwner` — relaunch succeeds; `holder.json` names the replacement run dir and keeps `owner` from the first note.
    - `TestTerminalDocAlwaysExposesRunRoot` — PASSED, FAILED, and HALTED documents all carry `RunRoot`.
  - `driver_concurrency_test.go`: re-target `TestTwoScopelessStartsOneWorktreeOneLaunch` and `TestSameWorktreeRaceAcrossOwnersRawAndAliasOneWinner` (exactly one launch). `countingProc.Launch` must **retain** `req.WorktreeLock` (append to a mutex-guarded slice closed in `t.Cleanup`) to emulate a live supervisor. In the alias race, set each request's `Cwd` to its worktree spelling (`wt` / `alias`) so the fake's `EvalSymlinks` maps both to one key, and replace the raw contender's `store.ReserveRawWorktreeExecution(...)` with `store.TryWorktreeLock(<EvalSymlinks(wt)>, nil)` (retain the winner's lock until cleanup). Accept only `ErrWorktreeBusy` for losers.
  - Every other fake `ProcessSeam.Launch` in gatedrive tests (`fakeProc`, `racingProc`, `terminalSettleProc`, `gatedLoserSeam`, `claimWindowProc`) closes `req.WorktreeLock` (a shared `releaseHandedLock(req process.LaunchRequest)` helper) unless a test explicitly needs a live holder.
  - **L1** (integration, prefix `TestIntegrationGatedrive`): `TestIntegrationGatedriveKilledSupervisorFreesWorktreeForNextStart` — real `process.Service`, real git worktree, a drive whose command sleeps; SIGKILL only the supervisor (read `supervisor_pid` from the run dir's `manifest.json`); the next `Start` on the same worktree is admitted with no recovery call; clean up the surviving group.
  - **L5** (`internal/app/gate_drive_lock_integration_test.go`, `//go:build integration`, prefix `TestIntegrationBuildStart`): `TestIntegrationBuildStartBusyWorktreeCreatesNoDriveAndChargesNothing` — real repo, `NewBuildGateDriveService`; hold the lock via `gatedrive.OpenStore(commonDir).TryWorktreeLock(<DiscoverWorktree root>, nil)`; `Start` with a `ChangeID` → reason `worktree-busy`, stage `worktree-admission`, no drive directory under `gate-drives/v2`, `SuiteBudgetUsage` used == 0; release; a second start is admitted (stop its run in cleanup). Also `TestIntegrationBuildStartBusyRefusalNamesRunningHolder` — first start admitted and running; a second start → locator `incumbent-drive:<first id>`, message names the change id.
  - `gate_drive_test.go`: add a `mapDriveHaltCause(gatedrive.CauseWorktreeBusy)` row → `GateHaltUnavailable`; replace `TestMapDriveResultWorktreeAdmissionRefusal` cases with: busy with a drive holder (stage, `incumbent-drive:<id>` locator, message naming change id and `run.cancel`), busy with a finalize holder (message says wait for finalize's gate), busy with a raw holder (`incumbent-run:<run-id>`, message names `gate stop '<dir>'`), busy with no holder (empty locator, "holder unknown" message naming `lsof` and `busy.lock`). Delete `TestBudgetedBuildReconcilesBeforeRefusal`, `TestBudgetedBuildAdvisoryReconcilesUnderPresentedRun`; re-point `TestBusyRefusalChargesNoSuiteAttempt` / `TestAdmitRefusalChargesNoSuiteAttempt` onto the engine's `Admit` returning `worktree-busy` (no advisory precheck exists any more).

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/gatedrive ./internal/app -count=1`
Expected: compile errors / FAIL on the new cases.

- [ ] **Step 3: Implement in `driver.go`.**
  - Add `func (d *Driver) lockWorktree(cwd string) (*WorktreeLock, error)`: `root, err := d.git.WorktreeRoot(cwd)`; error → `storeErr(ErrIO, opWorktreeAdmission, err)`; else `d.store.TryWorktreeLock(root, d.proc)`.
  - `Admit`: drop the finished-incumbent retry. Inside `runLaunchGated(req.RunID, req.Worktree, …)` call `admitScopeless(rec, ownerGen, req.Owner)`:

```go
func (d *Driver) admitScopeless(rec driveRecord, ownerGen, owner string) (*AdmissionTicket, error) {
	lock, err := d.lockWorktree(rec.Cwd)
	if err != nil {
		return nil, err
	}
	token, err := randomToken(genNBytes)
	if err != nil {
		lock.Release()
		return nil, storeErr(ErrIO, "start", err)
	}
	rec.AdmissionToken = token // the launch token; ResolveReservation maps a lost response to this run
	id, _, err := d.store.NewReservedDrive(rec)
	if err != nil {
		lock.Release()
		return nil, err
	}
	return &AdmissionTicket{id: id, ownerGen: ownerGen, rec: rec, lock: lock, owner: owner}, nil
}
```

  Lock order: `run.lock` (inside `runLaunchGated`) → worktree lock (tried) → the drive store's CAS lock.
  - `revalidateAdmittedLaunch`: delete step (b) `verifyAdmittedSlot`. On any error, `t.lock.Release()` (after `settleAdmittedAfterRunRefusal` on the not-entered leg). `settleAdmittedAfterRunRefusal` keeps the HALTED `run-cancelled` CAS and drops the slot release.
  - `AbandonAdmission`: `removeReservedDrive(t.id)`; `t.lock.Release()`; return nil.
  - `launchScopeless`:

```go
	out, lerr := d.proc.Launch(rec.launchRequest(rec.AdmissionToken, t.lock.TakeFile()))
	if lerr != nil {
		// Launch closed the lock on every path. Keep the drive as evidence.
		_ = d.store.ownerCAS(id, /* HALTED "launch-failed" as today */)
		return DriveDoc{}, fmt.Errorf("gatedrive: start launch: %w", lerr)
	}
	if err := d.store.attachLaunch(id, ownerGen, out.RunDir, out.RunID); err != nil {
		d.stopIfOwned(out.RunDir) // the supervisor's exit frees the worktree
		return DriveDoc{}, err
	}
	t.lock.WriteHolder(HolderNote{Kind: "drive", DriveID: id, RunDir: out.RunDir, ChangeID: rec.ChangeID, Owner: t.owner})
	claim.close()
```

  then `driveAndPersist` (no `startDocWithLegacy`). Delete `ConfirmWorktreeExecution` use, `resolveWorktreeAfterLaunchFailure`, `releaseOrUnresolveWorktree`, `verifyAdmittedSlot`, `startDocWithLegacy`, `Driver.reserveWorktreeExecution`.
  - `settledTerminalDoc`: a terminal record → `d.recordedDocWithRoot(id, ownerGen, rec, true)`; delete `releaseAdmissionIfProven`, `releaseFinding`, `stopProvesTeardown` (if Task 3 left it unused). `recordedDoc` exposes the root for every terminal outcome. Update the `DriveDoc.RunRoot` comment ("always exposed on a terminal document").
  - Delete `resolveDriveRun` and `recoveryRunRevoked`. `authorizeRelaunch(id, ownerGen)` becomes the bare `d.store.reserveRelaunch(id, ownerGen)` (rename it `reserveRelaunchClaim` or inline it; no run gate). `recoverReservedRelaunch` loses its `revoked` parameter and its `run-cancelled` arm. `Advance` drops the `recoveryRunRevoked` call.
  - `driveSlice` relaunch, immediately before `d.proc.Launch`:

```go
				lock, kerr := d.lockWorktree(rec.Cwd)
				if kerr != nil {
					claim.close()
					if oe, ok := AsOwnershipError(kerr); ok && oe.Kind == ErrWorktreeBusy {
						return halt(&res, CauseWorktreeBusy) // another gate got there first: never relaunch over it
					}
					return halt(&res, "relaunch-failed")
				}
				prior, _ := lock.PriorHolder()
				out, lerr := d.proc.Launch(rec.launchRequest(claim.token, lock.TakeFile()))
```

  and after a successful `attachReservedRelaunch`, write `HolderNote{Kind: "drive", DriveID: id, RunDir: out.RunDir, ChangeID: rec.ChangeID, Owner: ownerIf(prior, id)}` where `ownerIf` returns `prior.Owner` when `prior.DriveID == id`, else `""`. The existing `launch-unconfirmed` / `relaunch-failed` HALT legs on a launch error stay.
  - `launchRequest(token string, worktreeLock *os.File)` replaces `launchRequest()`/`launchRequestWithReservation`; comment: the token is the drive-minted launch token.
  - `drive.go`: add `CauseWorktreeBusy` to the exported cause block (comment: a relaunch found the worktree lock held by another gate). Update the `AdmissionToken` field comment (driver-minted launch token, JSON key unchanged).
  - `ownership.go`: rewrite the `ErrWorktreeBusy` comment around the lock; the `ErrUnresolvedLaunchTransition` comment now names only the claim-busy and settled-record legs of `revalidateAdmittedLaunch`.
  - Remove `StartRequest.RunID`'s slot sentences ("recorded on the worktree execution slot … fences the worktree"); it is now only the run-launch-gate locator (kept for 0491).

- [ ] **Step 4: Narrow the gatedrive launch-site guard** (`launch_sites_guard_test.go`). Its subject partly survives: the first launch must still cross the run launch gate (kept for 0491); the single relaunch no longer does (spec Problem fact 6 — no production drive both carries a run and can relaunch). Extend `analyzeGatedriveLaunchSites` so a launch site is satisfied when its function is run-gate-guarded (the existing rule) **or** its function itself calls `lockWorktree` before the launch. Update `TestLaunchSiteGuardIsFalsifiable`'s synthetic baseline: `driveSlice` calls `lockWorktree` then launches, and a new mutation strips that call → violation. Keep the existing mutation (strip `runLaunchGated` from the StartAdmitted path → violation). Population: `launchSites == 2`. Update the file header to say why the relaunch is lock-guarded instead of run-gated.

- [ ] **Step 5: Implement in `internal/app/gate_drive.go`.**
  - `driveEngine`: remove `ReconcileFinishedIncumbent` (and its `var _` assertion stays valid).
  - `startBudgetedBuild`: delete step 1 (advisory precheck + reconciliation); order becomes budget precheck → `Admit` → `reserveBuildSuiteAttempt` (on refusal `AbandonAdmission`) → `StartAdmitted`. Rewrite its doc comment ("the lock is taken before the charge").
  - `startRequest`: set `Owner: s.owner`.
  - `mapDriveResult`: for `oe.Kind == gatedrive.ErrWorktreeBusy` always set `Stage = stageWorktreeAdmission`, `Locator = incumbentRefusalLocator(oe.Incumbent)`, `Message = incumbentRemedyMessage(oe.Incumbent)` — also when `Incumbent` is nil. Keep the legacy-inventory and `Reconciliation` branches until Task 7 (they become unreachable from drives).
  - `incumbentRemedyMessage(inc *gatedrive.IncumbentSnapshot) string` (drop the kind parameter):
    - `nil` → "holder unknown: another gate's supervisor holds this worktree's lock and it could not be confirmed running; wait for it to finish — the worktree frees itself when that gate ends — or find the holding process with lsof on this worktree's busy.lock under the repository's Git common dir (docket/worktree-locks/<key>/busy.lock); never start a second gate here"
    - `Kind == "raw"` → "a raw gate run holds this worktree (run dir <quoted dir>); wait for it, or stop it with docket gate stop <quoted dir> --reason <why> — the worktree frees itself when the run ends"
    - `Owner == "finalize"` → "finalize's local gate for change <id> holds this worktree; wait for it to finish — it frees the worktree when it ends"
    - otherwise → "change <id>'s build gate (drive <drive id>) holds this worktree; wait for it, or stop the owning run with the run.cancel operation (--key <key> --run-id <id> --reason <why>) — the worktree frees itself when that gate ends"
  - `ownershipNextAction(ErrWorktreeBusy)`: same meaning, lock wording.
  - `GateDriveStartRequest.RunID` comment: drop "recorded on the worktree execution slot".

- [ ] **Step 6: Delete tests whose subject is gone in this task's scope** (premise deleted, not re-gated): in `driver_test.go` the slot-release/`ReleaseFinding`/legacy-summary cases (`TestStartCarriesLegacyHistorySummary` and siblings); in `driver_faults_test.go` the slot-transition fault cases; in `driver_runfence_test.go` the relaunch and recovery run-check cases (`authorizeRelaunch`/`recoveryRunRevoked`/`CauseRunLinkLost`) — keep and re-point the `runLaunchGate`-at-`Admit`/`StartAdmitted` cases; in `run_launch_gate_test.go` drop slot assertions, keep the start-time run check. Delete `internal/app/gate_drive_history_integration_test.go` (its two tests assert the legacy inventory and a slot refusal). For every deletion, the commit body names the surviving test that covers the property, or says the property is retired by the spec.

- [ ] **Step 7: Run to verify**

Run: `go test ./internal/gatedrive ./internal/app ./internal/repoguard -count=1` then `go test -tags integration ./internal/gatedrive -run 'TestIntegrationGatedrive' -count=1` and `go test -tags integration ./internal/app -run 'TestIntegrationBuildStart|TestIntegrationGateLifecycle|TestIntegrationFinalizeRebaseOps|TestIntegrationEvidence' -count=1` and `go test -race -tags integration ./internal/gatedrive -run TestRaceIntegrationGatedrive -count=1`
Expected: PASS. `TestGateLaunchAdmissionCoverage` stays green (gatedrive is interior to it; app/gate.go still reserves until Task 5). Mutation-check L4: delete the `lockWorktree` call in `driveSlice` → `TestRelaunchFindsWorktreeHeldHaltsWorktreeBusy` and the narrowed launch-site guard FAIL; restore from backup.

- [ ] **Step 8: Commit**

```bash
git add internal/gatedrive internal/app/gate_drive.go internal/app/gate_drive_test.go internal/app/gate_drive_lock_integration_test.go tests/test_go_integration_app_buildstart.sh
git rm internal/app/gate_drive_history_integration_test.go
git commit -m "refactor(gatedrive): drive admission, launch, and relaunch take the worktree lock (0490)"
```

(`git add internal/gatedrive` here means only the files this task changed — list them explicitly; never `git add -A`.)

---

### Task 5: Raw `gate.launch` takes the worktree lock; every `Launch` site must hand one over

**Tier: standard.** Risk: the raw path is the third launch site; the repoguard guard must key on syntactic shape and be mutation-tested.

**Files:**
- Modify: `internal/app/gate.go`
- Modify: `internal/app/gate_integration_test.go`, `internal/app/gate_test.go`, `internal/cli/gate_test.go`
- Modify: `internal/repoguard/gatelaunch_admission_test.go` (rewrite)

**Interfaces:**
- Consumes: `Store.TryWorktreeLock`, `WorktreeLock`, `LaunchRequest.WorktreeLock`, `incumbentRefusalLocator`.
- Produces: `func resolveWorktreeAdmission(cwd string) (worktreeRoot string, store *gatedrive.Store, ok bool)` (no repo identity, no settlement wiring).

- [ ] **Step 1: Write the failing tests.**
  - `internal/cli/gate_test.go` `TestGateLaunchInsideWorktreeSecondRefused`: re-target, don't delete — two raw launches into one worktree, exactly one launched, the second refused `worktree-busy`. Add: after the first run is stopped (`gate stop`), a third launch is admitted with no other step.
  - `gate_integration_test.go`: replace `…GateLaunchInsideWorktreeReservesSlot` with `TestIntegrationGateLifecycleRawLaunchHoldsWorktreeLock` (while running, `TryWorktreeLock` is busy; the busy refusal's `Cause` is `incumbent-run:<run id>`; `holder.json` names `kind=raw`, `owner=raw`, the run dir); replace `…GateStopReleasesRawSlot` with `TestIntegrationGateLifecycleStoppedRawRunFreesWorktree`. Delete `…GateLaunchLegacyInventoryRefusalNamesMatchedDrive` and `…GateLaunchSettlesFinishedRawIncumbent` (subjects retired). A launch whose `cwd` is outside git takes no lock (keep/adapt the existing non-git case).
  - `gate_test.go`: `TestGateLaunchRefusalCauseFromSnapshot` → assert the cause derives from the holder snapshot on `worktree-busy`.
  - `internal/repoguard/gatelaunch_admission_test.go`: rewrite `TestGateLaunchAdmissionCoverage` (see Step 3).

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/app ./internal/cli ./internal/repoguard -count=1`
Expected: FAIL on the re-targeted cases.

- [ ] **Step 3: Implement.**
  - `GateLaunch`:

```go
	worktreeRoot, store, admit := resolveWorktreeAdmission(cwd)
	var lock *gatedrive.WorktreeLock
	if admit {
		l, lerr := store.TryWorktreeLock(worktreeRoot, svc)
		if lerr != nil {
			r, reason := mapAdmissionFailure(lerr)
			return GateResult{Envelope: NewEnvelope(OperationGateLaunch, r), Reason: reason, Cause: admissionRefusalCause(lerr)}
		}
		lock = l
	}
	out, err := svc.Launch(process.LaunchRequest{Root: root, Cwd: cwd, Argv: argv, WorktreeLock: lock.TakeFile()})
	if err != nil {
		res, reason := mapGateFailure(err)
		return GateResult{Envelope: NewEnvelope(OperationGateLaunch, res), Reason: reason}
	}
	lock.WriteHolder(gatedrive.HolderNote{Kind: "raw", RunDir: out.RunDir, Owner: "raw"})
```

  (`TakeFile`/`WriteHolder` are nil-safe, so a cwd outside git passes a nil lock.) No reservation token (Launch mints its own).
  - Delete `rawGateRun`, `rawStaleRunRefusal`, `admissionRefusalLegacy`, `resolveRawLaunchFailure`, `rawStopIfOwned`, `releaseRawSlotForStop` and its call in `GateStop` (`gate.stop` now only stops), and `GateResult.LegacyHistory`. Keep `rawTeardownProven` (Task 6 replaces it). `admissionRefusalCause` becomes `incumbentRefusalLocator(oe.Incumbent)` for an ownership error, else `""`. `mapAdmissionFailure` unchanged. Rewrite `GateLaunch`'s doc comment around the lock.
  - Rewrite `TestGateLaunchAdmissionCoverage`: AST-walk every maintained non-test `.go` file under `internal/` (keep the existing `maintainedPop` and parse-failure behavior). A **launch site** is a call whose selector is `Launch` with exactly one argument, outside `internal/process`. Each site's argument must be either (a) a composite literal whose type is `process.LaunchRequest` (or `LaunchRequest`) and which has a `WorktreeLock` key, or (b) a call to a function/method declared in the **same package** whose body contains such a literal (resolve by the callee's name among that package's `FuncDecl`s). Population floor: at least 3 launch sites (app `GateLaunch`, the two gatedrive drive sites) and at least 1 site of each form. Keep a `detectors_non_vacuity` subtest over synthetic sources: a literal with `WorktreeLock:` passes; a literal without it fails; a helper-built request whose helper lacks it fails; a helper-built request whose helper has it passes. Rewrite the header comment (it guards the lock hand-over, not reserve/confirm).

- [ ] **Step 4: Run to verify**

Run: `go test ./internal/app ./internal/cli ./internal/repoguard -count=1` and `go test -tags integration ./internal/app -run 'TestIntegrationGateLifecycle' -count=1`
Expected: PASS. Mutation-check: remove `WorktreeLock:` from `GateLaunch`'s literal → the repoguard test FAILS naming `internal/app/gate.go`; remove it from `driveRecord.launchRequest` → FAILS naming gatedrive; restore from backup.

- [ ] **Step 5: Commit**

```bash
git add internal/app/gate.go internal/app/gate_integration_test.go internal/app/gate_test.go internal/cli/gate_test.go internal/repoguard/gatelaunch_admission_test.go
git commit -m "refactor(gate): raw gate.launch takes the worktree lock; guard every Launch site's hand-over (0490)"
```

---

### Task 6: The run tracker without the slot

**Tier: premium.** Risk: cancel, closeout, resume, and the mutation fence change their evidence; a wrong reduction either wedges `cancellation-pending` or reports success too early. Accepted losses are listed in the spec — do not reintroduce them.

**Files:**
- Modify: `internal/app/runtracker_cancel.go`, `runtracker_complete.go`, `runtracker_start.go`, `runtracker_fence.go`, `agent_guardian.go` (comments only), `gate.go` (`rawTeardownProven` → `supervisorGone`)
- Modify: `internal/app/runtracker_cancel_helpers_test.go`, `runtracker_cancel_integration_test.go`, `runtracker_complete_integration_test.go`, `runtracker_complete_helpers_test.go`, `runtracker_verdict_helpers_test.go`, `runtracker_verdict_integration_test.go`, `runtracker_start_resume_integration_test.go`, `runtracker_start_integration_test.go`, `runtracker_production_census_integration_test.go`, `runtracker_fence_integration_test.go`, `runtracker_no_run_record_resume_e2e_integration_test.go`, `runtracker_publication_settle_paths_test.go`

**Interfaces:**
- Consumes: `ReconcileRunLaunches(contextHash)`, `ObserveRunLaunches(contextHash)`, `runContextHash` (Task 3).
- Produces: `cancelSeams` without `retire`; `func supervisorGone(st process.State) bool { return st != process.StateRunning }` (app); `validateResumeQuiescence(seams, repoDir string, ep RunRecord) (bool, string)`; `verifyTerminalRunQuiescence(seams, repoDir string, ep RunRecord) (bool, []string)`.

- [ ] **Step 1: Write the failing tests first.**
  - Rewrite `newCancelFixture` and `newVerdictCompletionFixture` onto drives carrying the run's context (Task 3 already gave them the context; now delete every `ReserveWorktreeExecutionForRun`/`ConfirmWorktreeExecution`/`ReleaseWorktreeExecution`/`LoadWorktreeExecution` call and the `loadSlotRun`/`loadSlotState`/`installSuccessor`/`releaseFixtureSlot`/`seedResumeSlot`/`slotReservationToken` helpers).
  - `TestIntegrationRunCancelFreesWorktreeForNextStart` (Review Focus 4): a tracked run whose drive runs a sleeping suite under real `process.Service`; `run.cancel` → `cancelled`; the next `TryWorktreeLock` on that worktree root succeeds (or a next drive `Start` is admitted) with no other step.
  - `TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled`: the run's drive's supervisor already vanished (SIGKILL it) → `cancelled`, not `cancellation-pending`.
  - `TestIntegrationRunCancelRepeatOnTerminalRunRerunsCensus`: a repeat cancel of a cancelled run with no live drive → `already-cancelled`; with a live drive of the run (seeded) → `refused` naming the finding.
  - Completion: a successful closeout with only PASSED drives completes; one with a run-attributed HALTED drive whose supervisor runs → `completion-unaccounted` with `run-live:<id>`.
  - Resume: `run.start --resume` after a confirmed cancel admits exactly one replacement with no slot step; with a live drive of the predecessor → `cancellation-pending`.
  - Fence: a metadata write in a worktree with no readable owning run is admitted (the slot fallback is gone — accepted loss); keep every run-record-based fence case.

- [ ] **Step 2: Run to verify they fail**

Run: `go test -tags integration ./internal/app -run 'TestIntegrationRunCancel|TestIntegrationRunCompletion|TestIntegrationRunStart|TestIntegrationRunFence|TestIntegrationRunVerdict' -count=1`
Expected: FAIL / compile errors.

- [ ] **Step 3: Implement.**
  - `runtracker_cancel.go`: delete `cancelSeams.retire`, `retireSlot`, `retireWorktreeSlotOwnership`, `retireSlotOwnership`, `slotRetirement`, `resolveTerminalRunSlot`, `storedScopeWorktree`, `slotOwnershipClass` + `classifySlotOwnership`, `markWorktreeSlotStopping`, `reconcileWorktreeSlot`. `reconcileRunTeardown`: step (5) keeps the participant stop without the slot mark; delete (5b). `runCancel` step (8): `!accounted` → pending; else CAS `cancelling→cancelled` (no retire). `verifyTerminalRunQuiescence(seams, repoDir, ep)` runs the census for `runContextHash(repoDir, ep.RunKey)` plus the journal. `repairTerminalRun`: quiescent → `already-cancelled`; else `refused` with findings. `appGateStopper.stopProcess` → `out.Performed || supervisorGone(out.State)`. Rewrite the file header and the `cancelSeams`/`nativeTaskCanceller`/`runLaunchReconciler` comments without the slot.
  - `runtracker_complete.go`: delete `accountCompletionSlot` and the step (6) retire; `durableExecutionProof` keeps only the `TerminalDriveOutcomeForRunDir` PASSED/FAILED branch; `appGateObserver` uses `supervisorGone`. Rewrite the header's step list.
  - `runtracker_start.go`: `validateResumeQuiescence` = `verifyTerminalRunQuiescence`; drop the `worktree` parameter only if nothing else needs it.
  - `runtracker_fence.go`: delete `slotNamedRunUnresolved` and its call in `admitWorkflowMutation`; delete `findRunByID` and `ErrRunOwnerUnresolved` **only if** a whole-repo grep finds no other user.
  - `gate.go`: replace `rawTeardownProven` with `supervisorGone(st process.State) bool` (any non-`running` state proves the supervisor is gone).
  - `agent_guardian.go`, `runtracker_run_record.go`: comment-only updates naming the census instead of the slot.

- [ ] **Step 4: Delete or re-point the remaining slot tests** in the files listed above: delete retirement, foreign/linked/unowned slot, successor-slot, `rawStaleRunRefusal`, `resolveWorktreeAdmission`-resolver, slot-run-fence, and `slotNamedRunUnresolved` cases; re-point cases whose property survives (fence admission/refusal from run records, census attribution, resume one-replacement) onto the context-hash fixtures. The commit body lists each deleted test and the property's new home or "retired by spec".

- [ ] **Step 5: Run to verify**

Run: `go test ./internal/app -count=1` and `go test -tags integration ./internal/app -run 'TestIntegrationRun' -count=1` and `go test -race -tags integration ./internal/app -run TestRaceIntegrationAppConcurrency -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add <each internal/app file changed above>
git commit -m "refactor(runtracker): cancel, closeout, resume, and the mutation fence stop reading the slot (0490)"
```

---

### Task 7: Delete the slot store, the legacy inventory, `gate.history.cleanup`, and their dead fields

**Tier: standard.** Risk: wide mechanical deletion; the risk is a stale correspondence entry (catalog, schema registry, asset-independence set) or deleting a still-used helper. Every deletion is preceded by a whole-repo grep.

**Files:**
- Delete: `internal/gatedrive/admission.go`, `admission_retire.go`, `incumbent.go`, `admission_test.go`, `admission_retire_test.go`, `incumbent_test.go`, `slot_run_fence_test.go`, `history_integration_test.go`, `history_race_integration_test.go`; `internal/app/gate_history.go`, `gate_history_test.go`
- Modify: `internal/gatedrive/history.go` (keep `loadHistoricalDrive`, `historicalDrive`, `historicalView`, the schema-2 decoder; delete `recoverySeam`, `Legacy*`, `classifyLegacyDrive`, `legacyBindingMatches`, `HistoryCleanup*`, `cleanupHistory`, `assessLegacyRecord`, `tallyCleanupClasses`), `history_test.go` (keep historical-reader cases), `store.go` (`admissionRoot`, `runSettled`), `ownership.go` (`ErrLaunchUnconfirmed`, gatedrive `ErrStaleRunID`, `OwnershipError.Legacy`, `.Reconciliation`; `IncumbentSnapshot.State`, `.RunOwned`, `.RunUnresolved`), `drive.go` (`CauseRunLinkLost`, `DriveDoc.ReleaseFinding`, `DriveDoc.LegacyHistory`, the schema comment about "first-admission legacy-history assessment and the `docket gate history cleanup` recovery command" → "the launch census"), `driver.go` (`ProcessSeam.ClassifyRun`, `CleanupHistory`, `SetRunSettledResolver`/`RunSettledResolverWired` if they live there), `storage_reset_test.go`, `old_records_test.go`, `driver_faults_test.go`, `sequence_race_integration_test.go`, every fake that still implements `ClassifyRun` only for the seam (harmless to keep; delete if unused)
- Move: `TerminalDriveOutcomeForRunDir` from `incumbent.go` to `run_waiting.go` (unchanged body); `mkWorktree` from `admission_test.go` to a surviving test file if still used
- Modify (app): `gate_drive.go` (`GateDriveResult.LegacyHistory`, `legacyInventoryLocator`, `legacyInventoryMessage`, `appendReconciliationFinding`, the legacy/reconciliation branches of `mapDriveResult`, `ownershipNextAction` cases for `ErrLaunchUnconfirmed` and `gatedrive.ErrStaleRunID`, `HumanText`'s `release_finding:`/`legacy_history:` lines and `legacySummary`), `gate_drive_test.go` (legacy/release-finding/`CauseRunLinkLost` rows; `TestMapDriveFailureOwnershipKinds`/`…NextAction` lists), `finalize_rebase.go` (`mapDriveOutcome`'s `ReleaseFinding` leg; keep the defensive `RunRoot == ""` leg), its test `TestFinalizeCleanupRetainsRootOnUnsettledRelease` (delete), `runtracker_run_record.go` (`runSettledResolver`), `gate_drive.go` + `runtracker_continuation.go` (`SetRunSettledResolver` wiring), `runtracker_run_record_integration_test.go` (resolver tests), `schema_registry.go` (`gate.history.cleanup` row)
- Modify (cli): `gate.go` (`histCmd`, `cleanupHist`, remove `histCmd` from `gateCmd.AddCommand`), `install.go` (`"gate history"`, `"gate history cleanup"` in `assetIndependent`), `gate_test.go` (`TestGateHistoryCleanup*` ×4)

**Interfaces:** none produced; this task only removes.

- [ ] **Step 1: Inventory.** Run `git grep -n -E 'admissionRecord|ReserveWorktreeExecution|ReserveRawWorktreeExecution|ConfirmWorktreeExecution|ReleaseWorktreeExecution|MarkWorktreeExecution|LoadWorktreeExecution|WorktreeAdmissionRefusal|RetireWorktreeExecutionRun|ReconcileFinishedIncumbent|reconcileFinishedIncumbent|settleStaleReleasedRun|RunSettledFunc|SetRunSettledResolver|runSettledResolver|ErrRunRecordUnresolved|LegacyHistorySummary|LegacyFinding|inventoryLegacyDrives|classifyLegacyDrive|CleanupHistory|cleanupHistory|GateHistoryCleanup|gate\.history\.cleanup|ErrLaunchUnconfirmed|ReleaseFinding|CauseRunLinkLost|admissionRoot|admissionKey|canonicalizeMissingPath' -- ':!docs/superpowers/plans' ':!docs/results' ':!docs/changes/archive' ':!docs/adrs'` and sort each hit into delete / move / comment.
- [ ] **Step 2: Delete and move** per the file list. `ErrUnresolvedLaunchTransition` and `ErrWorktreeBusy` stay. `loadHistoricalDrive` stays (the census uses it).
- [ ] **Step 3: Verify correspondence guards.** Run `go test ./internal/cli -run 'TestSchemaCatalogCorrespondence|Asset|Capability|Representative' -count=1` and `go test ./internal/app -run TestEveryRequestAndResultStructIsBound -count=1`. Expected: PASS with `gate.history.cleanup` gone from the catalog, the schema registry, and the asset-independence set in one change.
- [ ] **Step 4: Run** `go build ./... && go vet ./... && go test ./internal/... -count=1` and `go test -tags integration ./internal/gatedrive ./internal/app -count=1`. Expected: PASS. Re-run the Step 1 grep: the only remaining hits are frozen records (excluded) and nothing else.
- [ ] **Step 5: Commit**

```bash
git rm <each deleted file>
git add <each modified file>
git commit -m "refactor: delete the worktree slot store, legacy inventory, and gate.history.cleanup (0490)"
```

---

### Task 8: Prose, generated mirror, and prose guards

**Tier: standard.** Risk: skill bodies ship into other repos (no repo-local claims); prose guards pin section terminators and word counts — edit them in this same commit (learning: intermediate-task-state-buildable).

**Files:**
- Modify: `skills/docket-build/references/gate-caller-loop.md` (section *Worktree admission — one live gate per worktree, and what `worktree-busy` means*)
- Modify: `skills/docket-finalize-change/references/gate-failure.md` (section *The finalize gate shares the worktree's one execution slot*)
- Regenerate: `internal/assets/embedded/…` via `go generate ./internal/assets/`
- Modify: `internal/repoguard/prose_contracts_test.go` (the `terminator:` naming the renamed heading), `internal/repoguard/budgets_test.go` (comment near "Change 0375 re-baselined…" and the two rows)
- Modify: `docs/reference/glossary.md`, `docs/concepts/run-tracker.md` (section `## The worktree slot`)
- Modify: `tests/test_go_integration_app_buildstart.sh` header (if Task 4 did not finish it)
- Modify comments in kept code: `internal/cli/gate.go` (`--run-id` help: "workflow run `id` the start is gated on (a locator, not a credential)"), `internal/cli/run.go` (`run.cancel` local-write justification), `internal/process/observe.go` (`Observation.Cwd` comment — it no longer serves `GateStop`), and every comment the Step 1 grep finds in `drive.go`, `reconcile.go`, `store.go`, `ownership.go`, `gate_drive.go`, `gate.go`, and the run-tracker cancel/complete/fence/start files

- [ ] **Step 1: Grep.** `git grep -n -i -E 'execution slot|worktree slot|launch-unconfirmed|gate history cleanup|gate\.history\.cleanup|gate-admission|history cleanup' -- ':!docs/superpowers/plans' ':!docs/results' ':!docs/changes' ':!docs/adrs' ':!docs/superpowers/specs'`. Every hit must be rewritten, except: `launch-unconfirmed` as a drive HALT cause (`driver.go`) and its glossary mention as such; `internal/repoguard/retired_vocabulary_test.go` row 69 and its synthetic negative-control strings (they guard spellings, not the slot).
- [ ] **Step 2: Rewrite `gate-caller-loop.md`'s section** (keep the heading text so links survive):

```markdown
## Worktree admission — one live gate per worktree, and what `worktree-busy` means

A canonical feature worktree carries **at most one** running gate at a time. Before
`gate.drive.start` launches anything, the driver takes that worktree's lock; the gate's
supervisor holds it for the gate's whole life, and the kernel releases it when the supervisor
exits — so a finished or killed gate frees the worktree on its own, with no recovery step. A start
against a worktree whose lock is held is **refused** `worktree-busy`: never queued, never silently
joined to the running gate, and it never stops that gate. The refusal names the holder (its drive
and change, or a raw run dir) only after confirming that gate is still running; otherwise it says
the holder is unknown.

`worktree-busy` is a **command failure**, distinct from the four dispositions above: the response
carries the bounded reason token and a next-action message, and exposes no drive document to
advance. A caller treats it as a **blocking diagnostic — not a retry trigger and not a `FAILED`
result** — that reserves no suite attempt and feeds no repair loop. Map it to the caller's own halt
posture (the build controller halts per its *Halting conditions*). Freeing the worktree is the
operator's act — wait for the holding gate to finish, or stop it (`run.cancel` for a tracked run,
`gate.stop <run-dir>` for a raw launch) — never a poll loop on `start`.
```

- [ ] **Step 3: Rewrite `gate-failure.md`'s section** with the new heading `## The finalize gate shares the worktree's one lock`:

```markdown
## The finalize gate shares the worktree's one lock

Finalize runs its post-rebase suite as a **scopeless** gate in the feature worktree, and that gate
takes the same worktree lock every other gate takes: one canonical worktree carries at most one
running gate at a time. So finalize's own gate can be **refused** before it launches when another
gate's supervisor holds the lock — reason `worktree-busy` — and its single automatic relaunch
halts `worktree-busy` instead of relaunching when another gate took the lock first. This is a
**blocking diagnostic, not a rebase conflict and not a red suite**: it is in neither the
abort-and-report set above nor a `contended`/`waiting` continuation. Do not race a second gate.
The remedy is operator-side — let the holding gate finish, or stop it (the `run.cancel` operation
with `--key <key> --run-id <id> --reason <why>` for a tracked run, `gate.stop <run-dir>` for a raw
launch) — then re-run finalize. The worktree frees itself when the holder ends.
```

Keep the following **Where the reason surfaces** paragraph unchanged. Update `prose_contracts_test.go`'s `terminator:` for `change_0411_recovery_exception_reference` to `## The finalize gate shares the worktree's one lock`.
- [ ] **Step 4: Glossary** (`docs/reference/glossary.md`):
  - Replace `### Worktree slot` with `### Worktree lock`: "The per-worktree lock that lets only one gate run at a time. Every gate start takes it without waiting; the gate's supervisor holds it for the gate's life, and the kernel releases it when the supervisor exits, so a finished or killed gate frees the worktree with no recovery step. A refusal names the holder only while that gate is running." Drop the `gate history cleanup` example.
  - `### \`worktree-busy\` / \`launch-unconfirmed\`` → `### \`worktree-busy\``: the reason a gate start is refused because another gate's supervisor holds the [worktree lock](#worktree-lock); a blocking diagnostic, neither a red suite nor a retry trigger, charging no suite attempt; the fix is to let the holder finish or stop it (`run.cancel` for a tracked run, `gate stop` for a raw launch). Add one sentence: `launch-unconfirmed` survives only as a gate-drive HALT cause (a relaunch whose launch could not be established), never as an admission refusal.
  - `### Process recovery / gate history cleanup` → `### Process recovery`: drop the history-cleanup sentences and commands; keep `gate recover`; state that recovery is not needed to free a worktree (the lock frees itself).
  - Delete `### \`run-link-lost\`` (its only emitter was deleted in Task 4).
  - Update the Gate entry bullet, the Finalize gate cross-reference ("it shares the worktree's single [worktree lock](#worktree-lock), so it can be refused with `worktree-busy`"), and the index entries (`worktree-lock`, `worktree-busy`, `process-recovery`; remove `run-link-lost`).
- [ ] **Step 5: `docs/concepts/run-tracker.md`** — replace `## The worktree slot` with `## The worktree lock`, describing: one gate per canonical worktree; the supervisor holds the lock; busy means a live supervisor; a dead run frees the worktree automatically; `run.cancel` finds a run's drives by the run context the drives record and counts a drive torn down once its supervisor is gone; the known process-tree gaps (a supervisor that dies alone, a KILL escalation, TERM ending `go run` while the runner stops targets) are tracked by change 0492. Remove `reserveWorktreeExecution`, `reconcileFinishedIncumbent`, `releaseRawSlotForStop`, and the history-cleanup/recovery bullets. Keep file-level links from `docs/README.md`, `docs/concepts/README.md`, `docs/guide/building-without-supervision.md` valid (they link the file, not the anchor).
- [ ] **Step 6: Regenerate and re-measure.** `go generate ./internal/assets/`; then update `budgets_test.go`: the comment naming "the one-live-gate-per-worktree admission section … the shared-slot note" → the lock wording, and the two rows (`gate-caller-loop.md`, `gate-failure.md`) to the new measured line/word counts with a `0490:` note.
- [ ] **Step 7: Run** `go test ./internal/assets ./internal/repoguard -count=1` and `go test ./... -count=1`. Expected: PASS (`TestEmbeddedMatchesAuthored`, `TestRebaseRecoveryDocContracts`, budgets, anchors). Re-run Step 1's grep: only the allowed residuals remain.
- [ ] **Step 8: Commit**

```bash
git add skills/docket-build/references/gate-caller-loop.md skills/docket-finalize-change/references/gate-failure.md internal/assets/embedded docs/reference/glossary.md docs/concepts/run-tracker.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go <each comment-only file>
git commit -m "docs: describe the worktree lock; retire slot and history-cleanup prose (0490)"
```

---

### Task 9: Re-measure runtime budgets after the deletions

**Tier: economy.** Risk: low; follow `tests/README.md` exactly — never raise a row to absorb growth.

**Files:**
- Modify: `tests/runtime-budgets.tsv`

- [ ] **Step 1: Run the whole suite** `go run ./cmd/docket development test` and read every `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, and `SERIAL CONFIRMED OVER BUDGET:` line, green run or not.
- [ ] **Step 2: Measure serially** the rows whose corpus shrank: `test_go_integration_gatedrive_process.sh`, `test_go_integration_gatedrive_race.sh`, `test_go_integration_app_buildstart.sh`, `test_go_integration_app_runcancel.sh`, `test_go_integration_app_runcompletion.sh`, `test_go_integration_app_runstart.sh`, `test_go_integration_app_gatelifecycle.sh`, `test_go_toolchain.sh`, `test_go_race.sh` (procedure in `tests/README.md`). Apply the table's seeding rule ("rounded up to the next multiple of 5 plus a 5s margin (min 10s)") to the **worst** standalone serial reading; lower a row only when that rule gives a smaller number. Any `SERIAL CONFIRMED OVER BUDGET:` is a finding to fix (split or speed up), not a number to raise.
- [ ] **Step 3: Run** `go test ./internal/repoguard -run TestRuntimeBudgets -count=1` (correspondence). Expected: PASS.
- [ ] **Step 4: Commit** (skip with a note in the hand-off if no row changed)

```bash
git add tests/runtime-budgets.tsv
git commit -m "test(budgets): re-measure rows after the slot deletions (0490)"
```

The full-suite build gate (`go run ./cmd/docket development test`) runs once after this task.

---

## Spec coverage map

| Spec item | Task |
|---|---|
| Decision 1 location/key/never-delete, acquisition at all three sites, busy/I-O semantics, handoff/release, lock order, process stdlib-only | 1, 2, 4, 5 |
| Decision 2 holder note + validated diagnosis + remedy by holder kind | 2, 4, 5 |
| Decision 3 delete slot store, `launch-unconfirmed` refusal, slot transitions, finished-incumbent reconciliation + advisory precheck, legacy inventory, `gate.history.cleanup`, run stamp readers | 4, 5, 6, 7 |
| Decision 3 launch token minted by the driver, JSON key kept | 4 |
| Decision 4 cancel/guardian without slot, census by context hash, first-launch resolution, teardown proof, pending causes, repeat cancel, resume, closeout, mutation fence | 3, 6 |
| Decision 5 no migration, upgrade contract | Global Constraints; hand-off (results file) |
| Prose and generated sites table + grep | 8 |
| Tests: re-target the three invariant tests | 4 (two gatedrive), 5 (cli) |
| Tests L1–L10 | L1: 1, 4 · L2: 1 · L3: 2 · L4: 4 · L5: 4 · L6: 1 · L7: 1 · L8: 2 · L9, L10: 3 (+6) |
| Delete / re-point / rewrite fixtures / pins (`TestGateLaunchAdmissionCoverage`, schema/asset correspondence, kind tests, launch-sites guard) | 4, 5, 6, 7 |
| Budgets; whole-suite build gate | 9 |
| New ADR + Update notes; results-file notes | Hand-off below (implement-next Step 6 / results) |

## Hand-off to implement-next Step 6 (not build tasks)

- **ADR** (dispatch `docket-adr`, next free number at the time — 0132 when this plan was written): *Worktree admission is a supervisor-held kernel lock*, `supersedes: [118]`, `relates_to: [95, 120, 124, 125]`. Context = spec Problem facts 1–8; Decision = spec Decisions 1–5, restating what ADR-0118 established and this keeps (one live gate per canonical worktree; every launch site acquires before launch; a busy start charges no suite attempt, is never queued, never stops the holder; distinct worktrees are independent; ADR-0118's cancellation and resume rules stand until 0491); Consequences = the spec's Accepted losses including the four process-tree gaps tracked by change 0492; Alternatives and "Answers to the earlier rejections" verbatim from the spec's Decision record. Also record this plan's decisions (HALTED drives in the census; `ErrUnresolvedLaunchTransition` kept; relaunch lock-guarded rather than run-gated).
- **Update notes** (dated) on ADR-0120 (first admission and history cleanup uses replaced; the historical reader stays for the census), ADR-0125 (finished-incumbent settlement, removed-path slot addressing, slot legs of relevance/owner rules replaced; "a probe error is never absence" stays), ADR-0124 (closeout slot retirement removed). Deliver each by listing the ADR id in the change's `adrs:` (learning: adr-update-delivery).
- **Results file**: *Important* — reinstall after the merge only while no gate is running, on every machine (a supervisor started by the old binary holds no worktree lock). *Optional* — `.git/docket/gate-admission/` may be deleted by hand once the new binary is installed. *Optional check* — re-run a `run.cancel` that was stuck `cancellation-pending` on a slot finding; it should reach `cancelled`. Record each budget row's remaining margin as a number (learning: budget-headroom-is-spent-before-it-is-breached).
