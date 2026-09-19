---
id: 435
slug: 'docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga'
title: 'docket run cancel leaves a stale RunEpochID on a released gate-admission slot'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-09-18'
updated: '2026-09-19'
depends_on: [437]
stacked_on:
related: [413, 427, 375, 368, 437]
discovered_from: [434]
adrs: []
spec: 'docs/superpowers/specs/2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-19T18:20:41Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md) |
<!-- docket:artifacts:end -->

## Why

Cancellation can leave a released worktree admission slot carrying the cancelled epoch's RunEpochID, so a legitimate replacement or standalone finalize gate is refused. Ordinary release deliberately retains that field to protect live runs between drives; cancellation needs a separate, ownership-checked retirement step after complete accounting. Repeating cancel currently returns already-cancelled without repairing an existing stale slot.

This blocked finalize of change 434 and build/resume of change 368. The 368 workaround of supplying the cancelled predecessor epoch exposed a separate admission defect, assigned to change 437. It is not a supported recovery technique. The review also reproduced foreign-slot teardown and found that the existing cancel fixture used an epoch-less raw slot, missing the owning-epoch case.

## What changes

Retire only the cancelled epoch's released slot after 437 proves all pending launches and replacement processes settled and all task/process/mutation accounting completes, using the existing locks and atomic writers. Preserve ordinary release semantics, protect foreign/successor slots before every teardown mutation, propagate write errors, and make authorized terminal-cancel retries repair historical stale released slots. Define safe retry after interruption between slot retirement and epoch completion. Unsafe terminal repair returns refused without reviving the epoch; resume must independently validate the same quiescence proof before authorizing a replacement. The linked revised spec supplies the state and acceptance contract.

## Implementation order

**Implement and merge 437 first; implement 435 second.** The hard dependency is `depends_on: [437]`, satisfied only when 437 is done. Use sequential PRs against integration, not parallel or stacked implementation. Change 437 fences starts, delayed launch tickets, and automatic/recovered relaunches, and makes pending launches visible to cancellation; only then does 435 make released slots reusable. Completing 437 alone intentionally does not fix stale-slot refusal. Do not bypass it with an old epoch or a durable-record hand edit.

## Out of scope

Launch fencing and pending-launch accounting belong exclusively to 437. This change reuses that accounting for cancellation retirement, historical terminal repair, and the corresponding resume check. Do not weaken the state-independent epoch mismatch fences, change ordinary ReleaseWorktreeExecution or takeover semantics, extend raw gate recover, or redesign mutation-owner lookup. No new daemon, background recovery loop, persistent store, schema, lifecycle state, configuration, CLI command, generic coordination framework, or retry layer. If the existing lock-and-replay model cannot satisfy the contract, report the specific design conflict instead of expanding scope during implementation.
