---
slug: foundational-test-discipline
hook: "Sentinel greps are sampling, not parsing — pair them with a whole-branch review that reads for meaning."
topics: [testing, sentinels, review]
changes: [1, 2, 5, 13, 137, 167]
created: 2026-06-02
updated: 2026-07-30
promotion_state: retained
promoted_to:
---

## Apply
Sentinel greps are sampling, not parsing — pair them with a whole-branch review that reads for meaning;
prove each assertion non-vacuous (deleting the clause it guards must flip the test to NOT OK); when order
is part of the contract, assert it explicitly rather than inferring it from presence; and build inline
when tasks share one artifact, fanning out only for genuinely independent work.

Richer, more specific restatements live in the `guards-are-code` and `green-suite-untested-branch`
findings.

## War story
- 2026-06-02–12 (#1, #2, #5, #13) — Foundational sentinel/test discipline (consolidated; richer,
  more specific restatements live in the guards-are-code and green-suite families above): sentinel
  greps are sampling, not parsing — pair them with a whole-branch review that reads for meaning; prove
  each assertion non-vacuous (deleting the clause it guards must flip the test to NOT OK); when order
  is part of the contract, assert it explicitly rather than inferring it from presence; and build
  inline when tasks share one artifact, fanning out only for genuinely independent work.
- 2026-07-27 (#137, PR #126) — The sharpest available demonstration that per-task mutation rounds
  cannot substitute for the whole-branch read. 21 mutations were run across four per-task rounds and
  **every one reddened**; the whole-branch review that followed still found **five green survivors**,
  each of which changed the shipped prose's meaning while the whole suite stayed PASS — including
  *inverting* the single most load-bearing sentence in the change (an assert keyed on two phrases
  appearing in order matched the inversion just as happily), and swapping the subjects of two rows in
  a table nothing cross-checked against its consuming sites. Rows 22–33 closed those five; a
  re-review of *those fixes* found two more. Each round's "all green-to-red" was true and
  insufficient. Coverage over the mutations you thought of is a claim about your imagination, not
  about the guard.
- 2026-07-30 (#167, PR #139) — What distinguishes a review that finds things is **stance**, not
  count. Five task-scoped reviews *and* a whole-branch review all passed; a separate independent
  whole-branch review then found five Important contract defects none of them could see. Every one
  was an off-happy-path **disposition** gap — the contract stated a predicate ("a task without a
  commit is not complete") where it owed an **action**. The difference in method was the whole
  difference: the earlier reviews read the skills as **grep targets**, the one that found the defects
  read them as an **operator executing from a cold start**, which is the only stance under which a
  missing disposition is visible at all. Incidental but pointed: this was the review topology the
  change itself was arguing for, so the change validated its own thesis mid-build.
