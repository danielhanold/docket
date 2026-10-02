---
id: 489
slug: 'delete-the-task-owned-gate-drive-machinery-once-no-skill-dri'
title: 'Delete the task-owned gate-drive machinery once no skill drives task tests'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: [488]
stacked_on:
related: [359, 405, 416, 452, 453, 457, 459, 467]
discovered_from: []
adrs: [107, 117, 120, 125]
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
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0117](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0117-sequential-test-drives-within-one-worker-recovery-scope.md), [ADR-0120](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0120-historical-gate-drive-schemas-are-assessed-never-executed.md), [ADR-0125](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0125-historical-gate-discovery-has-no-global-veto-relevance-to-th.md) |
<!-- docket:artifacts:end -->

## Why

Once change 0488 lands, no skill calls any of the following:

- `--owner task`;
- `gate.drive.prepare-scope` for a task;
- predecessor receipts;
- `gate.drive.acknowledge`;
- takeover of a task drive.

The code behind them stays in place, though:

- the slot lifecycle, predecessor receipts and pending-ack journal in `internal/gatedrive/scope.go`;
- `acknowledge.go`;
- the scoped paths in `driver.go` and `admission.go` (`admitScoped`, `rotateWorktreeExecutionForSuccessor`, `siblingMayHoldReservation`);
- the seams in `internal/app/gate_drive.go` and `internal/cli/gate.go`.

That is roughly 2,000 production lines and 5–6k test lines.

That code is where the recent fix chain lived: 0405, 0416, 0452, 0453, 0459, 0467. Open change 0457 is still there. Dead code with live concurrency bugs is a liability. It keeps the admission paths and lock order (worktree admission → scope → drive) complex for the owners that remain. It also forces anyone reading a refusal to reason through states no caller can reach any more.

## What changes

- **Remove the task path.** Delete the task-intent owner and its start flags (`--scope-id`, `--child-cap`, `--predecessor-drive-id`, `--predecessor-owner-gen`), task-scope reservation and rotation, the pending-ack journal, `gate.drive.acknowledge`, and takeover of task drives. Their tests and capability-catalog entries go with them.
- **Keep what other gates use.** Keep everything the build-owned and finalize gates still need. Grooming must trace whether the run tracker's outer scope (prepare-scope at `run.start`, the parent capability, takeover via `run.continue`) still needs scope code. If it does, keep the minimum; if not, remove scopes entirely.
- **Old records on disk.** Existing `gate-scopes/v2` records must not block admission once the code is gone. Use the assess-and-ignore posture from ADR-0120/0125. Any schema bump follows the stores' fail-closed rules.
- **Close moot work.** Close 0457 if the code it fixes is gone.
- **Verify.** Run the whole suite, and re-budget suite runtime rows that shrink.

Accepted loss: none beyond 0488's. This is removal of unreachable code.

## Out of scope

- The prose contract change (dependency 0488).
- Redesigning the worktree admission slot, and run-id fencing (follow-up changes).
- Build-owned drive mechanics: start/advance, owner generation, handoff/claim of build drives, fingerprinting, idempotent relaunch, and the suite-attempt budget.
