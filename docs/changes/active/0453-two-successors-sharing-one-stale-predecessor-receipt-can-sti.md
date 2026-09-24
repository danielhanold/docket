---
id: 453
slug: 'two-successors-sharing-one-stale-predecessor-receipt-can-sti'
title: 'Two successors sharing one stale predecessor receipt can still free a live worktree slot'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-24'
updated: '2026-09-24'
depends_on: []
stacked_on:
related: [452, 437, 446, 375, 405]
discovered_from: [452]
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

Change 452 fixed the first-start version of this race (a receipt-less loser rotating a same-scope executing slot out from under the winner). Whole-branch review of that change surfaced a sibling bug on the successor side, from code reading rather than reproduction: `admitScopedWorktree`'s rotation path checks only that the incumbent slot is same-scope and `executing`, not that the presenting start's predecessor receipt still names the scope's CURRENT drive.

When two successor starts present the same (now-stale) predecessor receipt, the first to admit can launch and mark the slot `executing`. The second can still rotate that live slot to its own token, then lose the scope check with `ErrStalePredecessor` — and its post-failure cleanup releases the rotated slot while the first successor's run is still live. A subsequent start can then take the worktree while a gate is still running in it, breaking the one-execution-per-worktree invariant the same way change 452's bug did.

See the write-up in change 452's results file (docs/results/2026-09-24-same-scope-first-start-loser-must-not-rotate-the-winner-s-ex-results.md, "Two successors sharing one stale receipt can still free a live slot") for the full trace.

## What changes

- In `admitScopedWorktree`'s rotation path, before rotating an executing same-scope slot, verify the presenting start's predecessor receipt still names the scope's CURRENT drive (not just that the slot is same-scope and executing). A stale receipt refuses `ErrStalePredecessor` without touching the slot.
- A deterministic two-successor regression test modeled on `TestSameScopeFirstStartLateLoserDoesNotRotate` (added in change 452): two successors present the same stale receipt, the first admits and executes, the second must be refused without rotating or releasing the first's slot. Mutation-check it against the unguarded code.

## Out of scope

- Any other admission/arbitration path unrelated to receipt staleness on rotation.
- Re-litigating change 452's fix, which is unrelated and already merged/mergeable.
