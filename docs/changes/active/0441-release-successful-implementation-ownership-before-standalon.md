---
id: 441
slug: 'release-successful-implementation-ownership-before-standalon'
title: 'Release successful implementation ownership before standalone finalize'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-21'
updated: '2026-09-21'
depends_on: []
stacked_on:
related: [375, 435, 437]
discovered_from: []
adrs: [118]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

A successful implementation can record run-complete while leaving its run epoch active and its released worktree execution slot owned by that epoch. A subsequent standalone finalize gate then receives stale-run-epoch. The reported local history contains 15 successful reports with active epochs and 14 released slots retaining epochs; these are historical observations, not a count of current blockages.

## What changes

Ensure verified successful implementation relinquishes run ownership so later standalone finalize can proceed safely. Investigate extending the existing cancellation accounting, epoch fencing, and ownership-checked slot retirement; this is a hypothesis to validate against implementation and prior decisions during grooming. Preserve protection against concurrent work and against old-epoch reuse.

## Out of scope

Do not weaken finalize checks or ordinary between-drive ownership. No new daemon, generic coordination framework, configuration switch, retry budget, or unrelated lifecycle redesign.
