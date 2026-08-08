---
slug: exit-code-encodes-a-non-failure
hook: "A new exit code for a non-failure condition reads as a hard failure at every bare non-zero consumer — enumerate the callers before minting it, and default the advisory case to 0."
topics: [exit-codes, contracts, gates]
changes: [227, 224, 259]
created: 2026-08-07
updated: 2026-08-08
promotion_state: candidate
promoted_to:
---

## Apply
A script that grows a **new exit code for a condition that is not a test failure** — over budget,
partial coverage, degraded-but-correct, nothing-to-do — has changed its contract for every existing
caller at once. Callers written against the old contract almost never switch on the value; they
check `if ! cmd`, or `[ $? -ne 0 ]`, or rely on `set -e`. To all of them the new code is
indistinguishable from "the suite went red."

Before minting the code, do two things in this order:

1. **Enumerate the consumers and read their check.** Not "who calls this" — what *shape* is the
   check. A bare non-zero test means the new code is a hard failure at that site no matter what
   your own contract document says it means. A gate that dispatches a repair agent on red will
   dispatch it to root-cause **zero failures**, which is worse than a false red: it burns an agent
   run and produces a confusing report.
2. **Decide which way the default fails.** For an advisory condition the safe default is **exit 0
   plus a loud report**, with the failing exit behind an opt-in flag (`--strict-*`). The report is
   what carries the signal; the exit code is what carries the *gating decision*, and those are not
   the same decision. Reserve the non-zero for callers that explicitly asked to be gated.

The trap is that the contract file makes the distinction perfectly clear while the consumers never
read it. A documented exit-code table is not a mechanism — it constrains nobody. If a consumer must
tell the codes apart, either change that consumer in the same change, or assert the wiring
([[sole-channel]], [[specified-but-unreachable]]).

Softening a gate to advisory is a real loss of enforcement, not a free fix. Say so where it happens
and name what still defends the property — a loud report plus a structural guard is weaker than a
hard gate, and the human merging deserves to be told that in the results file rather than discover
it later.

## War story
- 2026-08-07 (#227, PR #165 — merged) — `scripts/run-tests.sh` shipped exit **4** for "suite green,
  but at least one file exceeded its wall-clock budget." Both consumers check bare non-zero:
  `docket-finalize-change`'s test-command marker block, and `docket-build`'s final full-suite gate.
  So a **healthy suite that had merely gotten slow** would have blocked the merge *and* dispatched
  `docket-integration-repair` against a run with zero failing assertions. The review caught it as
  the change's single blocker.

  Fixed by inverting the default: a budget breach is now reported loudly as an `OVER BUDGET:` block
  and the run still exits 0; `--strict-budget` opts into exit 4, `--no-budget-check` skips the
  comparison entirely. That is a deliberate softening of one of the change's three pillars — the
  regrowth defence became the loud report plus the structural guard rather than a hard gate — and it
  was written into the results file as a human verify item rather than buried. Change **0229**
  tracks making the measurement contention-independent so it can be hard again.

  Worth keeping: the code was correct in isolation and its contract documented it accurately. The
  defect lived entirely in the **interaction** with two callers written before it existed, which is
  why it survived task-level review and surfaced only at whole-branch review
  ([[foundational-test-discipline]]).

- 2026-08-07 (#224, PR #174) — the **consumer-side** half of the same problem, one day later and in
  the same pair of consumers. Change 0224 was writing down what had never been stated: that
  `docket-build`'s gate verdict is keyed on the suite's exit status. The spec accepted as a benign
  residual that `run-tests.sh`'s non-failure exits (3 = produced no result at all, 4 = green but
  slow) would read as **red** — and red in that contract means "turn the failure into exactly one
  synthetic integration-repair task," i.e. dispatch an agent to root-cause zero failures. Review
  reversed the residual: the contract now states **green / halt / red**, a tri-state, without naming
  a single exit code.

  The generalization worth keeping: the caller-side rule above (default the advisory case to 0) is
  only half the fix, because it can only ever protect the codes the *producer* thought to soften. A
  consumer whose verdict is **binary** has no state to put a non-failure in — it must flatten it
  into one of the two, and the safe-looking choice (red) is the one that manufactures work. Give the
  consumer a third verdict — a **halt** — and delegate the classification to the resolved runner's
  own documented contract rather than to an exit-code taxonomy in the consumer, which would go stale
  the moment a repo configures a different runner. That delegation is a real, stated cost (ADR-0074
  names it): docket cannot mechanically enforce that a runner documents its non-failure exits.
- 2026-08-08 (#259, PR #177 — merged) — The mirror image, and this learning's own core claim made
  concrete: `render-board.sh`'s M4 read-back guard sat **inside the archive rendering block, below
  `--format digest`'s early exit**, so identical malformed input produced exit 3 with a diagnostic
  on the markdown path and **exit 0 with empty stderr and a wrong `backlog done` tally** on the
  digest path. `board-refresh.sh` froze the board while `docket-status.sh`'s `digest_only_pass`
  handed `docket-implement-next` a queue built from a corrupt archive. Three places already
  asserted the opposite and were false. Caught at whole-branch review as the branch's only blocker;
  fixed by hoisting validation into a feeder pass **above both emission paths**. A documented
  exit-code table constrains nobody — only the wiring does, and a guard's *placement relative to
  every early exit* is part of that wiring.
