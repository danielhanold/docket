---
id: 79
slug: shim-wrapper-frontmatter-pin-governs-the-parent-side-agent
title: A shim wrapper's frontmatter pin governs the parent-side agent
status: Accepted
date: 2026-08-08
supersedes: [38]
reverses: []
relates_to: [15, 67]
change: 269
---

## Context

ADR-0038 established the runner shim wrapper as the single dispatch chokepoint, and stated that a
shim's frontmatter `model:` line is kept "for bookkeeping — the effective pin is the baked
argument," listing byte-stability with the native wrapper shape among its accepted consequences.

That premise is false in practice. `sync-agents.sh`'s `emit_shim` built the shim's frontmatter from
the resolved CHILD model/effort, but the shim agent runs in the PARENT harness (Claude Code) and
does exactly one thing: a foreground `docket.sh runner-dispatch` call plus a stdout relay. Claude
Code reads that frontmatter line as the live pin for the shim agent, so it tries to run the relay on
a model it cannot resolve and the run dies with a bare harness error that never names the runner.

ADR-0067 (a runner-bearing agent must carry a user-configured model) makes this unavoidable rather
than incidental: the child's ID is always present to be copied into the wrong slot, so every
runner-delegated claude wrapper was born broken. Observed 2026-08-08 during a
`docket-implement-next 258` run, with wrappers pinned to
`openrouter/deepseek/deepseek-v4-flash-0731`.

## Decision

A shim wrapper's frontmatter pin governs the PARENT-side shim agent and must be resolvable by the
parent harness — it is never bookkeeping and never the child's model. The child's model and effort
ride only the baked `--model` / `--effort` arguments to `runner-dispatch`.

Two per-runner config keys govern the shim's own pin:

- `runners.<name>.shim_model` — default `inherit`, meaning emit no frontmatter model line and let
  the parent harness inherit the session model.
- `runners.<name>.shim_effort` — default `low`, the shim being a relay.

Both are validated as bare YAML scalars at generation time, before any wrapper is written.
Defaulting `shim_model` to `inherit` repairs every existing broken wrapper by regeneration alone,
with no config edit required.

## Consequences

Runner delegation works on the claude harness for the first time.

Byte-stability with the native wrapper shape — an accepted consequence of ADR-0038 — is given up
deliberately: a shim's frontmatter now differs from the delegated child's pin by design, and the two
must never be assumed equal.

The `shim_model`/`shim_effort` keys add a cost-optimization surface (pin the relay to a cheap model)
on top of the correctness fix, not as a prerequisite.

A second consumer of the `runners:` config block is knowingly added rather than unified; change 0256
should absorb both parsers.

The test that encoded the false premise (`shim keeps frontmatter model (bookkeeping)`) is inverted
into its regression assert: a shim's frontmatter model is never the value baked into `--model`.
