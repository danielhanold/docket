<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0547 — Measure whether the 1h cache TTL cut implement-next's cost](../changes/active/0547-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost.md)**
<!-- docket:backlink:end -->

# Measure whether the 1h cache TTL cut implement-next's cost — Results

**Human action:** None. The saving is confirmed over a larger sample, the pin did not leak to other agents, and no setting needs to change.

## Outcome

Claude Code honours the 1h cache TTL that #546 pins on `docket-implement-next`. Over 21 finished runs the agent's cost per run is $2.91, about 46% below the $5.35 baseline, and its cost per turn ($0.048) matches the first measurement ($0.047 over 13 runs).

**Window:** runs whose first transcript entry falls from `2026-10-12` up to `2026-10-19T02:20:00Z`. The end is set before the two implement-next runs still in flight when this was measured (this run, and the run building #560, which started at 02:23:29Z), so only finished runs are counted and the numbers reproduce. Counts: 21 `docket-implement-next`, 98 `docket-build-standard`, 14 `docket-build-premium`, 19 `docket-plan-writer`. Every assistant turn in the window ran on `claude-opus-5-5`, matching the pins in `~/.config/docket/config.yml` (implement-next: `claude-opus-5-5`, `cache_ttl: 1h`), so the script's Opus 5.5 prices apply.

| Metric | Baseline (95 runs) | Expected | First measurement (13 runs) | This measurement (21 runs) |
|---|---|---|---|---|
| 1h share of cache-write tokens | 0% | ~100% | 100% | 100% |
| Expiry rewrites per run | ~4.9 | near 0 | 0 | 0 (see below) |
| Cache-write tokens per run | ~696k | ~100k | 156k | 165k |
| Cost per run | $5.35 | ~$2.60 | $2.71 | $2.91 |
| Cost per turn | ~$0.08 | ~$0.04 | $0.047 | $0.048 |
| Cache-read tokens per run | ~8.2M | flat | 7.17M | 7.85M |
| Turns per run | 30–115 | — | 58 | 61 |

Notes on reading it:

- Cost per run sits slightly above the spec's $2.60–$2.80 band. Cost per turn is flat against the first measurement, and turns per run rose from 58 to 61, so the gap comes from somewhat longer runs, not from the cache. The spec says to judge on cost per turn and the multi-run average rather than one figure, and on that reading the saving holds.
- No implement-next run in the window wrote anything to the 5-minute bucket, so none was drawing on usage credits.
- The script reports 1.0 rewrite per run (47% of write tokens). A per-turn check found every one is the run's first turn, the initial write with no earlier cache to read. No implement-next turn after the first wrote more than 50k tokens with fewer reads, so there were no expiry rewrites.
- Cache reads per run (7.85M against 8.2M) and turns per run stay close to the baseline, so the comparison is clean.
- Run outcomes look normal: on the `docket` branch, 22 changes were claimed and reached `implemented` in the window (one of them, #553, after a halt and resume). The run tracker's local dispatch directory holds no records for the window, so the git history stood in for its verdicts.

## Guard checks

Same window. A guard writing to the 1h bucket would mean the pin leaked.

| Agent | Runs | 1h share | Cost per run (baseline / first measurement) |
|---|---|---|---|
| `docket-build-standard` | 98 | 0% | $0.67 ($0.96 / $0.57) |
| `docket-build-premium` | 14 | 0% | $1.34 ($2.23 / $1.06) |
| `docket-plan-writer` | 19 | 0% | $3.16 ($3.96 / $3.60) |

The setting did not reach other agents. Their cost movements come with no change in cache bucket, so they reflect a different mix of runs, not the pin. The few mid-run rewrites the guards show (one each for the two build tiers, two for the plan-writer) each followed a 5–9 minute gap, which is the ordinary 5-minute expiry those agents still have.

## Verification performed

- Commands, run read-only from a scratch directory outside the repository, with the script copied verbatim from the spec:
  - `python3 -I cache_ttl_check.py docket-implement-next 2026-10-12 2026-10-19T02:20:00Z`
  - the same for `docket-build-standard`, `docket-build-premium`, and `docket-plan-writer`.
- A second scratch script walked the same transcripts per run to list models per turn, any 5-minute writes per implement-next run, and every rewrite after a run's first turn together with the gap before it. Its core check, for re-running:

  ```python
  # per turn i>0: cw = cache_creation_input_tokens, cr = cache_read_input_tokens
  if i > 0 and cw > 50000 and cr < cw: report(run_start, i, cw, seconds_since_previous_turn)
  ```

- Run outcomes: `git log --since=2026-10-12 --format=%s` on the `docket` branch, reading the claim, implemented, and halt commits.
- Review: a `docket-review-standard` results-artifact review returned one important and two minor findings, all fixed here. Its first draft ended the window at this run's claim stamp, which still counted this run and the in-flight #560 run, because a transcript's first entry predates its claim. The window was moved back, every agent re-measured, and the run-outcome tally and this line corrected.
