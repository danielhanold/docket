---
id: 569
slug: 'flaky-and-failing-tests-standing-bucket'
title: 'Flaky and failing tests — standing bucket'
status: 'proposed'
priority: 'low'
type: 'chore'
deliverable: 'pr'
created: '2026-10-18'
updated: '2026-10-18'
depends_on: []
stacked_on:
related: []
discovered_from: [562]
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

A standing catch-all for tests that flake or fail intermittently, so each one has a home instead of being rediscovered and re-explained every time it reds a full-suite gate. First entry: `TestIntegrationRunCancelFreesWorktreeForNextStart` went red once during change 562's gate (a `claim-busy` timing failure in run-cancel code that 562 does not touch). It passed 3 of 3 when run alone, so it looks timing- or contention-sensitive under the parallel suite.

## What changes

Investigate and fix each flaky test listed here, one at a time, adding new entries as further flakes are found.

- `TestIntegrationRunCancelFreesWorktreeForNextStart`: reproduce the `claim-busy` failure under parallel load, find the race or timing assumption, and fix it without weakening the test.

## Out of scope

Slow tests and budget breaches (the budget report owns those). New features in run-cancel itself.
