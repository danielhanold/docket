---
id: 558
slug: 'name-the-serial-confirmation-not-counted-budget-line-in-agen'
title: 'Name the SERIAL CONFIRMATION NOT COUNTED budget line in AGENTS.md'
status: 'proposed'
priority: 'low'
type: 'docs'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [528]
discovered_from: [528]
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

AGENTS.md tells agents to read the suite runner's budget report even on a green run, and names the `BUDGET WATCH:`, `PARALLEL-SENSITIVE:` and `SERIAL CONFIRMED OVER BUDGET:` lines. #528 added a fourth line, `SERIAL CONFIRMATION NOT COUNTED:`, printed when the solo re-check saw a concurrent suite (or could not tell) and so would not count an over-the-line reading as a confirmed breach. AGENTS.md does not mention it, so an agent reading a green run may not know what it means or that the test stays due.

## What changes

Decide whether to add `SERIAL CONFIRMATION NOT COUNTED:` to the budget-report guidance in AGENTS.md, and if so add one sentence saying it is a visibility-only note: re-measure quietly (no other suite running) and the test stays due.

## Out of scope

The release-candidate workflow comment about the budget vocabulary (folded into #557). Any change to the runner's behavior.
