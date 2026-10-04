---
id: 508
slug: 'bring-tests-test-go-finalize-e2e-sh-back-under-its-serial-wa'
title: 'Bring tests/test_go_finalize_e2e.sh back under its serial wall-clock budget'
status: 'killed'
priority: 'low'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: []
discovered_from: [506]
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

Found while building change 0506: the final full-suite run reported `SERIAL CONFIRMED OVER BUDGET` for `tests/test_go_finalize_e2e.sh` — 49s run alone against a 45s limit. That line is an authoritative breach, not a parallel-load artifact, and it is unrelated to 0506 (which touches only gitignore code and tests). Nothing fails on it today, so nothing else will surface it.

## What changes

Trace why `tests/test_go_finalize_e2e.sh` takes ~49s serially and bring it under the 45s budget, by cutting redundant work in the test or its fixtures. Confirm with a serial run. Raising the budget is the fallback only if the work proves irreducible, with the evidence recorded.

## Out of scope

Other tests' budgets, the budget-report mechanism itself, and any new blocking gate (budget findings stay visibility-only).

## Why killed

Filed in error; consolidated into #507, which tracks flaky and slow tests.
