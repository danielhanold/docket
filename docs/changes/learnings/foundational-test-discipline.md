---
slug: foundational-test-discipline
hook: "Sentinel greps are sampling, not parsing — pair them with a whole-branch review that reads for meaning."
topics: [testing, sentinels, review]
changes: [1, 2, 5, 13]
created: 2026-06-02
updated: 2026-06-12
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
