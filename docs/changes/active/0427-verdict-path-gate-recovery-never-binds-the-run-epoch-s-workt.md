---
id: 427
slug: 'verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt'
title: 'Verdict-path gate recovery never binds the run epoch''s worktree'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-14'
updated: '2026-09-14'
depends_on: []
stacked_on:
related: [375]
discovered_from: [375]
adrs: [107, 118]
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
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Change 0375 (ADR-0107/ADR-0118) binds a run epoch's worktree at claim confirmation so the mutation fence and `run.cancel` teardown have something to act on. `internal/app/rungate_verdict.go` has two call sites that recover/confirm a gate claim through the verdict path, and both pass an empty worktree to `ConfirmGateClaim`, so a run recovered solely through that path never gets its epoch's `Worktree` field populated. `run.cancel` against such an epoch would report `cancelled` while the mutation fence stays inert, silently defeating the exact safety property 0375 built. The common first-dispatch claim path already binds the worktree correctly and is unaffected; this is scoped to the verdict-path recovery legs only.

## What changes

In `internal/app/rungate_verdict.go`, thread the resolved canonical worktree into both `ConfirmGateClaim` call sites on the verdict-recovery path, mirroring how the first-dispatch claim path binds `EpochRecord.Worktree`. Add a regression test that recovers a claim solely through the verdict path and asserts the epoch's worktree is bound, plus a mutation test that reverts the fix and confirms `run.cancel`'s fence goes inert for that path.

## Out of scope

Any other part of 0375's contract (run-epoch registry, mutation fencing itself, resume/cancel semantics) — those are already correct and covered. Not re-opening ADR-0107 or ADR-0118; this is a narrow bug-fix follow-on discovered during 0375's review, not a design change.
