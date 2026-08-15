---
id: 309
slug: isolated-metadata-transaction-engine
title: 'Isolated metadata transaction engine'
status: in-progress
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-15
depends_on: [307, 308]
stacked_on:
related: [247]
discovered_from: [303]
adrs: [1, 34, 89]
spec: docs/superpowers/specs/2026-08-15-isolated-metadata-transaction-engine-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/isolated-metadata-transaction-engine
pr:
blocked_by:
claimed_at: 2026-08-15T22:06:24Z
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-15-isolated-metadata-transaction-engine-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-15-isolated-metadata-transaction-engine-design.md) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0034](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0034-repo-root-anchored-to-main-worktree.md), [ADR-0089](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0089-shared-metadata-worktree-contention-survivable-not-impossible.md) |
<!-- docket:artifacts:end -->

## Why

Multiple agents must never share a mutable metadata index or observe and commit one another's
half-written work.

## What changes

Add a semantic-operation transaction engine around the landed repository and Git interfaces.
Every attempt uses a Docket-owned private detached worktree, validates complete before/after state,
checks exact entity versions, commits only declared paths, and pushes under an exact expected-ref
lease. Unrelated ref races re-plan from fresh origin; same-entity races return contention. Stable
request receipts make allocation replay-safe, and manifest-plus-live-lock proof bounds cleanup and
recovery to Docket-owned abandoned state.

## Out of scope

Read-only status composition; workflow-specific create/groom/claim/finalize operations; renderer
content; feature workspaces and branches; GitHub effects; process supervision; repository
migration; release and cutover work owned by changes 0310–0318.

## Design decisions

The linked spec keeps the landed read model pure and adds an outer transaction subpackage. Later
workflows supply semantic operations that return a closed file plan against each fresh attempt;
the engine owns isolation, validation gates, exact-path Git writes, a four-attempt semantic-retry
bound, request-ID history receipts, and ownership-safe cleanup. No user-facing command or later
workflow behavior is implemented here.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
