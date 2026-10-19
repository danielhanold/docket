---
id: 547
slug: 'measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost'
title: 'Measure whether the 1h cache TTL cut implement-next''s cost'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-10-11'
updated: '2026-10-19'
depends_on: [546]
stacked_on:
related: []
discovered_from: [546]
adrs: []
spec: 'docs/superpowers/specs/2026-10-19-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost-design.md'
plan:
results: 'docs/results/2026-10-17-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
deliverable: 'results-artifact'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-19-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost-design.md](../../superpowers/specs/2026-10-19-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost-design.md) |
| Results | [2026-10-17-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost-results.md](../../results/2026-10-17-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost-results.md) |
<!-- docket:artifacts:end -->

## Why

#546 ships a 1-hour prompt-cache TTL for the Claude `docket-implement-next` agent, on an estimate that it halves the agent's cost (~$5.35 → ~$2.60 per run). The estimate rests on Claude Code honouring `experimental.cacheTtl` for subagents, which an open Claude Code issue disputes, and the setting is ignored while a subscription draws on usage credits. A correct agent file proves nothing; only the transcripts show whether the saving is real.

A first measurement on 2026-10-17 (13 runs) found the saving held. This change re-measures over the larger sample that has built up since, and is the first change to go through the results-artifact flow end to end: it ends in its results file, with no code and no PR.

## What changes

Run the read-only measurement in the spec over every `docket-implement-next` run since #546 was installed, plus the guard agents (`docket-build-standard`, `docket-build-premium`, `docket-plan-writer`) over the same window. Compare against the pre-#546 baseline and the first measurement, record the outcome in a fresh results file that replaces the 2026-10-17 one, and close the change with `change complete`.

The spec holds the script, the window, the baseline table, how to read the numbers, and what each outcome means for the human-action line.

## Out of scope

- Changing any agent's model, effort, or cache setting; this only measures.
- Measuring finalize-change or the Sonnet trials listed as follow-ups in #546's spec.
- Adding the script to the repository or building a recurring cost report.
