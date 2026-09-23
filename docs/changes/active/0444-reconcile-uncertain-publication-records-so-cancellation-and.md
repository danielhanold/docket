---
id: 444
slug: 'reconcile-uncertain-publication-records-so-cancellation-and'
title: 'Reconcile uncertain publication records so cancellation and resume can finish'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-23'
updated: '2026-09-23'
depends_on: []
stacked_on:
related: [375, 437, 441]
discovered_from: []
adrs: [118, 124]
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
| ADRs | [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0124](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md) |
<!-- docket:artifacts:end -->

## Why

A publication whose result cannot be observed leaves an uncertain run-epoch journal entry. A later successful retry appends a separate completed entry without settling the original. Cancellation then repeatedly reports mutation-pending even with no running processes, and resume remains blocked waiting for confirmed cancellation. Reported by a separate diagnostic session; the current source confirms the status-only accounting gap.

## What changes

Make cancellation able to account for the original publication obligation using authoritative evidence, preserving existing fencing and resume safety. Investigate whether extending the existing journal with enough publication identity and reusing existing remote probes is sufficient; this is a design hypothesis, not a prescribed new subsystem. Trace implementation and relevant prior changes and ADRs during grooming, and select the smallest repair that covers the reported successful-retry case.

## Out of scope

New cancellation or resume policy, force-clearing uncertain work, undoing published work, background reconciliation services, generic retry frameworks, unrelated workflow redesign, and implementation during grooming.
