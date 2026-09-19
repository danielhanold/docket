---
id: 437
slug: 'reject-revoked-run-epochs-before-gate-start-admission'
title: 'Reject revoked run epochs before gate-start admission'
status: 'proposed'
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
| Spec | [2026-09-19-reject-revoked-run-epochs-before-gate-start-admission-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-19-reject-revoked-run-epochs-before-gate-start-admission-design.md) |
| ADRs | [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Review of change 435 found that gate-start admission compares the requested RunEpochID with the slot owner but does not establish that the requested epoch is active. In change 368, presenting the cancelled predecessor epoch bypassed the stale-slot mismatch. An isolated diagnostic on main ab9216d2 reproduced a scoped start reaching WAITING even with a revocation resolver that always returned true; the resolver was called zero times. This is distinct from 435's failure to retire stale ownership.

## What changes

Fence fresh and successor gate starts carrying an explicit run epoch against the existing epoch registry, including the race with cancellation. Reuse the existing epoch and worktree admission locks and records; keep epoch-less standalone gates unchanged. Refuse revoked, missing, ambiguous, unreadable, or wrongly bound explicit epochs before reserving a suite attempt or launching a process. Implement and merge this change FIRST, then implement 435's cancellation cleanup repair. The order closes old-epoch reacquisition before 435 makes released worktrees reusable; this change itself does not clear stale slots.

## Out of scope

435 owns cancellation-specific detachment, terminal-cancel replay repair, foreign-slot teardown protection, and cleanup write-error handling. No new daemon, background loop, persistent store, schema, lifecycle state, CLI command, configuration, generic coordination framework, or redesign of run attribution/mutation ownership. Preserve takeover and ordinary release semantics. This does not certify every existing relaunch/recovery path.
