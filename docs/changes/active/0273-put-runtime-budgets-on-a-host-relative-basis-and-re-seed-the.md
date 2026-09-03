---
id: 273
slug: put-runtime-budgets-on-a-host-relative-basis-and-re-seed-the
title: 'Put runtime budgets on a host-relative basis and re-seed the table'
status: proposed
priority: high
type: refactor
created: 2026-08-08
updated: 2026-08-09
depends_on: [251]
related: [251, 229]
discovered_from: [242]
adrs: []
spec: docs/superpowers/specs/2026-08-09-put-runtime-budgets-on-a-host-relative-basis-and-re-seed-the-design.md
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
| Spec | [2026-08-09-put-runtime-budgets-on-a-host-relative-basis-and-re-seed-the-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-put-runtime-budgets-on-a-host-relative-basis-and-re-seed-the-design.md) |
<!-- docket:artifacts:end -->

## Why

`tests/runtime-budgets.tsv` encodes each test file's serial wall-clock ceiling, and eight files now
measure **above their rows on an untouched merge-base**. A per-file budget table that breaches on
code nobody changed is a table nobody will read — every `OVER BUDGET:` trailer becomes noise, and
the documented posture (`AGENTS.md`: a trailing `OVER BUDGET:` line is a finding to act on) stops
being followable.

Measured during change 0242's build (2026-08-08), method recorded in
`docs/results/2026-08-08-close-the-claude-gap-caller-side-run-gate-results.md`:
`scripts/run-tests.sh -j 1 --no-budget-check --timings`, branch and merge-base (`487bfdc5`)
**interleaved** across four paired passes in both orderings so a drifting machine could not
masquerade as a diff.

- Absolute levels moved **20–25% between loaded and quiet passes on the same commit** —
  `test_sync_agents_codex.sh` measured 67s and 53s on `487bfdc5` hours apart.
- Breaching on the merge-base, i.e. not attributable to any branch: `test_sync_agents.sh` (59–60s
  vs row 55), `test_sync_agents_defaults.sh` (54–58s vs 55), `test_sync_agents_drift_docs.sh`
  (59–61s vs 55), `test_sync_agents_runners.sh` (61–76s vs 60), `test_board_checks.sh` (61s vs 55),
  `test_harness_defaults.sh` (50s vs 45), `test_harness_defaults_validator.sh` (53s vs 50) — a
  consistent ~10% overshoot on files that change never touched.

0242 acted on the one breach it caused (sharded `test_sync_agents_codex.sh`) and deliberately left
these alone: re-seeding a row upward because the machine got slower is the evasion
`tests/runtime-budgets.tsv` and `scripts/run-tests.md` both name — a ceiling moves when a file is
**re-shaped**, not when the host slows down. So the rows cannot simply be bumped, and the breaches
cannot simply be ignored. That tension is what this change is for.

**This is a different axis from #0251.** 0251 makes the **contention** comparison honest (the
parallel `5/2` screen demoted to a candidate filter, verdict by a serial re-run against
`ceiling * 3/2`). It explicitly puts rewriting the budget values out of scope, and parks the
absolute-speed problem as a residual in its spec's assumption 2 — "budget table values still encode
the calibration host's absolute speed … a serial-canary rescale is the named follow-up shape."
This change is that named follow-up. It should land **after** 0251, on top of the corrected regime.

### Corroborating evidence — a full serial sweep (2026-08-08)

A whole-suite **serial** run (`-j 1`, 96 files, 874s wall, on 0242's rebased branch) measured the
same overshoot directly, with no contention term to argue about. Every row below is a file this
change proposes to re-base; **none of them is a file 0242 touched**:

| file | row | serial | ratio |
|---|---|---|---|
| `test_board_checks.sh` | 55 | 69 | 1.25 |
| `test_sync_agents_runners.sh` | 60 | 73 | 1.22 |
| `test_docket_config.sh` | 55 | 67 | 1.22 |
| `test_harness_defaults.sh` | 45 | 54 | 1.20 |
| `test_sync_agents.sh` | 50 | 56 | 1.12 |
| `test_harness_defaults_validator.sh` | 50 | 55 | 1.10 |
| `test_sync_agents_defaults.sh` | 50 | 54 | 1.08 |
| `test_sync_agents_drift_docs.sh` | 55 | 58 | 1.05 |

The ratios cluster in a 1.05–1.25 band with no relation to what any file does — the signature of a
uniform host-speed difference, which is exactly what a host-relative basis cancels and what
per-file row edits cannot. The two files 0242 re-shaped land **under** their freshly-cut rows
(`test_sync_agents_codex.sh` 22 vs 30, `test_sync_agents_codex_dispatch.sh` 41 vs 50), and its new
`test_sync_agents_claude_surface.sh` measures 49 against a 45 row — ratio 1.09, mid-band, i.e.
indistinguishable from the untouched files and not a mis-cut row.

## What changes

Settled design (2026-08-09 auto-groom; detail in the linked spec, designed against #0251's
screen-then-serial-confirm regime and landing on top of it):

- **Run-time serial canary, no stored per-host state.** A dedicated workload script
  (`tests/lib/budget-canary.sh`, fork/git/IO-profiled, self-timing in milliseconds) is measured
  once per run when needed; `ratio = measured / calibrated` (calibration recorded as a structured
  comment in the TSV header) scales **both** the 5/2 screen and the 3/2 serial verdict at
  comparison time. Table values never change per-host; a file that doubles its own cost still
  breaches, host-speed drift cancels. Ratio honored inside a [0.5, 3.0] clamp; outside it the run
  falls back to ratio 1 with a named anomaly line and fail-closed `unconfirmed` candidates over
  the clamp-floor pre-screen set.
- **Lazy trigger.** The canary runs only when some measurement exceeds its clamp-floor-scaled
  threshold (`ceiling * 0.5 * slack`); comfortably-green runs pay nothing. A budgets table with
  no calibration line runs unscaled with one loud informational line (fixtures and downstream
  repos keep today's semantics); the repo's own line is pinned by `tests/test_runtime_budgets.sh`.
- **Whole-table re-seed, once.** Every row re-seeded from a quiet three-pass serial run on the
  build host, unchanged sizing rule; canary calibrated standalone (3x, median) in the same
  session so rows and calibration share one basis. Seeding conditions (date, host, method,
  calibration) recorded in the TSV header; `EXPECTED_TOTAL` re-seeded with "basis re-seed" named
  as a third legitimate move-case (only ever legitimate together with a same-diff canary
  recalibration).
- **Guards and proofs.** Mutation-proved in `tests/test_run_tests.sh`: 2x self-doubling still
  confirms at ratio ~1; a healthy near-ceiling file on a simulated 2x-slow host clears; anomaly
  path reports `unconfirmed` (exit 4 under `--strict-budget`, 0 at default); `--no-budget-check`
  runs no canary. Truncating fixed-point arithmetic (fail-closed); docs move in the same change
  (run-tests.sh comment block, run-tests.md budget sections, the stale "calibrated to one
  machine" caveat retired).
- Exit contract (0/1/3/4/2) and advisory-by-default posture byte-unchanged.

## Out of scope

- The contention axis and the config-suite shard — both owned by #0251.
- The advisory-by-default posture and the 0/1/3/4 exit contract of `run-tests.sh`.
- Sharding any file. Sharding is the remedy for a genuinely re-shaped file, not for host drift.

## Open questions

Resolved at grooming (2026-08-09 auto-groom, critic-gated: two rounds, 0 needs-human verdicts):
the table stays the instrument — per-file ceilings are what catch a file doubling its own cost —
with absolute-speed drift absorbed by the run-time canary rescale rather than encoded per-file
(the shape #0251's spec names as this follow-up). The CI question dissolves by construction: the
canary is measured fresh each run with no stored per-host state, so an ephemeral CI host
calibrates itself; the clamp band bounds pathological readings. Couplings: `depends_on: [251]`
(build order — this lands on 0251's confirm regime); no coupling to #0258 (different files).
- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Re-target: the budget regime lives in `internal/suiterunner` (budgets.go, aggregate.go) and `aggregate.go` still prints that the screening factor is calibrated to one machine. The spec's mechanics (`tests/lib/budget-canary.sh`, `run-tests.sh` exit codes, `tests/test_run_tests.sh`) and all eight measured files are deleted; re-measure against the Go wrapper rows in `tests/runtime-budgets.tsv`.

