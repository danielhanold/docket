---
id: 528
slug: 'make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a'
title: 'Make the solo budget re-check detect a concurrent suite in another worktree'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [273]
discovered_from: [507]
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

The runner's serial re-check of a budget breach cannot tell when another worktree's suite is running, so it can certify a wall-clock reading taken under that contention as an authoritative SERIAL CONFIRMED OVER BUDGET. Surfaced during change 507. Related to #273, but that change addresses a different cause (host-relative budgets).

## What changes

Have the solo re-check detect a concurrently running suite elsewhere (e.g. other worktrees' suite processes) and either wait, retry, or label the reading as contended rather than certifying it.

## Out of scope

Re-seeding or host-relativizing the budget table (#273).
