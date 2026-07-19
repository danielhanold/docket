---
slug: plan-supplied-test-code-is-unverified
hook: "Test code a plan hands you is unverified code, not an oracle — prove the assert CAN pass, and mutation-test its own key."
topics: [testing, plan, guards]
changes: [94]
created: 2026-07-19
updated: 2026-07-19
promotion_state: candidate
promoted_to:
---

## Apply
A plan's asserts arrive with the authority of the plan and none of its scrutiny: the plan author
wrote them against an implementation that did not exist yet, and nothing has ever executed them.
Treat every supplied assert as a draft under test until you have shown two things:

1. **It can pass at all.** An assert that is unsatisfiable by *any* correct implementation reads as
   a real regression, and the honest response — stop, report BLOCKED — burns a cycle chasing a
   defect in the test rather than the code. Before debugging the implementation against a red
   supplied assert, check the assert's own field indices, ranges, and expected values against the
   real output format.
2. **Its key is load-bearing.** Mutation-test the assert by deleting the thing it exists to check.
   If it stays green, it is decoration — and a *fixture* can hide this as easily as the assert can:
   a fixture set too narrow to distinguish two orderings makes a sort-key check pass under a
   coincidence rather than under the rule.

This is the defect class the plan author structurally cannot see, which is why it survives to the
implementer. It is not a reason to distrust plans — it is a reason to run the plan's tests as code.

## War story
- 2026-07-19 (#94, PR #108) — Three distinct defects in one plan's supplied test code, all caught
  during the build:
  - The membership-parity assert used awk field indices off by one against
    `change <id> <status> <readiness> <slug>`, leaving `exp_ready` unconditionally empty — the
    assert was **unsatisfiable by any correct implementation**.
  - A producer sentinel scoped itself with `awk "/^main\(\)/,/docket_preflight/"`, whose range
    closed on the explanatory **comment** containing that string, so it never reached the code.
    Presented as a false BLOCKED. (See [[specified-but-unreachable]].)
  - The lowest-`id` tie-break assert could not detect deletion of its own `-k3,3n` sort key: every
    fixture id was two digits, so `sort`'s lexicographic fallback happened to agree with numeric
    order. Fixed with a tie crossing a digit-width boundary (ids 9/10), then confirmed red under
    mutation — the fixture, not the assert, was what had been hiding the hole. (See
    [[guards-are-code]].)
