---
id: 442
slug: 'rebase-again-when-main-advances-after-finalize-publishes'
title: 'Rebase again when main advances after finalize publishes'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-09-22'
updated: '2026-09-22'
depends_on: []
stacked_on:
related: [316, 349, 396, 408, 411, 438]
discovered_from: []
adrs: [10, 105, 112, 113, 118]
spec: 'docs/superpowers/specs/2026-09-22-rebase-again-when-main-advances-after-finalize-publishes-design.md'
plan: 'docs/superpowers/plans/2026-09-22-rebase-again-when-main-advances-after-finalize-publishes.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/rebase-again-when-main-advances-after-finalize-publishes'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-22T04:01:52Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-22-rebase-again-when-main-advances-after-finalize-publishes-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-22-rebase-again-when-main-advances-after-finalize-publishes-design.md) |
| Plan | [2026-09-22-rebase-again-when-main-advances-after-finalize-publishes.md](https://github.com/danielhanold/docket/blob/fix/rebase-again-when-main-advances-after-finalize-publishes/docs/superpowers/plans/2026-09-22-rebase-again-when-main-advances-after-finalize-publishes.md) |
| ADRs | [ADR-0010](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0010-finalize-merge-gate-split-agents.md), [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md), [ADR-0112](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0112-a-completed-gate-publish-checkpoint-is-persisted-in-the-owne.md), [ADR-0113](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0113-resolver-dispatches-are-admitted-by-durable-pre-dispatch-res.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Change 0438 handles main advancing before finalize pushes its rebased head. When main advances after that push but before merge, re-entering finalize still checks the remote feature branch against its pre-push lease and refuses, even when the remote contains Docket's own tested result. Source inspection confirms this gap; the separate session's reproduction was not independently replayed during grooming.

## What changes

Extend the existing rebase refresh to recognize a published result proven by the owned test checkpoint and matching local, remote, and PR heads. Refresh the existing receipt with that exact remote lease and a fresh rewrite token, preserve prior conflict resolutions and consumed resolver budget, and run the configured finalize suite against the newer base. Reuse existing interruption, publication, and merge machinery. Add focused behavioral coverage and update finalize guidance.

## Out of scope

Checkpoint-less or ambiguous published heads; arbitrary remote edits; force-rewritten bases; rollback/reset; new commands, fields, stores, configuration, retry policies, or budget replenishment; merge-race detection changes; unrelated cleanup; implementation or implementation planning during grooming.

## Reconcile log

### 2026-09-22

2026-09-22: Reconciled at claim. Integration branch main is at 3ab594194d4ba7887a9384d5f6a0855c7ba0c31c — the exact commit the spec was groomed against — so the source trace holds without drift. Confirmed the named finalize internals are present and unchanged: internal/app/finalize_rebase.go (recoverFromReceipt, refreshOwnedRewrite, publishCheckpointOf, checkpointDecision) and internal/workspace/rewrite.go / rebasereceipt.go (OrigHead, OrigRemoteHead, BaseHead, Attempt, checkpoint). Cited ADRs (10, 105, 112, 113, 118) and related changes (316, 349, 396, 408, 411, 438) remain accurate. No scope change; the bounded published-case admission extension in refreshOwnedRewrite/recoverFromReceipt stands as designed. Proceeding to plan and build.
