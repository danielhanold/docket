---
id: 515
slug: 'make-finalize-merge-honor-the-repair-sign-off-block-when-an'
title: 'Make finalize.merge honor the repair sign-off block when an id is named'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [502]
discovered_from: [502]
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

After an integration repair, finalize records a `## Finalize blocked` section so a human must sign off (run `finalize.clear-block`) before the repaired head merges. The skill text says a named id never overrides sign-off, but the binary does not enforce it: in `internal/app/finalize_merge.go` the merge condition is `in.explicitID || !in.finalizeBlocked`, so `finalize merge` with an explicit id merges straight through the block. The safety rule lives only in prose. Found while building change 0502.

## What changes

Enforce the sign-off block in the binary so an explicit id cannot merge a change whose repair sign-off is outstanding. Grooming must decide the scope: whether every `## Finalize blocked` section blocks a named-id merge, or only the repair sign-off kind (in which case the block needs a distinguishable kind). Add a test that names the id of a sign-off-blocked change and asserts the merge refuses, and mutation-test it. Bring the finalize skill text in line with whatever the binary then enforces.

## Out of scope

Changing how the repair sign-off is recorded or cleared (`finalize.clear-block` stays the human's step). The integration-repair ladder itself.
