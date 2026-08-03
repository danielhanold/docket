---
slug: size-target-is-direction
hook: "On a behavior-neutral slim the size target is a direction, not a gate — behavior-neutrality outranks hitting the number."
topics: [process, refactoring, review]
changes: [55, 85, 201]
created: 2026-07-11
updated: 2026-08-03
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
- 2026-07-17 (#85, PR #95) — The second slimming round re-hit this on the same file, harder: after a
  full re-slim `docket-convention` landed at 288 L / 4,640 w against a ≤ ~200 / ≤ ~2,600 target — a
  ~44% line overshoot — while every other file landed at or under. Review again found the residual
  load-bearing: YAML blocks, tables, diagrams, and sentences pinned by test anchors. Two rounds now
  have set an aggressive number for this file and missed it, which is evidence the *estimate* is
  wrong rather than the execution: the convention has a compressibility floor its prose-heavy
  siblings do not. Set the next target from what the floor actually is, not from a word-count wish.
  The change also converted the direction into a real gate for regrowth
  (`tests/test_skill_size_budgets.sh` pins all 16 files at landed actuals + ~10%) — the honest move,
  since a budget keyed to the aspiration would have shipped red.
- 2026-08-03 (#201, PR #155) — Round three overshot on all four files (convention 5,773 w vs
  ~4,700; finalize 3,395 vs ~2,900; implement-next 3,654 vs ~2,900; build 2,348 vs ~2,100) and was
  accepted on the same rule. The third consecutive miss on `docket-convention` is the point: the
  floor is not a fixed word count to be re-estimated downward each round, it *rises with the
  subsystems the file governs* — round two's 4,640 w floor was measured before four more
  subsystems landed, so quoting it as round three's target was quoting a stale measurement. Set the
  next target by re-measuring what is sentinel-pinned and normative TODAY, and treat a round that
  moves prose to cold references as succeeding on the hot-path number (17,008 → 15,170 w, −10.8%)
  even when no per-file target is hit.
