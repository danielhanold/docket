---
id: 17
slug: docket-subagent-composition-wiring
title: docket subagent composition — nested status/adr/critic dispatch
status: proposed
priority: medium
created: 2026-06-15
updated: 2026-06-15
depends_on: [16]
related: [15, 16]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Why

0016 establishes the subagent wrappers and their pinned model/effort, but leaves
the internal sub-invocations running inline at the parent's model. Nested
subagents are confirmed supported (Claude Code ≥ v2.1.172, foreground, any
depth), so the composition can run each sub-invocation at *its own* configured
model — and, for `auto-groom`, give the adversarial critic a genuinely fresh
context. This change does that rewiring once 0016's foundation exists.

## What changes

Rewire the two composing skills to dispatch named subagents instead of inline
sub-invocations:

- **`implement-next` (opus/xhigh)** spawns the **`status`** subagent
  (sonnet/medium) at step 0 and the **`adr`** subagent (sonnet/medium) at step 6,
  so those run at their own models rather than inheriting Opus.
- **`auto-groom` (opus/xhigh)** spawns a **fresh `critic` subagent**
  (opus/xhigh) — real adversarial isolation, both pinned Opus, so the gate is not
  theater.

## Out of scope

- The foundation (wrappers, config, generator) — that is 0016.
- The TDD build's model (`subagent-driven-development`'s own config).

## Open questions

- Is `auto-groom`'s critic a distinct committed wrapper file
  (`agents/docket-auto-groom-critic.md`) or a variant the skill spawns by name
  with an inline model/effort? Resolve during grooming against 0016's actual
  artifacts.
- Confirm the nesting depth/foreground constraints hold for the real call sites
  (implement-next → status → its own sweep, etc.) and that no path needs a
  background subagent that would hit the depth-5 cap.
- Does spawning `status` as a nested subagent from `implement-next` change the
  board-refresh timing or commit ordering the current inline call relies on?

## Reconcile log
