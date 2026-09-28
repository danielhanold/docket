---
id: 466
slug: 'bring-test-go-race-back-under-its-60s-budget-row-transaction'
title: 'Bring test_go_race back under its 60s budget row (transaction, workspace, gatedrive)'
status: 'in-progress'
priority: 'medium'
type: 'chore'
created: '2026-09-28'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [465, 333, 362, 373]
discovered_from: [465]
adrs: [108]
spec: 'docs/superpowers/specs/2026-09-28-bring-test-go-race-back-under-its-60s-budget-row-transaction-design.md'
plan: 'docs/superpowers/plans/2026-09-28-bring-test-go-race-back-under-its-60s-budget-row-transaction.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/bring-test-go-race-back-under-its-60s-budget-row-transaction'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-28T20:52:09Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-28-bring-test-go-race-back-under-its-60s-budget-row-transaction-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-28-bring-test-go-race-back-under-its-60s-budget-row-transaction-design.md) |
| Plan | [2026-09-28-bring-test-go-race-back-under-its-60s-budget-row-transaction.md](https://github.com/danielhanold/docket/blob/chore/bring-test-go-race-back-under-its-60s-budget-row-transaction/docs/superpowers/plans/2026-09-28-bring-test-go-race-back-under-its-60s-budget-row-transaction.md) |
| ADRs | [ADR-0108](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md) |
<!-- docket:artifacts:end -->

## Why

Change 0465 removed `internal/app` from the race gate's critical path, but `tests/test_go_race.sh` still measures about 66–67s against its 60s budget row. `tests/test_go_toolchain.sh` measures 62–66s on a cold test cache against its 55s row. The time now comes from three packages under `-race`: `internal/repository/transaction` (~46s solo), `internal/workspace` (~42s), and `internal/gatedrive` (~42s). Grooming measured them. No single test is slow. Each package's time is the serial sum of dozens of real-git or real-process tests that mostly wait on subprocesses, and none of them runs in parallel. `internal/gatedrive` even has four real-process `integration*_test.go` files that were never build-tagged. Neither gate is near the 8m backstop or Go's 10m limit, so CI is not at risk. But the suite prints `BUDGET WATCH` for both files, and since 0465 made budget keys repo-relative, it will eventually print `SERIAL CONFIRMED OVER BUDGET`. Source: the 0465 results file, "Known issues and follow-ups".

## What changes

Extend the 0333/0465 integration partition to these three packages instead of raising the rows:

- Move the real-git and real-process tests of transaction, workspace, and gatedrive behind `//go:build integration` into new shard runners. Concurrency tests keep `-race` in `mode=race` shards (`TestRaceIntegration…`). The sequential bulk goes to `mode=normal` shards (`TestIntegration…`). The integration contract discovers the new packages automatically.
- Hoist 0465's no-real-git guard from `internal/app` into `internal/testsupport` and install it in transaction and workspace. The default corpus of those packages can then no longer silently regrow real-git tests. `internal/app` behaves identically; gatedrive (process-bound, not git-bound) relies on its budget row.
- Give each new shard a budget row sized from a serial-confirmed measurement with headroom. Leave the race and toolchain rows at 60/55, and serial-confirm both gates (toolchain on a cold cache) under them.

Full design, test-move rules, and acceptance criteria are in the linked spec.

## Out of scope

Raising any budget row or the 8m race backstop. Converting tests to `t.Parallel()` (considered and rejected at groom). Changing `internal/app` beyond the mechanical guard hoist. The three `internal/app` integration shards that measured over their rows under load in 0465's re-measure (`closeout` 64s/40, `rebaserecovery` 60s/40, `changeruntime` 40s/40): suspected, not confirmed, serial-confirm them separately. Closing the guard's documented bypass routes (`gitcli.WithExecutable` with an absolute path, wholesale PATH replacement). The pre-existing `TestIntegrationRepo` shard-prefix overlap.

## Reconcile log

### 2026-09-28

2026-09-28 — Reconciled against origin/main ef4a341d2. Design still holds: internal/gatedrive still carries four untagged integration*_test.go files, the no-real-git guard still lives only in internal/app (nogit_guard_test.go / nogit_guard_off_test.go), transaction and workspace have no integration-tagged files yet, and no change since 0462 touched the three packages. No scope adjustment.
