---
id: 313
slug: workspaces-github-pr-adapter-and-build-evidence
title: 'Workspaces, GitHub PR adapter, and build evidence'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [308, 309]
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

Feature-branch and pull-request effects need typed, ownership-safe, idempotent mechanics before an
agent workflow can compose them reliably.

## What changes

Implement feature-workspace prepare/inspect/cleanup, effective-base branch creation, the `gh`
adapter, PR lookup/create/update, external-effect probing, build-evidence mechanics, and verified
idempotent recovery.

## Out of scope

Agent dispatch, lifecycle orchestration, local process supervision, and merge/finalize policy.

## Open questions

Settle external-effect request/result interfaces and GitHub test seams during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
