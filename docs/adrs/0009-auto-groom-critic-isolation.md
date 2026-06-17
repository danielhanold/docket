---
id: 9
slug: auto-groom-critic-isolation
title: Auto-groom critic isolation — the adversary loads only the convention
status: Accepted
date: 2026-06-16
supersedes: []
reverses: []
relates_to: [8]
change: 17
---

## Context

`docket-auto-groom` designs a needs-brainstorm stub to build-ready by self-brainstorming
with a deliberate "commit to the conservative default" bias, then gates every
build-ready exit behind an adversarial critic. The gate is the only thing standing
between an autonomously-designed spec and the autonomous builder that will build it,
so it must be a *genuine* adversary — one that attacks the draft, not one that
re-derives and agrees with it.

The agent layer ([[0008]]) introduced model/effort-pinned wrapper files and named a
`(critic)` row whose materialization it deferred to change 0017. Materializing it
forced a concrete question: **what does the critic load, and at what tier?** Two
failure modes had to be designed out:

- **Self-agreement theater.** If the critic loads the `docket-auto-groom` designer
  skill body, it re-inherits the designer's commit-to-the-default bias — the model
  agreeing with itself while wearing a critic hat. The gate would look real and be
  hollow.
- **An under-powered adversary.** A critic running below the designer's tier cannot
  meaningfully challenge the designer's reasoning.

## Decision

The critic is a **dedicated committed wrapper** (`agents/docket-auto-groom-critic.md`)
that **wraps no skill**: it injects only `docket-convention` for shared vocabulary,
**never** the `docket-auto-groom` designer body. It is pinned **opus/xhigh** — at or
above the designer's tier, so the gate is not theater — and carries the standard
abort-and-report rule, where its "needs human context" verdict *is* the groom's
abstain. Its adversarial stance and the three-verdict protocol (sound /
wrong-but-fixable-in-one-bounded-round / needs-human ⇒ abstain) ride in the dispatch
prompt from `docket-auto-groom` Step 3, which stays the **single source** of the
critic's behavior; the wrapper adds only the pinned tier + convention + adversarial
stance + abort-and-report. Because it wraps no skill, its generator config key is
`auto-groom-critic` (the `sync-agents.sh` `agents/docket-*.md` glob + `short_name`
auto-discover it — no generator edit).

Rejected alternatives:

- **Reuse the `docket-auto-groom` wrapper for the critic** — injects the designer's
  bias straight into the adversary (the self-agreement failure mode).
- **Inline-spawn an unnamed fresh subagent** (0016's interim behavior) — inherits the
  parent's context/model, cannot portably pin *effort*, and hard-codes the tier into
  the skill body rather than config.

## Consequences

- **Enables** a genuinely independent adversarial gate: a fresh, isolated context with
  no designer bias, both ends pinned Opus.
- **A sixth generated wrapper that wraps no skill** — a deliberate exception to
  [[0008]]'s "every wrapper injects its skill via `skills:`". The exception is guarded
  in `tests/test_sync_agents.sh`: the critic is excluded from the per-wrapper loop that
  asserts `skills:` injects the same-named skill and `description` matches a skill body,
  and is separately asserted to inject `docket-convention` and to **exclude**
  `docket-auto-groom` (the isolation invariant).
- **The "five skills get a wrapper" count stays exact** — five *skills* wrap; the critic
  is a sixth *wrapper* attached to `auto-groom`. Conflating wrappers with skills is a
  known stale-count trap, so the count language distinguishes them.
- **Critic behavior stays single-sourced** in `auto-groom` Step 3 — the verdict protocol
  is not duplicated into the wrapper.
- **Cost:** a standalone committed artifact and a config key (`auto-groom-critic`)
  distinct from the skill it serves. Accepted as the only portable way to pin *effort*
  (not just model), keep it `.docket.yml`-overridable, and keep it clone-identical with
  the other wrappers.
