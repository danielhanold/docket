---
id: 485
slug: 'test-go-race-hits-its-8-minute-backstop-in-internal-repoguar'
title: 'test_go_race hits its 8-minute backstop in internal/repoguard under concurrent gate load'
status: 'killed'
priority: 'medium'
type: 'fix'
created: '2026-10-01'
updated: '2026-10-01'
depends_on: []
stacked_on:
related: [465, 373, 333, 362, 466]
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

During change 0482's build gate, a full-suite run at a results-only head went red once: `tests/test_go_race.sh` hit its 8-minute `-race` backstop in `internal/repoguard` while the machine's load average was 45-69. Change 0481's gate was running at the same time and hit the same backstop in the same package. A repair worker found no cause on the branch, and the re-run of the same head passed. Two concurrent gates on one machine can therefore fail each other, and a green re-run proves nothing about the code. Changes 0465 and 0373 handled the `internal/app` ceiling and integration race isolation, but nothing covers `internal/repoguard` under concurrent gates.

## What changes

Reproduce the timeout with two gates running concurrently, and find what makes `internal/repoguard` slow under `-race` load (the retired-vocabulary seal scan is the prime suspect, see the sibling budget stub). Then fix it, by bounding total test load across concurrent gates (ADR-0108 direction), partitioning the package, or making the scan cheaper. Prove the fix with a before/after run under the same load.

## Out of scope

Raising the 8-minute backstop. Re-sizing the `test_go_race` budget row (separate stub). Failures in other packages.

## Why killed

Consolidated into #0487 on 2026-10-01 at Daniel's request: the repoguard race-lane backstop timeout (#0485), the three serial-confirmed budget breaches (#0475, #0476, #0484), and the #0478 gofmt cleanup ship as one change.
