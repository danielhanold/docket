---
id: 438
slug: 'finalize-rebase-abort-can-t-recover-a-completed-but-unmerged'
title: 'finalize.rebase-abort can''t recover a completed-but-unmerged rebase whose base later moved'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-09-19'
updated: '2026-09-20'
depends_on: []
stacked_on:
related: [291, 309, 316, 349, 368, 396, 408, 411, 439]
discovered_from: []
adrs: [10, 105, 112, 113, 118]
spec: 'docs/superpowers/specs/2026-09-20-finalize-rebase-abort-can-t-recover-a-completed-but-unmerged-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/finalize-rebase-abort-can-t-recover-a-completed-but-unmerged'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-20T14:53:06Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-20-finalize-rebase-abort-can-t-recover-a-completed-but-unmerged-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-20-finalize-rebase-abort-can-t-recover-a-completed-but-unmerged-design.md) |
| ADRs | [ADR-0010](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0010-finalize-merge-gate-split-agents.md), [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md), [ADR-0112](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0112-a-completed-gate-publish-checkpoint-is-persisted-in-the-owne.md), [ADR-0113](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0113-resolver-dispatches-are-admitted-by-durable-pre-dispatch-res.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

An interrupted finalize of change 0368 completed its local rebase but stopped before publication. After main advanced, the next finalize refused the stale recorded base, and attempting to abort failed because Git no longer had a rebase in progress. Source inspection confirms both paths. The original incident was not independently replayed during grooming.

The initial proposal assumed recovery needed to reset the branch. The design review rejected that assumption: Git can rebase the current branch onto the newer base, preserving earlier conflict resolutions. Docket needs to refresh the owned rewrite and its test evidence safely through the existing finalize operation. Its receipt already separates the local starting head from the remote publication lease.

A related source-confirmed defect makes repeated finalize.block and empty finalize.clear-block requests return invalid-input: their zero-valued plans fail transaction validation before reaching the engine's existing no-op path.

## What changes

- Extend finalize.rebase so a completed, clean, owned unpublished rewrite can advance onto the newer effective base from its current head, preserving prior resolutions and committed work.
- Refresh the existing receipt and owned anchors with interruption-safe re-entry, preserving the exact remote publication lease and consumed resolver budget. Retain unknown, foreign, active, or conflicting ownership state.
- Invalidate old-base evidence and run the configured finalize suite on the updated code; retain existing continuation and valid checkpoint reuse for the current target.
- Repair block and clear-block no-op plans at their producers without relaxing the transaction engine.
- Update finalize guidance and add behavioral coverage for forward recovery, interruptions, refusals, retesting, and real transaction no-ops.

The linked spec records the implementation trace, prior changes and architecture decisions, alternatives, and acceptance criteria. The historical title describes the incident; forward rebasing is the agreed solution.

## Out of scope

Completed-rebase rollback or a completion-head receipt field; new commands, configuration, lifecycle states, recovery stores, retry layers, cancellation authority, or resolver-budget replenishment; transaction-engine relaxation; generalized scratch-cleanup repair; recovery across a force-rewritten base or after publication of the local rewrite; repairing the already-closed change 0368 incident. No implementation or implementation plan during grooming.

## Reconcile log

### 2026-09-20

2026-09-20: Reconciled against main@57794104. Source anchors in the spec verified present and unchanged: internal/app/finalize_rebase.go recoverFromReceipt still refuses on `rec.BaseHead != string(baseHead)` (the forward-progress gap), the RebaseReceipt still distinguishes OrigHead/OrigRemoteHead/BaseHead/Attempt/checkpoint fields, and finalizeBlockOp.Plan / finalizeClearBlockOp.Plan still return zero-valued no-op plans that fail validatePlan before the empty-Files no-op path. Related change 0439 has merged; no dependency or scope change results. depends_on remains empty, ADRs (0010, 0105, 0112, 0113, 0118) remain accurate. No re-scoping; proceeding to plan and build per the agreed forward-rebase design.
