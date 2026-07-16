---
slug: skill-fallback-degrades-discipline
hook: "Read a skill fallback warning as a build-loop defect to investigate, never as boilerplate — a degraded binding silently drops the discipline."
topics: [skills, subagents, process]
changes: [66]
created: 2026-07-13
updated: 2026-07-13
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
