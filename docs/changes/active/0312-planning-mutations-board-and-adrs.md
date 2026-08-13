---
id: 312
slug: planning-mutations-board-and-adrs
title: 'Planning mutations, inline board, and ADRs'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [309, 310]
stacked_on:
related: []
discovered_from: [303]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

The retained planning lifecycle is the first mutation vertical slice and must prove that primary
records and their derived views land atomically under concurrent writers.

## What changes

Implement coarse create, groom, block, defer, kill, manual-learning, and ADR operations together
with inline board, artifact, backlink, and ADR-index renderers.

## Out of scope

Autonomous grooming, automatic learning harvest, GitHub backlog mirroring, feature workspaces, and
implementation/finalize workflows.

## Open questions

Settle operation request types and exact renderer ownership after the transaction and status slices
land.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
