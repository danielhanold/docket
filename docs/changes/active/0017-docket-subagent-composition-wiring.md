---
id: 17
slug: docket-subagent-composition-wiring
title: docket subagent composition — nested status/adr/critic dispatch
status: in-progress
priority: medium
created: 2026-06-15
updated: 2026-06-16
depends_on: [16]
related: [15, 16]
adrs: []
spec: docs/superpowers/specs/2026-06-16-docket-subagent-composition-wiring-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/docket-subagent-composition-wiring
pr:
blocked_by:
reconciled: true
---

## Why

0016 establishes the subagent wrappers and their pinned model/effort, but leaves
the internal sub-invocations running inline at the parent's model. Nested
subagents are confirmed supported (Claude Code ≥ v2.1.172, foreground, any
depth), so the composition can run each sub-invocation at *its own* configured
model — and, for `auto-groom`, give the adversarial critic a genuinely fresh
context. This change does that rewiring once 0016's foundation exists.

## What changes

Rewire three whole-skill sub-invocations to dispatch **named subagents** instead
of running inline at the parent's model — foreground, with git state (not an
in-context return) as the contract:

- **`implement-next`** dispatches the **`docket-status`** subagent (sonnet/medium)
  at step 0 and the **`docket-adr`** subagent (sonnet/medium) at step 6.
- **`auto-groom`** dispatches a dedicated **`docket-auto-groom-critic`** subagent
  (opus/xhigh) for the adversarial gate — real isolation, both ends pinned Opus, so
  the gate is not theater.

The critic is a new committed wrapper (`agents/docket-auto-groom-critic.md`) that
loads only `docket-convention` (never the designer skill); `sync-agents.sh` already
globs `agents/docket-*.md`, so it needs no generator edit (override key
`auto-groom-critic`). Also: complete the `docket-convention` composition section
(present tense) and bump `tests/test_sync_agents.sh` from 5 → 6 wrappers with critic
assertions. Design detail — the dispatch contract, depth analysis, ADR call — is in
the linked spec.

## Out of scope

- The foundation (wrappers, config, generator) — that is 0016 (`done`).
- The TDD build's model (`subagent-driven-development`'s own config).
- **Board-pass wiring** — making the inline board refresh that every status-writing
  skill runs dispatch the `docket-status` subagent (so the board renders at
  sonnet/medium regardless of caller). A real but separate optimization; captured as
  its own change.

## Reconcile log

### 2026-06-16 — reconcile (build claim)

Verified the spec's §9 reconcile notes against current `origin/main` (`f9cbd1f` —
unchanged since the spec was written 2026-06-16, so 0016's shipped reality still
holds). All assumptions confirmed; nothing dropped, adjusted, or folded in:

- `agents/` ships exactly the 5 wrappers, no critic — 0017 adds the 6th
  (`agents/docket-auto-groom-critic.md`).
- `sync-agents.sh` globs `agents/docket-*.md` and derives keys via `short_name`
  (line 49 strips `docket-` + `.md` ⇒ critic key `auto-groom-critic`); the
  generator needs no edit, the glob auto-discovers the wrapper.
- `tests/test_sync_agents.sh` hardcodes `= "5"` at lines 17 ("exactly 5 built-in
  wrappers") and 61 ("all 5 wrappers land in .claude/agents"); the `AUTONOMOUS`
  loop (L19–31) excludes the critic, so its assertions go in a separate block —
  matching spec §7.
- The three call sites are exactly where §5 expects: `implement-next` step 0
  (invoke `docket-status`), step 6 (invoke `docket-adr`); `auto-groom` step 3
  ("Dispatch a **fresh subagent**").
- `docket-convention` L60 carries the forward-pointer ("built in change 0017 …
  will spawn … Until 0017 lands … run inline") to convert to present tense; L37
  "Five skills get a wrapper" stays accurate (the composition paragraph introduces
  the 6th wrapper, which wraps no skill).
- ADR-0008 is `Accepted` (immutable except status) ⇒ touched only via a dated
  `## Update`, or a new ADR for the critic-isolation decision — decided at build
  (step 6).
- Related: #15 (finalize rebase/retest) still `proposed`, independent — no impact;
  #16 (foundation) `done`.
