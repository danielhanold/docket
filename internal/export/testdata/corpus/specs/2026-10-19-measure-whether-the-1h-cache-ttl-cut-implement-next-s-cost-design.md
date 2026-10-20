<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0547 — Measure whether the 1h cache TTL cut implement-next's cost](../../changes/archive/2026-10-19-0547-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost.md)**
<!-- docket:backlink:end -->

# Measure whether the 1h cache TTL cut implement-next's cost — design

Change #547, groomed 2026-10-18.

## Goal

Confirm from this machine's subagent transcripts whether the 1-hour prompt-cache TTL that #546 pins on `docket-implement-next` really halves that agent's cost per run, and that the pin did not leak to any other agent. The change ends in its results file (`deliverable: results-artifact`): no code, no branch work, no PR.

A first measurement on 2026-10-17 (13 runs) found the saving held: 100% 1h writes, cost per run $5.35 → $2.71. This run re-measures over every implement-next run made since then, and replaces that results file with the larger sample. It is also the first change to go through the results-artifact flow end to end.

## Method

1. Save the script below to a scratch location outside the repository (never commit it) and run it read-only. Nothing in the repository or the docket configuration changes.
2. **Window.** Start at `2026-10-12`: #546 merged and was installed on 2026-10-11, and the one run that day that predates the install is excluded by starting the next day. End at this run's own start time (its UTC claim stamp, `claimed_at:`), passed as the third argument, so the measuring run's own half-written transcript is not counted.
3. Run it for `docket-implement-next`, then for the guard agents `docket-build-standard`, `docket-build-premium`, and `docket-plan-writer`, all over the same window.
4. **Model.** The prices in the script are Opus 5.5's. Before trusting a cost figure, confirm the agent's pin in `~/.config/docket/config.yml` (the Claude `implement-next` pin was `claude-opus-5-5` when this was groomed). If an agent ran on another model in the window, say so in the results and price it with that model's rates.
5. Compare against the baseline table below, then write the results file.

### Measurement script

```python
# usage: python3 cache_ttl_check.py [agentType] [since-ISO] [until-ISO]
import json, glob, os, sys, collections
root = os.path.expanduser('~/.claude/projects/-Users-homer-dev-docket')
agent = sys.argv[1] if len(sys.argv) > 1 else 'docket-implement-next'
since = sys.argv[2] if len(sys.argv) > 2 else ''
until = sys.argv[3] if len(sys.argv) > 3 else '9999'
P = dict(inp=4, w5=5, w1=8, rd=0.20, out=20)  # Opus 5.5 $/M; edit for another model
t = collections.Counter()
for meta in glob.glob(root + '/*/subagents/*.meta.json'):
    if json.load(open(meta)).get('agentType') != agent: continue
    seen, rows, first = set(), [], None
    for line in open(meta.replace('.meta.json', '.jsonl')):
        d = json.loads(line); m = d.get('message') or {}
        first = first or d.get('timestamp', '')
        u = m.get('usage') if isinstance(m, dict) else None
        if not u or m.get('id') in seen: continue
        seen.add(m.get('id')); rows.append(u)
    if not rows or first < since or first >= until: continue
    t['runs'] += 1
    for u in rows:
        cc = u.get('cache_creation') or {}
        cw, cr = u.get('cache_creation_input_tokens', 0), u.get('cache_read_input_tokens', 0)
        t['turns'] += 1; t['cw'] += cw; t['cr'] += cr
        t['w1'] += cc.get('ephemeral_1h_input_tokens', 0); t['w5'] += cc.get('ephemeral_5m_input_tokens', 0)
        t['in'] += u.get('input_tokens', 0); t['out'] += u.get('output_tokens', 0)
        if cw > 50000 and cr < cw: t['rewrites'] += 1; t['rewrite_tok'] += cw
n = max(t['runs'], 1)
cost = (t['in']*P['inp'] + t['w5']*P['w5'] + t['w1']*P['w1'] + t['cr']*P['rd'] + t['out']*P['out']) / 1e6
print(f"{agent}: runs={t['runs']} turns/run={t['turns']/n:.0f}")
print(f"1h share of cache writes: {100*t['w1']/max(t['cw'],1):.0f}%")
print(f"expiry rewrites/run: {t['rewrites']/n:.1f} ({100*t['rewrite_tok']/max(t['cw'],1):.0f}% of write tokens)")
print(f"cache write/run: {t['cw']/n/1e3:.0f}k   cache read/run: {t['cr']/n/1e6:.2f}M")
print(f"cost/run: ${cost/n:.2f}   cost/turn: ${cost/max(t['turns'],1):.3f}")
```

The script counts a run's first turn as a "rewrite" (its initial ~80k write has no cache to read from). Count only rewrites after a turn other than the first as expiry rewrites, and check the time gap before each: a wait of more than an hour is an expected rewrite, not a failure.

## Baseline and expected values

Baseline: 95 Opus 5.5 implement-next runs, measured 2026-10-09, before #546.

| Metric | Baseline | Expected after | First measurement (13 runs, 2026-10-17) |
|---|---|---|---|
| 1h share of cache-write tokens | 0% | ~100% | 100% |
| Expiry rewrites per run | ~4.9 | near 0 | 0 |
| Cache-write tokens per run | ~696k | ~100k | 156k |
| Cost per run (1h writes at $8/M) | $5.35 | ~$2.60 | $2.71 |
| Cost per turn | ~$0.08 | ~$0.04 | $0.047 |
| Cache-read tokens per run (should stay flat) | ~8.2M | ~8.2M | 7.17M |

Guard baselines (cost per run, 0% 1h writes): `docket-build-standard` $0.96, `docket-build-premium` $2.23, `docket-plan-writer` $3.96.

## Reading the result

- Check the 1h share first. If it is 0%, nothing else moves, and the rest of the table is not worth reading.
- Run size varies (about 30–115 turns), so judge on the multi-run average and on cost per turn, never one run.
- If cache reads per run moved a lot, the runs themselves changed and the comparison isn't clean. Say so.
- Mixed 5-minute and 1-hour writes across runs mean the plan was drawing on usage credits at the time; name those runs.
- The logged output tokens undercount (usage is recorded mid-stream). That is the same on both sides of the comparison.
- Any 1h writes for a guard agent mean the setting leaked to it. A guard's cost moving with no change in cache bucket reflects a different mix of runs, not the pin.
- Also confirm implement-next's turns per run and its `run.verdict` outcomes look the same as before. The run tracker's verdicts for the window are enough; no new tooling.

## Outcome rules

- **Saving confirmed** (1h share ~100%, cost per run near $2.60–$2.80, guards at 0%): the results say so, and the `**Human action:**` line is `None`.
- **Writes back in the 5-minute bucket** (for all or most runs): record it as a Claude Code-side limitation, name when it started, and set the human action to deciding whether to try the broader `CLAUDE_CODE_SUBAGENT_PROMPT_CACHE_TTL` control, noting that it makes the build workers 10–23% more expensive.
- **A guard agent shows 1h writes**: the pin leaked. Record which agent and since when, and make that the human action.

## The results file

Write the results with `change.attach-results`, replacing the 2026-10-17 file. Use the standard results shape: the `**Human action:**` line, `## Outcome` with the measured table set against the baseline and the first measurement, the guard-check table, and `## Known issues and follow-ups` only if something real came up. State the window (start date, end time, run counts) and the exact commands run. Then close the change with `change.complete`.

Follow-ups go in the results for a human to capture. Never mint a change, ADR, or learning from this run. #546's own follow-ups (measuring `docket-finalize-change`, the Sonnet 5.5 trials) are not part of this change.
