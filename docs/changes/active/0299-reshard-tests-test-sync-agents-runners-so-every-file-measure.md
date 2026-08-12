---
id: 299
slug: reshard-tests-test-sync-agents-runners-so-every-file-measure
title: 'Reshard tests/test_sync_agents_runners so every file measures under its wall-clock ceiling'
status: proposed
priority: medium
type: chore
created: 2026-08-12
updated: 2026-08-12
depends_on: []
related: []
discovered_from: [298]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — every full-suite run during change 0298's build and review reported a trailing
`OVER BUDGET: test_sync_agents_runners` line: the file takes 192-198s against its 60s ceiling, more
than three times over, and it is the single longest file in the suite (its 198s IS the suite's
198s wall clock). Change 0298 does not touch that file, so nothing in that branch caused or can
fix the breach.

**Opportunity** — the file has no shard boundary. `scripts/run-tests.sh` measures each file
against its own wall-clock budget and runs the files in parallel, so the suite's total wall clock
is pinned by its slowest single file. Splitting `test_sync_agents_runners` along a seam its 306
asserts already suggest — or moving a coherent group into an existing sibling shard — would bring
each part under its ceiling and shorten every suite run in the repo, including the build gate that
every `docket-implement-next` run pays twice.

**Independent value** — entirely independent of stacked changes. The breach predates change 0298
and outlives it: with that change reverted, the file still runs 3x over its ceiling and still sets
the suite's floor. The gain is paid back on every future build, review, and finalize gate.

**Boundary** — reshard `tests/test_sync_agents_runners.sh` (and its `tests/runtime-budgets.tsv`
rows) so each resulting file measures under its ceiling, preserving every existing assert and its
meaning. It deliberately leaves alone: the budget-checking machinery itself, the slack factor and
its one-machine calibration caveat (`scripts/run-tests.md`, change 0229), the `--strict-budget`
gating decision, and every other file's budget row.

**Reason for deferral** — the file is unrelated to the stacked-changes subsystem. Resharding it on
change 0298's branch would put a large, purely-infrastructural test refactor inside a feature diff
the human is reading for stacking semantics, and a resharding mistake would redden a suite that
change has no other reason to touch.
