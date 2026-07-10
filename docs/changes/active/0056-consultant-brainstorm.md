---
id: 56
slug: consultant-brainstorm
title: Consultant-authored brainstorm — opt-in pinned design agent for the brainstorm role
status: proposed
priority: medium
created: 2026-07-10
updated: 2026-07-10
depends_on: []
related: [16, 17, 49]
adrs: [8, 9, 18]
spec: docs/superpowers/specs/2026-07-10-consultant-brainstorm-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-10-consultant-brainstorm-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-consultant-brainstorm-design.md) |
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0009](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0009-auto-groom-critic-isolation.md), [ADR-0018](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0018-pluggable-skills-passthrough-degrade.md) |
<!-- docket:artifacts:end -->

## Why

The brainstorm is the only load-bearing design work in docket that runs at whatever model
the session happens to be on: ADR-0008 left the interactive skills inline with an ignorable
advisory nudge, so on a cheap session the spec that feeds `docket-implement-next` is
cheap-model prose, and there is no way to run day-to-day sessions on a fast model while
fanning design thinking out to a high tier. The premise behind that decision — subagents
are fire-and-forget, so a brainstorm cannot leave the session — fell when the harness
gained agent continuation. (The recalled "convention reload is token-inefficient" argument
appears in no ADR and was never the blocker; ADR-0009's critic reloads the convention
routinely.)

## What changes

- New `skills/docket-brainstorm`: the consultant-author flow — a pinned consultant agent
  returns approaches + key questions (call 1), the parent runs the real human dialogue
  inline, the consultant authors the spec (call 2, continuation preferred / recap-dispatch
  fallback), the parent presents and writes it. Stops at the spec per the 0049 role
  contract.
- New `agents/docket-brainstorm-consultant.md`: ADR-0009-pattern wrapper — wraps no skill,
  injects only `docket-convention`, default opus/xhigh, config key `brainstorm-consultant`,
  auto-discovered by `sync-agents.sh`.
- **Off by default.** The built-in brainstorm role default stays
  `superpowers:brainstorming`. Two opt-in channels: per-invocation (the human asks for a
  consultant-written spec when running `docket-new-change`/`docket-groom-next`; one-line
  discoverability note in each) and durable (`skills: brainstorm: docket-brainstorm`).
- Degrade rule per ADR-0018: consultant undispatchable ⇒ inline at session model + warn.
- One ADR at build time recording the pattern as a refinement (not reversal) of ADR-0008.

## Out of scope

- `docket-auto-groom` (designer already pinned; critic already adversarial).
- The plan/build/review/finish roles; the advisory mechanism (stays as-is).
- Relay/ping-pong dialogue proxying; simulated-human answering (ADR-0006 boundary).
- Flipping the built-in brainstorm default to `docket-brainstorm` (possible later change).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
