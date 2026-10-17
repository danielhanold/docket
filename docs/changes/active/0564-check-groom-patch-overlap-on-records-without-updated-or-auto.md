---
id: 564
slug: 'check-groom-patch-overlap-on-records-without-updated-or-auto'
title: 'Check groom patch overlap on records without updated or auto_groomable'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: []
discovered_from: [559]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

Suspected, not reproduced: change.groom's first patch pass may fail with an overlapping-edit error on a record that has neither an updated: nor an auto_groomable: line. Seen as a possibility while building change 559, which touched the groom edit path.

## What changes

Reproduce with a record lacking both fields. If it fails, fix the edit planning so the patches do not overlap and add a regression test; if it does not reproduce, close this out with a note.

## Out of scope

Redesigning the groom patch machinery.
