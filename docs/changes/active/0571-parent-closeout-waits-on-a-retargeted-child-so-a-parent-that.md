---
id: 571
slug: 'parent-closeout-waits-on-a-retargeted-child-so-a-parent-that'
title: 'Parent closeout waits on a retargeted child, so a parent that lands first never becomes done'
status: 'proposed'
priority: 'medium'
type: 'fix'
deliverable: 'pr'
created: '2026-10-19'
updated: '2026-10-19'
depends_on: []
stacked_on:
related: [567]
discovered_from: [567]
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

When a stacked parent lands on main before its child, the child's PR is retargeted to main. The parent's closeout still requires every stacked child to be stacked-merged into the parent and counts the retargeted child, so closeout refuses with `children-retarget-required` while that child is open. The parent then never reaches done. Found while building #567 (PR #433): the end-to-end test for the child-after-parent flow has to write the parent's done record by hand to get past this.

## What changes

Decide whether a retargeted child belongs in the parent's carry set, and change closeout so a parent whose remaining children were retargeted to main can reach done. Replace the hand-written done record in the #567 end-to-end test with the real closeout path.

## Out of scope

Cleanup of carried children's branches after the root lands (that is #567). Changing how a child is retargeted.
