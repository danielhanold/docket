---
id: 355
slug: build-review-roles-are-skill-invoked-that-fan-out-to-profile
title: 'Build/review roles are skill-invoked that fan out to profile agents — Step 5 ''dispatch'' vocabulary invites an agent-not-found misfire'
status: proposed
priority: medium
type: fix
created: 2026-08-26
updated: 2026-08-26
depends_on: []
stacked_on:
related: []
discovered_from: [351]
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

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

At the plan→build seam (Step 5 of `docket-implement-next`), the running skill tries to **dispatch
`docket-build` as a named agent** before falling back to invoking it as a skill:

```
⏺ docket-build(Build change 351 plan)
Error: Agent type 'docket-build' not found. Available agents: … docket-build-economy,
docket-build-max, docket-build-premium, docket-build-standard, …
```

It then self-corrects — "the build role is a skill I invoke inline, which then fans out to the
profile workers" — and proceeds via `Skill(docket-build)`. Observed live on the change 351 run
(discovered_from 351).

There is **deliberately no `docket-build` agent wrapper**: the build role is a skill that routes
each plan task to the profile workers (`docket-build-standard`/`-premium`/`-max`/`-economy`), which
*are* registered agents. The absence is by design, not a gap — so the fix is a wording/clarity
change, **not** adding an agent.

The misfire is invited by mixed vocabulary. `skills/docket-implement-next/SKILL.md:84` correctly
says the build skill is **"invoked"** (contrast: plan-writer/status/review say "dispatch … the
subagent") — but the *same paragraph* calls it "this long build **dispatch**" and frames the
Tier-C posture as "cannot **dispatch**," and the always-in-context "Docket agents — dispatch, don't
run inline" rule primes agent-dispatch as the default posture. The rule's own conditional already
covers this ("if no same-name agent is registered, do not invent one") — the model just failed to
check it against the muddy Step 5 wording.

Cost today is one wasted error round-trip that self-heals. The latent trap is worse: Step 5 makes a
build role that **cannot dispatch** a Tier-C **authorized-or-halt** condition ("any other resolved
value is abort-and-report"). A model that reads `Agent 'docket-build' not found` as the
cannot-dispatch trigger could **falsely halt a healthy run** instead of invoking the skill inline.
This run dodged it; the wording leaves it open, and it recurs every implement run.

## What changes

Make the build/review roles unambiguously **skills invoked inline that fan out to profile agents**,
so the model never tries a same-name agent dispatch and never mistakes "agent-not-found" for a
Tier-C cannot-dispatch halt. Candidate edits to evaluate at brainstorm time (not yet decided):

- **Step 5 vocabulary** (`docket-implement-next/SKILL.md`): drop the "build dispatch" phrasing for
  the build-role action; state plainly it is a skill invocation that fans out to the profile
  workers, and that there is no same-name `docket-build` agent by design.
- **Disambiguate the Tier-C trigger**: an "agent-not-found" for a role that is *invoked as a skill*
  is NOT the cannot-dispatch condition — the cannot-dispatch posture is resolved per the
  convention's *Dispatch-capability resolution*, never from a tool-name error.
- **Convention one-liner** (*Dispatch-capability resolution*): note which roles are agent-dispatched
  (plan-writer, status, review rungs, adr) vs skill-invoked-that-fan-out (build), so the seam is
  stated once in the shared contract rather than inferred per skill.

## Out of scope

- Adding a `docket-build` agent wrapper — the absence is intentional; build fans out to the profile
  workers. This change must not introduce one.
- The Tier-C authorized-or-halt safety invariant itself — clarify what does and does not *trigger*
  it, never relax it.

## Open questions

- Is the muddy vocabulary confined to Step 5, or should the same disambiguation land in the
  always-in-context "dispatch, don't run inline" rule (AGENTS.md / user CLAUDE.md) so it fires before
  the skill is even read?
- Do any other skill-invoked-that-fan-out roles share the same "dispatch"-worded seam and need the
  same treatment (e.g. a rebound `skills.build`/`skills.review`)?
