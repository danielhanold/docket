---
id: 413
slug: 'finalize-rebase-gate-cannot-clear-derived-embedded-asset-man'
title: 'Finalize rebase gate cannot clear derived embedded-asset manifest collisions'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-09-08'
updated: '2026-09-16'
depends_on: []
stacked_on:
related: [349, 419]
discovered_from: [327]
adrs: [10, 113]
spec: 'docs/superpowers/specs/2026-09-16-finalize-rebase-gate-cannot-clear-derived-embedded-asset-man-design.md'
plan: 'docs/superpowers/plans/2026-09-16-finalize-rebase-generated-bundle-fast-path.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/finalize-rebase-gate-cannot-clear-derived-embedded-asset-man'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-16T12:08:36Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-16-finalize-rebase-gate-cannot-clear-derived-embedded-asset-man-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-16-finalize-rebase-gate-cannot-clear-derived-embedded-asset-man-design.md) |
| Plan | [2026-09-16-finalize-rebase-generated-bundle-fast-path.md](https://github.com/danielhanold/docket/blob/fix/finalize-rebase-gate-cannot-clear-derived-embedded-asset-man/docs/superpowers/plans/2026-09-16-finalize-rebase-generated-bundle-fast-path.md) |
| ADRs | [ADR-0010](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0010-finalize-merge-gate-split-agents.md), [ADR-0113](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0113-resolver-dispatches-are-admitted-by-durable-pre-dispatch-res.md) |
<!-- docket:artifacts:end -->

## Why

Repeated regeneration of internal/assets/embedded/manifest.json causes mechanical conflicts while replaying a feature branch onto independently changed authored assets on main. Confirmed on 2026-09-16: an isolated 11-commit branch produced 11 manifest-only stops despite cleanly merged authored files; regeneration cleared every stop. The original two-dispatch limit is obsolete: change 0349 introduced a durable configurable resolver budget and change 0419 raised its default to 10, but generated collisions still consume that budget. The remaining fix is to re-derive this bundle inside the existing finalize rebase flow.

## What changes

Handle Docket embedded-bundle conflicts deterministically in the existing finalize rebase controller: resolve authored inputs through the normal resolver when needed, regenerate with the existing generator, and continue generated-only stops without spending resolver reservations. Preserve owned-rebase recovery, authored-conflict admission, and post-rebase validation.

## Out of scope

Changing cmd/genassets or SKILL.md ceilings; generic generated-artifact frameworks, new commands or configuration, separate state stores, history squashing, blanket resolver-budget increases, and unrelated finalize repairs.

## Reconcile log

### 2026-09-16

2026-09-16: Reconciled against current main (26b4cbd3). Necessity confirmed — the spec's design remains accurate and current. Related changes 349 (configurable resolver dispatch cap) and 419 (configurable finalize repair attempts, default 10) are both merged/done, so the durable configurable resolver budget exists but still does not distinguish generated-only conflicts, exactly as the spec describes. The finalize rebase controller lives in internal/app/finalize_rebase.go with resolver reservation in internal/app/finalize_reserve.go (mapBegunRebase/mapContinuedRebase route every conflict through resolver admission). The embedded bundle is internal/assets/embedded/{manifest.json,tree/}; the module identity is github.com/danielhanold/docket; and the generator has a clean in-process library entrypoint (internal/assets: Generate, WriteTree, DiffTree, DefaultAllowedRoots) reused by cmd/genassets, so regeneration inside the controller needs no new generator. No scope change: proposal body Why / What changes / Out of scope remain accurate.
