---
id: 170
slug: lean-whole-branch-review-skill
title: Lean Docket-owned whole-branch review skill
status: proposed
priority: medium
type: feat
created: 2026-07-30
updated: 2026-07-31
depends_on: [167]
related: [137]
discovered_from: [167]
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

Change 0167 removes SDD's per-task and final reviews but deliberately preserves Docket Step 6 as
one independent whole-branch review. The current default still delegates that boundary to
Superpowers, leaving its model, effort, recursion, and output cost outside Docket's explicit
control.

## What changes

Design and implement a Docket-owned `skills.review` replacement that dispatches exactly one
bounded, read-only whole-branch reviewer with an explicit model and effort, returns actionable
findings to `docket-implement-next`, and never launches recursive reviewer or fix agents.

The design must also determine where the full test suite runs, when one exists. Today
`docket-build` (change 0167) ends with a full-suite gate, and the default reviewer's checklist
("All tests passing?") pushes the Step 6 whole-branch reviewer to run the same suite again —
a ~10-minute suite executed twice back-to-back on the same branch state. This change decides
the suite's single home (or explicitly justifies both runs) and makes the reviewer's relationship
to the suite explicit: trust the build gate's recorded result, re-run, or spot-check.

## Out of scope

- Per-task review.
- Implementing findings inside the reviewer.
- Changing the build profiles or TDD contract from change 0167.

## Open questions

- Which default model and effort should the Claude reviewer use?
- What finding schema gives the controller enough information without recreating verbose review
  reports?
- Where does the full test suite run exactly once — the build's end-of-build gate, the reviewer,
  or both deliberately? If the reviewer trusts the build gate, what evidence (command + result in
  the build output) does it consume instead of re-running?
