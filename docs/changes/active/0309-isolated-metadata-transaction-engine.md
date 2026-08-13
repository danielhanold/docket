---
id: 309
slug: isolated-metadata-transaction-engine
title: 'Isolated metadata transaction engine'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [307, 308]
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

Multiple agents must never share a mutable metadata index or observe and commit one another's
half-written work.

## What changes

Implement Docket-owned isolated transaction worktrees, ownership locks, entity-version checks,
semantic retry, exact-ref leases, explicit-path commits, request-id idempotency, cleanup, and safe
recovery pruning.

## Out of scope

Feature workspaces, workflow-specific domain operations, GitHub effects, and process supervision.

## Open questions

Settle transaction interfaces and retry/idempotency records against the landed domain and Git
interfaces during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
