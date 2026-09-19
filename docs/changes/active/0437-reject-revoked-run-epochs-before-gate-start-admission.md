---
id: 437
slug: 'reject-revoked-run-epochs-before-gate-start-admission'
title: 'Reject revoked run epochs before gate-start admission'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-09-19'
updated: '2026-09-19'
depends_on: []
stacked_on:
related: [435, 375, 368, 427]
discovered_from: [435]
adrs: [118]
spec: 'docs/superpowers/specs/2026-09-19-reject-revoked-run-epochs-before-gate-start-admission-design.md'
plan: 'docs/superpowers/plans/2026-09-19-reject-revoked-run-epochs-before-gate-start-admission.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/reject-revoked-run-epochs-before-gate-start-admission'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-19T11:40:51Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-19-reject-revoked-run-epochs-before-gate-start-admission-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-19-reject-revoked-run-epochs-before-gate-start-admission-design.md) |
| Plan | [2026-09-19-reject-revoked-run-epochs-before-gate-start-admission.md](https://github.com/danielhanold/docket/blob/fix/reject-revoked-run-epochs-before-gate-start-admission/docs/superpowers/plans/2026-09-19-reject-revoked-run-epochs-before-gate-start-admission.md) |
| ADRs | [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Review of change 435 found that gate-start admission compares the requested RunEpochID with the slot owner but does not establish that the requested epoch is active. In change 368, presenting the cancelled predecessor epoch bypassed the stale-slot mismatch. An isolated diagnostic on main ab9216d2 reproduced a scoped start reaching WAITING even with a revocation resolver that always returned true; the resolver was called zero times. This is distinct from 435's failure to retire stale ownership.

## What changes

Fence fresh/successor admission, delayed StartAdmitted tickets, and automatic/recovered relaunches against the existing epoch registry. Reuse existing epoch, admission, scope/drive records and the per-drive launch claimant lock; give sequential successors distinct execution reservations. Make pending launch obligations and current replacement processes visible to cancellation, so it cannot finish while a process can still appear. Keep genuinely epoch-less standalone behavior unchanged. Invalid epochs refuse at admission before a suite charge; later cancellation can refuse an already-charged delayed launch without refund or reset. Implement and merge this change FIRST, then implement 435's cancellation cleanup repair. The order closes old-epoch reacquisition before 435 makes released worktrees reusable; this change itself does not clear stale slots.

## Out of scope

435 owns cancellation-specific detachment, terminal-cancel repair and its matching resume check, and the existing slot teardown ownership/error audit. This change owns launch fencing and the narrow pending-launch accounting those paths require. No new daemon, background loop, persistent store, schema, lifecycle state, CLI command, configuration, generic coordination framework, or redesign of run attribution/mutation ownership. Preserve takeover and ordinary release semantics. Automatic and recovered relaunches of epoch-backed gate drives are in scope; changing relaunch policy, standalone raw recovery, or normal successful-run epoch retirement is not. No additional retry layer or new ownership protocol.

## Reconcile log

### 2026-09-19

2026-09-19: Reconciled against current main. Confirmed the design still matches reality: the EpochRevokedFunc resolver is wired at all production gate-drive constructors (build/finalize/task/commandless-recovery/continuation via newOwnedGateDriveService, NewTaskGateDriveService, NewCommandlessGateDriveService, rungate_continuation) but is consulted ONLY on the takeover path (gatedrive/takeover.go), never in Driver.Admit, Driver.StartAdmitted, or the Advance automatic-relaunch path (gatedrive/driver.go) — precisely the defect 437 fences. Related changes 375, 427, 368 are all merged (archived); ADR-0118 is Accepted and its invariants stand. Downstream change 435 remains proposed with depends_on: [437]; implementation order (437 first) is unchanged and 437 carries no dependency on 435. No scope adjustment: 437 owns launch fencing (Admit/StartAdmitted/relaunch/recovery liveness validation, successor execution reservations, and the narrow pending-launch accounting cancellation needs) while 435 retains stale-slot retirement and terminal-cancel repair. No new machinery required beyond the existing epoch registry, epochCAS/epoch lock, per-drive relaunchClaim, worktree admission slot, and scope/drive reservation records.
