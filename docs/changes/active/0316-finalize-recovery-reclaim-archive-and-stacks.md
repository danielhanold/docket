---
id: 316
slug: finalize-recovery-reclaim-archive-and-stacks
title: 'Finalize, recovery, reclaim, archive, and stacks'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [315]
stacked_on:
related: [298]
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

The migration is not functionally complete until merged and interrupted work converges safely to
the correct terminal state in both repository modes.

## What changes

Implement local rebase/retest, merge verification, archive, stack close-out, reclaim, explicit
maintenance sweep, halted/finalize-blocked recovery, and ownership-safe cleanup.

## Out of scope

CI-only gates, terminal publishing, automatic learning harvest, release packaging, and self-hosting
cutover.

## Open questions

Settle resumable operation boundaries and failure-injection points against the landed implementation
workflow and current stacked-change invariant.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
