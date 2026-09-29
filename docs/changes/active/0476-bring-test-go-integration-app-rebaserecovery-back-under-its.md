---
id: 476
slug: 'bring-test-go-integration-app-rebaserecovery-back-under-its'
title: 'Bring test_go_integration_app_rebaserecovery back under its runtime budget'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: []
stacked_on:
related: [466, 434]
discovered_from: [468]
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

During change 0468's build gate, the suite runner reported `SERIAL CONFIRMED OVER BUDGET: tests/test_go_integration_app_rebaserecovery.sh`: the file took 61s when run on its own, against a 60s threshold. Its row in `tests/runtime-budgets.tsv` is 40s, run in parallel. 0468 only changed prose, so the slowness was already there. A serial-confirmed breach is authoritative, but it does not fail the run by default, so nothing else will catch it.

## What changes

Find out why the `TestIntegrationFinalizeRebaseRecovery` shard (split out by change 0434) grew past its budget, and bring it back under its `tests/runtime-budgets.tsv` row. The likely fix is to speed up or split the slow tests, as change 0466 did for `test_go_race`. Raising the budget row is acceptable only when the time is genuinely needed, and the change must say why.

## Out of scope

- Other `BUDGET WATCH` / `PARALLEL-SENSITIVE` lines from the same run; they are screening findings, not confirmed breaches.
- Changing how the suite runner measures or enforces budgets.
