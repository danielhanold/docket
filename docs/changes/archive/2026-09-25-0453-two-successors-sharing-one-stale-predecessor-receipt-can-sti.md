---
id: 453
slug: 'two-successors-sharing-one-stale-predecessor-receipt-can-sti'
title: 'Two successors sharing one stale predecessor receipt can still free a live worktree slot'
status: 'done'
priority: 'high'
type: 'fix'
created: '2026-09-24'
updated: '2026-09-25'
depends_on: []
stacked_on:
related: [452, 437, 446, 375, 405]
discovered_from: [452]
adrs: [118, 117]
spec: 'docs/superpowers/specs/2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti-design.md'
plan: 'docs/superpowers/plans/2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti.md'
results: 'docs/results/2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/two-successors-sharing-one-stale-predecessor-receipt-can-sti'
pr: 'https://github.com/danielhanold/docket/pull/333'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti-design.md) |
| Plan | [2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti.md) |
| Results | [2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-09-25-two-successors-sharing-one-stale-predecessor-receipt-can-sti-results.md) |
| ADRs | [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0117](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0117-sequential-test-drives-within-one-worker-recovery-scope.md) |
<!-- docket:artifacts:end -->

## Why

Change 452 fixed the first-start version of this race (a receipt-less loser rotating a same-scope executing slot out from under the winner). Whole-branch review of that change surfaced a sibling bug on the successor side, from code reading rather than reproduction: `admitScopedWorktree`'s rotation path checks only that the incumbent slot is same-scope and `executing`, not that the presenting start's predecessor receipt still names the scope's CURRENT drive.

When two successor starts present the same (now-stale) predecessor receipt, the first to admit can launch and mark the slot `executing`. The second can still rotate that live slot to its own token, then lose the scope check with `ErrStalePredecessor` — and its post-failure cleanup releases the rotated slot while the first successor's run is still live. A subsequent start can then take the worktree while a gate is still running in it, breaking the one-execution-per-worktree invariant the same way change 452's bug did.

See the write-up in change 452's results file (docs/results/2026-09-24-same-scope-first-start-loser-must-not-rotate-the-winner-s-ex-results.md, "Two successors sharing one stale receipt can still free a live slot") for the full trace.

## What changes

- Guard the successor rotation in `admitScopedWorktree`: before rotating an executing same-scope slot, confirm the presented receipt names the scope's CURRENT drive (the same predicate `reserveScopeDrive` already applies); a stale receipt refuses `ErrStalePredecessor` without touching the slot. It reuses the existing scope record and token-checked rotation, and adds no new mechanism.
- A deterministic two-successor regression test modeled on `TestSameScopeFirstStartLateLoserDoesNotRotate`, mutation-checked against the unguarded code.

## Out of scope

- Any other admission/arbitration path unrelated to receipt staleness on rotation.
- Re-litigating change 452's fix.
- Changing the worktree-first admission order fixed by ADR-0118.

## Reconcile log

### 2026-09-25

2026-09-25 — Re-read against origin/main (de5eb974): admitScopedWorktree still rotates an executing same-scope slot for any receipt-bearing start without checking the receipt names the scope current drive; change 452 fix (receipt-less refusal) is in place. Spec design and test plan still apply unchanged; no scope adjustment.
