---
id: 528
slug: 'make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a'
title: 'Make the solo budget re-check detect a concurrent suite in another worktree'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [273, 557]
discovered_from: [507]
adrs: []
spec: 'docs/superpowers/specs/2026-10-16-make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-17T02:35:45Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-16-make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a-design.md](../../superpowers/specs/2026-10-16-make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a-design.md) |
<!-- docket:artifacts:end -->

## Why

The runner's solo re-check of a budget breach cannot tell when another worktree's suite is running, so it can certify a wall-clock reading taken under that contention as an authoritative `SERIAL CONFIRMED OVER BUDGET`. Change #507 found that every over-the-line reading behind its timing items was contention: measured alone on a quiet machine, the same files came in well under their lines. Each false breach became results-file follow-ups and grooming work. This is related to #273, but that change addresses a different cause (host-relative budgets).

## What changes

When the runner re-times a slow test by itself to confirm a budget breach, it first checks whether another `docket development test` run, in any worktree of this repository, was running at the same time. Every run puts up a crash-safe "I'm running" sign (a file under a kernel lock, beside the budget-history store) for as long as it runs. The re-check looks for other runs' signs just before and just after it times the test.

- No other run seen: behavior is unchanged.
- Another run seen and the test was still under the line: it counts as cleared, because contention only slows a test down.
- Another run seen and the test was over the line: not counted. The report prints `SERIAL CONFIRMATION NOT COUNTED:` naming the other worktree, no breach is recorded, and the test stays due so the next quiet run re-checks it first.

Nothing waits, no time is added, and nothing new can fail a gate. Under the opt-in strict mode, which no gate uses, a not-counted reading fails the run the same way a crashed re-check does today. `tests/README.md` and the glossary describe the new line. Detail is in the linked spec.

## Out of scope

- Waiting, queueing, or making suites take turns.
- Detecting load from anything other than a `docket development test` run of this repository, such as a hand-run `go test` in another worktree, other repositories, or load average. This blind spot is accepted.
- Direct readings from a single-job run or a serial-mode budget row, which nothing uses today (#557 reviews that path).
- Retiring strict mode (#557).
- Re-seeding or host-relativizing the budget table (#273).

## Reconcile log

### 2026-10-17

2026-10-17 — Reconciled against origin/main dedd4189e. The spec (groomed 2026-10-16) still matches the code: `ScheduleConfirmation`, `StrictConfirmCandidates`, and `soloConfirm` in internal/suiterunner/budgetstate.go and `process.TryExclusiveLock`/`process.ProbeLock` in internal/process/lock.go are unchanged in shape; the only recent suiterunner commit (state folder resolved through layout) keeps the state path in the per-repo folder the sign folder sits beside. #273 is still deferred and #557 still proposed; no overlap moved. No scope change.
