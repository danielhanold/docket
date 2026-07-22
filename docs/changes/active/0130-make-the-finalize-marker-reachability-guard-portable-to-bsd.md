---
id: 130
slug: make-the-finalize-marker-reachability-guard-portable-to-bsd
title: Make the finalize marker reachability guard portable to BSD grep
status: proposed
priority: medium
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: []
discovered_from: [116]
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

`tests/test_finalize_disposition.sh` uses the ERE interval `.{0,600}` to prove that the Finalize
blocked marker write is reachable from the abort-and-report procedure. BSD grep rejects repetition
bounds above 255 with `maximum repetition exceeds 255`, so the assertion fails before examining
the unchanged finalize skill. This prevents a portable whole-suite green run on macOS.

## What changes

Replace the oversized interval with a portable structural extraction or bounded multi-stage check
that preserves the reachability claim. Mutation-test it by removing the procedure's marker-write
call and confirming only the intended guard reddens.

## Out of scope

- Changes to finalize behavior or the `## Finalize blocked` contract.
- Broad rewrites of other disposition assertions.

## Open questions

- None.
