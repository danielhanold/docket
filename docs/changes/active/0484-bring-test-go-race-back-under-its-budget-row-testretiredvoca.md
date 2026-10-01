---
id: 484
slug: 'bring-test-go-race-back-under-its-budget-row-testretiredvoca'
title: 'Bring test_go_race back under its budget row (TestRetiredVocabularySeal scan cost)'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-10-01'
updated: '2026-10-01'
depends_on: []
stacked_on:
related: [466, 465, 475, 476, 289]
discovered_from: [482]
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

During change 0482's build gate the runner printed `SERIAL CONFIRMED OVER BUDGET: tests/test_go_race.sh — 94s solo; solo threshold 90s`. Its row in `tests/runtime-budgets.tsv` is `60 parallel`. A serial-confirmed line is the runner's authoritative breach signal but never fails the suite, so nothing else will surface it. 0482 changed only comments and test wording, so the slowdown is already on `main`: the solo time has grown from 32s to 94s in step with rows added to `TestRetiredVocabularySeal` in `internal/repoguard`. The slow lane is also what lets a loaded machine hit the 8-minute `-race` backstop (see the sibling flaky-gate stub).

## What changes

Make the seal's per-line scan cheaper, for example by pre-filtering each line before running the per-row regexes, and measure solo time before and after on an untouched merge-base. Only if the time is genuinely needed, re-size the `tests/test_go_race.sh` row from measured worst solo readings, following change 0466's precedent, and say why in the change. Keep the budget ledger narration bound to the numbers (change 0289).

## Out of scope

Raising the `-race` timeout or backstop. The flaky load-induced timeout is its own stub. Other budget rows (0475, 0476).
