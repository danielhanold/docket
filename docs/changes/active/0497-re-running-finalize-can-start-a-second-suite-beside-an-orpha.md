---
id: 497
slug: 're-running-finalize-can-start-a-second-suite-beside-an-orpha'
title: 'Re-running finalize can start a second suite beside an orphaned one'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-03'
depends_on: []
stacked_on:
related: [490, 492, 493]
discovered_from: [493]
adrs: [135]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0135](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0135-gate-drives-never-relaunch-automatically.md) |
<!-- docket:artifacts:end -->

## Why

When a gate's supervisor dies but its test suite keeps running (change 0492's `tree-survives` finding), the gate drive halts (0493, ADR-0135) and the worktree lock is released. If someone re-runs finalize right away, the new run takes the free lock and can start a second suite in the same worktree while the orphaned suite is still running. The two suites share the checkout, ports, and temp state, so results can be wrong or the run can wedge. Build gates and the retired relaunch already had this risk; 0493 added none, but its review surfaced it. Today the only workaround is to check by hand that no `go test` processes are still running from that worktree before re-running.

## What changes

Hypothesis, to be confirmed at grooming: before a gate drive starts a suite in a worktree, consult the launch census that 0492 added. If a prior run in that worktree reports `tree-survives`, refuse to start and report the surviving run instead of launching beside it. Trace the existing census, the worktree lock (0490), and gate start before choosing where the check belongs; prefer extending that machinery over a new mechanism. Per docket policy, decide at grooming whether the outcome is a visible finding or a refusal.

## Out of scope

- Stopping or killing an orphaned suite automatically.
- Making `run.cancel` wait for a leftover suite.
- Reviving any automatic relaunch (retired by 0493).
- Process leaks inside individual tests (`t.Cleanup` hygiene).
