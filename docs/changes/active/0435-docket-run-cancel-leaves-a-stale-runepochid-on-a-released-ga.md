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
plan: 'docs/superpowers/plans/2026-09-19-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga.md'
results: 'docs/results/2026-09-19-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-19T18:45:29Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md) |
| Plan | [2026-09-19-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga.md](https://github.com/danielhanold/docket/blob/fix/docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga/docs/superpowers/plans/2026-09-19-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga.md) |
| Results | [2026-09-19-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-results.md](https://github.com/danielhanold/docket/blob/fix/docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga/docs/results/2026-09-19-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-results.md) |
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

## Reconcile log

### 2026-09-19

2026-09-19 — Reconciled against current main (501acffd). Dependency 437 (reject-revoked-run-epochs-before-gate-start-admission) is merged and done, so 435 builds against integration as the spec requires. Verified the spec's named symbols exist unchanged on main: runCancel/reconcileEpochTeardown/reconcileWorktreeSlot (internal/app/rungate_cancel.go), reserveWorktreeExecution + the RunEpochID mismatch fence and ReleaseWorktreeExecution which retains RunEpochID (internal/gatedrive/admission.go), rawStaleEpochRefusal (internal/app/gate.go), and 437's launch accounting: Driver.ReconcileEpochLaunches/EpochLaunchReport (internal/gatedrive/reconcile.go) and the EpochLaunchGate/epochGated fence (internal/gatedrive/driver.go). The new epoch-retirement store operation lands as one more admissionCAS mutate closure in admission.go beside ReleaseWorktreeExecution, clearing only RunEpochID on a released slot for the expected epoch while preserving DriveID/RawRunID/RawRunDir/ExecutionGen. Scope holds: authorized run.cancel completion retires ownership after complete accounting; per the spec the death guardian performs teardown but never retires ownership (it leaves the epoch cancelling), so retirement is invoked only on the authorized-cancel path and the matching terminal-repair/resume quiescence check, not from guardianFenceAndReap. The epoch-less raw-slot cancel fixture (newCancelFixture in rungate_cancel_test.go) must be updated to record a real RunEpochID so the owning-epoch retirement case is actually exercised. No design change, no scope expansion; ADR-0118 remains the governing contract. reconciled=true.
