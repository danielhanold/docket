---
slug: skill-extraction-and-stub-pointer
hook: "Invoking a skill presents only its SKILL.md — extract only a section that is heavy AND off the common path, and leave a stub + pointer."
topics: [skills, docs, refactoring]
changes: [20]
created: 2026-06-17
updated: 2026-06-17
promotion_state: retained
promoted_to:
---

## Apply
Extract only a section that is heavy AND off the common path (opt-in, or its work is script-delegated —
like the GitHub mirror → `github-mirror.sh`); leave a stub + pointer under the original heading so
name-based cross-refs still resolve, and add a pointer in the one consumer that needs the mechanics.
Verify the MOVE by byte-diffing the sibling against the base section and mutation-testing each new grep
assertion.

## War story
- 2026-06-17 (#20, PR #33) — Invoking a skill presents only its `SKILL.md`; sibling files are NOT
  auto-loaded, so a section moved out for progressive disclosure leaves every consumer's context
  unless something Reads it.
