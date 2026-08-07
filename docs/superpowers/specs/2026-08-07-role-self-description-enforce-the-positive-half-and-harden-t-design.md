<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0248 — Role self-description: enforce the positive half and harden the guard](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0248-role-self-description-enforce-the-positive-half-and-harden-t.md)**
<!-- docket:backlink:end -->

# Design: Role self-description — enforce the positive half and harden the guard (change 0248)

Consolidates killed siblings #0198 (positive half unenforced and violated) and #0199 (three guard
gaps), all edits to `tests/test_role_skill_self_description.sh` plus one prose edit to
`skills/docket-review/SKILL.md`. The 0194 convention rule itself does not change.

## Context (verified 2026-08-07)

- Convention *Skill layer* bullet (skills/docket-convention/SKILL.md:126–127): "A role skill body
  names its `skills.<role>` binding key, never whether that binding is the default."
- `skills/docket-build/SKILL.md:8` ("bound by `skills.build`") and
  `skills/docket-brainstorm/SKILL.md:10` ("bound by `skills.brainstorm`") conform.
  `skills/docket-review/SKILL.md` contains zero occurrences of `skills.review` — the violation.
- The guard has two positive anchors (file non-empty; skill-name presence) plus the absence
  assert; `ROLE_SKILLS` is hardcoded; `claim_hits()` requires `superpowers:` AND a default word on
  one line; `WORDS='alternative|default|instead of|opt-in'` — bare `default` substring-matches
  operational phrasings such as "defaults to".

## What changes

### 1. Positive half — enforce (skills/docket-review/SKILL.md + guard)

Add the binding-key mention to `skills/docket-review/SKILL.md`'s opening, in the house pattern the
other two role skills already use: a clause of the form "docket's review role, bound by
`skills.review`" in the H1 paragraph or `## Scope` opening (exact wording is the builder's; the
greppable token is `skills.review` on one unwrapped line).

Add a positive assert to the per-skill loop:

    role="${s#docket-}"
    assert "skills/$s/SKILL.md names its binding key skills.$role" 'grep -qF -- "skills.$role" "$f"'

This doubles as a stronger non-vacuity anchor than the name-presence check; keep both existing
anchors (adding, not replacing).

### 2. Population — derive from the convention's role nouns (guard)

Replace the hardcoded `ROLE_SKILLS` with a derivation over the five role nouns the convention's
*Skill layer* resolution defines (`brainstorm plan build review finish`, the `SKILL_*` exports of
`docket-config.sh`): include `docket-<role>` when `skills/docket-<role>/SKILL.md` exists. Two
anchors so the derivation cannot rot:

- Population floor: assert the derived set contains at least `docket-build docket-review
  docket-brainstorm` (the three that exist today) — "whatever exists" alone pins nothing
  (learnings: marker-scoped-guard-needs-a-population-floor).
- Noun-list anchor by SET EQUALITY: extract the set of `SKILL_[A-Z]*` variables
  `scripts/docket-config.sh` actually emits (lines 533–537 / 885–889 today; exactly the five) and
  assert it EQUALS the guard's noun list — presence-only checking would detect a removed role but
  never an added sixth; equality reddens in both directions.

A future `skills/docket-plan/` or `skills/docket-finish/` is then covered on arrival with no guard
edit.

### 3. Co-occurrence gap — second matcher + honest disclosure (guard)

Add a second matcher for the most likely post-0193 recurrence, which names no `superpowers:` token
at all: a claim-shaped default word co-occurring on one line with the skill's own role noun
(`role_claim_hits()` — grep for `\b$role role\b|docket-$role([^-]|$)` lines, then the WORDS
filter). The role-name alternate MUST exclude hyphen-compounds — a bare `\bdocket-$role\b` matches
inside `docket-build-standard`, and docket-build's profile-routing table row (SKILL.md:38) contains
a legitimate profile-tier "the default" on such a line, so the naive shape ships a guard born red on
conforming text; `docket-$role([^-]|$)` was verified to leave zero hits across all three current
bodies. Per the guard's existing non-vacuity discipline, it gets its own synthetic fire probe
("docket-build is the shipped default for the build role" — which fires only under assumption 5's
modifier-tolerant WORDS) and ignore probe (an operational sentence naming the role with no claim
word).

Update the header's disclosed-limitations block: the LINE-SCOPED and VOCABULARY-SCOPED bullets
stay; the co-occurrence condition is now stated honestly (two matchers, each line-scoped; a claim
naming neither a `superpowers:` token nor the role noun still escapes).

### 4. Tighten WORDS to claim-shaped phrasing (guard)

`WORDS='the ([a-z]+ )?default|by default|instead of|alternative to|opt-in'`. The first alternate is
modifier-tolerant because the house phrasing of the forbidden claim itself is "the **shipped**
default" (the guard's own header states the rule that way, and the section-3 fire probe uses it) —
a plain `the default` alternate misses it and leaves the new matcher's fire probe vacuous. Verified:
this WORDS fires on "the shipped default", still ignores "defaults to", and keeps all three current
role-skill bodies green under both matchers. Probe changes, per the mutation-in-fixture discipline:

- The existing fire probe ("The lean alternative to `superpowers:…`") still fires under
  `alternative to` — keep it.
- Add an ignore probe: a legitimate `superpowers:` + "defaults to" operational sentence must NOT
  fire. Phrase it WITHOUT "the <word>" immediately before "defaults" (e.g. "`skills.build`
  defaults to …", not "the binding defaults to …") — the modifier-tolerant alternate matches the
  prefix of "defaults" in that shape (critic-verified; the residual surface is true-positive-shaped
  and all current bodies stay green).
- The existing bare-operational ignore probe stays.

Both matchers share the single `WORDS` definition so guard and probes cannot drift apart (the
file's existing single-home comment extends to cover the second matcher).

## Out of scope (carried from the stub)

- Softening the 0194 convention rule (rejected — enforce).
- New role skills; widening to the whole `skills/` restatement class (#0154).

## Verification

- `bash tests/test_role_skill_self_description.sh` green after the edits; red under mutation
  (delete the `skills.review` clause → positive assert fails; add the fire-probe sentence to a
  role skill body → the relevant matcher fails).
- `bash tests/test_skill_size_budgets.sh` green: `skills/docket-review/SKILL.md` measures 104
  lines / 842 words (`wc`, critic-verified) against a 110/900 budget, so the one added clause fits; if a re-measure lands
  within the table's near-zero margins anyway, follow the budget table's in-diff raise rules.

## Assumptions

1. **Enforce, not soften, the positive half.** Chosen: add the `skills.review` mention + a
   positive assert. Rejected: rewording the convention bullet to prohibition-only. Why: #0198's
   own analysis prefers enforce (the assert is a stronger non-vacuity anchor), the stub pins it
   ("rejected — enforce"), and two of three governed files already conform — softening would
   un-state a rule the tree mostly follows.
2. **Binding mention lands in docket-review's opening, house-patterned.** Chosen: a "bound by
   `skills.review`" clause in the H1/Scope opening, mirroring docket-build:8 and
   docket-brainstorm:10. Rejected: frontmatter-description-only mention (docket-build carries it
   there too, but the rule says *body*, and the guard reads the whole file — an opening-paragraph
   clause is what the next author sees when copying the house pattern).
3. **Population by derivation over the five convention role nouns, double-anchored — the noun
   anchor is SET EQUALITY against docket-config.sh's emitted `SKILL_*` set.** Chosen:
   existence-filtered derivation + population floor + equality anchor. Rejected: (a) full
   `skills/**/*.md` auto-discovery à la test_skill_size_budgets.sh — over-inclusive, most skills
   are not role skills; (b) hardcode + static completeness assert — two parallel lists; (c)
   presence-only anchoring (critic-caught: detects a removed role but never an added sixth, so it
   cannot deliver the stated property — equality reddens in both directions).
4. **Close the co-occurrence gap with a second matcher, not disclosure alone; the role-name
   alternate excludes hyphen-compounds.** Chosen: `\b$role role\b|docket-$role([^-]|$)` + WORDS,
   with own fire/ignore probes and an updated disclosure. Rejected: disclosure-only third bullet
   (#0199 calls this gap "the important one"); bare `\bdocket-$role\b` (critic-caught: matches
   inside `docket-build-standard`, and docket-build SKILL.md:38's legitimate profile-tier "the
   default" row makes the guard born red on conforming text — the exclusion form verified to leave
   zero hits on all three current bodies).
5. **Tighten WORDS to claim-shaped phrases with a modifier-tolerant first alternate.** Chosen:
   `the ([a-z]+ )?default|by default|instead of|alternative to|opt-in`. Rejected: keeping bare
   `default` (false-positives on operational "defaults to" — #0199 gap 3); plain `the default`
   (critic-caught: misses the house phrasing "the **shipped** default" the guard header and the
   new fire probe both use, leaving that probe vacuous). Consequences verified: existing fire
   probe still fires via `alternative to`; "defaults to" escapes; current bodies stay green.
6. **No budget raise expected; table rules apply if measured otherwise.** docket-review sits at
   104/838 against 110/900. The builder re-measures after the edit and touches
   tests/test_skill_size_budgets.sh only if the table's near-zero-margin rule demands it, with the
   in-diff justification the table requires.
7. **Couplings.** `related: [194]` (the rule and guard this change enforces/hardens shipped
   there); no `depends_on` — 0194 is archived/done, #0198/#0199 are killed-consolidated and
   already in `discovered_from`. #0154 stays a prose mention only (out of scope, not a coupling).
