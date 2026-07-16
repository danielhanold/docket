---
slug: size-target-is-direction
hook: "On a behavior-neutral slim the size target is a direction, not a gate — behavior-neutrality outranks hitting the number."
topics: [process, refactoring, review]
changes: [55]
created: 2026-07-11
updated: 2026-07-11
promotion_state: retained
promoted_to:
---

## Apply
On a behavior-neutral slim, the size target is a direction, not a gate — once review shows the
remaining lines are load-bearing, accept the size and stop trimming; behavior-neutrality outranks
hitting the number.

## War story
- 2026-07-11 (#55, PR #67) — A behavior-neutral skill slim landed all five files modestly over the
  spec's line-count targets, and review confirmed the residual was load-bearing/test-anchored content,
  not un-cut prose — the spec's size estimates were simply optimistic.
