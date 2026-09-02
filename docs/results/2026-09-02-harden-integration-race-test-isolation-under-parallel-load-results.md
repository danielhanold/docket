<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0373 — Harden integration/race test isolation under parallel load](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0373-harden-integration-race-test-isolation-under-parallel-load.md)**
<!-- docket:backlink:end -->
# Harden integration/race test isolation under parallel load — results

Change: #0373 · Branch: chore/harden-integration-race-test-isolation-under-parallel-load · Plan: docs/superpowers/plans/2026-09-02-harden-integration-race-test-isolation-under-parallel-load.md · ADRs: ADR-0108

Machine for every measurement below: Darwin arm64, `hw.ncpu = 11`, gate `-j = runtime.NumCPU() = 11` (the `development test` default). Gate command: `go run ./cmd/docket development test`.

## Verify (human)

<!-- No genuinely-manual merge-gate checks: the suite gate and the recorded evidence below cover it. -->
- [x] None required — the full-suite gate plus the recorded stability streak and regression cases are the verification; no manual step is reachable that they do not already cover.

## Findings

**Root-cause confirmed as two structural mechanisms, not per-test flakes** (verified against the tree at reconcile):
- **Oversubscription** — the runner launched every `tests/test_*.sh` target at `-j = NumCPU` and the whole-module Go wrappers each ran `go test` at `-p = NumCPU`, with nothing bounding the product.
- **Post-test writers into `t.TempDir()`** — Go's single no-retry `os.RemoveAll` races a still-writing detached child (a `Setsid` supervisor, or git auto-gc/maintenance).

**ADR-0108** records the standing decision: bound total Go test load at the runner (the `DOCKET_GO_TEST_CONCURRENCY` cap honored by the heavy Go wrappers) and isolate real-process test temp dirs behind the shared `internal/testsupport` fixture, enforced by a fail-closed mutation-tested `repoguard`.

### Measured constant 1 — load multiplier (M)

Swept four candidates once each at head `94b45287` (all 39 files passed every run). Per-target cap = `GoTestConcurrency(11,11)`: M=1/1→1, M=3/2→1, M=2/1→2, M=3/1→3.

| M | cap | wall (s) | BUDGET WATCH rows (measured > 2.5x ceiling) |
|---|-----|----------|---------------------------------------------|
| 1/1 | 1 | 288 | 4: finalize_e2e(116) app_rebase(64) race(287) toolchain(192) |
| 3/2 | 1 | 295 | 4: same set (integer-division cap ≡ 1/1) |
| **2/1** | **2** | **210** | **4: finalize_e2e(96) app_rebase(78) race(208) toolchain(161) — PINNED** |
| 3/1 | 3 | 191 | 6: adds app_concurrency(63) app_merge(80) — isolation regresses |

**M = 2/1** pinned (`internal/suiterunner/sandbox.go` `goLoadMultNum/goLoadMultDen`): the smallest multiplier that holds the watch set at the cap-1 floor of 4 rows while recovering the wall clock cap-1 wastes; M=3/1 is faster but widens the watch set to 6. No candidate produced a `SERIAL CONFIRMED OVER BUDGET`.

### Measured constant 2 — cleanup tolerance

From the same instrumented sweep (`removeAllTolerant` retry-count logging): **0 of ~4699 fixture cleanups per run needed a retry** — drain-before-removal made the first `os.RemoveAll` succeed every time. Longest observed drain-to-removable interval across all candidates: **945ms**. `cleanupTolerance = 4s` pinned (`internal/testsupport/testsupport.go`) — ≥4x the heaviest observation, above the 2s floor; the loose direction only costs wall clock on a genuine leak, which fails anyway.

### Re-seeded budget rows (`tests/runtime-budgets.tsv`)

Re-seeded from SOLO measurements per the table's rule (next multiple of 5 above serial seconds, +5s margin, min 10s; no row over 60s; `parallel` mode kept, no `serial` pins):

| wrapper | before (s) | after (s) | reason |
|---------|-----------|-----------|--------|
| tests/test_go_integration_app_rebase.sh | 25 | 45 | solo 38s crossed the old ceiling's serial threshold (25×1.5) |
| tests/test_go_finalize_e2e.sh | 25 | 30 | solo 21s, cleanest run under load |

### Stability streak — five consecutive green full gates

At one head `0c02592` (no commits between runs): **five greens, zero unrelated reds** — every run `SUITE files=39 passed=39 failed=0 asserts=364`. Wall clocks (measured / SUITE seconds): 188/187, 209/187, 188/187, 189/188, 189/188. No `SERIAL CONFIRMED OVER BUDGET` on any run; the only budget-clause output was `PARALLEL-SENSITIVE`/`BUDGET WATCH` screening on the whole-module wrappers (race/toolchain/finalize_e2e), which do not fail the run.

### Regression cases — five named sightings, serial, green

| # | Sighting | Command | Result |
|---|----------|---------|--------|
| 1 | process-group liveness under -race (folds stub 381) | `go test -race -count=1 -run 'TestObserveRunningThenTerminal' ./internal/process/` | PASS |
| 2 | internal/app per-package timeout package | `go test -count=1 ./internal/app/` | PASS |
| 3 | gitcli teardown | `go test -tags integration -count=1 ./internal/gitcli/` | PASS |
| 4 | keyed-commit trailers | `go test -count=1 -run 'TestKeyedCommitCarriesFiveTrailers' ./internal/repository/transaction/` | PASS |
| 5 | shared-`$TMPDIR` recover-scan pollution | `go test -count=1 -run 'Recover' ./internal/process/` with a hand-built decoy supervisor run dir under the real `$TMPDIR` | PASS (decoy byte-unchanged after; no scan/mutation) |

Honest scope on sighting 5: a green probe is evidence about the probe, not proof of absence — but `svc.Recover(root)` scans only the explicit fixture root each test passes, so the shared-`$TMPDIR` vector is closed structurally, not just observationally.

### Review dispositions

Whole-branch deep review (rung `docket-review-deep`) returned 3 findings, all fixed in-branch:

| Finding | Severity | State |
|---|---|---|
| repoguard fixture-guard hardcoded to `internal/` (module-partial coverage) | important | fixed — `3f22012b` (scope made explicit; `cmd/` adoption reported as follow-up) |
| cap plumbing reaches only the heavy Go wrappers (3 light wrappers uncapped) | minor | fixed — `3f7b68c3` (documented as intentional; light wrappers stay at Go defaults) |
| `backgroundOffGitEnv` temp-dir process-lifetime leak | minor | fixed — `3f7b68c3` (comment accepts it explicitly; no-`t` design justified) |

## Follow-ups

- **Extend the fixture + guard to `cmd/` real-process test packages** (out of scope for 0373, whose derived package set was deliberately `internal/`-scoped). `cmd/docket/gate_cli_test.go` spawns real git into temp dirs, uses bare `t.TempDir()`, and carries its own private `gateTempDir` drain-then-retry helper — a divergent third implementation of the drain-retry idea that belongs on `testsupport.TempDir`. Capture deliberately with `docket change create`.

## Notable build deviations

- **Mid-build architectural fix (folded into the build):** a Go test import cycle surfaced when adopting the fixture in `internal/suiterunner` — `internal/testsupport` (test-only) imported `internal/suiterunner` for the shared git-background-off constant, and suiterunner's own `package suiterunner` tests then import `testsupport`. Resolved by relocating the single-source constant to a neutral leaf package `internal/gitbg`, imported by both; suiterunner keeps an exported alias `GitBackgroundOff = gitbg.BackgroundOff`. This design constraint is recorded in ADR-0108.
- **In-scope defect fixed during measurement (Task 9):** Task 8's `TestSandboxExportsGoTestConcurrency` asserted `Sandbox(dir, 0)` omits the cap env var, but under the gate the runner exported that var into the test process and the ambient value leaked through, reddening the gate on two targets. Fixed by clearing the ambient value before the cap-0 assertion (assertion unchanged, mutation-verified).
