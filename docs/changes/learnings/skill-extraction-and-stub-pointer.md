---
slug: skill-extraction-and-stub-pointer
hook: "Invoking a skill presents only its SKILL.md — extract only a section that is heavy AND off the common path, and leave a stub + pointer."
topics: [skills, docs, refactoring]
changes: [20, 201]
created: 2026-06-17
updated: 2026-08-03
promotion_state: retained
promoted_to:
---

## Apply
Extract only a section that is heavy AND off the common path (opt-in, or its work is script-delegated —
like the GitHub mirror → `github-mirror.sh`); leave a stub + pointer under the original heading so
name-based cross-refs still resolve, and add a pointer in the one consumer that needs the mechanics.
Verify the MOVE by byte-diffing the sibling against the base section and mutation-testing each new grep
assertion. When the stub keeps a RULE, check that the rule's *rationale* landed somewhere too — an
extraction is only behavior-neutral if the why moved with the what.

## War story
- 2026-06-17 (#20, PR #33) — Invoking a skill presents only its `SKILL.md`; sibling files are NOT
  auto-loaded, so a section moved out for progressive disclosure leaves every consumer's context
  unless something Reads it.
- 2026-08-03 (#201, PR #155) — A behavior-neutral extraction round shipped with both of review's
  minor findings being the same defect: the RULE was preserved in the stub while its RATIONALE was
  dropped instead of relocated. `## Finalize blocked`'s named-id override kept "a named id
  overrides the skip" but lost the anti-deadlock reason it exists; the convention's
  never-a-mint-site clause kept the provable-termination invariant but lost the concrete
  `auto_groom` × `auto_capture` backlog-growth loop it prevents. Every grep sentinel stayed green,
  because sentinels are anchored on rule text — the "why" has no anchor, so its loss is invisible
  to the suite and shows up only in a reading review. When compressing, the split is stub = rule,
  reference = rule + why; "obvious from the rule" is exactly the judgment the next reader will not
  share.
