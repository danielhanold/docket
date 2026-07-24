---
slug: skill-fallback-degrades-discipline
hook: "Read a skill fallback warning as a build-loop defect to investigate, never as boilerplate — a degraded binding silently drops the discipline."
topics: [skills, subagents, process]
changes: [66, 136]
created: 2026-07-13
updated: 2026-07-24
promotion_state: retained
promoted_to:
---

## Apply
Read a fallback warning as a build-loop defect to investigate, never as boilerplate — when a role
degrades, check whether the skill is installed in the harness the SUBAGENT runs in (not merely the
parent session), because a degraded binding silently drops the discipline while every artifact it
should have produced is still there.

## War story
- 2026-07-13 (#66, PR #73) — The entire build ran under the Skill-layer `auto` fallback:
  `superpowers:writing-plans`, `subagent-driven-development`, and `requesting-code-review` were not
  invocable inside the implementer subagent's session, so plan, build, AND review all degraded —
  correctly per the Missing-skill rule, and disclosed in the results file and PR body. The artifacts
  all still look right, but it means docket's own autonomous builds are not actually running the
  SDD/TDD/review discipline their `skills:` bindings name.
- 2026-07-24 (#136, PR #124 — re-hit) — `subagent-driven-development` degraded again, this time
  because the run's runtime exposed **no subagent-dispatch (Task) tool at all**, so SDD could not
  dispatch fresh implementer/reviewer subagents and review fell back to whole-branch self-review.
  Distinct trigger from #66 (skill-not-invocable-in-subagent) but the same class: the *dispatch*
  degrades whenever the harness lacks the machinery. The refinement worth keeping: degradation need
  not silently drop the discipline — here the agent ran the plan inline with SDD's TDD loop
  (test-first → verify-fail → implement → verify-pass → commit per task) and disclosed it in results
  + PR, so the *discipline* was preserved even though the *isolation/independent-review* was lost.
  The residual cost is the missing independent perspective, not skipped tests.
