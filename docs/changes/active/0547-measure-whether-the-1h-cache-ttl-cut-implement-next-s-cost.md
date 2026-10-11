---
id: 547
slug: 'measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost'
title: 'Measure whether the 1h cache TTL cut implement-next''s cost'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-10-11'
updated: '2026-10-11'
depends_on: [546]
stacked_on:
related: []
discovered_from: [546]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

#546 ships a 1-hour prompt-cache TTL for the Claude `docket-implement-next` agent, on an estimate that it halves the agent's cost (~$5.35 → ~$2.60 per run). The estimate rests on Claude Code honouring `experimental.cacheTtl` for subagents, which an open Claude Code issue disputes, and the setting is ignored while a subscription draws on usage credits. A correct agent file proves nothing; only the transcripts show whether the saving is real.

This is a human-attended measurement, not a build. It needs 10–20 implement-next runs made after #546 is installed, so it's left as an ungroomed stub on purpose: the autonomous implementer must not pick it up before that data exists.

## What changes

After #546 is merged, installed, and the Claude Code session restarted, let 10–20 `docket-implement-next` runs accumulate, then run the script below (with the install date as the second argument) and compare against the baseline. Record the result in this change's results file.

**Baseline (95 Opus 5.5 runs, measured 2026-10-09) and expected values:**

| Metric | Baseline | Expected after |
|---|---|---|
| 1h share of cache-write tokens | 0% | ~100% (check first; if 0%, nothing else moves) |
| Expiry rewrites per run (turn writes >50k cache tokens, reads fewer) | ~4.9 | near 0 |
| Cache-write tokens per run | ~696k | ~100k |
| Cost per run (1h writes at $8/M) | $5.35 | ~$2.60 |
| Cost per turn | ~$0.08 | ~$0.04 |
| Cache-read tokens per run (should stay flat) | ~8.2M | ~8.2M |

**Reading the result:**

- Run size varies (about 30–115 turns), so judge on the multi-run average and on cost per turn, never one run.
- If cache reads per run moved a lot, the runs themselves changed and the comparison isn't clean.
- A few leftover rewrites are expected where a wait exceeded an hour; check the time gap before each.
- Mixed 5-minute and 1-hour writes across runs point to the plan drawing on usage credits at the time.
- The script's logged output tokens undercount (usage is recorded mid-stream); that's the same on both sides of the comparison.

**Guard checks:**

- Run the script for `docket-build-standard`, `docket-build-premium`, and `docket-plan-writer`: they must still show 0% 1h writes and unchanged cost per run (baselines $0.96, $2.23, $3.96). Any 1h writes mean the setting leaked.
- `run.verdict` outcomes and turns per run for implement-next should look the same as before.

**Outcome:** if the saving is confirmed, note it and close. If writes stay in the 5-minute bucket, record it as a Claude Code-side limitation and decide whether to try the broader `CLAUDE_CODE_SUBAGENT_PROMPT_CACHE_TTL` control, knowing it makes the workers 10–23% more expensive.

**Measurement script** (reads subagent transcripts on this machine; read-only):

```python
# usage: python3 cache_ttl_check.py [agentType] [since-ISO-date]
import json, glob, os, sys, collections
root = os.path.expanduser('~/.claude/projects/-Users-homer-dev-docket')
agent = sys.argv[1] if len(sys.argv) > 1 else 'docket-implement-next'
since = sys.argv[2] if len(sys.argv) > 2 else ''
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
    if not rows or first < since: continue
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

## Out of scope

- Changing any agent's model, effort, or cache setting; this only measures.
- Measuring finalize-change or the Sonnet trials listed as follow-ups in #546's spec.
- Adding the script to the repository or building a recurring cost report.
