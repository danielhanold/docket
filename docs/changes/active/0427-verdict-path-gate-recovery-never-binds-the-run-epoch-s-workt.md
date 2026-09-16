---
id: 427
slug: 'verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt'
title: 'Verdict-path gate recovery never binds the run epoch''s worktree'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-09-14'
updated: '2026-09-16'
depends_on: []
stacked_on:
related: [375, 422, 428]
discovered_from: [375]
adrs: [107, 118]
spec: 'docs/superpowers/specs/2026-09-16-verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-16T10:32:06Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-16-verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-16-verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt-design.md) |
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Change 0375 (ADR-0107/ADR-0118) binds a run epoch's worktree at claim confirmation so the mutation fence and `run.cancel` teardown have something to act on. `internal/app/rungate_verdict.go` has two call sites that recover/confirm a gate claim through the verdict path, and both pass an empty worktree to `ConfirmGateClaim`, so a run recovered solely through that path never gets its epoch's `Worktree` field populated. `run.cancel` against such an epoch would report `cancelled` while the mutation fence stays inert, silently defeating the exact safety property 0375 built. The common first-dispatch claim path already binds the worktree correctly and is unaffected; this is scoped to the verdict-path recovery legs only.

## What changes

Bind the intended feature worktree in both verdict-path claim-recovery legs using the same repository-root and change-slug derivation as normal claim confirmation. Keep recovery valid before the workspace exists, and refuse unresolved identity instead of confirming with an empty path. Add focused regressions for both paths that prove epoch binding and post-cancel mutation refusal, plus a mutation check of each repaired call. The linked spec defines the narrow fix.

## Out of scope

New commands, configuration, schemas, persistence, or recovery subsystems; changes to cancellation, resume, retry, or admission policy; repairs to already-confirmed historical bindings or unrelated partial-write windows; unrelated refactoring; and implementation during grooming. ADR-0107 and ADR-0118 remain unchanged.
