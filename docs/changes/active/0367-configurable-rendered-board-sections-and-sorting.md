---
id: 367
slug: 'configurable-rendered-board-sections-and-sorting'
title: 'Configurable rendered-board sections and sorting'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-08-29'
updated: '2026-08-29'
depends_on: [370]
stacked_on:
related: [22, 261, 318, 369, 370]
discovered_from: []
adrs: [12, 52]
spec: 'docs/superpowers/specs/2026-08-29-configurable-rendered-board-sections-and-sorting-design.md'
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-29-configurable-rendered-board-sections-and-sorting-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-configurable-rendered-board-sections-and-sorting-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md) |
<!-- docket:artifacts:end -->

## Why

The inline board currently mirrors lifecycle statuses and ascending IDs rather than the way a human
scans work. Spec-backed build-ready changes are mixed with proposals that still need grooming,
implemented and stacked work is split, finalize-blocked work remains under the built state, and no
common config layer can choose section or row order.

## What changes

- Render the active board as six configurable presentation groups: In progress, Built, Blocked,
  Groomed, Proposed, and Deferred.
- Put finalize-blocked implemented changes in the rendered Blocked group without changing their
  lifecycle status; combine healthy implemented and stacked-merged work under Built.
- Put only spec-backed, fully build-ready proposals under Groomed; keep every other proposed change,
  including trivial build-ready work, under Proposed.
- Add a layered `board` config block for a complete section-order permutation and per-section
  `id`/`updated`/`created`, ascending/descending sorting. Default to updated descending everywhere.
- Keep Archive fixed after the active board and preserve date-descending, ID-descending-within-day
  ordering with an explicit regression guard.
- Implement only after 0370's final Go-only cutover, in the Go config and renderer; leave digest,
  selection, Mermaid, GitHub mirror, and lifecycle data unchanged.

## Out of scope

Lifecycle or readiness changes, board filtering, configurable Archive behavior, merge timestamps,
Git/GitHub lookups during rendering, digest or autonomous-queue ordering, GitHub mirror layout,
moving dependency-waiting proposals into Blocked, change 0261's future Run halted presentation,
and any Bash renderer or Bash config work.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
