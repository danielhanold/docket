<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0466 — Bring test_go_race back under its 60s budget row (transaction, workspace, gatedrive)](../../changes/archive/2026-09-29-0466-bring-test-go-race-back-under-its-60s-budget-row-transaction.md)**
<!-- docket:backlink:end -->

# Change 0466 — Bring test_go_race back under its 60s row by partitioning transaction, workspace, and gatedrive

## Problem, measured

Change 0465 removed `internal/app` from the race gate's critical path. `tests/test_go_race.sh` still measures about 66–67s solo against its 60s row. `tests/test_go_toolchain.sh` measures 62–66s on a cold test cache against its 55s row. The time now sits in three packages. Grooming measured each one solo with `go test -race -count=1 -json` (2026-09-28, load ~2.5):

| package | wall | tests | `t.Parallel` | user CPU | time in tests ≥0.3s |
|---|---|---|---|---|---|
| `internal/repository/transaction` | 46s | 102 | 0 | 9s | 44 tests, 41s |
| `internal/workspace` | 42s | 91 | 0 | 20s | 52 tests, 39s |
| `internal/gatedrive` | 42s | 290 | 0 | 16s | 26 tests, 27s (15s of it `TestIntegration*`) |

No single test is slow; the largest is ~4.6s. Each package's wall time is the serial sum of many 0.3–4s tests that build real git repos or start real supervised processes. CPU is low because the time is spent waiting on subprocesses. This is the same shape 0333 and 0465 fixed for `internal/app`, and the fix here reuses their machinery.

`internal/gatedrive` already has four real-process test files (`integration_test.go`, `integration_history_test.go`, `integration_sequence_test.go`, `integration_takeover_test.go`) whose tests use the `TestIntegration` prefix. They carry **no** build tag, so they run in the default corpus.

## Decision (settled with the human at groom)

1. **Partition, don't parallelize.** Move the real-git and real-process tests of the three packages behind `//go:build integration` into `*_integration_test.go` files (and `*_race_integration_test.go` where race-mode), served by new shard runners on `tests/lib/go-integration-shard.sh`. We considered and rejected converting the packages to `t.Parallel()`. It would be a new pattern in this repo. The transaction and workspace harnesses set `GIT_CONFIG_GLOBAL` process-wide via `t.Setenv`, explicitly "safe because this package runs no test in parallel". It would also reopen the change-0373 teardown-race class. Partitioning extends established machinery. `tests/test_go_integration_contract.sh` discovers packages structurally from the `*_integration_test.go` census (change 0362), so the new packages join the contract without any registry edit.
2. **Race instrumentation is kept where races can occur (0333's split).** Tests that exercise concurrency move to `mode=race` shards with the `TestRaceIntegration…` prefix. Examples: transaction's `TestConcurrency*` and `TestSetPhaseAtomicUnderConcurrentReads`, and gatedrive's takeover and concurrent-scope tests such as `TestIntegrationSequenceConcurrentScopesResolveOwnWork` and `TestIntegrationTakeoverKeepsRunIdentity`. Sequential real-git and real-process tests move to `mode=normal` shards with the `TestIntegration…` prefix. When in doubt, a test whose body starts more than one goroutine or process against shared state goes to race. The contract's checks (5) and (9) already enforce the prefix↔mode correspondence.
3. **The partition is an enforced invariant in transaction and workspace.** Hoist 0465's no-real-git PATH-shim guard (`installNoGitGuard`, `internal/app/nogit_guard_test.go`) into `internal/testsupport`, keeping its mechanism, its violation-log verdict, its probe argument, and its documented limits. Install it from `TestMain` in `internal/app`, `internal/repository/transaction`, and `internal/workspace`. `internal/app` must keep behaving byte-identically: same diagnostic, same exit code, and its existing proving tests still pass or move with the helper. `internal/gatedrive` does **not** get the guard, because its slow tests start supervised processes rather than git and a git shim would not detect their regrowth. There the budget row stays the growth detector.
4. **What moves is keyed on shape, not on a timing threshold.** Every test in transaction and workspace that starts real git moves; the guard makes that the definition. In gatedrive, every test that drives the real `internal/process.Service` supervisor or real git moves, including the four existing untagged `integration*_test.go` files. Fixture helpers used by both corpora go in a file without a build tag. Helpers used only by the tagged corpus move with it, so `go vet` stays clean in both tag sets.

## Shards and budgets

- Create one runner per (package, prefix) group. Split a package into several runners only when one runner would exceed a sensible row (≤40s, the size of the existing integration rows), following the 0333 per-domain naming, e.g. `test_go_integration_transaction_engine.sh`, `…_transaction_race.sh`, `…_workspace_prepare.sh`, `…_gatedrive_process.sh`. Prefixes must not overlap: no runner's prefix may be a string prefix of another runner's in the same package. This avoids repeating the pre-existing `TestIntegrationRepo` overlap 0465 noted.
- Each new runner gets a `tests/runtime-budgets.tsv` row sized from its own serial-confirmed measurement with real headroom. Do not size rows at parity (learning `budget-headroom-is-spent-before-it-is-breached`). Record every margin as a number in the results file.
- **Do not raise** the `test_go_race.sh` (60) or `test_go_toolchain.sh` (55) rows. Do not touch the 8m `RACE_TIMEOUT` backstop.
- Update the "PARTITION AND LANE" paragraph in `tests/test_go_race.sh` so it names the partitioned packages and the guard's new home. Update the "BACKSTOP TIMEOUT" paragraph's named worst package if it changes.

## Acceptance

1. `tests/test_go_race.sh` is serial-confirmed under 60s with margin. `tests/test_go_toolchain.sh` is serial-confirmed under 55s **on a cold test cache** (`go clean -testcache` first). Both are measured solo on an idle machine, with the margin recorded.
2. After the move, each of the three packages' default-corpus `-race` wall time is recorded in the results file. The expected shape is single-digit seconds for transaction and workspace.
3. `tests/test_go_integration_contract.sh` passes with the three new packages discovered automatically, with no allowlist edit.
4. Mutation-test the guard in each newly guarded package: add a throwaway default-tag test that runs `git`, watch that package's `go test` redden with the guard diagnostic, then revert. `internal/app`'s existing guard proofs still pass after the hoist.
5. The whole suite is green, and its budget report prints no `SERIAL CONFIRMED OVER BUDGET` line for any file this change touched or created.

## Out of scope

Raising any budget row or the race backstop. `t.Parallel` conversion. `internal/app` beyond the mechanical guard hoist. The three `internal/app` shards 0465 suspected were over their rows under load (`closeout`, `rebaserecovery`, `changeruntime`). Closing the guard's documented bypass routes (`gitcli.WithExecutable`, wholesale PATH replacement). The pre-existing `TestIntegrationRepo` prefix overlap.
