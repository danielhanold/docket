---
id: 498
slug: 'results-file-puts-the-whole-branch-review-under-human-action'
title: 'Results file puts the whole-branch review under Human actions and testing'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-03'
depends_on: []
stacked_on:
related: []
discovered_from: [494]
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

Change 0494's results file put its `### Whole-branch review` subsection inside `## Human actions and testing`. That section is meant for things a human should do or check (Important/Optional items). Review outcomes are a record of verification the run performed, so filing them there makes the human-action section look heavier than it is and hides the review where a reader won't look for it. If the results-authoring guidance or template allows this, every future run can repeat it.

## What changes

Find out why implement-next's results authoring (the Step 6.5 checkpoints and the results template) let the review subsection land under `## Human actions and testing`. Then make the guidance say where whole-branch review outcomes belong (most likely `## Verification performed`), so later runs file them there.

## Out of scope

Editing the archived 0494 results file. Merged plans and results are frozen build records, so the misplaced subsection stays as it is. No change to the results template's required or conditional section set beyond saying where review outcomes go.
