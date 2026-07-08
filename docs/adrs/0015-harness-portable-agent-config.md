---
id: 15
slug: harness-portable-agent-config
title: Harness-portable agent model config — direct model IDs, per-repo generation to an explicit harness list
status: Accepted
date: 2026-07-08
supersedes: []
reverses: []
relates_to: [8, 1]
change: 45
---

## Context

ADR-[[0008]] established the agent layer: `.docket.yml` `agents:` → `sync-agents.sh` →
committed `<repo>/.claude/agents/docket-*.md`, per-agent `model`/`effort`. Two of its
properties are load-bearing here:

- **Config values pass through unvalidated** (0008, consequence 4) — `sync-agents.sh` writes
  the `model:` string verbatim (`field_of` regex `[A-Za-z0-9._-]+`, no allowlist).
- **Project-level generation is `.claude/agents/`-only.** The *user-level* pass fans out to
  every present harness (`HARNESS_AGENT_DIRS` includes `~/.cursor/agents`, `~/.codex/agents`,
  …); the *per-repo* pass writes to `<repo>/.claude/agents/` alone.

A new force broke the reproducibility guarantee 0008 promised. docket is being run on
**non-Claude / mixed model rosters** — concretely, through **Cursor** at work, whose models
are arbitrary IDs (`gpt-5.5-medium-fast`), not Claude aliases. Two questions surfaced:

1. Should model selection be a docket-defined **tier vocabulary** (proposed change 0043:
   `critical`/`standard`/`economy` → concrete Claude models) or **direct model IDs**? A tier
   map bakes Claude-lineage assumptions into the config and is exactly wrong for an arbitrary
   external roster.
2. A committed per-repo `agents:` block **never reaches Cursor**, because project-level
   generation is `.claude/agents/`-only. So 0008's reproducibility guarantee does not hold
   for a Cursor user — the committed config silently applies to nothing Cursor reads.

Verified 2026-07-08 with a throwaway probe subagent: **Cursor honors an arbitrary
project-level `model:`** — set `gpt-5.5-medium-fast` in `<repo>/.cursor/agents/…`, Cursor's
model indicator ran that exact model. So 0008's unvalidated passthrough is not merely
tolerated; it is the **load-bearing enabler** of cross-harness portability.

## Decision

Two coupled rules, extending ADR-[[0008]]:

1. **Agent model values are direct model IDs, passed through verbatim — no tier layer.** The
   running harness interprets the string (a Claude alias/ID under Claude Code; a Cursor model
   ID under Cursor). docket neither defines, maps, nor validates model tiers. Change 0043's
   tier indirection is **rejected** (killed 2026-07-08). This promotes 0008's "unvalidated
   passthrough" consequence from a tolerated quirk to a deliberate design property.

2. **Per-repo (committed) generation fans out to an explicit harness list** —
   `.docket.yml` `agent_harnesses:` (global default `[claude]`). Each listed harness `H` gets
   committed `<repo>/.<H>/agents/docket-*.md`. Default `[claude]` is byte-identical to today
   (backward-compatible); a Cursor repo sets `agent_harnesses: [claude, cursor]`. **Explicit —
   chosen over present-directory auto-detection** — so generation targets are predictable and
   a stray `.cursor/` directory never silently starts minting committed agent files.

The same "direct model IDs, no tiers" vocabulary governs the reshaped change 0044 (`build:`
SDD-dispatch models); it is recorded here as the shared decision, with 0044's own mechanism
deferred.

## Consequences

- **Enables** running docket per-repo on any harness's model roster with 0008's reproducibility
  guarantee intact — committed, clone-identical files per listed harness.
- **Cost of dropping tiers:** no single "one lever" remap to move many agents at once; a
  whole-repo model change edits each agent entry. Accepted — tiers were Claude-specific and the
  remap convenience did not justify the coupling or the Claude lock-in.
- **`agent_harnesses` governs project-level generation only.** Open for change 0045's build:
  whether it also narrows the **user-level** fan-out (today "every present harness"). It must
  NOT silently stop writing `~/.cursor/agents/` for existing global-config users — the default
  and semantics must preserve current user-level behavior.
- **Silent-failure surface sharpens.** Some harnesses (Cursor) **ignore an unknown `model:`
  string** rather than erroring, so a wrong ID fails silently, running the harness default.
  This turns 0008's "emits an invalid wrapper" into "may silently run the wrong model on that
  harness"; docs must stress exact, harness-correct IDs.
- **The `--check` drift gate extends** to the newly-generated per-harness committed files.
- Implemented by change **0045** (per-repo multi-harness generation); the model-ID-not-tier
  half also governs the reshaped change 0044.
