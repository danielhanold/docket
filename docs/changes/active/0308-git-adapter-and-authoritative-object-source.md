---
id: 308
slug: git-adapter-and-authoritative-object-source
title: 'Git adapter and authoritative object source'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [304]
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

Authoritative reads and Git identities must be available without modifying the user's checkout or
letting Git command details leak into domain code.

## What changes

Implement typed Git discovery and execution, fetch/ref/object reads, repository-root resolution,
both metadata modes, blob entity versions, and immutable object-source interfaces.

## Out of scope

Domain snapshot assembly, metadata transactions, GitHub pull requests, and feature workspaces.

## Open questions

Settle adapter interfaces, environment policy, error taxonomy, and fixture strategy during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
