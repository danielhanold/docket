---
id: 296
slug: shard-tests-test-docket-status-sh-its-runtime-row-is-at-the
title: 'Shard tests/test_docket_status.sh — its runtime row is at the table''s hard 60s ceiling'
status: proposed
priority: medium
type: chore
created: 2026-08-11
updated: 2026-08-11
depends_on: []
related: []
discovered_from: [118]
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

**Trigger** — surfaced while building change 0118, which added six sweep fixtures to
`tests/test_docket_status.sh`. Its `tests/runtime-budgets.tsv` row went 45 → 60 during the run, and
60 is the table's own hard ceiling ("NO row may exceed 60 seconds. If a file outgrows its ceiling,
shard it or move the new assertions into a shard with room. Raising a number here is not the
remedy."). The file is now AT that ceiling with no next raise available.

**Opportunity** — shard `tests/test_docket_status.sh` so each part sits comfortably under its own
ceiling. It is the single largest test file in the suite and owns coverage for several independent
concerns at once — the digest/backlog read path, the board pass, `sweep_execute` close-out
chaining, the change-0083/0118 publish-deferred marks, the change-0064/0084/0117 gating, health
checks, and reclaim. Those are natural shard seams, and no mechanism exists today to add sweep
coverage without breaching the ceiling.

**Independent value** — stands with 0118 reverted. The ceiling is a property of the file's size,
not of 0118's six fixtures: measured base (pre-0118) worst standalone serial was 41.75s against the
old 45s row, i.e. the file was already inside a few seconds of parity before this change touched
it. Every future change that needs sweep coverage benefits, and the suite's wall-clock improves
because a sharded file parallelizes.

**Boundary** — cut `tests/test_docket_status.sh` along its existing section seams into shards, give
each a `tests/runtime-budgets.tsv` row measured per the table's rounding rule, re-seed
`EXPECTED_TOTAL`, and update `tests/README.md`'s guidance on where a new `docket-status` test
belongs. It deliberately leaves alone: the assertions themselves (a shard is a move, not a rewrite),
`scripts/docket-status.sh`, and the two other files this run found over budget
(`test_docket_config`, `test_sync_agents` — see the note below, they are a separate concern).

**Reason for deferral** — sharding is a whole-file re-cut, which is the one edit that cannot rebase
cleanly past the changes already queued against this same file (#0268 and #0154 both target
`tests/test_docket_status.sh`). Doing it inside 0118 — a close-out marking change — would both
balloon that branch's scope far past its spec and guarantee a conflict for two queued changes. The
shard belongs to a change that can take the whole file.
