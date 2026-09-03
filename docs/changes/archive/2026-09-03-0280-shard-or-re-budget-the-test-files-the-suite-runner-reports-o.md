---
id: 280
slug: shard-or-re-budget-the-test-files-the-suite-runner-reports-o
title: 'Shard or re-budget the test files the suite runner reports OVER BUDGET'
status: 'killed'
priority: medium
type: chore
created: 2026-08-09
updated: '2026-09-03'
depends_on: []
related: []
discovered_from: [276, 397]
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

**Trigger** — surfaced by change 0276's build gate. Every full-suite run during that build
closed with an `OVER BUDGET:` advisory naming ten to twelve files, consistently including
`test_sync_agents`, `test_sync_agents_runners` (227s against a 60s ceiling),
`test_sync_agents_drift_docs` (150s/55s), `test_sync_agents_defaults` (138s/50s),
`test_sync_agents_claude_surface` (119s/45s), `test_sync_agents_validator` (39s/15s),
`test_docket_config`, `test_board_checks`, `test_harness_defaults`, and
`test_harness_defaults_validator`.

**Opportunity** — the runner already computes and prints the breach and names its own remedy
("shard this file or extend an existing shard so each part stays under its ceiling"), but the
advisory does not fail the run, so nothing forces the shard and the breaches accumulate. The
work is to actually shard the offenders, or to re-derive ceilings that no longer describe the
files, so the advisory means something again.

**Independent value** — entirely independent of 0276: every one of these files was over budget
before that branch existed and stays over budget with it reverted. The payoff is wall-clock on
every build gate in the repo — the suite's tail is dominated by the `sync_agents` family, so
sharding it shortens every autonomous run's gate.

**Boundary** — shard or re-budget only the files the runner currently names; do not change what
any test asserts, do not merge or delete test files, and do not touch the budget-registry
mechanism itself (`tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`) beyond the rows
the sharding requires. A ceiling that is raised rather than met must carry a written reason.

**Reason for deferral** — 0276 ships a config knob and a convention section; sharding the
harness-sync test family is unrelated work touching a dozen files it never otherwise reads, and
folding it in would have made an already-large review diff unreviewable.

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): superseded by the Go migration — every named offender was a deleted Bash test; tests/runtime-budgets.tsv was re-seeded for the Go wrappers at change 0370.
