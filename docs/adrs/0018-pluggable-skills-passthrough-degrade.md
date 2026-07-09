---
id: 18
slug: pluggable-skills-passthrough-degrade
title: Pluggable workflow skills — unvalidated skill-name passthrough + degrade-to-auto (not abort) on a missing skill
status: Accepted
date: 2026-07-08
supersedes: []
reverses: []
relates_to: [15]
change: 49
---

## Context

docket's workflow quality came from five superpowers skill invocations hard-coded into the
skill bodies (brainstorm/plan/build/review/finish). Change 0049 makes each rebindable via a
`skills:` map in `.docket.yml` (or the sentinel `auto` = do the step inline). Two design forces
had to be resolved: (1) how much docket validates a configured skill name; (2) what happens at
runtime when the resolved skill cannot be invoked (superpowers not installed, plugin
unavailable, a typo). docket's autonomous subagents normally follow an **abort-and-report**
rule for an unmet precondition.

## Decision

1. **Unvalidated passthrough.** A `skills:` value is a skill name passed verbatim to the Skill
   tool; docket never validates it against a registry — the same posture ADR-[[0015]] set for
   harness-portable agent model IDs. The passthrough is exactly what lets any third-party or
   in-repo skill plug in. Unknown *role keys* (not values) are warned-and-ignored (the
   `board_surfaces` posture).
2. **Degrade to `auto` + warn, NOT abort.** When the resolved skill cannot be invoked, the
   invoking skill degrades to that role's `auto` inline fallback and warns prominently (run
   output; and, for build-time roles, a note in the PR body) — deliberately softer than the
   autonomous abort-and-report rule, because skill availability is a **per-machine property**,
   not a repo-state error. Aborting would make docket unusable for exactly the no-superpowers
   users this change serves; degrading gives them zero-config out-of-the-box operation. The
   `auto` fallback defines only the final artifact / stop-point per role, never the method (no
   new model plumbing — inline runs at the already-selected model).

## Consequences

- Enables non-superpowers repos and arbitrary substitute skills to work with zero or minimal
  config; the cost is loss of early typo detection on a skill *value* (mitigated by the
  prominent runtime warning, and by the fact that an absent `skills:` map is byte-identical to
  prior behavior).
- Records the intentional divergence from abort-and-report so a future reader doesn't "fix" it
  into an abort.
- Relates to ADR-[[0015]] (passthrough philosophy) and change 0044 (its SDD build-model knobs
  stay inert unless `skills.build` resolves to SDD).
