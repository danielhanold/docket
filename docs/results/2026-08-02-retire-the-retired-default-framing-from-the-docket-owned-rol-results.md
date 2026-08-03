<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0194 — Retire the retired-default framing from the docket-owned role skill bodies](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0194-retire-the-retired-default-framing-from-the-docket-owned-rol.md)**
<!-- docket:backlink:end -->

# Retire the retired-default framing from the docket-owned role skill bodies — results
Change: #0194 · Branch: feat/retire-the-retired-default-framing-from-the-docket-owned-rol · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-02-retire-the-retired-default-framing-from-the-docket-owned-rol-plan.md · ADRs: none

## Verify (human)

- [ ] **Decide the word-budget raise.** `tests/test_skill_size_budgets.sh` raises
      `skills/docket-convention/SKILL.md`'s word ceiling **6350 → 6400**. The alternative is to trim
      the new convention bullet to fit 6350. See *Findings* for the full argument on both sides —
      this is a deliberate call surfaced for you, not a silent bookkeeping edit.
- [ ] **Read the new convention bullet as shipped.** It is the single home of a rule that now binds
      every future docket-owned role skill, so the wording is worth your eyes:
      *"A role skill body names its `skills.<role>` binding key, never whether that binding is the
      default — this section's table and `README.md` own that."*

## Findings

**The budget raise (the one judgment call in this change).** The convention file measured 6321 words
before and 6349 after the 28-word bullet, against a ceiling of 6350 — one word of headroom. That
1-word margin is verbatim the near-zero failure mode `tests/test_skill_size_budgets.sh`'s own comment
block records from changes 0102 and 0167. The raise applies the rounding rule that block already
documents (next multiple of 50 → 6350 → margin 1 < 25 → take 6400) and records its justification in
the same style as every prior entry. The **line** budget was correctly left alone (363 actual, 365).

This was flagged deliberately rather than decided quietly, because this repo's own learning
`guard-remedy-must-not-teach-the-evasion` treats a count-bump as *"a finding to investigate, never
bookkeeping — the interesting question is always whether the growth earned its lines."* Both the task
reviewer and the whole-branch reviewer were asked the question directly and independently, and both
concluded the growth earned its words and sided against trimming the bullet. The counter-argument is
available to you: the guard exists to stop accretion, and 28 words is 28 words.

**Whole-branch review: 0 blockers, 2 important, 3 minor.** No blocker means nothing entered a fix
round; the important and minor findings are recorded here and in the PR body for merge-time judgment,
per the build loop's severity contract. The two important ones both became follow-up changes rather
than in-scope fixes, because each settles a question this change's spec deliberately left closed:

1. *(important)* **The new rule's positive half ships already violated.** The convention bullet states
   two obligations — name your binding key, never state your default status. `skills/docket-review/SKILL.md`
   satisfies the second but contains no occurrence of `skills.review` at all, so 0194's premise that
   docket-review "already conformed" holds only for the negative half. The guard tests absence only.
   → change **#0198**.
2. *(important)* **The guard's condition is narrower than the remedy it prints.** A forbidden line must
   carry a `superpowers:` token *and* a default word, so the most likely post-0193 recurrence —
   "docket-build is the shipped default for the build role", naming no superpowers skill — passes
   silently. The header discloses LINE-SCOPED and VOCABULARY-SCOPED limits but not this one.
   → folded into change **#0199** with the two related minors (hardcoded `ROLE_SKILLS` population with
   no completeness anchor; `default` broad enough to false-positive on legitimate operational prose).

**Minor, not captured:** the convention file is left with 2 lines of line-budget headroom (363/365).
The reviewer notes the same comment block raised other files' line budgets at 1-2 lines of margin, so
the next one-line edit to that file will redden CI on arrival. Left as-is deliberately — the next
edit can raise it in its own diff, which is the file's stated posture.

**No ADR.** The spec reserved step 6's ADR dispatch in case the guard's scoping turned out to carry a
real architectural judgment. It did not: the scoping gaps the review found were *recorded and
deferred*, not decided, so there is no decision to memorialize. The budget raise follows an existing
documented procedure rather than establishing a new one.

## Follow-ups

- **#0198** — settle the role-self-description rule's positive half (enforce `skills.<role>` in
  `docket-review`, or soften the convention bullet to the prohibition it actually enforces).
- **#0199** — harden the guard: co-occurrence gap, hardcoded population, broad `default` matcher.
- **#0154** (pre-existing) — the wider `skills/`-tree restatement audit. 0194 settled the *rule* for
  the role-self-description construct only; 0154's per-site sweep can now apply it rather than
  re-litigate it.

**Plan deviations worth recording** — both are defects in the plan text, not in the shipped code:

- The plan's verification step (both tasks) pipes each test to `tail -1` and expects the literal
  `PASS`. That is a bad oracle: 46 of the 76 suites legitimately end with `ALL PASS`, `ALL OK`, or
  their last assert line, so the command reports 46 false failures on a fully green tree. Every suite
  run in this build was keyed on **exit status** instead. Worth knowing before anyone copies that
  command into another plan.
- The plan's mutation-proof step (both tasks) undoes the mutation with
  `git checkout -- <file>` while the task's own edit is still uncommitted — running it verbatim
  discards the edit itself, not just the mutation. Both implementers hit it live and both recovered
  (one re-applied the edit and staged first; the other used a `cp` backup/restore). The mutation
  proofs themselves are sound and are recorded with `grep -c` before/after counts proving each
  mutation landed.
