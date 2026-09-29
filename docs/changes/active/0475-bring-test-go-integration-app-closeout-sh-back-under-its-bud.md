---
id: 475
slug: 'bring-test-go-integration-app-closeout-sh-back-under-its-bud'
title: 'Bring test_go_integration_app_closeout.sh back under its budget row'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: []
stacked_on:
related: [466, 280]
discovered_from: [470]
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

During change 0470's first build-gate suite run (2026-09-29), the runner's budget report printed `SERIAL CONFIRMED OVER BUDGET` for `tests/test_go_integration_app_closeout.sh`: 69s solo against a 60s threshold (148s under parallel load). Its row in `tests/runtime-budgets.tsv` is `40 parallel`. A serial-confirmed line is the runner's authoritative breach signal, but it never fails the suite, so nothing else will act on it. The breach did not recur on 0470's second gate run, so it may be intermittent. It is unrelated to 0470, which touched only `internal/gatedrive` tests; it is recorded under Known issues in 0470's results file.

## What changes

Determine whether this is a genuine slowdown (a regression in the closeout integration shard or the code it drives) or a budget row that is sized below the shard's real worst-case solo time. Then either fix or split the shard, or re-size its row in `tests/runtime-budgets.tsv` from measured worst solo readings, following change 0466's precedent for the transaction_race row. Keep the ledger narration consistent with the new number.

## Out of scope

Changing the runner's budget regime, slack factor, or report semantics. Other shards' budget rows unless the investigation shows a shared cause.
