---
name: docket-brainstorm
description: Docket-owned brainstorm role implementing the single-dispatch consultant-author flow — the parent runs the dialogue inline with the real human, then dispatches the pinned docket-brainstorm-consultant once to author a spec or return critique concerns. Bindable via `skills: brainstorm:` (the 0049 passthrough); invoked by docket-new-change / docket-groom-next in place of the default `superpowers:brainstorming`.
---

# docket-brainstorm — the consultant-author flow

## Overview

`docket-brainstorm` is an opt-in alternative to the built-in `superpowers:brainstorming`
role. It keeps the ADR-0006 boundary — the design dialogue stays with the real human,
inline, at whatever model the session runs — but adds one thing the built-in role
cannot: every build-ready spec is authored (or audited) by a pinned high-tier
consultant before it is written. The parent conducts the conversation; the consultant
fires **once**, at the end, as author-and-audit. No relay, no simulated human, no
follow-up agent turns anywhere — a single fresh dispatch, fully portable.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill
tool — unless it was already invoked earlier in this session and its content is in
context. `docket-brainstorm` is only ever invoked from `docket-new-change` or
`docket-groom-next`, whose own blocking Step 0 already loads `docket-convention`, so in
the normal case this is a no-op check, not a reload. Everything below uses convention
vocabulary (build-ready, the spec path, the metadata working tree, …) without
redefinition.

## Step 1 — Dialogue (inline, real)

The parent explores the idea with the human directly, one question at a time,
generating approaches and trade-offs itself at the session model. This step is
identical in spirit to `superpowers:brainstorming`'s own dialogue: no relay, no
auto-answerer, no subagent standing in for the human (ADR-0006). Keep going until the
design is settled enough to write down — the open questions resolved, the approach
chosen, the trade-offs named.

## Step 2 — Dispatch (author or critique)

Once the design is settled, dispatch the `docket-brainstorm-consultant` agent — a
single foreground call, in-context return, run at the model/effort its wrapper
resolves (no model or effort literal belongs in this skill; the pinned wrapper owns
that). Hand it:

- the settled design from Step 1's dialogue;
- the stub/idea being groomed;
- neighbouring changes (`related:`/`depends_on:`) and relevant ADRs;
- relevant `LEARNINGS.md` excerpts;
- the compact brief: the spec path and expected format, the PM-altitude boundary
  (design detail belongs in the spec; intent and scope stay in the change), and the
  requirement for an explicit `## Assumptions` section.

The consultant performs zero docket operations (no git, no status writes, no board)
and returns **in-context** exactly one of:

- **an authored spec** (markdown, ready to write), or
- **critique concerns** — the settled design has a hole the human must see.

On critique concerns, take them back to the human, resolve them in dialogue (back to
Step 1), and re-dispatch. This author-or-critique gate is what keeps the pinned tier
load-bearing even though the dialogue and option generation ran at the session model:
nothing becomes build-ready without pinned-tier sign-off.

## Step 3 — Present + write

Show the authored spec to the human. Change requests loop back as further dispatch
rounds (Step 2 again, with the requested changes folded into the brief). On approval,
write the spec to the configured spec path and **STOP AT THE SPEC** — the 0049 role
artifact/stop-point is unchanged. Do NOT continue to `superpowers:writing-plans`;
planning is build-time, owned by `docket-implement-next`.

## Degrade rule (ADR-0018)

If the consultant cannot be dispatched — agents not synced, harness without dispatch,
or any other per-machine unavailability — `docket-brainstorm` degrades to running the
whole flow inline at the session model, **WITH A PROMINENT WARNING** to the human that
the consultant audit was skipped. This is exactly today's behavior without this skill,
so the failure mode is "no worse than now." Agent/skill availability is a per-machine
property, not a repo-state error — never a hard abort.
