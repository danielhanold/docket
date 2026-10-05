<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0507 — Fix the observe-test hang and bring two test files back under their time limits](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0507-flaky-tests-track-and-stabilize-intermittent-suite-failures.md)**
<!-- docket:backlink:end -->

# Fix the observe-test hang and bring two test files back under their time limits: design

Change #507, groomed interactively on 2026-10-05.

## Summary

This change fixes the four suite problems #507 was opened for, in one PR, and then closes:

1. `internal/process` `TestObserveRunningThenTerminal` sometimes hangs for its full 30-second wait. Find and fix the cause, or, if the hang cannot be reproduced, make the next hang report what was stuck.
2. `tests/test_go_integration_app_merge.sh` measures 41s alone against its 30s row. Bring it back under the row.
3. `tests/test_go_finalize_e2e.sh` measures 36s alone against its 30s row. Bring it back under the row.
4. `tests/test_go_race.sh` measures 54s alone against its 60s row. Record the measurement; no code work.

The stub's idea of keeping a running list of flaky tests inside this change is dropped. A docket change is one PR that is built, merged and archived, so it cannot hold a list that keeps growing. A future flake or slow file becomes its own change, captured from results-file follow-ups the way such findings already are.

## Evidence gathered at grooming

### The hang

- **Sightings.** Twice during change 0378's gates (late August 2026, recorded in killed change #381), and once in change 0504's final gate on 2026-10-04. The 0504 failure was `--- FAIL: TestObserveRunningThenTerminal (30.06s)` inside `tests/test_go_race.sh`, with the `internal/process` package taking 66.5s. Each time a plain re-run passed.
- **Normal duration.** 40 consecutive runs under `go test -race -count=40 -run '^TestObserveRunningThenTerminal$' ./internal/process` on a quiet machine (2026-10-05) took 0.05 to 0.07s each. The failure waited 30s, about 500 times longer. Something stayed stuck; the transition was not merely slow. Raising the 30s deadline would hide it.
- **History.** Change #373 (ADR-0108) folded #381 in as sighting 1 and shipped the runner-level Go test concurrency cap and the shared real-process fixture. The hang recurred after that, so those measures did not remove its cause.
- **What the test does.** It launches the `sleep` helper (which sleeps for an hour with default signal handling), observes `running`, reads the manifest, sends SIGKILL to the recorded process group, then polls `Observe` every 20ms for up to 30s until it reports `vanished`. Two results are discarded: the `readManifest` error (`m, _ := readManifest(...)`) and the `signalGroup` return value. Errors from `Observe` during polling are also swallowed ("lock release can race; keep polling"). If the kill is never delivered, or does not reach every process holding the live lock, the helper keeps sleeping and the test waits out the full 30s. That matches the 30.06s failure exactly.

### The timing items

The suite runner treats a file as officially over budget only when a run of that file alone exceeds 1.5 times its row in `tests/runtime-budgets.tsv` (`SoloOver` in `internal/suiterunner/budgets.go`). Each file was measured alone on 2026-10-05 from the primary checkout, with a private HOME and TMPDIR like the runner's sandbox, while no other suite was running (load average 3 to 7 on 11 CPUs):

| File | Row | Official line (1.5×) | Quiet solo, 2026-10-05 | Reading that prompted the item |
|---|---|---|---|---|
| `tests/test_go_finalize_e2e.sh` | 30s | 45s | 36s | 49s (change 0506) |
| `tests/test_go_integration_app_merge.sh` | 30s | 45s | 41s | 92s (change 0520) |
| `tests/test_go_race.sh` | 60s | 90s | 54s | 117s (change 0517); 94s (PR 361) |

The runner's stored budget history (`<git-common-dir>/docket/development-test-budget-state.tsv`) agrees with the quiet numbers. `test_go_finalize_e2e.sh`'s recorded solo readings are mostly 20 to 31s, with one 38 and the one 49.

None of the three crosses its official line on a quiet machine. The over-line readings were most likely taken while another worktree's suite was running on the same machine. The runner's solo re-check only waits for its own run's other files, and it cannot see a suite running in another worktree. PR 361 recorded its 94s race-suite reading at a machine load of 45 to 69 while another change's gate was running. Fixing that measurement is a separate change (see *Discovered work*).

Two files are still over their own rows when the machine is quiet. Under the table's sizing rule (the measured serial time rounded up to the next multiple of 5, plus a 5s margin), a 30s row fits a file that measures 25s or less. The merge file is 4s from its official line. The next change that adds to it, or the next moderately loaded measurement, will report a breach that is half real.

`TestIntegrationFinalizeMerge*` (12 tests in `internal/app/finalize_merge_integration_test.go`) all run sequentially. None calls `t.Setenv`, `os.Setenv`, `os.Chdir` or `t.Chdir`. The row was cut on 2026-08-27 (change 0333), when the file had 10 tests; it now has 12. `TestE2E*` (12 tests in `internal/app/finalize_e2e_test.go`) already run in parallel and build the docket binary once (`sharedBinOnce`). The wrapper also runs `go vet -tags e2e ./internal/app/` before the tests.

## Design

### Shared measurement protocol

Every timing number this change reports is taken the same way. That way a before/after comparison measures the code, not the machine.

- Run from the feature worktree, not the primary checkout (see the `TestProcessExitSitesAreAllowlisted` note under *Discovered work*).
- Run the file the way the runner's solo re-check does: `bash tests/<file>.sh` with `DOCKET_GO_TEST_CONCURRENCY` unset, and a private HOME, TMPDIR and XDG_CONFIG_HOME.
- **Quiet machine.** No other `docket development test`, `go test` or test binary from another worktree is running (check the process list before each run), and the 1-minute load average is below the CPU count at the start of each run.
- Warm the Go build cache with one discarded run, then take three runs and report the median. Record each run's seconds and starting load average.

### Part A: the `TestObserveRunningThenTerminal` hang

**A1. Reproduce first.** Loop the test under the race detector while the machine is loaded. Use at least two load profiles: CPU stress above the CPU count, and a full suite run from another worktree alongside. Example: `go test -race -count=<n> -run '^TestObserveRunningThenTerminal$' ./internal/process`. A hang is unmistakable (about 30s against about 0.06s). Record the profile, iteration count, and hang count for every attempt. The effort is bounded: stop after 5,000 iterations across both profiles, or one hour of attempts, whichever comes first.

**A2. Suspects to check first.** These are where the test discards information:
- `signalGroup(m.PGID, SIGKILL)`'s return value: was the kill delivered?
- `readManifest`'s error: was `m.PGID` what the test thinks it is?
- Which processes still hold `live.lock` after the kill, and whether every holder was in the killed group.
- What `Observe` actually returns during the 30s: a persistent error (and which), or a persistent `running`.

These are starting points, not conclusions. The reproduction decides.

**A3. If the hang reproduces.** Fix the real cause, in the test or in `internal/process`. Then prove the fix under the profile that reproduced it, with a hang-free streak of at least three times the iterations it took to see one hang (at least 1,000 iterations either way). If the cause is shared, apply the fix to every test with the same kill-then-wait-for-gone shape. Find those sites by grepping for `signalGroup(` in `internal/process` tests, not from a hand-made list. Today they include `recover_test.go`, `worktree_lock_test.go`, `leftover_test.go` and `launch_test.go`.

**A4. If it does not reproduce within the bounded effort.** Make the next hang explain itself, and close the item:
- The test stops discarding the two results. A failed `readManifest` or a failed `signalGroup` fails the test immediately, naming the error. This tightens the test; it does not loosen it.
- When the 30s wait expires, the failure message reports what was stuck: the last `Observe` state and the last `Observe` error, whether the killed group still has live members (a zero-signal probe of the group), and whether `live.lock` is still held.
- The results file records the reproduction attempts (profiles, iteration counts) and states that the root cause remains open, so the next sighting is read against them.

**A5. Never.** Raise the 30s deadline, add retries, loosen or remove an assertion, or skip the test under load.

### Part B: `tests/test_go_integration_app_merge.sh` (41s against a 30s row)

- Measure the baseline with the shared protocol.
- The main lever is running the 12 `TestIntegrationFinalizeMerge*` tests in parallel (`t.Parallel()`). First confirm that no test swaps or mutates package-level state: package variables, test seams or hooks, shared fakes. Any test that does stays sequential, with a one-line comment saying why. Under the suite runner, ADR-0108's concurrency cap (`DOCKET_GO_TEST_CONCURRENCY`, which becomes `GOMAXPROCS`) still bounds how many run at once.
- Cut any other redundant work found while tracing, such as fixture setup repeated identically across tests. Never cut assertions or coverage.
- Target: a quiet solo median of 25s or less, so the 30s row is honest under its own sizing rule.

### Part C: `tests/test_go_finalize_e2e.sh` (36s against a 30s row)

- Measure the baseline with the shared protocol, timing each phase separately: the `go vet -tags e2e` pass, the test binary compile, the docket binary build, and the `TestE2E*` run.
- Cut work that is redundant. Keep the vet coverage of the tagged file, the build-tag assert, and the completeness asserts (every declared `TestE2E` ran and passed).
- Target: a quiet solo median of 25s or less.

### Fallback for Parts B and C

If a file cannot reach 25s, its row is re-set by the table's own sizing rule from the post-cut median, and the PR argues it: what was cut, what remains, and why the remainder cannot be cut. `tests/README.md` calls this "a decision to argue in the diff, not a number to bump". A row is never raised without the cut attempt and that evidence.

### Part D: `tests/test_go_race.sh` (54s against a 60s row)

No code work. Record its quiet solo median with the shared protocol in the results file. That way the 117s and 94s readings are on record as contended measurements, not growth.

## Acceptance criteria

- Part A ends in exactly one of two recorded outcomes. Either a root-cause fix proven by the A3 streak, or the A4 diagnostics plus the recorded reproduction attempts. The 30s deadline is unchanged in both.
- `tests/test_go_integration_app_merge.sh` and `tests/test_go_finalize_e2e.sh` each have before/after quiet solo medians recorded. Each is either at or below 25s, or its row is re-set by the sizing rule with the argued evidence.
- `tests/test_go_race.sh`'s quiet solo median is recorded.
- The full suite passes at the build gate. Its budget report is read and every line in it is addressed in the results file.

## Out of scope

- Retry-on-failure machinery, and any change to the gate or budget policy: the 5/2 and 3/2 factors, exit codes, and the screen-then-confirm regime.
- Making the runner's solo re-check notice another suite running on the machine. That is a separate change (see *Discovered work*).
- Flakes and slow files not named here. A new sighting becomes its own change.
- #273's host-relative budgets. These readings point to contention between concurrent suites, not a slower host, so they do not meet #273's revival criterion.

## Discovered work, for a human to capture

- **Contention-aware solo re-check.** `internal/suiterunner`'s scheduled solo confirmation (`soloConfirm` / `ScheduleConfirmation`) certifies a breach even when another worktree's suite is running on the same machine. That produced every over-line reading behind this change's timing items. A separate change should decide how to detect a concurrent suite and what a contended re-check reports and stores, including under strict mode, without adding a blocking gate.
- **`TestProcessExitSitesAreAllowlisted` walks into `.worktrees/`.** Run from the primary checkout, `go test ./cmd/docket` fails this guard with a finding for every `main.go` under `.worktrees/<slug>/cmd/`. Gates run inside feature worktrees, which contain no nested `.worktrees/`, so gates never see it. A person running the suite from the main checkout gets a false red.
