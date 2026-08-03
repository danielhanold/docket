---
id: 199
slug: harden-the-role-self-description-guard-co-occurrence-gap-har
title: Harden the role-self-description guard — co-occurrence gap, hardcoded population, broad default matcher
status: proposed
priority: medium
type: chore
created: 2026-08-03
updated: 2026-08-03
depends_on: []
related: []
discovered_from: [194]
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

`tests/test_role_skill_self_description.sh` (shipped by change 0194) is the negative guard stopping
docket-owned role skill bodies from re-asserting default status. Its whole-branch review found three
coverage gaps that are worth closing together, since they are all edits to the same small file.

1. **Co-occurrence gap (the important one).** A forbidden line must contain a `superpowers:` token
   AND a default word. So the most likely *post-0193* recurrence — "docket-build is the shipped
   default for the build role", naming no superpowers skill at all — passes silently. The guard's
   header discloses LINE-SCOPED and VOCABULARY-SCOPED limits but never this one, so a reader takes
   the remedy text ("never whether that binding is the shipped default") as the enforced
   specification when the condition is narrower than the sentence.
2. **Hardcoded population.** `ROLE_SKILLS="docket-build docket-review docket-brainstorm"` has no
   completeness anchor. A future `skills/docket-plan/` or `skills/docket-finish/` is silently
   uncovered — the guard passes without ever reading it. `tests/test_skill_size_budgets.sh` already
   solved this exact shape with an auto-discovery guard over `skills/**/*.md`.
3. **`default` is broad enough to false-positive.** A future line mentioning a `superpowers:` skill
   alongside "default" in a non-claim sense — the missing-skill degrade rule, or `SKILL_PLAN`
   defaulting when unset — trips the guard, and the remedy then tells the author to delete a claim
   that is not there. The conforming-line probe only covers a bare operational reference with no
   default word, so it does not cover this case.

## What changes

- Close the co-occurrence gap: either a second matcher (a default claim on a line naming the skill's
  own role noun, independent of the `superpowers:` token) with its own fire/ignore probe, or a third
  disclosed-limitation bullet naming it honestly.
- Derive the role-skill population, or assert that every existing `skills/docket-{brainstorm,plan,
  build,review,finish}` directory appears in `ROLE_SKILLS`.
- Tighten `WORDS` to claim-shaped phrasing (`the default|by default|instead of|alternative to|
  opt-in`) and add a probe asserting a legitimate `superpowers:` + "default" operational sentence
  does not fire.

Every added matcher needs its own synthetic fire-and-ignore probe — the guard's existing non-vacuity
discipline, not a new one.

## Out of scope

- The rule's positive half (docket-review naming no `skills.review`) — separate follow-up.
- Widening the guard to other skill files or to the whole `skills/` restatement class (#0154).
