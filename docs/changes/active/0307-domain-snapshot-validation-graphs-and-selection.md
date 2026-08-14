---
id: 307
slug: domain-snapshot-validation-graphs-and-selection
title: 'Domain snapshot, validation, graphs, and selection'
status: in-progress
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-14
depends_on: [305, 306]
stacked_on:
related: [298, 301]
discovered_from: [303]
adrs: [92]
spec: docs/superpowers/specs/2026-08-13-domain-snapshot-validation-graphs-and-selection-design.md
plan: docs/superpowers/plans/2026-08-14-domain-snapshot-validation-graphs-and-selection.md
results:
trivial: false
auto_groomable:
branch: feat/domain-snapshot-validation-graphs-and-selection
claimed_at: 2026-08-14T02:39:23Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-13-domain-snapshot-validation-graphs-and-selection-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-domain-snapshot-validation-graphs-and-selection-design.md) |
| Plan | [2026-08-14-domain-snapshot-validation-graphs-and-selection.md](https://github.com/danielhanold/docket/blob/feat/domain-snapshot-validation-graphs-and-selection/docs/superpowers/plans/2026-08-14-domain-snapshot-validation-graphs-and-selection.md) |
| ADRs | [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md) |
<!-- docket:artifacts:end -->

## Why

Docket's lifecycle and graph rules must become typed, filesystem-independent policy rather than
remaining distributed across skills and text-processing helpers.

## What changes

Add a pure `internal/domain` policy package and the in-memory read-model portion of
`internal/repository`. Decode already-supplied loss-preserving documents and resolved supported
configuration into immutable typed changes, ADRs, learnings, and a complete validation report.
Implement named lifecycle actions, readiness and deterministic selection, dependency and stack
graphs, effective-base resolution over supplied branch facts, claim/reclaim rules, ADR evolution
rules, and retained learning consumption without filesystem or subprocess access.

## Out of scope

Git/object access, document patch mechanics, transactions, status presentation, health rendering,
planning mutations, feature workspaces, process supervision, workflow orchestration,
finalize/archive/reclaim execution, installation, release work, and harness behavior owned by
changes 0308–0318.

## Design decisions

The approved focused design is in the linked spec. Domain policy remains independent of config,
Markdown, Git, filesystems, CLI, processes, and harnesses. A thin repository decoder translates
landed `config` and `document` values into a deep-copied snapshot. Validation returns the snapshot
plus deterministic typed findings so read-only consumers can report damage, while any error-level
finding blocks future mutation. Lifecycle changes use named actions rather than a generic status
setter, and every external fact such as time, branch presence, or merge destination is injected.

Dependency satisfaction remains `done` only; selection remains priority, creation date, then ID;
stack bases retain ADR-0092's merge-destination rule; reclaim requires a strictly expired valid
lease and no recorded or conventional branch. Existing ADR bytes are frozen except for a legal
status change or an append-only `## Update`. Learning relevance remains skill judgment.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

**2026-08-14** — Reconciled against origin/main at e9892fb1. Dependencies 0305 (config envelope)
and 0306 (loss-preserving document layer) are done and their deliverables verified on main:
`internal/config` exports `Effective`/`Snapshot`, `internal/document` exports
`Document.DecodeFrontmatter` and the patch/marker surface the spec cites. `internal/domain` and
`internal/repository` do not yet exist — scope is intact as specced. Note: 0311 (installer,
embedded assets, harnesses) has since landed on main; it shares no files with this change and
required no scope adjustment. Spec (2026-08-13) taken as-is; no auto-capture candidates surfaced.
