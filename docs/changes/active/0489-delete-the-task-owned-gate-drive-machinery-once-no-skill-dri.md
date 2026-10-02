---
id: 489
slug: 'delete-the-task-owned-gate-drive-machinery-once-no-skill-dri'
title: 'Delete the task-owned gate-drive machinery; the outer takeover recovers only live drives'
status: 'in-progress'
priority: 'critical'
type: 'refactor'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: [488]
stacked_on:
related: [359, 405, 416, 452, 453, 457, 459, 467, 490, 491]
discovered_from: []
adrs: [107, 117, 120, 125, 130]
spec: 'docs/superpowers/specs/2026-10-02-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri-design.md'
plan: 'docs/superpowers/plans/2026-10-02-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/delete-the-task-owned-gate-drive-machinery-once-no-skill-dri'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-02T10:37:09Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-02-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-02-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri-design.md) |
| Plan | [2026-10-02-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri.md](https://github.com/danielhanold/docket/blob/refactor/delete-the-task-owned-gate-drive-machinery-once-no-skill-dri/docs/superpowers/plans/2026-10-02-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri.md) |
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0117](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0117-sequential-test-drives-within-one-worker-recovery-scope.md), [ADR-0120](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0120-historical-gate-drive-schemas-are-assessed-never-executed.md), [ADR-0125](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0125-historical-gate-discovery-has-no-global-veto-relevance-to-th.md), [ADR-0130](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0130-build-task-workers-run-focused-tests-directly-under-a-fixed.md) |
<!-- docket:artifacts:end -->

## Why

Change 0488 moved build-task workers off the gate driver. ADR-0130 left the task-drive Go machinery "in place but unused until change 0489 deletes it". No workflow calls any of these any more:

- `--owner task`;
- scoped starts with predecessor receipts;
- `gate.drive.prepare-scope`, `gate.drive.acknowledge`, and `gate.drive.takeover`.

The code behind them is about 2,000 production lines and 6,850 test lines. It includes:

- the scope slot lifecycle and pending-ack journal (`scope.go`, `acknowledge.go`);
- the scoped admission paths (`admitScoped`, `rotateWorktreeExecutionForSuccessor`, `siblingMayHoldReservation`);
- the task-intent owner and its CLI surface.

That code is where the fix chain 0405 → 0416 → 0452 → 0453 → 0459 → 0467 lived, and open change 0457 still targets it. Dead code with live concurrency bugs is a liability. It keeps admission and the lock order complex for the owners that remain.

Grooming also traced a live defect in the part that stays. The run tracker keeps one recovery scope per run. `run.start` prepares it, and `run.verdict` uses it to turn a `run-incomplete` into `run-continue` by taking over a drive that a dead implement-next left behind. The candidate rule counts a finished drive as recoverable until something marks it consumed, and only the task path ever did that.

Since 0488's review fix made build-owned starts carry `--change-id`, every finished build gate stays a candidate for the rest of the run. So an implement-next that dies after a red-then-green gate, or after committing past its last gate, now ends in a terminal `run-stop … takeover-ambiguous` or a fingerprint halt instead of a retry.

## What changes

- **Delete the task path:**
  - the three operations, with their catalog, schema, and install-map entries;
  - `gate.drive.start`'s `--owner task` and its four scope flags;
  - the task-intent owner;
  - scoped starts and slot rotation;
  - the scope's per-test lifecycle and terminal acknowledgement;
  - the task-only branches in shared code, including `Takeover`'s run-revocation check (the outer scope never carries a run id) and the cancel census's scope walk;
  - their tests.
- **Keep a minimal outer scope, in-process.** `run.start` prepares it, `run.verdict` binds and takes over through it, and `run.cancel` reads its worktree. The record keeps repo, change, branch, worktree, the two capability hashes, and the closed flag.
- **The outer takeover recovers only a still-running drive.** A finished drive is never a candidate. A run that dies mid-gate still continues; a run that dies after its gate finished gets `run-retry-once`.
- **Old records stay unread.** There is no schema bump, no reset, and no migration.
  - Outer scopes keep working across the upgrade.
  - Task-scope and worker-drive files are never opened again.
  - Legacy `scoped` slots still settle.
- **0457** was killed at this groom as superseded.
- **Docs and decision record:**
  - update the glossary, the Codex dispatch clause, and comments in kept code;
  - record one new ADR for the takeover rule (`relates_to: [107, 130]`).
- **Verify:** run the whole suite, and re-budget the runtime rows that shrink.

Accepted loss: a gate verdict that finished before implement-next died is not reused. The retry re-runs the suite, which costs one build attempt and the run's retry.

## Out of scope

- The worker prose contract (0488, done).
- Replacing the worktree admission slot (0490) and retiring run-id fencing (0491), apart from the takeover revocation check, which is unreachable today.
- Build-owned drive mechanics: start/advance, owner generation, handoff/claim, fingerprinting, the idempotent relaunch, and the suite-attempt budget.
- The run tracker's attribution and retry model, `run.continue`, and the run-context claim binding.
- The `--task-id`/`--phase` recording flags.
- Deleting old record files from disk.

## Reconcile log

### 2026-10-02

Reconciled against origin/main 23a2cc370 (change 0488 merged, including 9acf686fa which made build-owned starts carry --change-id). The spec was groomed today against this same tip; the task-owned machinery it targets is still present and unreferenced by any skill, FindScopeDriveIDs still carries the terminal-unconsumed clause, and 0457 is already killed. No scope adjustment needed; depends_on 488 is done.
