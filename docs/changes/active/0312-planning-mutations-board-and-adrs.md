---
id: 312
slug: planning-mutations-board-and-adrs
title: 'Planning mutations, inline board, and ADRs'
status: proposed
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
| Spec | [2026-08-16-planning-mutations-board-and-adrs-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-16-planning-mutations-board-and-adrs-design.md) |
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
