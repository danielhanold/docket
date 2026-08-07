---
id: 230
slug: a-self-scanning-population-floor-pins-test-docket-config-sh
title: a self-scanning population floor pins test_docket_config.sh's size and blocks sharding
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

Change 0227 sharded the test suite's slowest files so no file exceeds a ~60s wall-clock ceiling.
`tests/test_docket_config.sh` (50s, the second-slowest file) could **not** be sharded, and the
blocker is a guard inside the file itself.

The change-0126 prelude-correspondence guard scans `${BASH_SOURCE[0]}` — its own file — and
asserts a whole-file population floor:

```bash
assert "0126 T: guard reached a real population (>= 60 sites)" '[ "$t_sites" -ge 60 ]'
```

against a corpus of 64 `eval "$…"` sites, with a derived cross-check
`t_sites == t_raw - t_helper - t_comments - t_selflit` computed over the same `BASH_SOURCE`. Any
two-way split leaves both halves near 32 and falsifies both assertions; a measured split produced
`sites=31` and one `NOT OK`. The guard is correct — a population floor is exactly the right
defence against a marker-scoped guard silently covering nothing — but it is written against a
single file, so it pins that file's size as a side effect.

This is not urgent: 50s is inside the ceiling, and `tests/runtime-budgets.tsv` plus
`tests/test_runtime_budgets.sh` will redden the moment the file outgrows its budget.
It is a decision worth making *before* that happens, because at that point the cheapest path back
to green will be raising a budget number — the exact evasion the budget guard exists to catch.

## What changes

- Decide what the prelude guard's population should be when its corpus spans several files:
  a floor asserted across the whole `test_docket_config*.sh` family rather than per file, a
  per-file floor scaled to the family's size, or an explicitly enumerated corpus.
- Rework `prelude_report` and the `r9_poison_site_line` derivation to accept a multi-file corpus.
- Then shard `tests/test_docket_config.sh` at a measured boundary, with assertion-count parity
  proven before and after, as change 0227 did for its other tail files.

## Out of scope

- Weakening or deleting the population floor. The floor is the point; only its unit of
  measurement is in question.

## Open questions

- Are there other self-scanning `${BASH_SOURCE[0]}` guards in the suite with the same
  file-size-pinning side effect? `tests/test_docket_config.sh` was found by attempting a split;
  nothing surfaces them proactively.

## Why killed

Consolidated into #0251 at the 2026-08-07 backlog triage: the population-floor rework is designed jointly with the slack-factor retune; headroom is tighter than filed (~5s at the 55s budget).
