---
id: 203
slug: define-the-per-step-git-state-postcondition-docket-implement
title: Define the per-step git-state postcondition docket-implement-next now names but never states
status: proposed
priority: medium
type: docs
created: 2026-08-03
updated: 2026-08-03
depends_on: []
related: []
discovered_from: [113]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0113's prose rider added this clause to `docket-implement-next` §5:

> the step is not complete until its git-state postcondition holds

`git-state postcondition` occurs exactly once across `skills/` — in that new sentence. Step 5 states
no postcondition to hold, and neither does the *Terminal disposition* section nor
`docket-convention`. An agent that already misread this step is now told to satisfy a named
condition with no referent, which is weaker than the enumerated obligations around it.

The gap is worse where it matters most. Both observed incidents stopped at the **Step 4/5 seam** —
Step 4 is where the plan file is written, committed, and `plan:` recorded — and Step 4 received no
rider at all. Its closest thing to a postcondition is the prose "Record the plan path in `plan:` per
the field-write rule", which is exactly the sentence two runs are known to have narrated past.

0113's own thesis is that step completion must be **verifiable rather than narrated**. Its
deterministic half delivers that; its prose half left the narration in place and added an undefined
term. The change file itself already argued the check has to be "per-step and uniform" — that
reasoning was applied to the oracle and not to the prose.

Surfaced by the deep-rung review of change 0113 and left unfixed by merge-time judgment, because
settling what a step's postcondition *is* for each of Steps 2 through 7 is a design decision, not a
cleanup.

## What changes

Settle, then state, a per-step git-state postcondition for `docket-implement-next` — the concrete,
git-readable condition that must hold before each step may be called complete. At minimum:

- Decide whether the postcondition is stated **inline at each step** or once as a table the steps
  point at, and whether it is normative prose or something a script can read.
- Give **Step 4** the treatment the incident record most demands: complete only when the plan file
  is committed on `feat/<slug>` **and** `plan:` is written and pushed on `metadata_branch` —
  verified by reading git, never by the sub-skill's report.
- Either give the **§5** clause real content or delete it; a rule with no referent is worse than no
  rule.
- Check the same shape at Steps 6 and 7, whose bookkeeping (`adrs:`, `status: implemented`, `pr:`,
  `results:`) is separable from their visible work in exactly the way that produced the third
  observed stop.

Word budget is a live constraint: 0113 already raised `docket-implement-next/SKILL.md` from 3950 to
4050 words, leaving 37 words of margin.

## Out of scope

- Reversing ADR-0044 or re-litigating call-site pre-specification.
- Changing the `aborted-run` check or its predicates — the deterministic oracle is not what is
  under-specified here.
