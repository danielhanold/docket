---
id: 69
slug: mode-conditioned-clause-discriminates-on-provenance
title: A mode-conditioned clause in a loadable skill body discriminates on provenance, and the second person belongs to the continue branch
status: Accepted
date: 2026-08-05
supersedes: []
reverses: []
relates_to: [24, 44]
change: 212
---

## Context

`docket-implement-next` Step 5 invokes the resolved build skill **inline via the Skill tool**, which
loads that skill's body into the driver's own context. `docket-build`'s second-person sentence
*"Then you stop — review is not yours."* therefore resolved "you" to the driver: on 2026-08-05 a run
executed Steps 0–5 in full and then ended its turn at the Step 5/6 boundary — branch unpushed, no
review, no PR. This is the third instance of the class (changes 0096, 0113, 0212).

Change 0212's remedy is a **mode-conditioned scoping clause** placed beside every terminal stop and
every second-person prohibition in each docket-owned skill body that can be loaded into a caller's
context. The first attempt at that clause conditioned on the reader's **employment status**:

> loaded inline into a caller's context, this stop ends this role only and that caller continues to
> its own next step; dispatched as a subagent, your turn ends here.

A deep whole-branch review returned this as a **blocker**, and the reasoning generalizes past this
change. Both antecedents are simultaneously true of the very reader the clause exists to steer: a
`docket-implement-next` instance is itself a dispatched subagent (its wrapper carries
`context: fork`, ADR-0024) *and* is a caller that loaded the body inline. The two branches were
therefore not mutually exclusive from the reader's viewpoint. Worse, the **continue** branch was
written in the third person ("that caller") while the **abort** branch was written in the second
person ("your turn ends here"), so the branch that survives a pronoun-resolution reading is the
aborting one — reproducing the original defect inside its own fix.

## Decision

A mode-conditioned clause in a skill body that can be loaded into a caller's context obeys three
rules. The landed 0212 clause is the reference shape:

> **Scope of this stop:** if you invoked this skill yourself, this stop ends only the build role —
> you continue to your own next step; only an agent whose entire assignment is this role ends its
> turn here.

1. **Discriminate on provenance, never on employment status.** The antecedent asks "did I invoke
   this body?" — not "am I a subagent?". Employment status is not mutually exclusive with inline
   loading; provenance is. A reader can always answer the provenance question about itself with
   certainty, and exactly one branch can be true.
2. **The second person belongs to the continue branch.** The turn-ending branch is stated in the
   third person, about a different agent. Under inline loading a second-person directive binds
   whoever is reading, so the pronoun must point at the outcome that is safe when misread.
3. **Corollary — never more second-person stop directives than continuations** in one body. The
   aggregate reading of a body full of second-person stops is "stop", regardless of how carefully
   any single clause is scoped.

This governs bodies docket **owns** and can edit. ADR-0044's call-site pre-specification remains the
sanctioned remedy for *vendored* skills docket cannot edit; the two are complementary, not
alternatives.

## Consequences

Future docket-owned role skills get a single reference shape to copy, and any fourth instance of
this class has a rule to be measured against rather than a fresh judgement call. The provenance
antecedent also stays correct if the agent layer changes how skills are dispatched — it does not
depend on wrapper mechanics.

The cost is that the property itself is **not mechanically assertable**.
`tests/test_inline_role_stop_scoping.sh` proves the clause is present, two-sided, and adjacent to
its stop site, but it cannot prove how a model resolves the pronoun. The guard is a floor, not the
guarantee; regressions in wording quality inside a conforming clause remain invisible to CI and are
caught only by review. Rule 3 is likewise a review-time reading, not a countable assertion the guard
enforces.
