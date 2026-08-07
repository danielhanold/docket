---
id: 229
slug: the-runner-s-budget-slack-factor-is-a-hardware-dependent-con
title: the runner's budget slack factor is a hardware-dependent constant
status: killed
priority: medium
type: refactor
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [227]
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

Change 0227 added `scripts/run-tests.sh`, which enforces a per-file wall-clock budget from
`tests/runtime-budgets.tsv`. Budget rows are a claim about a file's cost measured **serially**,
but enforcement happens during a **parallel** run where every job competes for CPU. The gap is
bridged by a single hard-coded constant:

```bash
SLACK_NUM=5; SLACK_DEN=2   # breach = measured > ceiling * 5/2
```

That 5/2 was derived from one machine: measured contention inflation on the 0227 build host
(11 cores) peaked at **2.22x** (`test_render_board.sh` 18s -> 40s, `test_harness_defaults.sh`
39s -> 86s, `test_board_checks.sh` 48s -> 101s). It started at 3/2, which rejected 11 healthy
files and made the enforced suite exit 4; widening it to 5/2 is what made the gate green.

The constant is therefore load- and hardware-dependent in both directions. On a machine with
fewer cores relative to the job count, inflation exceeds 2.5x and the gate flakes red on healthy
files — and the documented escape is `--no-budget-check`, which disarms the guard entirely. On a
much larger machine, 2.5x slack makes enforcement nearly vacuous: a 60s-ceiling file would have
to reach 150s to breach.

## What changes

Settle a contention-independent basis for the budget check. Candidate approaches, to be chosen at
grooming:

- Enforce budgets only at `-j 1`, where the serial ceiling is the honest comparison, and have the
  parallel run report times without failing on them.
- Normalize each file's measured time by the run's own observed contention factor (e.g. the ratio
  of summed per-file time to wall time) before comparing against the ceiling.
- Scale the slack factor from the job count and the machine's core count rather than pinning it.

Whichever is chosen, mutation-test that the check still reddens on a file that genuinely doubles
its own serial cost — the regrowth the table exists to catch.

## Out of scope

- The budget table's contents and the completeness/relief-counter guard
  (`tests/test_runtime_budgets.sh`) — those are correct and independent of this.

## Open questions

- Is a wall-clock assertion in the merge gate the right instrument at all, or should budget
  enforcement be advisory in CI and hard only in a dedicated performance check?

## Why killed

Consolidated into #0251 at the 2026-08-07 backlog triage: both 0227-discovered budget-regime legs (slack factor + population floor) constrain each other and land as one design.
