---
slug: stacked-gap-regex-hangs-instead-of-failing
hook: "Two or more bounded gaps in one ERE backtrack catastrophically on NON-matching input — the mutation test hangs instead of reddening, so the guard looks slow rather than broken."
topics: [testing, shell-portability, guards]
changes: [226]
created: 2026-08-07
updated: 2026-08-07
promotion_state: candidate
promoted_to:
---

## Apply
A phrase-grep over wrapped prose reaches for a bounded gap — `a[^.]{0,160}b` — to span a line
break. Stacking two of them (`a[^.]{0,160}b[^.]{0,60}c`) is the natural extension when the claim has
three parts, and it is a trap:

- On **matching** input it returns immediately, so it looks correct in every green run.
- On **non-matching** input the engine must try every split of the input across the two gaps before
  it can report failure. That is quadratic in the gap bounds, and under ugrep it runs for minutes.

The non-matching case is exactly the mutated file — the input the assert exists to catch. So the
failure mode is not a red test, it is a **hang**, which reads as a slow suite rather than a broken
guard, and the mutation test gets abandoned or mis-scored rather than corrected.

**Use one bounded gap per pattern.** When a claim needs three anchored parts, write two separate
asserts, each with a single gap, or scope the match to a structural unit (a table cell via `[^|]`,
a line, a marker-bounded block) so no gap is needed at all.

This is invisible to the portability suite. `tests/test_grep_portability.sh` guards the BSD ceiling
— a single repetition bound above 255 — and both bounds here are well under it. Two legal bounds
compose into an illegal cost, and nothing checks the composition. Related:
[[phrase-grep-over-wrapped-prose]] (which motivates the gap in the first place) and
[[guards-are-code]].

## War story
- 2026-08-07 (#226, PR #168) — A guard added for a three-part convention claim used two stacked
  bounded gaps. Against the real file it matched instantly and the suite was green. During the
  mutation test — the run where the guarded clause is deleted and the assert must go red — it did
  not fail; it hung for minutes with no output, and the reading was initially misattributed to a
  slow suite rather than to the pattern. Every pattern the branch added was rewritten to a single
  bounded gap. Two other test files (`tests/test_dispatch_capability.sh`,
  `tests/test_docket_build.sh`) were found to carry the same shape and are outside the change's
  diff, so the sweep was auto-captured as change #233. Note the asymmetry that makes this durable:
  the defect is *unobservable* in the passing direction, and the only run that exposes it is the
  one a hurried build is most likely to skip.
