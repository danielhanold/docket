---
id: 315
slug: claim-to-implemented-workflow
title: 'Claim-to-implemented agent workflow'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [312, 313, 314]
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

The essential implementation lifecycle must work end to end through Claude Code without direct
metadata edits or moving model judgment into the binary.

## What changes

Compose candidate context, claim and lease refresh, reconciliation, plan/results attachment,
workspace inputs, local-gate results, run verification, PR association, and the transition to
`implemented` behind agent-first skills.

## Out of scope

Cross-harness delegation, autonomous grooming, finalize/merge/archive, and deferred role rebinding.

## Open questions

Settle the revised skill surfaces and exact coarse operation sequence after all three predecessor
vertical slices land.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
