---
id: 527
slug: 'fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te'
title: 'Fix test-suite hygiene gaps found while stabilizing flaky tests'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-16'
depends_on: []
stacked_on:
related: [273, 536]
discovered_from: [507, 531]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 507 (flaky-test stabilization) surfaced several unrelated test-suite defects that no existing change covers. Each lets a test silently stop running, fail for environmental reasons, or fail falsely.

Change 531's build surfaced two more: a gate-driver test that fails under disk load, and runtime budgets that were raised where `tests/README.md` says to split the test instead.

## What changes

- The e2e test-count check misses a test renamed to lower case: the test silently stops running and the file stays green. Make the check catch it.
- `TestNoGitGuardShadowsGitOnPath` fails when `TMPDIR` ends in `/` because it compares paths as strings. Compare cleaned paths.
- `TestAgentEnterCLIPreservesRequestAndReceipt` showed a one-off EOF in the same hand-run measurement; investigate whether it is flaky.
- `TestProcessExitSitesAreAllowlisted` fails falsely when run from the main checkout because it walks into `.worktrees/`. Skip that directory.
- The gate-driver test `child-signal-death` fails under heavy disk load: 34 of 40 runs on unmodified `main` under the same load (found during change 531, not caused by it). Make it robust to slow disk, or fix what it races on.
- Change 531 raised runtime budgets instead of splitting the over-budget tests, against the rule in `tests/README.md` (flagged in PR #399). The workflowlifecycle shard (35→55, since raised to 80) is split by #536. This change handles the other two, measured serially on 2026-10-16:
  - concurrency (35→45): runs in 30s across 13 tests, so it needs no split; lower its row back to 35.
  - reposetup (15→35): runs in 61s across 56 tests, past its 35s row and past the 52.5s solo-breach line. Its 23 private-mode tests account for 34.8s of that. Split them into their own shard, then set both rows from fresh serial measurements by the table's seeding rule; the remaining tests (about 25s) cannot fit the original 15.

## Out of scope

The runner's solo re-check not detecting another worktree's concurrent suite (tracked separately). Putting budgets on a host-relative basis and re-seeding the budget table (#273); this change only re-shapes the concurrency and reposetup shards. The workflowlifecycle shard split is #536.
