---
id: 6
slug: autonomous-grooming-bounds
title: Autonomous grooming bounds — critic gates every build-ready exit; kill/defer never autonomous
status: Accepted
date: 2026-06-12
supersedes: []
reverses: []
relates_to: [4]
change: 14
---

## Context

Change 0014 added `docket-auto-groom`: an unattended drain that designs needs-brainstorm
stubs to build-ready. In docket, build-ready IS build permission — `docket-implement-next`
autonomously builds whatever is build-ready and stops only at the human merge gate. So
every exit the autonomous groomer can produce that yields build-readiness (a spec, or a
`trivial: true` verdict) directly feeds an autonomous builder with no human in between.
The design question was where autonomy must stop: which verdicts may an unattended agent
reach about the backlog, and what guards the verdicts it may reach? Two failure modes
dominated the brainstorm: a confidently-wrong default riding a plausible spec into an
unsupervised build, and an agent pruning work (kill/defer) the human still wanted. A
simulated-human auto-answerer driving `superpowers:brainstorming` was considered and
rejected as approval-gate theater — a subagent picking "the recommended option" is the
model agreeing with itself while faking a human checkpoint.

## Decision

Autonomous grooming has exactly three exits — **spec**, **trivial**, **abstain** — and two
bounds:

1. **An adversarial critic (a fresh subagent, never the designer reviewing itself) gates
   every build-ready exit.** Specs and trivial verdicts alike are emitted only if every
   design assumption survives the critic; any needs-human-context verdict aborts the whole
   groom into an abstain. A spec may only be emitted when every decision in it is safe to
   auto-commit, because emission = build-ready = the autonomous builder will build it.

2. **Kill and defer are never autonomous.** Verdict authority over the backlog's
   composition stays human; the strongest the drain may say is an abstain whose
   `## Auto-groom blocked` section carries the recommendation ("this should probably be
   killed/deferred because …"). Abstain itself is the safety valve: flip
   `auto_groomable: false` + record what was undecidable; the stub stays needs-brainstorm
   in the interactive queue.

## Consequences

- A wrong autonomous default must get past an independent adversarial pass before it can
  reach an unsupervised build; the residual risk is auditable via the spec's
  `## Assumptions` block at the merge gate.
- The backlog can never shrink without a human: obsolete stubs accumulate as
  abstain-with-recommendation until someone confirms the kill — the deliberate cost of
  making autonomous backlog pruning impossible.
- Abstains gate cleanly: each one removes the stub from the autonomous queue (also the
  drain's termination guarantee) and surfaces as "auto-groom blocked — needs you" on the
  board; re-arm = supply context, flip the flag, delete the blocked section.
- The critic adds a second model pass per stub — autonomous grooming costs roughly double
  an ungated design pass; accepted as the price of unattended build-readiness.
