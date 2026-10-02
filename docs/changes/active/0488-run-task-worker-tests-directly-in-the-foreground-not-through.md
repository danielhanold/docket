---
id: 488
slug: 'run-task-worker-tests-directly-in-the-foreground-not-through'
title: 'Run task-worker tests directly in the foreground, not through gate drives'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: []
stacked_on:
related: [333, 359, 405, 412, 416, 459, 479, 486]
discovered_from: []
adrs: [107, 117]
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
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0117](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0117-sequential-test-drives-within-one-worker-recovery-scope.md) |
<!-- docket:artifacts:end -->

## Why

Since change 0359 (2026-09-02, ADR-0107), every test a build-task worker runs goes through the gate driver. That covers baseline, RED, GREEN, and even `gofmt -l`. Each one is `gate.drive.start --owner task` inside a recovery scope. The worker passes the 8-value identity bundle from `gate.drive.prepare-scope`, and from the second test on also a predecessor receipt: the drive id and owner generation captured from the previous drive's JSON. It closes the task with `gate.drive.acknowledge`.

A start can be refused about 25 ways. Many of them leave the scope permanently unusable:

- a HALTED drive leads to `predecessor-not-reusable`;
- a failed launch leaves the slot reserved, so every later start is `scope-busy`.

The worker then returns BLOCKED, the build halts, and a human has to run `change.resume-halted`.

This is now the main cause of stuck builds. The metadata branch has 27 `run halted` reports since 2026-09-09. Roughly two-thirds trace to gate ownership machinery, not red tests.

A recent Cursor build worker needed about 19 steps to run one task's focused checks:

- It re-ran the start with cwd set, because the scope compares the worktree path as a raw string and `--repo-dir` defaults to cwd (`scope-identity-mismatch`).
- It re-ran the start without a predecessor. Inside the scope that is `scope-second-live-drive`. Outside it, the slot is stamped with a run id the worker never receives, so it is `stale-run-id`.
- It ran `gate.history.cleanup`, a dead end: history only matters the first time a worktree starts a gate.
- It grepped docket's source and the git common dir for drive records, because no read-only operation shows a scope's current drive.

For focused tests this machinery buys almost nothing:

- Nothing downstream reads task drives. docket-build accepts COMPLETE by commit ancestry, and `evidence.record` accepts only the full-suite command.
- Nothing enforces it except skill prose.
- The full-suite build gate re-runs everything anyway.

The motivating incident (0333) was a full-package `go test -race ./internal/app` run as if it were a focused test. The fix tracked every test instead of limiting what a worker may run; 0479 now limits that exact command.

The simpler path worked. The original 0405 stub recorded workers that "correctly fell back to running their fast, sub-second focused test … directly in the foreground, so no build was harmed." The 0359 spec ruled that path out ("No merged intermediate may allow both direct test execution and driver execution as supported workflow paths") without weighing it.

`references/fix-loop.md` also dispatches docket-build-task fix workers without preparing a scope. Under the current contract, their tests are either refused or run outside the contract.

## What changes

This is a contract and prose change only. Deleting the Go code is the dependent follow-up change.

- **docket-build-task.** Workers run baseline, RED, GREEN, focused, and ad-hoc tests directly in the foreground under a hard wall-clock cap, for example `timeout 300 <cmd>`. They never background a test and never yield. A command that hits the cap is not a focused test: narrow it, or return BLOCKED naming the command. Remove the scope bundle, predecessor receipts, `gate.drive.acknowledge`, task-level WAITING/handoff, and the continuation-with-fresh-scope rules. `WAITING` leaves the worker's outcome list.
- **docket-build.** Stop running `gate.drive.prepare-scope` before each task. Remove the controller's handling of task WAITING, claim, and takeover. The build-owned full-suite gate (`--owner build`, start/advance) does not change.
- **The rest of the prose:** `docket-implement-next` (fix-loop, edge-paths), `references/gate-caller-loop.md`, `references/gate-execution.md`, the convention, and the generated dispatch blocks all match the new contract. Integration-repair's build-owned post-fix re-run of the full suite stays on the driver.
- **ADR.** Record an ADR that supersedes ADR-0117 and narrows ADR-0107 to build-owned and finalize drives.
- **Guards.** Update the repository guards that pin the old prose (capability surface, feature-dispatch block) so they still bite on the new contract. Do not delete them.

Grooming must settle:

- the cap value, and whether it is configurable;
- whether package-wide runs need an explicit rule;
- how AGENTS.md's "mutation-test every guard" rule reads once no drive is involved (this makes 0486 moot).

Accepted loss: a parent can no longer adopt a focused test that is still running after its worker died. Also, no durable record shows that a worker ran its tests; nothing ever read that record.

## Out of scope

- Deleting the Go machinery: `--owner task`, scopes, predecessor receipts, acknowledge, and task takeover. The dependent change removes it once no caller uses it.
- The build-owned full-suite gate and its evidence path.
- Finalize's gate.
- The worktree admission slot and run-id fencing, which have their own changes.
- The run tracker's attribution and retry accounting.
