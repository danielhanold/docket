---
id: 562
slug: 'sweep-retries-deleting-a-leftover-local-branch'
title: 'Sweep retries deleting a leftover local branch'
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

The maintenance sweep retries removing a leftover worktree for a finished change, but never retries deleting its leftover local branch. If the branch delete failed at closeout, nothing ever tries again. Change 559 found this while adding cleanup for results-artifact changes; ordinary PR changes have the same gap.

## What changes

Make the sweep (and finalize cleanup retry) also retry deleting the leftover local feature branch of a done change, for both PR and results-artifact changes, reporting it like the worktree retry.

## Out of scope

Remote branch deletion; changing when closeout first attempts cleanup.
