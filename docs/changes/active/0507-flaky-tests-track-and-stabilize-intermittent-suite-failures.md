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
related: [381, 273]
discovered_from: [504, 506, 520, 517]
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

Maintain a list of known flaky tests in this ticket, each with name, package, observed failure, frequency and the change that saw it. Then stabilize them one at a time: reproduce under parallel load and -race, find the real timing race, fix the test or the code under test, and never mask it with retries or loosened assertions. Seed list: (1) internal/process TestObserveRunningThenTerminal. (2) tests/test_go_finalize_e2e.sh: not a flake but a slow test, seen by change 0506 — the final full-suite run reported `SERIAL CONFIRMED OVER BUDGET` at 49s run alone against a 45s limit. It is unrelated to 0506 (gitignore code only). Trace why it takes ~49s serially and bring it under the 45s budget by cutting redundant work in the test or its fixtures; raise the budget only if the work proves irreducible, with the evidence recorded. (3) tests/test_go_integration_app_merge.sh: also a slow test, not a flake — seen by change 0520, whose final gate reported it at 92s run alone against a 45s limit. Change 0520 does not touch it. Apply the same treatment as (2): trace why it takes ~92s serially, cut redundant work in the test or its fixtures, and raise the budget only if the work proves irreducible, with the evidence recorded. (4) tests/test_go_race.sh: also a slow test, not a flake — seen by change 0517, whose first full-suite run reported `SERIAL CONFIRMED OVER BUDGET` at 117s run alone against a 90s limit, probably inflated by another change's suite running concurrently on the same machine. Change 0517 does not touch it. First re-measure it on an otherwise idle machine; if it still exceeds 90s, apply the same treatment as (2): trace the time, cut redundant work, and raise the budget only if the work proves irreducible, with the evidence recorded. Related: #273 (host-relative runtime budgets).

## Out of scope

Adding retry-on-failure machinery to the suite runner. Changing the gate or budget policy. Fixing flakes not recorded on the list.
