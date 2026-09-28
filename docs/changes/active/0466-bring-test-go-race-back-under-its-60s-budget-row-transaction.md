---
id: 466
slug: 'bring-test-go-race-back-under-its-60s-budget-row-transaction'
title: 'Bring test_go_race back under its 60s budget row (transaction, workspace, gatedrive)'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-09-28'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [465]
discovered_from: [465]
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

Change 0465 took internal/app out of the race gate's critical path (~238s -> ~14s under -race), but tests/test_go_race.sh still measures ~66.5-67.6s solo on a loaded developer machine against its 60s budget row. The time now comes from other packages under -race: internal/repository/transaction (~61s), internal/workspace (~57s) and internal/gatedrive (~43s). 0465 deliberately left the row at 60 rather than raising it. Separately, tests/test_go_toolchain.sh measures 61.7-65.8s on a cold test cache against its 55s row (warm runs are ~3s because it runs go test ./... without -count=1). Neither is near the 8m -timeout or Go's 10m limit, so CI is not at risk, but the suite now prints BUDGET WATCH for these files and, since 0465 made budget keys repo-relative, will eventually print SERIAL CONFIRMED OVER BUDGET. Source: 0465 results file, 'Known issues and follow-ups'.

## What changes

Serial-confirm the current race-gate and toolchain-gate timings on an idle machine, then profile the slow -race packages (internal/repository/transaction, internal/workspace, internal/gatedrive) to find where the time goes (real git/subprocess work, sleeps/polling, fixture setup). Bring tests/test_go_race.sh back under its 60s row, and tests/test_go_toolchain.sh under its 55s row on a cold cache, by speeding up or partitioning those packages' slow tests - e.g. following the 0333/0465 pattern of moving real-git/subprocess tests behind //go:build integration into shard runners, where that is the actual cost. Do not raise the budget rows. Treat this approach as a hypothesis for grooming: trace the existing partition machinery (0333, 0465) first and prefer extending it.

## Out of scope

Raising any budget row. internal/app (already fixed by 0465). The three untouched integration shards that measured over their rows under load in 0465's re-measure (app_closeout 64s/40, app_rebaserecovery 60s/40, app_changeruntime 40s/40) - suspected, not confirmed; serial-confirm them separately. A static check banning no-git-guard bypasses (gitcli.WithExecutable with an absolute path, wholesale PATH replacement). The pre-existing TestIntegrationRepo shard prefix overlap.
