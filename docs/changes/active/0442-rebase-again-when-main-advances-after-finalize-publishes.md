---
id: 442
slug: 'rebase-again-when-main-advances-after-finalize-publishes'
title: 'Rebase again when main advances after finalize publishes'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-22'
updated: '2026-09-22'
depends_on: []
stacked_on:
related: [438, 408]
discovered_from: []
adrs: [105, 112, 113]
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
| ADRs | [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md), [ADR-0112](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0112-a-completed-gate-publish-checkpoint-is-persisted-in-the-owne.md), [ADR-0113](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0113-resolver-dispatches-are-admitted-by-durable-pre-dispatch-res.md) |
<!-- docket:artifacts:end -->

## Why

Change 0438 handles main advancing before finalize pushes its rebased head. If main advances after that push but before merge, the existing receipt still expects the remote feature branch at its pre-push head and finalize refuses another rebase, including when the remote contains its own tested result.

## What changes

Extend the existing finalize rebase path to handle this exact post-publication case, preserving prior conflict resolutions, an exact remote lease, and fresh test evidence. During grooming, trace the implementation and relevant prior changes and ADRs; treat recognizing the published head and refreshing the owned attempt as a hypothesis. Prefer established receipt, refresh, and gate machinery.

## Out of scope

New commands, configuration, recovery stores, retry policies, budget replenishment, rollback/reset, acceptance of arbitrary remote changes, force-rewritten base recovery, unrelated finalize cleanup, and implementation during grooming.
