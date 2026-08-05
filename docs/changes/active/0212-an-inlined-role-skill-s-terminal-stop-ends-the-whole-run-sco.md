---
id: 212
slug: an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco
title: An inlined role skill's terminal stop ends the whole run — scope docket-build's stop and enforce the run disposition
status: proposed
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
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

On 2026-08-05 the `docket-implement-next` fork building change 0206 ran Steps 0–5 in full — four
build commits, clean tree, 78-file suite green, `plan:` recorded — then ended its turn at the
**Step 5/6 boundary**, leaving the branch unpushed with no review and no PR.

Its closing report is the diagnosis:

> **Build disposition: complete — the plan is executed.** Review is not mine; stopping here.

That is verbatim `docket-build`'s own output contract. `skills/docket-build/SKILL.md:11` reads *"Then
you stop — review is not yours."* The fork invoked `docket-build` **inline via the Skill tool** and
then dispatched the profile agents itself, so the build role's terminal instruction was loaded into
the same context as the driver's step sequence — and outranked it. The run adopted the sub-role's
scope boundary as its own terminal boundary.

**This is 0096/0113's class with a different sub-skill.** 0096 and 0113 both diagnosed an invoked
skill's hand-off language ending the caller's run, and 0113 hardened the Step-4 call site against
`superpowers:writing-plans`. Nobody swept the *build* skill for the same shape. `docket-build`'s stop
sentence is correct for a dispatched build role and hazardous for an inlined one, and the skill
cannot know which it is.

**A second, independently checkable tell.** `docket-implement-next` requires every run to end by
declaring exactly one of four **run** dispositions — `advanced`, `contended`, `drained`, `halted` —
so a driver keys on the outcome instead of parsing prose. This run declared a **build** disposition.
The wrong disposition vocabulary in the final report is, on its own, proof the run never reached its
terminal step, and it would have caught all four observed incidents (0109, 0194 twice, 0206), each of
which closed with a step-scoped or invented disposition rather than one of the four.

**Why 0113's Step 5 rider did not prevent it.** Step 5 already carries 0113's language — *"Proceed
through the build — the deliverable is the executed plan, never the decision about how to execute
it"* and *"the step is not complete until its git-state postcondition holds."* Both were satisfied.
The build ran, the plan was executed, and Step 5's postcondition held. The rider guards **within** a
step; the failure was the **transition** out of it. No obligation anywhere states that the run is not
over until a run disposition is declared and its git state proven.

## What changes

Two prose levers, both aimed at the step-to-step transition rather than at any one step's contents.

**Scope the build role's stop sentence to the build role.** `docket-build` SKILL.md line 11 and its
`## Output` section ("the terminal build disposition") should say plainly that the stop ends the
**build role**, not the caller's run, and that a driver receiving the build disposition continues to
its own next step. Then sweep the other role skills docket invokes inline — `docket-review`,
`docket-adr`, the resolved finish skill — for the same shape; this is a class, not one sentence. See
also change 0154's audit of stale restatements across skill bodies for the sweep pattern.

**Make the run-level disposition an enforced closing obligation.** `docket-implement-next`'s
*Terminal disposition* section currently reads as guidance to a driver on how to interpret an
outcome; it should also bind the agent: the run does not end until one of the four is declared, and
`advanced` specifically requires the PR URL plus `status: implemented`, `pr:` and (when a results
file exists) `results:` written. A report declaring any other disposition vocabulary is by
construction an aborted run.

Grooming should decide whether the second lever is purely prose or gets a mechanical companion — the
final report is model output, so a check would have to live in the driver or in a wrapper, not in
`board-checks.sh`.

## Out of scope

- The deterministic oracle half — extending `aborted-run` with a built-but-not-delivered leg — which
  is a separate change.
- Defining the per-step git-state postconditions Step 5 names but never states; that is change 0203,
  which this change should be sequenced against rather than duplicate.
- Reversing ADR-0044 (pre-specification at the call site) or re-litigating ADR-0024 (`context: fork`).
  Both stand; this is their remedy meeting a sub-skill contract nobody scoped.
