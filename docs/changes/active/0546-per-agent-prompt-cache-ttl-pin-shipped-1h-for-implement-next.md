---
id: 546
slug: 'per-agent-prompt-cache-ttl-pin-shipped-1h-for-implement-next'
title: 'Per-agent prompt-cache TTL pin, shipped 1h for implement-next'
status: 'proposed'
priority: 'high'
type: 'feat'
created: '2026-10-09'
updated: '2026-10-09'
depends_on: []
stacked_on:
related: [195]
discovered_from: []
adrs: [15, 64, 77]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0015](../../adrs/0015-harness-portable-agent-config.md), [ADR-0064](../../adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md), [ADR-0077](../../adrs/0077-orphan-effort-dropped-as-docket-policy-not-vendor-constraint.md) |
<!-- docket:artifacts:end -->

## Why

A usage audit of the docket subagent transcripts on this machine (2026-10-09) showed that `docket-implement-next` costs the most of any docket agent: about $5.35 per run and $508 across 95 Opus 5.5 runs. Two-thirds of that is prompt-cache writes, not model output, and **87% of those writes are the cache expiring**. Claude Code subagents write cache with the 5-minute TTL. implement-next sits idle longer than that while a build worker or the test suite runs, so its whole ~120k-token context is written again at the full write price, 461 times across those runs.

Claude Code has a per-agent fix: `experimental: { cacheTtl: 1h }` in an agent's frontmatter. Docket can't emit it today. The generated wrappers carry only `model` and `effort`, so a hand edit to `~/.claude/agents/docket-implement-next.md` is wiped by the next install. With the 1-hour TTL, implement-next is estimated at ~$2.60/run, about half its current cost, with no change of model and no quality risk.

The build workers and plan-writer rarely expire (5–24% of their writes), and 1-hour writes cost 2× input instead of 1.25×, so turning it on everywhere would make them 10–23% more expensive. The pin has to be per agent.

## What changes

- A new optional per-agent field, `cache_ttl`, beside `model` and `effort` in the agent pin table: in `agents/harness-defaults.yml` and in the global config's `agents.<harness>.<agent>` entries. It follows the same layering as the other pin fields: built-in, then global, per field.
- The Claude Code adapter renders it into the generated wrapper as `experimental:` / `cacheTtl: <value>`. When it's unset, the frontmatter is the same as today.
- Docket ships `cache_ttl: 1h` for the claude `implement-next` entry only.
- A `cache_ttl` set for a harness with no such setting (cursor, codex, opencode) is left out of that harness's files with a warning, the same way an unattributable effort is dropped.
- Docs: the agent-layer reference documents the key, the shipped default, why it's scoped to implement-next, and the other Claude Code controls it deliberately doesn't use. The full research findings live in the spec.

## Out of scope

- Changing any agent's model or effort. Sonnet 5.5 for build-standard and a context diet for plan-writer are recommendations recorded in the spec for separate changes.
- Turning on the 1-hour TTL for any other agent, including finalize-change, until its transcripts are measured.
- Using the global subagent controls (`subagentPromptCacheTtl`, `CLAUDE_CODE_SUBAGENT_PROMPT_CACHE_TTL`, `ENABLE_PROMPT_CACHING_1H`). They're documented, not used.
- Any keep-warm or ping mechanism, which Claude Code doesn't document.
