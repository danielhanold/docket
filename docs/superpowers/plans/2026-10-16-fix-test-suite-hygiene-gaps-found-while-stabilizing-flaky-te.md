<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0527 — Fix test-suite hygiene gaps found while stabilizing flaky tests](../../changes/archive/2026-10-16-0527-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te.md)**
<!-- docket:backlink:end -->

# Fix test-suite hygiene gaps found while stabilizing flaky tests — Implementation Plan

> **For agentic workers:** executed by `docket-build`, task by task, under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the eight test-suite hygiene gaps in the spec. Add a repo-wide guard for unrunnable test functions, a clean no-git guard path, a codex client that keeps a crashing server's stderr, a shared-walk exit-sites guard, go-signal children for the timer-racing gate-driver tests, and a two-step kill in the abandoned-run recover test. Split the private reposetup tests into their own shard and re-seed the budget rows. Confirm item 7 is on `main`.

**Architecture:** Each item is independent and lands as its own task and commit. The only production code that changes is `internal/codexentry/process.go` (item 3). Every other edit is test source, a test-support helper, a shard wrapper, or a budget row. No existing test changes what it asserts.

**Tech Stack:** Go 1.27 (`go/parser`, `go/ast`, `os/exec`), Go integration tests (`//go:build integration`), Bash shard wrappers (`tests/lib/go-integration-shard.sh`), `tests/runtime-budgets.tsv`.

**Spec:** `docs/superpowers/specs/2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-design.md`

## Global Constraints

- Change no existing test's assertions. Every item changes how a test runs, is named, or is counted, never what it proves.
- Do not edit point-in-time records that name renamed tests or removed modes: archived changes, results files, `docs/release/**`, older plans and specs.
- Budget seeding rule (header of `tests/runtime-budgets.tsv`): round the measured serial wall time up to the next multiple of 5, add a 5 s margin, minimum 10 s. Apply it to the **worst** standalone serial reading of each shard.
- One literal prefix per shard. `SHARD_PREFIX` is a plain string, matched as a literal glob prefix within its `SHARD_PKG`.
- New prefix: `TestIntegrationPrivateRepoSetup`. New shard file: `tests/test_go_integration_app_reposetup_private.sh`.
- Any run that checks for a change in outcome uses `-count=1` (learning `cached-runner-serves-a-mutated-tree`).
- For a mutation probe, back up the file with `cp` and restore it with `mv`, never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`). Before reading any probe result, confirm with `git diff` that the mutation actually landed.
- Out of scope: concurrent-suite detection in the solo re-check (#528), host-relative budgets and re-seeding the whole table (#273), and the workflowlifecycle split (#536, already done).

## Review Focus

1. **A lower-case `func(*testing.T)` helper that is used by value but not as a `t.Run` argument.** Examples are an entry in a `[]func(*testing.T)` table or a map value. The guard must not report it. Task 1's fixture includes a table entry.
2. **Test files that import `testing` under an alias or as a dot import.** A lower-case `func x(t *tt.T)` or `func x(t *T)` is still unrunnable and must be reported. Task 1's fixture covers both.
3. **`TMPDIR` with exactly one trailing slash.** This is the macOS default, and it must keep working after the clean. Task 2's test runs both the `/` and `//` suffixes.
4. **`Close` after a `Recv` whose bounded wait timed out, and a second `Close`.** Neither may hang or panic, and the process is waited only once. Task 3's linger test calls `Close` twice.
5. **A test that fails before it creates the go-signal file.** The child must not live forever. Task 5 gives the poll loop a hard cap, and the existing `stopAllRuns` cleanup still ends the run.

---

### Task 1: Repo-wide guard for unrunnable test functions (item 1)

**Files:**
- Create: `internal/repoguard/unrunnable_tests_test.go` (package `repoguard`)

**Interfaces:**
- Consumes: `guardRoot(t)`, `maintainedPop(t, root)`, `readMaintained(t, root, rel)` from `internal/repoguard/guards_test.go`
- Produces:
  - `type unrunnableTestFunc struct{ File, Name string }`, where `File` is the slash-relative path
  - `func findUnrunnableTestFuncs(srcs map[string][]byte) ([]unrunnableTestFunc, error)`. The keys of `srcs` are slash-relative `.go` paths. The result is sorted by `File`, then `Name`. A parse error returns an error that names the file.
  - `func goTestRuns(name, prefix string) bool`

Detector rules (from spec §1):
- **Candidate.** A top-level `*ast.FuncDecl` qualifies when all of these hold:
  - It is in a `_test.go` file.
  - It has no receiver and no type parameters.
  - It has exactly one parameter, of type `*X.T`, `*X.B`, or `*X.F`. `X` is the file's local name for the `"testing"` import (its alias if one is set, otherwise `testing`). In a file that dot-imports `"testing"`, the parameter types are the bare `*T`, `*B`, or `*F`.
  - It has no results.
  - `go test` does not run its name. `goTestRuns` requires the name to start with the prefix for its parameter type (`T`→`Test`, `B`→`Benchmark`, `F`→`Fuzz`). The rest of the name must be empty or start with a rune that is not `unicode.IsLower`.
- **Reported.** A candidate is reported when no `*ast.Ident` with its name appears in any `.go` file in the same directory (`path.Dir` of the key), across all packages and all files there. The declaration's own `Name` ident does not count.
- Parse each file with `parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)`. Parsing ignores build constraints, so tagged files are covered.

- [ ] **Step 1: Write the failing detector test `TestUnrunnableTestFuncsDetector`**

The test builds an in-memory `srcs` map and asserts that `findUnrunnableTestFuncs` returns exactly the expected list, compared with `reflect.DeepEqual`:

```go
srcs := map[string][]byte{
	"p/a_test.go": []byte(`package p
import "testing"
func TestRuns(t *testing.T) { t.Run("sub", referencedSub) }
func referencedSub(t *testing.T) {}
var table = []func(*testing.T){tableSub}
func tableSub(t *testing.T) {}
func testOrphanExample(t *testing.T) {}
func Testlower(t *testing.T) {}
func Test(t *testing.T) {}
func Test_under(t *testing.T) {}
func Test2(t *testing.T) {}
func benchmarkOrphan(b *testing.B) {}
func BenchmarkFine(b *testing.B) {}
func fuzzOrphan(f *testing.F) {}
func BenchmarkWrongType(t *testing.T) {}
func helperTwoParams(t *testing.T, n int) {}
func helperResult(t *testing.T) error { return nil }
type s struct{}
func (s) methodOrphan(t *testing.T) {}
func usedFromNonTest(t *testing.T) {}
func usedFromExternal(t *testing.T) {}
`),
	"p/b.go":      []byte("package p\nvar _ = usedFromNonTest\n"),
	"p/c_test.go": []byte("package p_test\nvar _ = usedFromExternal\n"),
	"q/alias_test.go": []byte("package q\nimport tt \"testing\"\nfunc aliasOrphan(t *tt.T) {}\n"),
	"q/dot_test.go":   []byte("package q\nimport . \"testing\"\nfunc dotOrphan(t *T) {}\n"),
	"q/x_test.go":     []byte("package q\nimport \"testing\"\nfunc referencedSub(t *testing.T) {}\n"),
}
want := []unrunnableTestFunc{
	{"p/a_test.go", "BenchmarkWrongType"},
	{"p/a_test.go", "Testlower"},
	{"p/a_test.go", "benchmarkOrphan"},
	{"p/a_test.go", "fuzzOrphan"},
	{"p/a_test.go", "testOrphanExample"},
	{"q/alias_test.go", "aliasOrphan"},
	{"q/dot_test.go", "dotOrphan"},
	{"q/x_test.go", "referencedSub"}, // referenced only in p/: scoping is per directory
}
```

In the same test, also assert that `findUnrunnableTestFuncs(map[string][]byte{"r/bad_test.go": []byte("package r\nfunc (")})` returns a non-nil error that contains `r/bad_test.go`.

- [ ] **Step 2: Run it and confirm it fails to compile**

Run: `go test -count=1 ./internal/repoguard -run TestUnrunnableTestFuncsDetector`
Expected: build failure, `undefined: findUnrunnableTestFuncs`.

- [ ] **Step 3: Implement `goTestRuns`, `findUnrunnableTestFuncs`, and the repo guard `TestEveryTestShapedFunctionRuns`**

The guard works like this:
1. Get `root := guardRoot(t)`.
2. Build `srcs` from every `.go` file in `maintainedPop(t, root)`, reading each one with `readMaintained`.
3. Call `findUnrunnableTestFuncs`. On error, call `t.Fatalf`.
4. Call `t.Errorf` once per finding, with this message:
   `"%s: %s has a test signature, but go test never runs it and nothing in its directory references it — rename it to a name go test runs (Test/Benchmark/Fuzz followed by a non-lower-case rune or nothing), or call it from one"`.

Add a population floor (learning `marker-scoped-guard-needs-a-population-floor`). Fail if `srcs` has no key `internal/app/finalize_e2e_test.go`. That file is `e2e`-tagged, so its presence proves tagged files are in the population. Add a file comment that states what the guard covers and its limitation: a function used only from another directory, or only by reflection, is invisible to it, and the trace found none.

- [ ] **Step 4: Run both tests and confirm they pass**

Run: `go test -count=1 ./internal/repoguard -run 'TestUnrunnableTestFuncsDetector|TestEveryTestShapedFunctionRuns' -v`
Expected: PASS. The tree has zero findings today. The 11 lower-case subtests in `absence_test.go` and `retired_vocabulary_test.go` are referenced, so they are not reported.

- [ ] **Step 5: Mutation-test the guard against the real e2e case**

```bash
f=internal/app/finalize_e2e_test.go; cp "$f" "$f.bak"
sed -i '' 's/^func TestE2EOrdinaryFinalize(/func testE2EOrdinaryFinalize(/' "$f"
git diff --stat -- "$f"    # must show 1 file changed; otherwise the probe did not land
go test -count=1 ./internal/repoguard -run TestEveryTestShapedFunctionRuns
mv "$f.bak" "$f"; git diff --quiet -- "$f" && echo restored
```
Expected: FAIL, and the message names `internal/app/finalize_e2e_test.go: testE2EOrdinaryFinalize`. Then `restored` prints. `tests/test_go_finalize_e2e.sh` is not edited.

- [ ] **Step 6: Commit**

```bash
git add internal/repoguard/unrunnable_tests_test.go
git commit -m "test(repoguard): flag test-shaped functions go test never runs"
```

### Task 2: No-git guard stores a cleaned path (item 2)

**Files:**
- Modify: `internal/testsupport/nogit_install.go` (the `noGitGuardDir = dir` assignment and the uses of `dir` before it)
- Test: `internal/testsupport/nogit_install_test.go`

**Interfaces:**
- Consumes: `InstallNoGitGuard(pkg, shardGlob string) (func(code int) int, error)`, `NoGitGuardDir() string`, `AssertNoGitGuardShadowsGit(t testing.TB)`, `TempDir(t testing.TB) string`
- Produces: nothing new.

- [ ] **Step 1: Write the failing test `TestInstallNoGitGuardRecordsCleanDir`**

The test is table-driven over the suffixes `"/"` and `"//"`. Each subtest does the following:
1. Calls `t.Setenv("TMPDIR", TempDir(t)+suffix)` and `t.Setenv("PATH", os.Getenv("PATH"))`. The second call restores PATH at cleanup.
2. Saves `noGitGuardDir` and restores it in `t.Cleanup`.
3. Calls `finish, err := InstallNoGitGuard("internal/app", "tests/test_go_integration_app_*.sh")`, requires `err == nil`, and registers `t.Cleanup(func() { finish(0) })`.
4. Asserts `NoGitGuardDir() == filepath.Clean(NoGitGuardDir())`.
5. Calls `AssertNoGitGuardShadowsGit(t)`.

- [ ] **Step 2: Run it and confirm the `//` case fails**

Run: `go test -count=1 ./internal/testsupport -run TestInstallNoGitGuardRecordsCleanDir -v`
Expected: the `/` subtest passes. The `//` subtest fails, either on the clean-form assert or with `git resolves to … want the guard shim in …//…`.

- [ ] **Step 3: Clean the path once, right after `os.MkdirTemp` succeeds (`dir = filepath.Clean(dir)`)**

All later uses then see one spelling: the log path, the shim, the PATH entry, `noGitGuardDir`, and the finisher's `RemoveAll`.

- [ ] **Step 4: Run the unit test and the four package wrappers under a doubled slash**

Run: `go test -count=1 ./internal/testsupport -run TestInstallNoGitGuard -v` → PASS
Run: `TMPDIR=/tmp// go test -count=1 ./internal/workspace ./internal/gatedrive ./internal/repository/transaction ./internal/app -run TestNoGitGuardShadowsGitOnPath` → `ok` for all four packages.

- [ ] **Step 5: Commit**

```bash
git add internal/testsupport/nogit_install.go internal/testsupport/nogit_install_test.go
git commit -m "fix(testsupport): store the no-git guard dir in clean form"
```

### Task 3: Codex client keeps a crashing app-server's stderr; the fake server explains its exits (item 3)

**Files:**
- Modify: `internal/codexentry/process.go`
- Create: `internal/codexentry/process_test.go`
- Modify: `internal/cli/agent_test.go` (`TestAgentEnterServerProcess` only)

**Interfaces:**
- Consumes: `Transport` (from `client.go`; `Send`, `Recv`, `Close`)
- Produces (package-internal):
  - `func startAppServerCommand(cmd *exec.Cmd, cwd string) (Transport, error)`. `StartAppServer` becomes `startAppServerCommand(exec.CommandContext(ctx, "codex", "app-server", "--stdio"), cwd)`. The argv stays closed and unchanged.
  - `var recvStderrGrace = 2 * time.Second`. This is the longest `Recv` waits for the process after a read or decode error.
  - On `processTransport`: `waitOnce sync.Once`, `waitDone chan struct{}`, `waitErr error`, `closeOnce sync.Once` (replacing `once`), and `func (p *processTransport) wait() <-chan struct{}`. `wait` starts `p.cmd.Wait()` exactly once in a goroutine, which stores `waitErr` and then closes `waitDone`.

Behavior:
- **`Recv`, on a decode error:** `select` on `p.wait()` or `time.After(recvStderrGrace)`, then read `p.stderr.String()`. Build the error so it wraps the decode error with `%w`:

  | Process exited? | `waitErr` | stderr | Error text |
  |---|---|---|---|
  | yes | non-nil | non-empty | `"%w (app-server exited: %v; stderr: %s)"` |
  | yes | non-nil | empty | `"%w (app-server exited: %v)"` |
  | any other case | — | non-empty | `"%w (app-server stderr: %s)"` |
  | any other case | — | empty | the bare error |

  Read `waitErr` only after a receive from `waitDone`.
- **`Close`:** inside `closeOnce`, close stdin, kill the process if it is still running, then `<-p.wait()` and return `waitErr`. A second `Close` returns the same value, and the process is waited only once.

- [ ] **Step 1: Write the failing tests in `internal/codexentry/process_test.go`**

The helper process is `TestHelperAppServerProcess(t *testing.T)`. It returns immediately unless `DOCKET_CODEXENTRY_TEST_SERVER` is set. Its modes are:
- `stdout-closes-first`: run `os.Stdout.Close()`, sleep 100 ms, write `app-server boom: config unreadable\n` to stderr, `os.Exit(7)`.
- `linger`: run `os.Stdout.Close()`, write `still running\n` to stderr, sleep 30 s.

A shared helper starts the helper process. It builds `cmd := exec.Command(os.Args[0], "-test.run=^TestHelperAppServerProcess$")`, sets `cmd.Env = append(os.Environ(), "DOCKET_CODEXENTRY_TEST_SERVER="+mode)`, and calls `startAppServerCommand(cmd, testsupport.TempDir(t))`. Use `testsupport.TempDir`, not `t.TempDir`, to match the repo's temp-dir fixture guard.

- `TestRecvReportsStderrOfServerThatClosedStdoutFirst`: `Recv` returns a non-nil error that contains both `app-server boom: config unreadable` and `exit status 7`. `Close` returns.
- `TestRecvAndCloseDoNotHangOnLiveServer`: set `recvStderrGrace = 200 * time.Millisecond` and restore it in `t.Cleanup`. `Recv` returns a non-nil error in under 5 s. `Close` then returns in under 5 s. A second `Close` returns the same error value (compare with `errors.Is` or `==`).

- [ ] **Step 2: Run them against the old code and confirm the first test fails**

First add only `startAppServerCommand` as a pure extraction, so the tests compile.
Run: `go test -count=5 ./internal/codexentry -run 'TestRecv' -v`
Expected: `TestRecvReportsStderrOfServerThatClosedStdoutFirst` FAILS on every run, because the error is a bare `EOF`. Stdout closes 100 ms before the stderr write, so the old code always reads an empty buffer.

- [ ] **Step 3: Implement the bounded wait in `Recv` and the once-only wait in `Close`, as specified above**

- [ ] **Step 4: Run the new tests repeatedly under the race detector, then the package**

Run: `go test -race -count=20 ./internal/codexentry -run 'TestRecv'` → `ok`
Run: `go test -race -count=1 ./internal/codexentry` → `ok`

- [ ] **Step 5: Make the fake server in `internal/cli/agent_test.go` explain each exit**

Before each `os.Exit` in `TestAgentEnterServerProcess`, write one line to stderr with this prefix: `fake app-server: `. The line names the failed check and the values involved:
- argv tail
- a request line that is not JSON (include the line)
- the cwd mismatch (both paths)
- the developer-instructions, model, and effort mismatches (got and want)
- the turn text or skill mismatch (got text, want text, and whether the skill was seen)

The per-key branch already prints its line; add the prefix there too. After the read loop, if `scanner.Err() != nil`, print `fake app-server: reading requests: <err>` and `os.Exit(7)`. Exit codes 2–6 stay as they are.

Run: `go test -count=3 ./internal/cli -run TestAgentEnter` → `ok`

- [ ] **Step 6: Commit**

```bash
git add internal/codexentry/process.go internal/codexentry/process_test.go internal/cli/agent_test.go
git commit -m "fix(codexentry): keep a crashing app-server's stderr in read errors"
```

### Task 4: Exit-sites guard uses the shared maintained-file walk (item 4)

**Files:**
- Modify: `cmd/docket/exit_sites_test.go` (`TestProcessExitSitesAreAllowlisted`)

**Interfaces:**
- Consumes: `repoguard.MaintainedFiles(root string) ([]string, error)` (import `github.com/danielhanold/docket/internal/repoguard`; `internal/config` already imports it from a test)
- Produces: nothing new.

- [ ] **Step 1: Show the defect with a planted sibling checkout**

```bash
mkdir -p .worktrees/exit-site-probe/cmd/probe
printf 'package main\n\nimport "os"\n\nfunc main() { os.Exit(1) }\n' > .worktrees/exit-site-probe/cmd/probe/main.go
go test -count=1 ./cmd/docket -run TestProcessExitSitesAreAllowlisted
```
Expected: FAIL, naming `.worktrees/exit-site-probe/cmd/probe/main.go`. Leave the probe in place for Step 3.

- [ ] **Step 2: Replace the `filepath.WalkDir` block with a loop over `repoguard.MaintainedFiles(root)`**

Keep the `.go` files that do not end in `_test.go`, and keep `root` with its `go.mod` sanity check. Keep the `scanned == 0` floor. Strengthen the allowlist check so each allowlisted path must appear in the scanned population; a `Stat` alone is not enough. Update the comment to say the walk is the shared repo-guard population, with its hidden-directory, `testdata`, `docs`, and fixture exclusions.

- [ ] **Step 3: Rerun with the probe present, then remove the probe**

Run: `go test -count=1 ./cmd/docket -run TestProcessExitSitesAreAllowlisted` → PASS
Run: `rm -rf .worktrees/exit-site-probe && git status --porcelain` → no probe paths listed.
The allowlist is unchanged: `cmd/docket/main.go`, `cmd/genassets/main.go`, `cmd/releasepkg/main.go`.

- [ ] **Step 4: Commit**

```bash
git add cmd/docket/exit_sites_test.go
git commit -m "test(cmd/docket): walk only maintained files in the exit-sites guard"
```

### Task 5: Gate-driver children wait for a go-signal instead of a timer (item 5)

**Files:**
- Modify: `internal/gatedrive/supervisor_integration_test.go` (`runIntChild` and its doc comment, `runDriveStartChild`, `TestIntegrationGatedriveDriverSlicesAcrossLiveChildThenPasses`, `TestIntegrationGatedriveFreshProcessResumesAndChildSurvives`, and the `child-signal-death` subtest of `TestIntegrationGatedriveProcessDeathHaltsSupervisorDied`)
- Modify: `internal/gatedrive/takeover_race_integration_test.go` (`TestRaceIntegrationGatedriveTakeoverKeepsRunIdentity`)

**Interfaces:**
- Consumes: `intStartRequest(exe, runRoot, cwd, childMode, childArg string) StartRequest`, `advanceUntilTerminal`, `d.Advance(id, gen)`, `testsupport.TempDir`
- Produces (in `supervisor_integration_test.go`):
  - Child modes `pass-on <path>` and `selfkill-on <path>`. `selfkill-after` is removed; `pass-after` stays.
  - `const intChildSignalCap = 5 * time.Minute`
  - `func waitForSignalFile(path string) bool`. It polls `os.Stat(path)` every 5 ms and returns `true` once the file exists, or `false` after `intChildSignalCap`.
  - `func newGoSignal(t *testing.T) string`. It returns `filepath.Join(testsupport.TempDir(t), "go")`, a path that does not exist yet.
  - `func releaseChild(t *testing.T, path string)`. It runs `os.WriteFile(path, nil, 0o600)` and calls `t.Fatalf` on error.

Child behavior:
- `pass-on`: once the signal arrives, write `intChildStdout` and return 0. If the cap expires first, return 93.
- `selfkill-on`: once the signal arrives, run `syscall.Kill(syscall.Getpid(), syscall.SIGKILL)`, the same body `selfkill-after` has today. If the cap expires first, return 94.
- `drive-start`: its third argument is now the signal path, and it starts a `pass-on` child with it. Update the usage line in the doc comment to `drive-start <gitCommonDir> <runRoot> <signalPath>`.

- [ ] **Step 1: Add the two modes, the helpers, and the doc-comment entries, and point `drive-start` at `pass-on`**

- [ ] **Step 2: Convert the four tests (spec §5 placement)**

- **`child-signal-death`.** Start with `"selfkill-on", sig`. After `Start` returns `WAITING`, call `releaseChild`, then run `advanceUntilTerminal` as before. The `StateSignaled` and `supervisor-died` asserts are unchanged.
- **`…DriverSlicesAcrossLiveChildThenPasses`.** Start with `"pass-on", sig`. After the first `WAITING`, call `d.Advance` twice and require `WAITING` both times, then call `releaseChild`. Call `advanceUntilTerminal` and assert on `2+waiting >= 2`, keeping the existing message.
- **`…TakeoverKeepsRunIdentity`.** Start with `"pass-on", sig`. Call `releaseChild` after the takeover's checks and before `advanceUntilTerminal`.
- **`…FreshProcessResumesAndChildSurvives`.** Pass `sig` in place of `"1500"`. Call `releaseChild` after the `pidAlive` assert and before `advanceUntilTerminal`.

Then confirm the old mode is gone:
Run: `grep -rn 'selfkill-after' internal/` → no output.

- [ ] **Step 3: Run the converted tests repeatedly**

Run: `go test -tags integration -count=20 ./internal/gatedrive -run 'TestIntegrationGatedriveProcessDeathHaltsSupervisorDied|TestIntegrationGatedriveDriverSlicesAcrossLiveChildThenPasses|TestIntegrationGatedriveFreshProcessResumesAndChildSurvives'` → `ok`
Run: `go test -race -tags integration -count=20 ./internal/gatedrive -run TestRaceIntegrationGatedriveTakeoverKeepsRunIdentity` → `ok`

- [ ] **Step 4: Optional load run**

Do this only if you can produce sustained fsync load on the machine. For example, run two background loops of `dd if=/dev/zero of=<tmp>/f bs=1m count=1000 && sync`, or several concurrent `go test -tags integration ./internal/process ./internal/gatedrive -count=1` runs. Then run `go test -tags integration -count=40 ./internal/gatedrive -run 'TestIntegrationGatedriveProcessDeathHaltsSupervisorDied/child-signal-death'`.

Record the pass count and the load you used in the task's completion note. If you could not produce load, record that instead, and say that the fix rests on the construction argument plus the unloaded `-count=20` runs.

- [ ] **Step 5: Commit**

```bash
git add internal/gatedrive/supervisor_integration_test.go internal/gatedrive/takeover_race_integration_test.go
git commit -m "test(gatedrive): release live children by go-signal, not a timer"
```

### Task 6: Two-step kill in `TestRecoverMarksCleanlyAbandonedOwnedRun`; confirm item 7 (items 7 and 8)

**Files:**
- Modify: `internal/process/recover_test.go` (`TestRecoverMarksCleanlyAbandonedOwnedRun`)

**Interfaces:**
- Consumes: `launchHelper`, `readManifest`, `syscall.Kill`, `processAlive`, `signalGroup`, `groupAlive`, `probeFlock`, `waitFor`, `abandonedPreconditionUnmet` (all in `internal/process`)
- Produces: nothing new.

- [ ] **Step 1: Confirm item 7 by content**

Run: `grep -n 'm.SupervisorPID\|signalGroup' internal/process/observe_test.go`
Expected: inside `TestObserveRunningThenTerminal`, a `syscall.Kill(m.SupervisorPID, …)` and a `waitFor(… processAlive(m.SupervisorPID) == probeAbsent)` appear before `signalGroup(m.PGID, …)`. This was observed at commit 95d6b56c0. Nothing to change. Note the confirmation in the completion note.

- [ ] **Step 2: Sweep for other one-step abandoned-shape builders**

Run: `grep -n 'signalGroup(.*SIGKILL' internal/process/*_test.go`
Classify each site. At planning time, only `recover_test.go:73` builds an abandoned or vanished shape with a single group kill. The other sites are cleanups or teardowns, or already kill the supervisor first: `leftover_test.go`, `launch_test.go`, `observe_test.go`, `recover_test.go:283`, `worktree_lock_test.go`, `platform_test.go`. If a new builder site has appeared, apply Step 3 to it too. List the result in the completion note.

- [ ] **Step 3: Rewrite the setup**

1. Delete `setupAttempts`, the `for` loop, and the 0328 retry comment.
2. Set `root := testsupport.TempDir(t)` and `out := launchHelper(t, svc, root, "sleep")`, and read the manifest (fatal on error or nil).
3. Run `t.Cleanup(func() { _ = signalGroup(m.PGID, syscall.SIGKILL) })`.
4. Run `syscall.Kill(m.SupervisorPID, syscall.SIGKILL)`, fatal on error.
5. Run `waitFor(t, "the killed supervisor to be reaped", 60*time.Second, … processAlive(m.SupervisorPID) == probeAbsent)`.
6. Run `signalGroup(m.PGID, syscall.SIGKILL)`.
7. Keep the existing `lock release` and `group gone` waits (60 s), including their comment.
8. Add: `if why := abandonedPreconditionUnmet(t, out.RunDir, m.PGID); why != "" { t.Fatalf("setup did not reach the abandoned shape: %s", why) }`.

Replace the header comment with the two-step-kill explanation, worded as in `TestObserveRunningThenTerminal`. Everything from `svc.Recover(root)` onward stays byte-identical.

- [ ] **Step 4: Run it once, then under sustained CPU load**

Run: `go test -count=1 ./internal/process -run TestRecoverMarksCleanlyAbandonedOwnedRun -v` → PASS
Load run: start three background `go test -count=1 ./internal/...` loops, then run `go test -count=200 ./internal/process -run TestRecoverMarksCleanlyAbandonedOwnedRun`. Expect `ok`. Stop the background loops. Record the count and load in the completion note.

- [ ] **Step 5: Commit**

```bash
git add internal/process/recover_test.go
git commit -m "test(process): build the abandoned run by killing the supervisor first"
```

### Task 7: Split private reposetup tests into their own shard; re-seed three budget rows (item 6)

**Files:**
- Modify: `internal/app/repository_private_integration_test.go` (21 function renames, their doc comments, and the file-header paragraph)
- Modify: `internal/app/reposetup_harnesses_integration_test.go` (2 renames and the file-header paragraph)
- Create: `tests/test_go_integration_app_reposetup_private.sh`
- Modify: `tests/runtime-budgets.tsv` (the concurrency row, the reposetup row, and a new row)

**Interfaces:**
- Consumes: `tests/lib/go-integration-shard.sh` (`shard_inspect_maybe`, `run_integration_shard`)
- Produces: shard `SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationPrivateRepoSetup"`, `SHARD_MODE="normal"`.

- [ ] **Step 1: Re-check the new prefix against the live `./internal/app` prefix set**

Run: `grep -l 'SHARD_PKG="./internal/app"' tests/*.sh | xargs grep -h 'SHARD_PREFIX=' | sort`
Confirm three things:
- No listed prefix is a prefix of `TestIntegrationPrivateRepoSetup`.
- `TestIntegrationPrivateRepoSetup` is not a prefix of any listed prefix.
- No existing test name already starts with it.

At planning time the closest prefixes were `TestIntegrationPrivateInstructions`, `TestIntegrationPrivateMode`, `TestIntegrationRepoSetup`, and `TestRaceIntegrationRepoSetup`, and none overlap. If an overlap has appeared, stop and report BLOCKED. Do not pick another name.

- [ ] **Step 2: Rename the 23 tests**

Apply both substitutions to both files, including doc-comment mentions:
- `s/TestIntegrationRepoSetupGlobalPrivateDefault/TestIntegrationPrivateRepoSetupGlobalDefault/g`
- `s/TestIntegrationRepoSetupPrivate/TestIntegrationPrivateRepoSetup/g`

Then update the two file headers:
- `repository_private_integration_test.go`: the private-visibility slice now carries prefix `TestIntegrationPrivateRepoSetup`, matched by `tests/test_go_integration_app_reposetup_private.sh`.
- `reposetup_harnesses_integration_test.go`: the two private harness tests carry `TestIntegrationPrivateRepoSetup` and run in the private shard. The rest stay `TestIntegrationRepoSetup`.

Run: `grep -rn 'RepoSetupPrivate\|RepoSetupGlobalPrivate' internal/ tests/` → no output.

- [ ] **Step 3: Create the shard wrapper**

Copy `tests/test_go_integration_app_reposetup.sh` and change only the header and `SHARD_PREFIX`. The header should say: Go integration shard (change 0527); the private-visibility repository setup tests (private init layout, store adoption and refusals, metadata-remote flags, global private default, private check and migrate refusals, reclone and moved-clone recovery, private harness choice), behind the `integration` build tag, prefix `^TestIntegrationPrivateRepoSetup`; split out of `tests/test_go_integration_app_reposetup.sh`. End it with the same declarations-only and contract sentence as the original. Make the file executable (`chmod +x`), matching its sibling.

- [ ] **Step 4: Verify selection and the contract**

Run: `go test -tags integration -list '^TestIntegrationPrivateRepoSetup' ./internal/app | grep -c '^Test'` → `23`
Run: `go test -tags integration -list '^TestIntegrationRepoSetup' ./internal/app | grep -c '^Test'` → `33`
Run: `go test -tags integration -list '^TestIntegrationRepoSetup(Private|GlobalPrivate)' ./internal/app | grep -c '^Test'` → `0`
Run: `bash tests/test_go_integration_contract.sh` → every assert `ok`, exit 0.

- [ ] **Step 5: Measure the three shards serially and set their rows**

Make sure no other `go test` is running (`pgrep -fl 'go test|\.test '` prints nothing). Then run each shard standalone twice: `/usr/bin/time -p bash tests/test_go_integration_app_concurrency.sh`, then the same for `…_reposetup.sh` and `…_reposetup_private.sh`. Use the worst `real` reading of each and apply the seeding rule.

Expected readings, from the spec's 2026-10-16 measurement:

| Shard | Expected | Expected row |
|---|---|---|
| `tests/test_go_integration_app_concurrency.sh` | ~30 s | 35 |
| `tests/test_go_integration_app_reposetup.sh` | ~25 s | 30–35 |
| `tests/test_go_integration_app_reposetup_private.sh` | ~35 s | ~40–45 |

If a reading is far off, re-measure once; another worktree's suite inflates solo readings. Edit the concurrency and reposetup rows. Add `tests/test_go_integration_app_reposetup_private.sh	<n>	parallel` (tab-separated) directly after the reposetup row. Put the raw readings in the completion note.

Run: `go test -count=1 ./internal/repoguard -run TestRuntimeBudgetsCorrespondence` → PASS

- [ ] **Step 6: Commit**

```bash
git add internal/app/repository_private_integration_test.go internal/app/reposetup_harnesses_integration_test.go tests/test_go_integration_app_reposetup_private.sh tests/runtime-budgets.tsv
git commit -m "test: split private reposetup tests into their own shard and re-seed budgets"
```

## Final verification (docket-build's full-suite gate)

- `build.test_command` (`go run ./cmd/docket development test`) is green.
- The budget report has no `BUDGET WATCH:` or `SERIAL CONFIRMED OVER BUDGET:` line for `test_go_integration_app_concurrency.sh`, `test_go_integration_app_reposetup.sh`, or `test_go_integration_app_reposetup_private.sh`.
- The completion notes carry the results-file facts:
  - item 7 confirmed on `main`;
  - the item-8 sweep result;
  - the `-count=200` load run for the recover test;
  - the `child-signal-death` load run, or a statement that none was possible;
  - the three serial budget readings.
