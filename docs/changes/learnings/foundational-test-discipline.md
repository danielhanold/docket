---
slug: foundational-test-discipline
hook: "Sentinel greps are sampling, not parsing — pair them with a whole-branch review that reads for meaning."
topics: [testing, sentinels, review]
changes: [1, 2, 5, 13, 137]
created: 2026-06-02
updated: 2026-07-27
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
