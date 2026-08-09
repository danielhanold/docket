---
id: 251
slug: retune-the-run-tests-budget-regime-for-portability-and-shard
title: 'Retune the run-tests budget regime for portability and sharding'
status: proposed
priority: high
type: refactor
created: 2026-08-07
updated: 2026-08-09
depends_on: []
related: [258, 273]
discovered_from: [229, 230]
adrs: []
spec: docs/superpowers/specs/2026-08-07-retune-the-run-tests-budget-regime-for-portability-and-shard-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-retune-the-run-tests-budget-regime-for-portability-and-shard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-retune-the-run-tests-budget-regime-for-portability-and-shard-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0229 and #0230 (2026-08-07 triage): both discovered from 0227, both about the run-tests budget/sharding regime; a fix to one constrains the other.

Verified 2026-08-07:

- **Hardware-pinned slack factor (#0229).** `scripts/run-tests.sh:78` — `SLACK_NUM=5; SLACK_DEN=2`, consumed at `:306`: parallel wall-clock is compared against serially-measured budgets via a constant derived from one 11-core machine. Smaller hosts flake red; larger hosts make the check vacuous. The file's own comment at `:56` concedes the failure mode ("teaches people to pass `--no-budget-check`"). Note the flag set has moved since the stub: `--strict-budget` now exists (`:107-111`) — re-read the current levers before designing.
- **Self-scanning population floor pins the file (#0230).** `tests/test_docket_config.sh:2623` asserts the 0126 poison-prelude guard reached `>= 60` sites by scanning `${BASH_SOURCE[0]}` (`:2594`, cross-checks `:2615-2647`) — any file split falsifies the floor, so the guard blocks sharding the suite's biggest file (2769 lines, 55s budget in runtime-budgets.tsv, measured ~50s — ~5s headroom, closer than the stub's "not urgent" framing). It is the only file with this shape (`test_comment_anchor_style.sh:47` and `test_grep_portability.sh:34` use BASH_SOURCE for self-exclusion only).

## What changes

Settled design (2026-08-07 auto-groom; detail in the linked spec):

- **Budget leg — verdict by serial confirmation.** The parallel-phase `5/2` comparison is demoted
  from verdict to *screen*; a screened candidate is re-run once serially after the parallel phase
  and the breach verdict compares that serial re-measure against `ceiling * 3/2` (at `-j 1`, direct
  3/2 comparison, no confirm). The advisory-by-default posture and the 0/1/3/4 exit contract are
  unchanged; `--strict-budget` stays the opt-in gate, now honest across hosts on the contention
  axis. A failed confirm never clears a candidate (unconfirmed; breach under `--strict-budget`),
  and the confirm re-run never changes the suite pass/fail verdict. Mutation-proved in
  `tests/test_run_tests.sh` (a ~3x-cost fixture over an unpadded row confirms at any `-j`).
- **Floor leg — family-corpus guard, then the split.** `prelude_report`, the raw-grep cross-check
  extractors, and the r9 site derivation move to a glob-discovered corpus over
  `tests/test_docket_config*.sh` (computed membership, ADR-0050 shape; whole-corpus floors keep
  today's values; new `>= 2` files corpus floor; SITE lines gain file attribution). Then
  `tests/test_docket_config.sh` (2868 lines, ~50s vs a 55s budget) is split two ways at a measured
  section boundary with summed assertion-count parity, budget rows re-cut and `EXPECTED_TOTAL`
  re-seeded.
- Docs move in the same change: run-tests.sh comment block, run-tests.md budget sections,
  tests/README.md ("argued whole" paragraph + placement guidance), stale 0229/0230 references
  repointed to 0251, suite file counts corrected (88 today, 89 post-split).

## Out of scope

- Rewriting the budgets themselves (`runtime-budgets.tsv` values) beyond the mechanical re-cut of
  the split file's two rows.
- Any non-config-suite shard; no new default gating (advisory posture not revisited).

## Open questions

Resolved at grooming: the stub's "genuine forks with no house preference" read was too pessimistic —
run-tests.md and the budget learnings state the direction (contention-independent measurement so the
gate can be sharp again), and the critic passed the design with 0 needs-human verdicts across two
rounds. Coordination with #0258 (same test file) is at build time: whichever lands second rebases;
the glob corpus makes the guard indifferent to where 0258's asserts land. Residual (parked, in the
spec's assumption 2): budget table values still encode the calibration host's absolute speed —
confined to the opt-in strict path; a serial-canary rescale is the named follow-up shape if that
path proves flaky on slower hosts.
