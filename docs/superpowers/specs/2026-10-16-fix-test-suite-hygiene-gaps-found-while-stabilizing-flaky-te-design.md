<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0527 — Fix test-suite hygiene gaps found while stabilizing flaky tests](../../changes/active/0527-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te.md)**
<!-- docket:backlink:end -->

# Fix test-suite hygiene gaps found while stabilizing flaky tests — design

Change #527, groomed 2026-10-16.

## Problem

Changes 507 (flaky-test stabilization), 531 (private visibility), and 539 (finalize) surfaced
seven unrelated test-suite defects. Each one lets a test silently stop running, fail for an environmental
reason, or fail when nothing is wrong. Items 1–6 were re-traced against `main` at `2f47107e4`
on 2026-10-16; the findings below are what that trace showed, not the original stub's wording.
Item 7 was observed once while finalizing change 539 and has not been traced; it is
diagnose-first.

## Goals

- A test function that stops being run because of its name is caught, in any test file.
- No test in the seven items fails because of `TMPDIR` spelling, an unrelated worktree, or a slow
  disk.
- The concurrency and reposetup shards have budget rows that match the rule in
  `tests/README.md`: split an oversized shard rather than raise its ceiling.
- The one observed `TestObserveRunningThenTerminal` timeout has a known cause, or is recorded
  as not reproducible; it is not hidden by a longer timeout.

## Non-goals

- Changing what any existing test asserts. Every item here repairs how a test runs or is
  counted, never what it proves.
- Host-relative budgets or re-seeding the whole budget table (#273).
- The workflowlifecycle shard split (#536).
- Detecting another worktree's concurrent suite in the solo re-check (#528).
- Editing point-in-time records (archived changes, results files, release logs) that name
  renamed tests.

## Design

### 1. Repo-wide guard for unrunnable test functions

**Defect (confirmed).** `tests/test_go_finalize_e2e.sh` counts declared tests with
`grep -c -- '^func TestE2E'` and run tests with `grep -c -- '^--- PASS: TestE2E'`. Renaming
`TestE2EFoo` to `testE2EFoo` drops both counts by one, so the check stays green while the test
never runs. Nothing else catches it: `go test -list` (used by
`tests/test_go_integration_contract.sh` and `tests/test_release_partition_fidelity.sh`) never
lists lower-case functions, `go vet`'s `tests` analyzer flags only malformed `Testfoo` names,
and the repo has no unused-code linter.

**Fix.** A new guard test in `internal/repoguard` (the human chose this over hardening only the
e2e script; the guard covers the e2e case and every other test file). It flags a function that
looks like a test but that `go test` will never run and nothing calls:

- Walk files with `repoguard.MaintainedFiles` (the same exclusions every other repo-root guard
  uses), keep `_test.go` files, and parse each with `go/parser`. Parsing directly ignores build
  constraints, so `e2e`- and `integration`-tagged files are covered.
- A **candidate** is a top-level function declaration (no receiver) with exactly one parameter
  of type `*testing.T`, `*testing.B`, or `*testing.F`, and no results, whose name is NOT one
  `go test` runs. A name that `go test` runs is `Test`/`Benchmark`/`Fuzz` followed by the end of
  the name or a rune that is not lower-case (the `go test` rule); the prefix must match the
  parameter type. Covering B and F as well as T is the implementer-facing call made at grooming:
  the cost is identical and a lower-cased benchmark goes silent the same way.
- A candidate is **reported** when its name is never used as an identifier anywhere else in the
  same directory's Go files (test and non-test, all packages in that directory). The 11 existing
  lower-case `func(t *testing.T)` subtests in `internal/repoguard/absence_test.go` and
  `internal/repoguard/retired_vocabulary_test.go` are all passed to `t.Run`, so they are
  referenced and not reported. The trace found zero reportable functions today.
- The failure message names each file and function and says what to do: rename it to a runnable
  `Test…` name, or call it from one.
- Per `AGENTS.md` *Guards and tests*: the guard needs a negative control that proves it fires.
  Plant an unreferenced `func testOrphanExample(t *testing.T)` in a fixture (a `testdata` tree
  or an in-memory source string) and assert it is reported, and assert a referenced one is not.
  It also must be mutation-tested once during the build: rename a real `TestE2E…` function to
  lower case locally, watch the guard redden, revert.

`tests/test_go_finalize_e2e.sh` needs no edit: the guard catches its case.

### 2. No-git guard compares an uncleaned path

**Defect (confirmed, narrower than the stub said).** `AssertNoGitGuardShadowsGit` in
`internal/testsupport/nogit.go` compares `filepath.Dir(p)` (cleaned) against `noGitGuardDir`,
which `InstallNoGitGuard` in `internal/testsupport/nogit_install.go` stores raw from
`os.MkdirTemp`. A single trailing slash is harmless (including the macOS default
`/var/folders/…/T/`), because `os.MkdirTemp` does not add a second separator. A `TMPDIR` that is
not in clean form fails: `TMPDIR=/tmp//` fails the `TestNoGitGuardShadowsGitOnPath` wrappers in
`internal/workspace`, `internal/gatedrive`, `internal/repository/transaction` (and `internal/app`).

**Fix.** Store `filepath.Clean(dir)` in `InstallNoGitGuard`, so every comparison and use of the
guard dir sees one spelling.

### 3. `TestAgentEnterCLIPreservesRequestAndReceipt` EOF

**Finding.** The one-off EOF did not reproduce: 0 failures in 230 runs (`-count=30` with
`TMPDIR=/tmp/`, `-count=200` with the default). The test is unchanged since 2026-10-02. An EOF
can only mean the fake app-server (`TestAgentEnterServerProcess` in `internal/cli/agent_test.go`)
closed stdout before replying. Every failure branch of that fake exits through `os.Exit(2)`…`(6)`
without writing anything, except the per-key branch, so any early exit surfaces as a bare `EOF`
with no cause.

There is also a production defect behind the bare `EOF`. `processTransport.Recv`
(`internal/codexentry/process.go`) appends the app-server's stderr to a read error, but
`StartAppServer` points `cmd.Stderr` at a `boundedBuffer` (not an `*os.File`), so `os/exec`
copies stderr on a background goroutine, and `Recv` reads `p.stderr.String()` the moment stdout
hits EOF without waiting for that copy. A real codex app-server that crashes can lose its error
message the same way the fake's would.

**Fix (two parts; the human chose to fix the client, not only the test).**

- *Client:* when `Recv` gets a read/decode error, let the stderr copy finish before reading the
  buffer, so the server's last words are in the error. Waiting on the command (which completes
  the copy) is the natural way, but it must not hang when the server has closed stdout yet is
  still running: bound the wait (a short grace period, then read whatever stderr holds) and keep
  the transport's existing `Close`/`Wait` bookkeeping consistent (the process is waited at most
  once; `Close` after such a wait still succeeds). Optionally include the exit status in the
  error. Add a unit test in `internal/codexentry` with a server that writes to stderr and exits
  immediately, asserting the stderr text appears in the error, run enough times (`-count`) to
  show it is not timing-dependent; it must fail against the current code.
- *Fake server:* before each `os.Exit` in `TestAgentEnterServerProcess`, write one line to stderr
  naming the check that failed and the values involved (for the cwd check: both paths). Also
  report `scanner.Err()` before the final exit after the read loop, so an over-long request line
  is not a silent `EOF` either.

No further flake investigation and no change to what `TestAgentEnterCLIPreservesRequestAndReceipt`
asserts.

### 4. `TestProcessExitSitesAreAllowlisted` walks into `.worktrees/`

**Defect (confirmed, reproduces from the main checkout).** `cmd/docket/exit_sites_test.go` walks
`filepath.Abs("../..")` with its own skip list (`.git`, `vendor`, `testdata`, `node_modules`), so
every feature worktree's `cmd/*/main.go` under `.worktrees/` is reported. It is the only
repo-root walker that does not use the shared helper.

**Fix.** Replace the hand-rolled walk with `repoguard.MaintainedFiles(root)`, filtered to non-test
`.go` files. That helper already skips every hidden directory except `.github` (so `.git`,
`.worktrees`, `.docket`, `.claude`, …) and `testdata`. Confirm the allowlisted exit sites are
unchanged by the switch (none of the helper's other exclusions — `docs`, `tests/fixtures`,
`internal/install/legacydata` — holds Go source today).

### 5. Gate-driver tests whose child dies on a timer

**Defect (confirmed by code reading; the 34-of-40 failure is from change 531's results).**
`child-signal-death` (a subtest of `TestIntegrationGatedriveProcessDeathHaltsSupervisorDied`,
`internal/gatedrive/supervisor_integration_test.go`) starts a child in mode `selfkill-after 120`:
it sleeps 120 ms and SIGKILLs itself. The timer starts at the child's start, and before
`d.Start` can return `WAITING` the run must finish three atomic writes with file and directory
fsyncs (the supervisor's running manifest, the store's `attachLaunch` owner CAS, the worktree
lock holder) and then observe `StateRunning` across the 40 ms slice. Under fsync contention that
exceeds 120 ms, the child is already dead, `Start` returns `HALTED/supervisor-died`, and the
"want WAITING" assertion fails. Unloaded, the subtest takes 0.17–0.19 s against the 120 ms
timer — the headroom is thin even on a quiet machine.

Three more tests depend on a timed child still being alive, with more headroom:

- `TestIntegrationGatedriveDriverSlicesAcrossLiveChildThenPasses` (`pass-after 600`): asserts
  the first slice is `WAITING` and that the child spans at least two further `WAITING` slices.
- `TestRaceIntegrationGatedriveTakeoverKeepsRunIdentity`
  (`internal/gatedrive/takeover_race_integration_test.go`, `pass-after 500`): asserts the first
  slice is `WAITING` and that the child is still live when the takeover happens.
- `TestIntegrationGatedriveFreshProcessResumesAndChildSurvives`: runs the `drive-start` helper
  subprocess with a `pass-after 1500` child. The helper fails (`return 84`) if its first slice is
  not `WAITING`, and after the helper exits the test asserts the supervisor is still alive, which
  holds only while the child runs. (Found by the pre-plan builder review; converting it follows
  the human's direction to convert every timer-dependent test.)

`TestIntegrationGatedriveTerminalConsumedFromFreshProcess` (`pass-after 300`) was considered and
left alone: it only drives to a terminal pass and asserts nothing about the child being alive at
any slice, so the timer cannot make it fail. `pass-after 0` (`…FastCompletionReturnsImmediately`)
is likewise unaffected.

**Fix — the child waits for a go-signal from the test, not a timer.** Add two child modes in the
test binary's child dispatcher (beside `pass-after` / `selfkill-after`):

- `selfkill-on <path>`: poll for `<path>` every 5 ms; once it exists, SIGKILL only this process
  (not the group), exactly as `selfkill-after` does today.
- `pass-on <path>`: poll for `<path>` every 5 ms; once it exists, write the stdout marker and
  exit 0, exactly as `pass-after` does.

`<path>` is a file in the test's own temp dir that does not exist at start. Each test creates it
at the point where the child is allowed to proceed:

- `child-signal-death`: after `Start` returns `WAITING`, then `advanceUntilTerminal` as today.
  The child still dies by its own SIGKILL with no stop intent, so `StateSignaled` and
  `supervisor-died` are exercised as before.
- `…DriverSlicesAcrossLiveChildThenPasses`: after `Start` returns `WAITING` **and** at least two
  further `Advance` calls have returned `WAITING`, then advance to the terminal `PASSED`. The
  `waiting >= 2` assertion keeps its meaning (the WAITING count must include those pre-signal
  advances).
- `…TakeoverKeepsRunIdentity`: after the takeover, before the fresh owner drives to `PASSED`.
- `…FreshProcessResumesAndChildSurvives`: `drive-start` takes the signal-file path in place of
  its child milliseconds and starts a `pass-on <path>` child; the test creates the file after its
  post-helper liveness assertions, before it resumes the drive to the terminal.

The child is then alive at every point the test asserts liveness, by construction, however slow
the disk. The existing 30 s limits in `advanceUntilTerminal` and `waitWorktreeFree` already bound
a slow exit after the signal. Remove `selfkill-after` once nothing uses it; keep `pass-after`,
which the unaffected tests still use. Update the child-mode list in the doc comment above the
dispatcher (including `drive-start`'s new argument).

A signal handler (SIGUSR1) was rejected: the run manifest records the supervisor's pid and
process group, not the child's, so the test has no clean way to address the child. Raising the
timers was rejected as a band-aid that only narrows the window and adds seconds per run.

### 6. Concurrency and reposetup budget rows

Change 531 raised these rows instead of splitting the shards, against `tests/README.md`. Serial
measurements on 2026-10-16:

| Shard | Row now | Measured | Tests |
|---|---|---|---|
| `tests/test_go_integration_app_concurrency.sh` | 45 (raised from 35) | 30 s | 13 |
| `tests/test_go_integration_app_reposetup.sh` | 35 (raised from 15) | 61 s | 56 |

**Concurrency** fits its original ceiling: no split. Set its row from a fresh serial measurement
by the table's seeding rule (header of `tests/runtime-budgets.tsv`: round the measured wall time
up to the next multiple of 5, add a 5 s margin, minimum 10 s). The 2026-10-16 reading predicts 35.

**Reposetup** is past its row and the 52.5 s solo-breach line. Its 23 private-mode tests account
for 34.8 s; the remaining 33 take about 25 s. Move the 23 into a new shard.

*Prefix constraint.* `tests/test_go_integration_contract.sh` requires every tagged test to match
exactly one runner prefix, matched as a literal glob. Keeping the names
`TestIntegrationRepoSetupPrivate…` would match both `TestIntegrationRepoSetup` and any new
`…RepoSetupPrivate` prefix, so the 23 tests are renamed to a new prefix,
`TestIntegrationPrivateRepoSetup`. It overlaps no existing prefix in either direction
(`TestIntegrationPrivateInstructions`, `TestIntegrationRepoVisibilityPrivate`,
`TestIntegrationRepoSetup`, `TestRaceIntegrationRepoSetup`, and #536's planned
`TestIntegrationPrivateMode`). Re-check against the live set of `SHARD_PREFIX` declarations at
build time rather than trusting this list.

*Renames (functions only; files stay where they are).* Replace the leading
`TestIntegrationRepoSetupPrivate` with `TestIntegrationPrivateRepoSetup` in the 22 tests that
carry it — 20 in `internal/app/repository_private_integration_test.go`, 2
(`…PrivateInitHarnessesFlag`, `…PrivateConfigureHarnesses`) in
`internal/app/reposetup_harnesses_integration_test.go` — and rename
`TestIntegrationRepoSetupGlobalPrivateDefault` to `TestIntegrationPrivateRepoSetupGlobalDefault`.
For example, `TestIntegrationRepoSetupPrivateRerunIsNoOp` becomes
`TestIntegrationPrivateRepoSetupRerunIsNoOp`. Derive the list from
`go test -tags integration -list '^TestIntegrationRepoSetup' ./internal/app` filtered to the
private ones; the count must be 23. Update each renamed function's doc comment and any file-header
paragraph that names the shard or prefix.

*New shard.* `tests/test_go_integration_app_reposetup_private.sh`, declarations only, copied from
the reposetup shard's shape: `SHARD_PKG="./internal/app"`,
`SHARD_PREFIX="TestIntegrationPrivateRepoSetup"`, `SHARD_MODE="normal"`, then
`shard_inspect_maybe` / `run_integration_shard`. Its header says what it holds and that it was
split out of the reposetup shard. The contract test discovers runners structurally and needs no
edit.

*Rows.* Measure both shards serially after the move and set each row by the seeding rule. Add the
new shard's row beside the other app shards; lower the reposetup row. Measure on a quiet machine;
another worktree's suite inflates a solo reading, so re-measure once if a number looks off. The
remaining reposetup tests (about 25 s) cannot fit the original 15 — that is expected.

### 7. `TestObserveRunningThenTerminal` timed out once

**Observation (not yet traced).** During the finalize of change 539 on 2026-10-16,
`TestObserveRunningThenTerminal` (`internal/process/observe_test.go`) failed once: after the test
SIGKILLed the helper's process group, `Observe` did not report `vanished` within the test's 30 s
wait. The next run passed, and so did a full suite. Several whole-suite gate runs and a second
worktree's suite were running at the same time, so the machine was heavily loaded. The failure
text was not kept, so the state the run was stuck in is unknown. The test already reports it on
failure (`describeStuckRun`, added by an earlier fix): last `Observe` state and error, group and
supervisor liveness, and `live.lock` state.

**Fix: diagnose first, then repair the cause.** Run the test with a high `-count` while the
machine is under sustained CPU and fsync load (for example several other `go test` runs at once),
and keep the `describeStuckRun` line from any failure. Then:

- If the line shows `Observe` never leaving `running` or returning an error (a supervisor or
  lock state `Observe` misreads), that is a defect in `internal/process`: fix it there and add a
  test that fails without the fix.
- If it shows the supervisor or group still alive when it should be gone (a process not yet
  reaped, or a test-side race with the kill), fix the test so it waits on the real condition.
- If it cannot be reproduced in a few hundred loaded runs, change nothing: record the run count
  and load used in the results file. Do not raise the 30 s limit or loosen the assertion to
  make a flake disappear.

## Verification

- The new unrunnable-test guard passes on the tree, reports its planted orphan, does not report a
  referenced lower-case subtest, and reddens on a local lower-case rename of a real `TestE2E…`
  function (reverted).
- `TMPDIR=/tmp// go test ./internal/workspace ./internal/gatedrive ./internal/repository/transaction ./internal/app -run TestNoGitGuardShadowsGitOnPath -count=1`
  passes.
- `TestProcessExitSitesAreAllowlisted` passes when run from the main checkout with feature
  worktrees present under `.worktrees/`.
- The new `internal/codexentry` test fails against the old `Recv` and passes after the fix.
- `child-signal-death` and the three other converted tests pass `-count=20`. If the implementer can
  produce sustained fsync load on the build machine, also rerun `child-signal-death` 40 times
  under it (the 531 measurement was 34 of 40 failing); otherwise say in the results that the
  fix rests on the construction argument above plus unloaded runs.
- `tests/test_go_integration_contract.sh` passes: every renamed test matches exactly one runner,
  and the new runner selects 23 tests; the reposetup shard selects 33.
- `go test -tags integration -list '^TestIntegrationRepoSetup(Private|GlobalPrivate)' ./internal/app`
  lists nothing.
- `TestObserveRunningThenTerminal` passes at a high `-count` under sustained load, or the results
  file records how many loaded runs failed to reproduce the timeout and what the failure line
  showed if one did.
- The whole suite (`build.test_command`) is green, and its budget report shows no
  `BUDGET WATCH:` or `SERIAL CONFIRMED OVER BUDGET:` line for the concurrency, reposetup, or new
  shard.

## Left to the builder

- Function and file names for the guard, and whether its negative control uses a `testdata`
  tree or in-memory source.
- The child-mode spellings (`selfkill-on` / `pass-on` are suggestions) and the poll mechanics.
- Exact wording of the fake server's stderr lines and the guard's failure message.
- Commit grouping; the seven items are independent and can land in any order.
