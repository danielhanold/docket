---
id: 568
slug: 'add-a-typed-way-to-clear-branch-on-a-proposed-change'
title: 'Add a typed way to clear branch on a proposed change'
status: 'proposed'
priority: 'low'
type: 'fix'
deliverable: 'pr'
created: '2026-10-18'
updated: '2026-10-18'
depends_on: []
stacked_on:
related: [563, 565]
discovered_from: [563]
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

Change 563 made change.groom refuse restacking (not-restackable) when a proposed change already has a branch: recorded, because its feature branch was cut from the old base. The remedy for a human is to clear branch:, but no typed operation does that. Today it means hand-editing the record on the docket branch, or killing the change and capturing it again. Deleting the git branch does not help, since the check reads the record's branch: field. The gap matters for a killed stack parent's proposed child (see #565), which may already carry a branch: and so cannot be un-stacked or re-parented.

## What changes

Give a human a typed, atomic way to clear branch: on a proposed change, so a restack or un-stack can follow. Decide where it lives (for example, an option on change.groom, or a dedicated operation) and what it must check first, such as that no open PR and no unmerged work on the old branch would be orphaned. Point the not-restackable finding's remedy at it.

## Out of scope

How change.groom sets or clears stacked_on (done in #563). Deciding the killed-parent blocking policy (#565). Changes in any status other than proposed.
