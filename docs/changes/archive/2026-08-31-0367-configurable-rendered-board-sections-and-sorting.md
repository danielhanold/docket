---
id: 367
slug: 'configurable-rendered-board-sections-and-sorting'
title: 'Configurable rendered-board sections and sorting'
status: 'done'
priority: 'medium'
type: 'feat'
created: '2026-08-29'
updated: '2026-08-31'
depends_on: [370]
stacked_on:
related: [22, 261, 318, 369, 370, 371, 372]
discovered_from: []
adrs: [12, 52]
spec: 'docs/superpowers/specs/2026-08-29-configurable-rendered-board-sections-and-sorting-design.md'
plan: 'docs/superpowers/plans/2026-08-31-configurable-rendered-board-sections-and-sorting.md'
results:
trivial: false
auto_groomable:
branch: 'feat/configurable-rendered-board-sections-and-sorting'
pr: 'https://github.com/danielhanold/docket/pull/259'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-29-configurable-rendered-board-sections-and-sorting-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-configurable-rendered-board-sections-and-sorting-design.md) |
| Plan | [2026-08-31-configurable-rendered-board-sections-and-sorting.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-31-configurable-rendered-board-sections-and-sorting.md) |
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

### 2026-08-31

Reconciled against current `main`/`docket`. The spec's central assumption holds: change 0370's Go-only cutover is complete, `scripts/render-board.sh` is gone from the live tree, and `internal/render/board.go` (entry point `Board(BoardInput)`) is the sole inline-board authority. There is no `board:` config block today — the only board-related leaf is `board_surfaces` — so this change adds the block net-new. Section order and per-section sort are currently compile-time constants (`boardActiveStatuses`, `sortByID`); classification/readiness predicates (`domain.EvaluateReadiness`, `Change.HasFinalizeBlocked`, `domain.ResolveEffectiveBase`) already exist for the renderer to compose. The `finalize.*`/`review.*` fixed-nested config blocks plus the `change_types` string-list leaf are the templates for the new `board.section_order` + per-section `board.sorting.<section>.{by,direction}` leaves; config inspection (`internal/app/config.go` `effectiveLines`) and the canonical example (`.docket.example.yml`, embedded mirror) need the new leaves. Related changes 0318/0369/0371/0372/0370 are all archived and 0261 remains proposed (correctly out of scope). No scope change: the change proceeds as specified. Follow-up noted for deliberate capture (out of this change's scope): the `board.go` header comment and the embedded skill/docs prose (docket-status, docket-convention SKILL.md) still describe the now-absent `render-board.sh`/`board-refresh.sh` byte authority and should be refreshed in a separate docs change.
