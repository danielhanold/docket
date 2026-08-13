---
id: 310
slug: read-only-status-and-health-vertical-slice
title: 'Read-only status and health vertical slice'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [307, 308]
stacked_on:
related: [261]
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

The first retained user workflow should prove that Go can open, interpret, and report an existing
repository without mutation before it is trusted to write.

## What changes

Compose config, document parsing, domain snapshots, and Git objects into status context, selection,
dependency/stack presentation, health findings, readable text, and protocol JSON. Keep maintenance
explicitly separate.

## Out of scope

Board writes, lifecycle mutations, maintenance sweep, and feature-branch work.

## Open questions

Settle the human report and JSON context shapes against the landed protocol and domain types.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
