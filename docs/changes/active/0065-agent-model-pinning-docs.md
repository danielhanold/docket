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
spec: docs/superpowers/specs/2026-07-12-agent-model-pinning-docs-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-07-12-agent-model-pinning-docs-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-12-agent-model-pinning-docs-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0061 added `context: fork` + `agent: docket-<name>` to the four headless-safe skills so a direct invocation forks into the pinned wrapper and runs at its `model`/`effort`. It works — but two things about it are undocumented, and a user hit both on 2026-07-12.

**The fork is invisible.** A forked skill returns to the parent as a Skill tool result (`completed (forked execution)`); the TUI offers no expandable box to drill into, so you cannot watch the run. The full log *does* exist — Claude Code writes `~/.claude/projects/<slug>/<session-id>/subagents/agent-<id>.jsonl` — but nothing tells you that. The natural (wrong) conclusion is that the fork silently failed and the skill ran inline. Dispatching the wrapper **agent** instead (`@docket-status`) yields the identical pinned run *and* a drillable subagent, but that second invocation path is nowhere in the docs.

**0061 left an open question unverified.** Its spec asked whether `context: fork` composes with the wrapper's `skills:` preload — i.e. whether an agent that preloads the very skill that forks into it recurses or breaks. It was never tested; the change shipped on the assumption.

Both were settled empirically on 2026-07-12 with four probe skills on Claude Code 2.1.207 (see *What changes*), and the findings deserve the ADR ledger rather than a chat log.

The docs gap is also **wider than docket**. The per-agent `model`/`effort` pin is docket's most load-bearing and least understood feature: it is what lets a board refresh run on haiku while a build runs on opus/xhigh, in one session, without the human choosing a model. Most people using coding harnesses today still assume one session = one model, and pay opus prices for a merge sweep. docket's agent layer already solves this and the README barely says so.

## What changes

Documentation plus one ADR. No script, no schema, no skill behavior changes.

**1. ADR-0026 — accept fork opacity; two invocation paths; no tooling.** A new ADR (`relates_to: [8, 17, 20, 24]`) whose *decision* is the one the 2026-07-12 findings force: a forked run is unobservable in the TUI by design of the harness, and docket **accepts that rather than tooling around it** (no log-tailer, no progress protocol) — instead documenting two first-class invocation paths into the same pinned wrapper, plus the on-disk transcript path as the escape hatch. Its `## Context` carries the five verified findings (Claude Code 2.1.207, four probe skills + one live invocation): `context: fork` is honored through the Skill tool; the wrapper's `model`/`effort` pin holds inside the fork; the **self-preloading cycle is safe** (this closes 0061's open question); the fork is not drillable in the TUI; skills and agents register at **process start**, so a stale session runs old definitions — the failure mode that made a healthy fork look broken.

ADR-0024 is not edited, but gets a dated `## Update` note pointing forward to 0026 — the ADR index renders no back-links, so without it a reader of 0024 never learns its open question was closed.

**2. README: the two invocation paths.** Skill-invoke (`/docket-status` — forks, pinned, cheapest, opaque) vs agent-dispatch (`@docket-status` — same pin, drillable, costs a dispatch turn), when to reach for each, where the fork's transcript lands on disk (flagged as an observed harness internal, not a contract), and the restart-your-session caveat.

**3. README: per-agent model pinning as a first-class idea.** The general principle the agent layer embodies — match model tier and effort to the *task*, not the *session*; why a single-model session overpays or underthinks; how `agents:` expresses it. Teaching altitude, for a reader who has never considered that one session can span several models. Both README additions land as bold-lead paragraph blocks inside the existing `## Tuning agent models & effort` (that section uses no `###` headings).

**4. `references/agent-layer.md`** gets the mechanics in ~6 lines (both paths land on the same pinned wrapper), and `tests/test_skill_fork_dispatch.sh` gains positive-anchor doc sentinels so this prose cannot go stale unnoticed — the exact drift 0061's review caught.

Full design, the eight audited assumptions, and the file-by-file scope are in the linked spec.

## Out of scope

- Replacing `context: fork` with a thin-dispatcher SKILL.md, or any change to how skills are invoked — the mechanism works; this change only documents it.
- Any change to `sync-agents.sh`, wrapper generation, or the `agents:` schema.
- A helper script to tail a running fork's log (documenting the path is enough for now).
- Changing which skills are forked (the fork-exclusion principle from 0061 stands).

## Open questions

Both resolved at auto-groom (2026-07-12); the reasoning and rejected alternatives are audited in the spec's `## Assumptions` block.

- **Supersede or extend ADR-0024?** → **Extend.** 0024's decision is confirmed, not replaced, so ADR-0026 is parallel and additive — plus a dated `## Update` note on 0024, because the ADR index renders no back-links and a reader of 0024 would otherwise never find 0026 (spec A1).
- **README or a `docs/` guide?** → **README.** It is the repo's only prose doc on `main` (no guides tree exists), it already carries teaching-altitude sections, and a second home for agent-layer prose is precisely how this material drifted before (spec A3). ~540 lines is the accepted cost; extracting later is a pure move.

## Reconcile log
