---
id: 312
slug: planning-mutations-board-and-adrs
title: 'Planning mutations, inline board, and ADRs'
status: implemented
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-16
depends_on: [309, 310]
stacked_on:
related: []
discovered_from: [303]
adrs: [1, 4, 5, 12, 13, 21, 41, 71]
spec: docs/superpowers/specs/2026-08-16-planning-mutations-board-and-adrs-design.md
plan: docs/superpowers/plans/2026-08-16-planning-mutations-board-and-adrs.md
results: docs/results/2026-08-17-planning-mutations-board-and-adrs-results.md
trivial: false
auto_groomable:
branch: feat/planning-mutations-board-and-adrs
claimed_at: 2026-08-17T01:50:37Z
pr: https://github.com/danielhanold/docket/pull/214
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-16-planning-mutations-board-and-adrs-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-16-planning-mutations-board-and-adrs-design.md) |
| Plan | [2026-08-16-planning-mutations-board-and-adrs.md](https://github.com/danielhanold/docket/blob/feat/planning-mutations-board-and-adrs/docs/superpowers/plans/2026-08-16-planning-mutations-board-and-adrs.md) |
| Results | [2026-08-17-planning-mutations-board-and-adrs-results.md](https://github.com/danielhanold/docket/blob/feat/planning-mutations-board-and-adrs/docs/results/2026-08-17-planning-mutations-board-and-adrs-results.md) |
| PR | [#214](https://github.com/danielhanold/docket/pull/214) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0004](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0004-grooming-takes-no-claim.md), [ADR-0005](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0005-close-out-only-harvest.md), [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0013](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0013-adr-0012-boundary-extends-to-docket-adr-surface.md), [ADR-0021](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0021-pipeline-script-authored-mechanical-commits.md), [ADR-0041](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0041-learnings-findings-directory-and-promotion-valve.md), [ADR-0071](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0071-writer-guarantees-yaml-validity-by-construction.md) |
<!-- docket:artifacts:end -->

## Why

The retained planning lifecycle is the first complete mutation vertical slice. The transaction
engine and versioned read context from changes 0309 and 0310 provide the foundations; this change
must prove that authored source records and every affected v1-owned view land as one valid commit
under concurrent writers.

## What changes

Add typed change create, groom, block, defer, and kill operations; manual learning record/update;
and ADR record/supersede/reverse. Existing records accept only owned fields and structured authored
sections, while pure renderers maintain the inline board, change artifacts, artifact backlinks,
and ADR index in the same metadata transaction.

## Out of scope

Foundations already owned by changes 0305 through 0311; autonomous grooming; automatic learning
capture, harvest, indexing, capacity, or promotion; GitHub backlog mirroring; feature workspaces;
claims and implementation transitions; process supervision; pull requests; finalize, recovery,
cleanup, and terminal publishing; release and cutover work owned by changes 0313 through 0318.

## Design decisions

Mutation requests are coarse and typed. Authored Markdown is submitted as named section edits, not
as a replacement record, so unknown bytes survive unchanged. A pure renderer package owns canonical
new records and the v1-owned board, artifact, backlink, and ADR-index outputs. Kill includes the
required metadata archive move but no external cleanup or finalization behavior.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-16

Reconciled against current `main`. Foundations this slice consumes are present and merged:
`internal/repository/transaction` carries the 0309 engine (engine, candidate, idempotency,
concurrency, exact-lease commit/push), and `internal/app` carries the 0310 status / versioned
read-context surface, with `internal/domain` holding the lifecycle, selection, readiness, lease,
stack, ADR, and learning rules this change builds on. `depends_on: [309, 310]` are satisfied
(both merged) — the digest reports the change build-ready.

`internal/render` does not yet exist; this slice introduces it as the pure-renderer package the
spec describes. `internal/app` today holds only status operations — the ten typed mutation
operations (`change create/groom/block/defer/kill`, `learning record/update`,
`adr record/supersede/reverse`) are net-new here, layered on the existing transaction and domain
seams rather than reimplementing them.

Scope unchanged: the design (spec dated 2026-08-16, Approved) was authored against the current
package layout and neighboring-change boundaries (0305-0311 foundations upstream; 0313-0318
downstream, all already stubbed as 314-318 in the backlog). No work has drifted elsewhere, no
constraint has changed, and no adjacent follow-up work surfaced that is not already tracked.
No re-scope needed.
