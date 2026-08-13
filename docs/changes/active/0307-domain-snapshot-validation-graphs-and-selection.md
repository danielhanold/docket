---
id: 307
slug: domain-snapshot-validation-graphs-and-selection
title: 'Domain snapshot, validation, graphs, and selection'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [305, 306]
stacked_on:
related: [301]
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

Docket's lifecycle and graph rules must become typed, filesystem-independent policy rather than
remaining distributed across skills and text-processing helpers.

## What changes

Model immutable repository snapshots, lifecycle transitions, readiness and deterministic
selection, dependencies and stacks, claims and reclaim, ADR rules, learning consumption, and
repository-wide validation.

## Out of scope

Git object access, document patch mechanics, transactions, rendering, and harness behavior.

## Open questions

Settle the exact aggregate and policy interfaces after configuration and document types land.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
