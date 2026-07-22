---
slug: plan-supplied-test-code-is-unverified
hook: "Test code a plan hands you is unverified code, not an oracle — prove the assert CAN pass, and mutation-test its own key."
topics: [testing, plan, guards]
changes: [94, 104, 112]
created: 2026-07-19
updated: 2026-07-22
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
- 2026-07-20 (#104, PR #113) — Three more, in one plan, caught by running the supplied tests as code:
  - `has_finding "$out" malformed-id "?"` was **vacuous**: the helper built an unescaped ERE and `?`
    is a quantifier, collapsing the pattern to `^malformed-id\t` and matching any line of that
    check-id. It was green even against the **pre-implementation baseline**. Fixed at the helper's
    *definition* (literal `case` match + here-string, which also removed a `printf | grep -q`
    pipefail hazard) rather than at the call site — `cid` can legitimately be `?`, so a call-site
    patch would have been re-hit by later tasks. **Fix a shared helper's hazard where it is
    defined; a call-site fix leaves the trap armed for every other caller.** (See
    [[escape-ere-metacharacters-in-key]].)
  - Two asserts used a literal `\t` inside `grep -E`, which **BSD grep does not interpret** —
    rewritten to the repo's portable `grep -E "$(printf '^x\ty\t')"` idiom.
  - The plan's own Step-2 *verification command* was broken: anchored to line start, it missed
    `emit` calls following `||` guards and found only 9 of 11 check-ids. A plan's verification
    commands are unverified code too — an under-counting verifier reports a gap that does not exist
    (or, reversed, misses one that does). A corrected unanchored derivation confirmed zero gaps.
- 2026-07-22 (#112, PR #118) — **The control case: what it looks like when this rule is paid up
  front.** All three per-task reviews and the whole-branch review came back with zero Critical or
  Important findings — an outlier in this backlog, and the change's own results file explains it
  rather than celebrating it. Two things were done before any subagent was dispatched: the fixtures
  were **fully specified in the plan**, and **the plan's own values were checked against the running
  code**. The consequence is a shift in where the effort lands — verification went into *proving the
  guards fire* (an 18-cell mutation matrix, every cell matching prediction) instead of into
  repairing asserts mid-build. The three preceding changes in this family each spent multiple review
  rounds discovering that supplied test code was wrong; this one spent that budget on mutation
  evidence and shipped clean.
  Two smaller instances of the same discipline in the same change. (a) A **forward claim** in Task
  1's header asserted how `s8`/`s9` would behave under mutations — a claim about fixtures that did
  not yet exist — and it was checked against the completed matrix at Task 3 rather than left
  standing. (b) A **fixture comment's stated reason** was verified both ways: the comment says
  `.docket.yml` is kept (key absent) for main-mode shape consistency, *not* because omitting it
  breaks resolution, so the reviewer built the fixture both ways and ran the resolver to confirm
  both halves. A comment encoding a false reason is what [[verify-the-claim]] exists to stop, and
  the cheap version of that check is running the alternative once.
  The generalizable claim is narrow but real: this family's defects are **front-loadable**. The cost
  of checking a plan's asserts against running code before dispatch is bounded and paid once; the
  cost of discovering them at review is a round per defect, and #102 shows that reaching five.
