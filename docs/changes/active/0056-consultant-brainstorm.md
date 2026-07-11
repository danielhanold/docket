---
id: 56
slug: consultant-brainstorm
title: Consultant-authored brainstorm — opt-in pinned design agent for the brainstorm role
status: implemented
priority: medium
created: 2026-07-10
updated: 2026-07-11
depends_on: []
related: [16, 17, 49]
adrs: [8, 9, 18, 22]
spec: docs/superpowers/specs/2026-07-10-consultant-brainstorm-design.md
plan: docs/superpowers/plans/2026-07-11-consultant-brainstorm.md
results: docs/results/2026-07-11-consultant-brainstorm-results.md
trivial: false
auto_groomable:
branch: feat/consultant-brainstorm
pr: https://github.com/danielhanold/docket/pull/68
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-10-consultant-brainstorm-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-consultant-brainstorm-design.md) |
| Plan | [2026-07-11-consultant-brainstorm.md](https://github.com/danielhanold/docket/blob/feat/consultant-brainstorm/docs/superpowers/plans/2026-07-11-consultant-brainstorm.md) |
| Results | [2026-07-11-consultant-brainstorm-results.md](https://github.com/danielhanold/docket/blob/feat/consultant-brainstorm/docs/results/2026-07-11-consultant-brainstorm-results.md) |
| PR | [#68](https://github.com/danielhanold/docket/pull/68) |
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0009](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0009-auto-groom-critic-isolation.md), [ADR-0018](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0018-pluggable-skills-passthrough-degrade.md), [ADR-0022](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0022-consultant-authored-brainstorm.md) |
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

- New `skills/docket-brainstorm`: the single-dispatch consultant-author flow — the parent
  runs the real human dialogue inline at the session model; once the design settles, one
  fresh pinned consultant dispatch either **authors the spec** or returns **critique
  concerns** for another human round (nothing becomes build-ready without pinned-tier
  sign-off). No `SendMessage`/continuation anywhere — fully harness-portable. Stops at
  the spec per the 0049 role contract.
- New `agents/docket-brainstorm-consultant.md`: wraps no skill and injects **no
  convention** (documented deviation from the ADR-0009 critic — the consultant authors
  prose, performs zero docket operations); a compact brief rides the dispatch prompt.
  Default opus/xhigh, config key `brainstorm-consultant`, auto-discovered by
  `sync-agents.sh`.
- **Off by default.** The built-in brainstorm role default stays
  `superpowers:brainstorming`. Two opt-in channels: per-invocation (the human asks for a
  consultant-written spec when running `docket-new-change`/`docket-groom-next`; one-line
  discoverability note in each) and durable (`skills: brainstorm: docket-brainstorm`).
- README documents the opt-in status **prominently** (top-level feature section), plus
  the capture-then-groom guidance: to run *all* portions of a brainstorm at a specific
  model, stub via `docket-new-change`, then run `docket-groom-next` from a session on
  that model — no new machinery, docs only.
- Degrade rule per ADR-0018: consultant undispatchable ⇒ inline at session model + warn.
- One ADR at build time recording the pattern as a refinement (not reversal) of ADR-0008
  and the wrapper's no-convention deviation from ADR-0009.

## Out of scope

- `docket-auto-groom` (designer already pinned; critic already adversarial).
- The plan/build/review/finish roles; the advisory mechanism (stays as-is).
- Relay/ping-pong dialogue proxying; pre-dialogue consultant analysis calls;
  `SendMessage`/continuation dependence; simulated-human answering (ADR-0006 boundary).
- Flipping the built-in brainstorm default to `docket-brainstorm` (possible later change).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- 2026-07-11 — Reconciled at claim. `depends_on: []`; related #16/#17/#49 and cited ADRs 8/9/18
  all present. Verified the agent-layer mechanism this change plugs into: `sync-agents.sh` rewrites
  ONLY the `model:`/`effort:` frontmatter lines and uses the built-in `agents/docket-*.md` file
  **verbatim** otherwise — so the consultant's "no skill, no convention" shape is achieved simply by
  authoring `agents/docket-brainstorm-consultant.md` with NO `skills:` line (no sync-agents.sh code
  change needed for that; the glob auto-discovers it). The existing `test_sync_agents.sh` no-skill
  loops enumerate FIXED lists (`docket-auto-groom-critic`; `docket-rebase-resolver
  docket-integration-repair`) that each assert `skills: docket-convention`, so they will NOT wrongly
  include the consultant — but a NEW test block is required asserting the consultant injects NEITHER
  a wrapped skill NOR `docket-convention` (the deliberate ADR-0009 deviation). The 0049 skill
  passthrough (`skills: brainstorm:`) and `$SKILL_BRAINSTORM` resolution are live. #0053's convention
  slimming is adjacent but NOT a dependency (spec §5). Scope unchanged.
