---
id: 251
slug: retune-the-run-tests-budget-regime-for-portability-and-shard
title: 'Retune the run-tests budget regime for portability and sharding'
status: proposed
priority: medium
type: refactor
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [229]
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

Consolidates #0229 and #0230 (2026-08-07 triage): both discovered from 0227, both about the run-tests budget/sharding regime; a fix to one constrains the other.

Verified 2026-08-07:

- **Hardware-pinned slack factor (#0229).** `scripts/run-tests.sh:78` — `SLACK_NUM=5; SLACK_DEN=2`, consumed at `:306`: parallel wall-clock is compared against serially-measured budgets via a constant derived from one 11-core machine. Smaller hosts flake red; larger hosts make the check vacuous. The file's own comment at `:56` concedes the failure mode ("teaches people to pass `--no-budget-check`"). Note the flag set has moved since the stub: `--strict-budget` now exists (`:107-111`) — re-read the current levers before designing.
- **Self-scanning population floor pins the file (#0230).** `tests/test_docket_config.sh:2623` asserts the 0126 poison-prelude guard reached `>= 60` sites by scanning `${BASH_SOURCE[0]}` (`:2594`, cross-checks `:2615-2647`) — any file split falsifies the floor, so the guard blocks sharding the suite's biggest file (2769 lines, 55s budget in runtime-budgets.tsv, measured ~50s — ~5s headroom, closer than the stub's "not urgent" framing). It is the only file with this shape (`test_comment_anchor_style.sh:47` and `test_grep_portability.sh:34` use BASH_SOURCE for self-exclusion only).

## What changes

- Replace the fixed 5/2 slack with a portable regime — candidates from #0229: normalize by measured contention (e.g. re-measure a canary serially), derive slack from host parallelism, or demote the wall-clock check to report-only outside `--strict-budget`. Decide whether a wall-clock assertion belongs in the merge gate at all; if kept, it must not teach `--no-budget-check`.
- Rework `prelude_report`/`r9_poison_site_line` to a population unit that survives splitting `test_docket_config.sh` (family floor across shards / scaled per-file floor / enumerated corpus — pick one, then split the file with assertion-count parity).

## Out of scope

- Rewriting the budgets themselves (`runtime-budgets.tsv` values).
- Any non-config-suite shard.

## Open questions

- Both legs are genuine design forks with no stated house preference — an auto-groom abstain back to the human queue is acceptable.
