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
spec:
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

To be settled at grooming. The shape the evidence points at:

- A **host-relative basis** so the table stops encoding one machine's absolute speed — e.g. a serial
  canary file measured at run time, with each ceiling compared after normalizing by the canary's
  ratio to its own calibration value. This keeps the property the table exists to catch (a file that
  doubles *its own* cost) while removing the property it accidentally encodes (the 0227 host was
  faster than today's).
- Whatever basis is chosen, **re-seed the rows once from a quiet machine** as part of the same
  change, and record the seeding conditions alongside them so the next drift is diagnosable rather
  than re-litigated.
- Mutation-prove that the check still reddens on a file that genuinely doubles its serial cost —
  the same bar 0229 set before it was consolidated.

## Out of scope

- The contention axis and the config-suite shard — both owned by #0251.
- The advisory-by-default posture and the 0/1/3/4 exit contract of `run-tests.sh`.
- Sharding any file. Sharding is the remedy for a genuinely re-shaped file, not for host drift.

## Open questions

- Is a wall-clock table the right instrument at all once the contention axis is fixed, or should
  absolute-speed drift be absorbed by a calibration step rather than encoded per-file?
- Does the canary approach hold on CI hardware, or does it need a recorded per-host calibration?
