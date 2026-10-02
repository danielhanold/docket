---
id: 488
slug: 'run-task-worker-tests-directly-in-the-foreground-not-through'
title: 'Run task-worker tests directly in the foreground, not through gate drives'
status: 'in-progress'
priority: 'critical'
type: 'fix'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: []
stacked_on:
related: [333, 359, 405, 412, 416, 459, 467, 479, 486, 491]
discovered_from: []
adrs: [24, 107, 117, 130]
spec: 'docs/superpowers/specs/2026-10-02-run-task-worker-tests-directly-in-the-foreground-not-through-design.md'
plan: 'docs/superpowers/plans/2026-10-02-run-task-worker-tests-directly-in-the-foreground-not-through.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/run-task-worker-tests-directly-in-the-foreground-not-through'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-02T07:26:37Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-02-run-task-worker-tests-directly-in-the-foreground-not-through-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-02-run-task-worker-tests-directly-in-the-foreground-not-through-design.md) |
| Plan | [2026-10-02-run-task-worker-tests-directly-in-the-foreground-not-through.md](https://github.com/danielhanold/docket/blob/fix/run-task-worker-tests-directly-in-the-foreground-not-through/docs/superpowers/plans/2026-10-02-run-task-worker-tests-directly-in-the-foreground-not-through.md) |
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0117](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0117-sequential-test-drives-within-one-worker-recovery-scope.md), [ADR-0130](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0130-build-task-workers-run-focused-tests-directly-under-a-fixed.md) |
<!-- docket:artifacts:end -->

## Why

Since change 0359, every test a build-task worker runs goes through `gate.drive.start --owner task` inside a recovery scope: an identity bundle, predecessor receipts, `gate.drive.acknowledge`, and a handoff on the first `WAITING`.

That protocol is now the main cause of stuck builds:

- A start can be refused about 25 ways, and several leave the scope unusable until a human resumes the run.
- Roughly two-thirds of the 27 run halts since 2026-09-09 trace to gate ownership machinery rather than red tests.

For focused tests it buys almost nothing:

- nothing reads a task drive;
- nothing enforces it beyond skill prose;
- the full-suite build gate re-runs everything anyway.

The incident behind it was a four-minute package run treated as a focused test, and a time limit on the command is enough to handle that. Fix-loop workers were also being dispatched with no scope at all, so their tests were refused or ran outside the contract.

## What changes

- **Workers run their own tests directly.** Build-task workers run every test in the feature worktree, in the foreground, as `timeout --kill-after=10s 10m <test command>`. `gtimeout` is the fallback; if neither exists, the worker returns `BLOCKED`. The exit status says whether the run was green, red, or hit the limit. Workers never background a test, never run the full suite, and call no gate operation. Their outcomes become `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`.
- **The controller runs every full-suite attempt.** The build controller stops preparing scopes and drops task-level `WAITING`, claim, and takeover handling. It runs every full-suite attempt itself, including the one after a repair worker's fix.
- **A non-blocking time-limit audit.** The controller checks each worker's reported test commands for the `timeout` wrapper. A missing wrapper becomes an informational line in the results file. It is never a halt and never makes a return malformed.
- **The run id leaves every worker prompt.** Implement-next keeps it on its own build-gate starts until 0491 retires it entirely, because today's `run.cancel` depends on it.
- **Prose and docs.** Every prose site that restates the old contract is updated, along with the glossary and the install prerequisites (GNU coreutils). The embedded skill mirror is regenerated.
- **Guards.**
  - A new absence guard forbids task-drive instructions anywhere in maintained workflow markdown.
  - Sentinels pin the 10-minute `timeout` rule and the audit's never-a-halt clause.
  - Guards whose subject is gone are retired; the rest are narrowed.
- **ADR.** A new ADR supersedes ADR-0117 and narrows ADR-0107 to the boundary between the coordinator and implement-next.

## Out of scope

- Deleting the task-owned drive Go machinery (change 0489).
- Replacing the worktree admission slot (change 0490).
- Retiring the run id entirely, along with its gate fence (change 0491, per the human's direction at this groom).
- Changing the build-owned full-suite gate, evidence, finalize's gate, or the run tracker's attribution and retry model.
- The controller-side background-and-yield cases in change 0412.

## Reconcile log

### 2026-10-02

### 2026-10-02

Reconciled against origin/main at 97cdefec7 and the metadata branch. Change 0486 was killed as superseded by this change (its mutation-check trap disappears once workers run tests without a drive), so this change also carries 0486's intent. Change 0487 merged; it touched only test sharding and the retired-vocabulary seal scan, not the worker contract. Follow-ups 0489, 0490, and 0491 remain proposed and out of scope as stated. Every file the spec names (the docket-build-task and docket-build skills, gate-caller-loop.md, gate-execution.md, fix-loop.md, and the repoguard tests it retires or narrows) still exists in its described shape. No scope change; relations unchanged.
