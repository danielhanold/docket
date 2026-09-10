---
id: 405
slug: 'investigate-the-gate-drive-prepare-scope-gate-drive-start-ha'
title: 'Investigate the gate.drive.prepare-scope -> gate.drive.start handshake rejecting a build-task worker''s focused gate'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-09-04'
updated: '2026-09-10'
depends_on: [416]
stacked_on:
related: [359, 402, 412, 416]
discovered_from: [402]
adrs: [107, 117]
spec: 'docs/superpowers/specs/2026-09-10-investigate-the-gate-drive-prepare-scope-gate-drive-start-ha-design.md'
plan: 'docs/superpowers/plans/2026-09-10-sequential-test-drives-within-one-worker-recovery-scope.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/investigate-the-gate-drive-prepare-scope-gate-drive-start-ha'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-10T13:04:36Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-10-investigate-the-gate-drive-prepare-scope-gate-drive-start-ha-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-10-investigate-the-gate-drive-prepare-scope-gate-drive-start-ha-design.md) |
| Plan | [2026-09-10-sequential-test-drives-within-one-worker-recovery-scope.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-10-sequential-test-drives-within-one-worker-recovery-scope.md) |
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0117](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0117-sequential-test-drives-within-one-worker-recovery-scope.md) |
<!-- docket:artifacts:end -->

## Why

Workers need several test runs within one task: a baseline, a failing regression, and verification of the fix. Change 416 has fixed the incomplete identity bundle that prevented their first scoped start. A separate limitation remains: a prepared recovery scope permanently binds one drive, so a completed test prevents the worker from starting its next test under the same parent-issued scope.

The generic invalid-request diagnostic hides this distinction. Supporting sequential runs also requires careful result acknowledgement and concurrency control so a parent can recover the current run without mistaking earlier completed tests for live work. The original change-402 reports remain historical symptoms; consumed capabilities and cross-build collisions were hypotheses, not established root causes.

## What changes

Allow one worker recovery scope to carry sequential task-owned test drives, while keeping at most one current execution or launch reservation. Each successor explicitly acknowledges the previous PASSED or FAILED result, keeps its own execution fingerprint and evidence, and preserves the parent's ability to recover current work. Finishing the task acknowledges the last result and closes the scope.

Reject overlapping starts, stale ownership, pending handoffs, HALTED or unresolved predecessors, and identity/capability mismatches with specific safe diagnostics. Serialize start, acknowledgement, and recovery transitions before process launch. Update the worker/controller contracts and their generated copies together.

Add behavioral regression coverage for baseline/RED/GREEN sequences, competing starts, durable failure recovery, and concurrent scopes in distinct worktrees. The linked spec defines the transition contract and acceptance criteria. Dependency 416 is already done; change 412's foreground/yield issue remains separate.

## Out of scope

Redoing change 416's identity-bundle fix; weakening scope identity, fingerprint, capability, or parent/child ownership checks; concurrent test drives inside one worker scope; the background/yield behavior tracked by 412; run-gate attribution or retry redesign; cross-machine recovery; unrelated suite-timing findings; and implementing the change during grooming.

## Reconcile log

### 2026-09-10

2026-09-10: Reconciled against current main (2f83683c) and origin/docket. Dependency 416 is done (archived 2026-09-10-0416-scoped-build-task-gate-starts-omit-prepared-scope-identity.md); its identity-bundle fix and tests are present and preserved. Verified the spec's three source observations still hold in internal/gatedrive: Driver.Start (driver.go:183) rejects any nonempty scope.BoundDriveID before checking whether the bound drive has finished; Store.bindScopeDrive (scope.go:167) permanently rejects a different drive id; ownership.go defines the OwnershipErrorKind vocabulary while app/gate_drive.go mapDriveFailure collapses ownership errors toward generic invalid-request; FindScopeDriveIDs-style enumeration includes terminal records with a remaining owner generation. Scope, relations (depends_on [416], related [359,402,412,416], adrs [107], discovered_from [402]), and acceptance criteria remain valid as authored; no scope adjustment needed. Proceeding to plan and build.
