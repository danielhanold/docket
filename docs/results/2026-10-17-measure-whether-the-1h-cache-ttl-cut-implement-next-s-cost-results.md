<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0547 — Measure whether the 1h cache TTL cut implement-next's cost](../changes/active/0547-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost.md)**
<!-- docket:backlink:end -->

# Measure whether the 1h cache TTL cut implement-next's cost — Results

**Human action:** None. The saving is confirmed and no setting needs to change.

## Outcome

Claude Code honours the 1h cache TTL that #546 pins on `docket-implement-next`, and the agent's cost per run dropped by about half, from $5.35 to $2.71.

Measured on 2026-10-17 with this change's script, `python3 cache_ttl_check.py docket-implement-next 2026-10-14`. That covers the 13 implement-next runs made after #546 was installed and the session restarted.

| Metric | Baseline | Expected | Measured (13 runs) |
|---|---|---|---|
| 1h share of cache-write tokens | 0% | ~100% | 100% |
| Expiry rewrites per run | ~4.9 | near 0 | 0 (see below) |
| Cache-write tokens per run | ~696k | ~100k | 156k |
| Cost per run | $5.35 | ~$2.60 | $2.71 |
| Cost per turn | ~$0.08 | ~$0.04 | $0.047 |
| Cache-read tokens per run | ~8.2M | flat | 7.17M |
| Turns per run | 30–115 | — | 58 |

Notes on reading it:

- Every one of the 13 runs wrote only to the 1h bucket, with no 5-minute writes. So no run was drawing on usage credits at the time.
- The script counts 1.0 rewrite per run, but each of those is the run's first turn: the initial ~80k write with no earlier cache to read from. None followed a wait, so there were no expiry rewrites. That one-time write is also why writes per run (156k) sit above the ~100k estimate.
- Cache reads per run (7.17M against 8.2M) and turns per run are close to the baseline, so the comparison is clean.
- A run on 2026-10-11 at 13:37 is excluded. It started before #546 was installed and the session restarted, and wrote 550k tokens to the 5-minute bucket only. Running the script with no start date mixes all 126 runs since the baseline and reproduces the old numbers (2% 1h, $5.93 per run). It is not a regression.

## Guard checks

Measured over the same period (since 2026-10-11 11:00):

| Agent | 1h share | Cost per run (baseline) |
|---|---|---|
| `docket-build-standard` (67 runs) | 0% | $0.57 ($0.96) |
| `docket-build-premium` (7 runs) | 0% | $1.06 ($2.23) |
| `docket-plan-writer` (12 runs) | 0% | $3.60 ($3.96) |

The setting did not reach other agents. Their costs per run came in below baseline with no change in cache bucket, so that reflects a different mix of runs, not the pin.

## Known issues and follow-ups

- None from this measurement. #546's follow-ups (measuring `docket-finalize-change` for the same TTL, the Sonnet 5.5 trials) still stand; a human may capture them as new changes.
