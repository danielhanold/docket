---
id: 14
slug: docket-auto-groom
title: docket-auto-groom — autonomous grooming drain over auto-groomable stubs
status: proposed
priority: high
created: 2026-06-12
updated: 2026-06-12
depends_on: []
related: [8, 9, 12]
adrs: []
spec: docs/superpowers/specs/2026-06-12-docket-auto-groom-design.md
plan:
results:
trivial: false
branch:
pr:
blocked_by:
reconciled: false
---

## Why

The build half of docket is already autonomous — `docket-implement-next` takes any
build-ready change to an open PR with no human — but the grooming half is not. Every
needs-brainstorm stub waits for an interactive `docket-groom-next` session, even when
the human would only click through the agent's recommended defaults. For repos where
the owner wants the agent to "just build," grooming is the human bottleneck. Change
0012 excluded autonomous spec writing deliberately ("the brainstorm stays interactive");
this change is that excluded sibling.

The brainstorm's key insight: no auto-*build* flag is needed. Build-readiness already is
build permission — once a stub is groomed to build-ready, the existing autonomous
builder consumes it. Autonomous grooming is the only missing capability; with it, "auto
mode" reduces to running auto-groom then implement-next, and the chaining is a thin
future orchestration change (with 0008).

## What changes

- A seventh operating skill, **`docket-auto-groom`**: a fully autonomous drain (not a
  "-next") that loops over eligible stubs — needs-brainstorm AND effective-auto-groomable,
  shared deterministic order — grooming each to spec or `trivial: true`, or abstaining,
  until the queue is empty; per-stub CAS commit+push with a riding Board pass, then a
  final report. Markdown only — never branches, worktrees, or code.
- **Trust model:** new `.docket.yml` knob `auto_groom: false` (repo default) + tri-state
  `auto_groomable:` change frontmatter override (unset = inherit). `docket-new-change`
  can set it at create time when the human provides rich context and says so.
- **Mechanism:** superpowers' brainstorming *reasoning* without its waiting-for-a-human
  protocol — a designer pass enumerates decision points and commits to conservative
  defaults, recording every choice in the spec's `## Assumptions` block; an adversarial
  critic subagent gates every build-ready exit (specs and trivial verdicts alike). A
  simulated-human auto-answerer driving `superpowers:brainstorming` was explicitly
  rejected as approval-gate theater.
- **Abstain path:** any needs-human-context decision ⇒ no spec; flip
  `auto_groomable: false` + dated `## Auto-groom blocked` body section (unresolved
  decision, missing context, any recommendation). Kill/defer are never autonomous —
  they surface as abstain-with-recommendation. Abstain doubles as the drain's
  termination/dedup guard; re-arm by supplying context and flipping the flag back.
- **`docket-groom-next` selection amendment:** interactive grooming still sees every
  needs-brainstorm stub but prefers the ones needing a human — abstained first, then
  non-auto-groomable, then auto-groomable last with an "auto-groom will handle it" note.
- **Convention + board:** `docket-convention` gains the shared vocabulary (knob, field,
  effective resolution, eligible queue, abstain rule, selection bands); the board shows
  abstained stubs as "auto-groom blocked — needs you."

## Out of scope

- Chaining groom → build in one autonomous run (future change, with/after 0008).
- Notifications or escalation delivery (0009 + cloud-routine layer; the abstain note is
  forward-compatible with it, not dependent on it).
- A repo-level "auto mode" umbrella beyond the single `auto_groom` knob.
- Any change to `docket-implement-next`.
- Autonomous kill/defer — permanently out by design.

## Open questions

- Critic transcript: keep only the surviving `## Assumptions` block, or also persist the
  critic's refutations somewhere for audit?
- Token/runtime budget per drain: cap the number of stubs per run, or always drain dry?

## Reconcile log
