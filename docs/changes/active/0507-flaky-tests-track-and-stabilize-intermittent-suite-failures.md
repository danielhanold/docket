---
id: 507
slug: 'flaky-tests-track-and-stabilize-intermittent-suite-failures'
title: 'Flaky tests: track and stabilize intermittent suite failures'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [381]
discovered_from: [504]
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

The full suite occasionally fails on tests that pass on a plain re-run, which costs gate re-runs and can be mistaken for a regression. First observed entry: internal/process TestObserveRunningThenTerminal failed the final gate of change 0504 and passed on re-run with no code change. It was earlier chased in killed change 0381. This ticket is the single place to collect such tests so each is stabilized deliberately rather than re-discovered per run.

## What changes

Maintain a list of known flaky tests in this ticket, each with name, package, observed failure, frequency and the change that saw it. Then stabilize them one at a time: reproduce under parallel load and -race, find the real timing race, fix the test or the code under test, and never mask it with retries or loosened assertions. Seed list: (1) internal/process TestObserveRunningThenTerminal.

## Out of scope

Adding retry-on-failure machinery to the suite runner. Changing the gate or budget policy. Fixing flakes not recorded on the list.
