---
slug: compensating-assert-must-exist-when-cited
hook: "Relaxing a guard because coverage 'moved elsewhere' is only honest if the elsewhere already exists — write the compensating assert first, mutation-test it, then relax."
topics: [testing, guards, review]
changes: [324]
created: 2026-08-15
updated: 2026-08-15
promotion_state: candidate
promoted_to:
---

## Apply
A change that makes an existing guard inconvenient has a tempting escape: relax the guard and leave
a comment saying the property is now covered somewhere else. The comment reads as a citation, so a
reviewer — and every future reader — treats the coverage as a fact. It is not a fact until the
compensating assert exists, and nothing in the suite checks the claim: the relaxed guard is green
because it is relaxed, and the assert it names is green by not existing.

The failure is asymmetric in a way that hides it. Relaxing the guard is one edit that keeps the
suite green; writing the compensating assert is separate work that can be deferred, forgotten, or
argued into a later change. Between those two moments the property has **zero** coverage while the
repo carries prose asserting it has some — which is worse than an honest gap, because a later agent
looking for coverage finds the citation and stops looking.

So the ordering is the rule, not a preference:

1. **Write the compensating assert first**, in whatever file will own it going forward.
2. **Mutation-test it** — strip the thing it guards and watch it redden. An assert that passes
   against the mutation is not compensation; it is a second decoration.
3. **Only then relax the original**, and make the comment point at the assert by **name** (a
   greppable symbol or verbatim clause), never at a description of coverage that exists in someone's
   plan.

The same test applies when reviewing: a comment claiming coverage lives elsewhere is a claim to
**verify by opening the elsewhere**, never a claim to accept. Grep for the named assert; if the
comment names no assert you can grep for, that is itself the finding.

## War story
- 2026-08-15 (#324, PR #209 — merged) — the deep review's single *important* finding. While
  registering the seventeenth shipped agent, a learnings-enablement guard in
  `tests/test_learnings_ledger.sh` was relaxed, with a comment citing a compensating assert in
  `tests/test_plan_writer_agent.sh` for the new agent's learnings-read gate. The cited assert **did
  not exist** — the relaxation and the citation landed together, the compensating work did not. The
  whole 118-file suite was green throughout, because green was exactly the symptom: the relaxed
  guard passed by being relaxed, and the named assert passed by being absent. Nothing mechanical
  could have caught it — no test greps another test's comments for the asserts they promise — so it
  took a whole-branch semantic read to notice that a citation had been written in the future tense.
  The fix ran the ordering above in the correct order: the compensating assert was authored in
  `test_plan_writer_agent.sh`, mutation-tested against the child's learnings-read gate, and the
  ledger comment was corrected to name it. Worth noting how cheap the correct order was — the same
  two edits, sequenced — and how completely the wrong order defeats both the suite and the reader.
