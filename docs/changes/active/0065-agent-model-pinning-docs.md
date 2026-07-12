---
id: 65
slug: agent-model-pinning-docs
title: Document the two invocation paths and per-agent model pinning; ADR the context:fork findings
status: proposed
priority: medium
created: 2026-07-12
updated: 2026-07-12
depends_on: []
related: [16, 45, 46, 61]
adrs:
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
issue:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0061 added `context: fork` + `agent: docket-<name>` to the four headless-safe skills so a direct invocation forks into the pinned wrapper and runs at its `model`/`effort`. It works — but two things about it are undocumented, and a user hit both on 2026-07-12.

**The fork is invisible.** A forked skill returns to the parent as a Skill tool result (`completed (forked execution)`); the TUI offers no expandable box to drill into, so you cannot watch the run. The full log *does* exist — Claude Code writes `~/.claude/projects/<slug>/<session-id>/subagents/agent-<id>.jsonl` — but nothing tells you that. The natural (wrong) conclusion is that the fork silently failed and the skill ran inline. Dispatching the wrapper **agent** instead (`@docket-status`) yields the identical pinned run *and* a drillable subagent, but that second invocation path is nowhere in the docs.

**0061 left an open question unverified.** Its spec asked whether `context: fork` composes with the wrapper's `skills:` preload — i.e. whether an agent that preloads the very skill that forks into it recurses or breaks. It was never tested; the change shipped on the assumption.

Both were settled empirically on 2026-07-12 with four probe skills on Claude Code 2.1.207 (see *What changes*), and the findings deserve the ADR ledger rather than a chat log.

The docs gap is also **wider than docket**. The per-agent `model`/`effort` pin is docket's most load-bearing and least understood feature: it is what lets a board refresh run on haiku while a build runs on opus/xhigh, in one session, without the human choosing a model. Most people using coding harnesses today still assume one session = one model, and pay opus prices for a merge sweep. docket's agent layer already solves this and the README barely says so.

## What changes

**1. An ADR recording the verified `context: fork` behavior** (Claude Code 2.1.207, four probe skills + one live in-session invocation):

- `context: fork` **is honored**, including when the skill is reached through the Skill tool — a real subagent is spawned with its own `subagents/agent-<id>.jsonl` and `agentType` metadata.
- The wrapper's **`model`/`effort` pin is honored** inside the fork (haiku-pinned wrappers ran at `claude-haiku-4-5` under an opus/sonnet parent).
- The **self-preloading cycle is safe** — an agent whose `skills:` preloads the skill that forks into it neither recurses nor degrades to inline. This closes 0061's open question.
- A forked run is **not drillable in the TUI**, unlike an Agent-tool dispatch; its log is only reachable on disk.
- Skills and agents are registered at **process start**, so a session that predates a frontmatter change runs the old definition — the failure mode that made the fork look broken.

**2. README: the two invocation paths.** Document skill-invoke (`/docket-status` — forks, pinned, cheapest, opaque) vs agent-dispatch (`@docket-status` — same pin, drillable, costs the dispatch turn), when to reach for each, and where the fork's log lands on disk.

**3. README: per-agent model pinning as a first-class idea.** Expand beyond docket-specific mechanics into the general principle the agent layer embodies — matching model tier and reasoning effort to the task rather than to the session, why a single-model session overpays or underthinks, and how `agents:` in `.docket.yml` expresses it. This is teaching material, not reference: assume the reader has never considered that one session can span several models.

## Out of scope

- Replacing `context: fork` with a thin-dispatcher SKILL.md, or any change to how skills are invoked — the mechanism works; this change only documents it.
- Any change to `sync-agents.sh`, wrapper generation, or the `agents:` schema.
- A helper script to tail a running fork's log (documenting the path is enough for now).
- Changing which skills are forked (the fork-exclusion principle from 0061 stands).

## Open questions

- Does the ADR supersede or merely extend ADR-0024 (`claude-context-fork-skill-dispatch`)? Leaning extend — 0024's decision holds; this adds verified consequences and the observability caveat.
- Does the model-pinning explainer belong in README, or in a `docs/` guide that README links to, given README length?

## Reconcile log
