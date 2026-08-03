<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0194 — Retire the retired-default framing from the docket-owned role skill bodies](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-03-0194-retire-the-retired-default-framing-from-the-docket-owned-rol.md)**
<!-- docket:backlink:end -->

# Retire the retired-default framing from the docket-owned role skill bodies

Design for removing the default-status claims from docket's own role skill bodies and settling the
rule that put them there: a role skill body may name its role and its binding config key, but never
whether that binding is the shipped default.

## Context

docket's five workflow steps are pluggable roles (`skills:` in `.docket.yml`, change 0049). Three of
those roles now have a docket-owned implementation: `docket-build` (change 0167), `docket-review`
(change 0170), and `docket-brainstorm` (the consultant-author flow). Each shipped as an **opt-in
alternative** to a superpowers default, and two of the three said so in the first paragraph of their
own skill body.

Change 0193 flipped `skills.build` and `skills.review` to resolve to `docket-build` and
`docket-review` by default, and swept the surfaces that named the old defaults: `docket-config.sh`,
`.docket.example.yml`, `README.md`, `scripts/docket-config.md`, the convention's *Skill layer* role
table, and three restatements in `docket-implement-next/SKILL.md`. It deliberately scoped the role
skills' own bodies **out**, with an explicit judgment recorded in its `## What changes`:

> `skills/docket-build/SKILL.md:8` describes docket-build as "the lean alternative to
> `superpowers:subagent-driven-development`" — a comparative framing, not a default claim; out of scope.

The whole-branch review of 0193 disagreed and filed it as a `minor` finding, which became this
change. The finding is right on the narrow point — "the lean alternative to X" *is* read as a
statement about which one you get by default — and it exposes the wider inconsistency:
`docket-review/SKILL.md` carries no equivalent sentence at all, so a reader comparing the two role
skills gets two different stories about what docket ships.

Nothing breaks. This is a truthfulness and tone change to prose, with one small guard.

### The wider class

This is one instance of the restatement class change 0154 exists to audit across `skills/`: a skill
body copies a fact owned by another artifact, that artifact moves, and the copy goes stale
unnoticed. The learnings ledger carries the operating rule as
[`restatement-accumulates-its-own-guards`](../../changes/learnings/restatement-accumulates-its-own-guards.md)
— *"deleting a restatement is never a one-file edit; tests grep the COPY, not the source."* That
rule was applied during grooming rather than deferred to build; see *Verified inventory* below.

### Verified inventory (grooming pass, 2026-08-02)

Every live occurrence of the class in `skills/`, verified by grep against `origin/main`:

| Site | Text | Status today |
|---|---|---|
| `skills/docket-build/SKILL.md` §1 | "The lean alternative to `superpowers:subagent-driven-development`." | **Stale** post-0193 |
| `skills/docket-brainstorm/SKILL.md` §*Overview* | "`docket-brainstorm` is an opt-in alternative to the built-in `superpowers:brainstorming` role." | **Accurate** today |
| `skills/docket-review/SKILL.md` | — | No such claim; already conformant |

The two frontmatter `description:` fields are **already conformant** and are not touched:
`docket-build` opens *"Use as docket's build role (`skills.build`)"* and `docket-review` says *"for
docket's review role"* — each names the binding without asserting a default. `docket-brainstorm`'s
description says *"Bindable via `skills: brainstorm:` … in place of the default
`superpowers:brainstorming`"*, which names another role's default and is in scope (see Task 2).

**Test-grep result — the deletion is cheap.** No test asserts on either sentence:

- `tests/test_consultant_brainstorm.sh:27` pins `off by default|opt-in` against **`README.md`**, not
  the skill body. README keeps that prose (brainstorm really is opt-in), so the assert stays green
  and is not repointed.
- `tests/test_docket_build.sh:502`, `:506`, `:516` pin the *resolved config default*, which is change
  0193's surface, not this one.
- `tests/test_skill_size_budgets.sh` measures size only; both edits shrink their files.

This inventory is the reason the change is small. It is recorded here so the builder does not
re-derive it — but it was taken before 0193 merged, so the reconcile pass must re-run the greps
(0193 rewrites neighbouring prose in `README.md` and `docket-implement-next/SKILL.md`, not these
lines, but the line numbers will have moved).

## Decision

**A docket-owned role skill body may name its role and the `skills.<role>` key that binds it. It must
never state whether that binding is the shipped default, nor position itself as an "alternative" to
another role skill.**

The two halves of the retired sentences are not equally stable, and that asymmetry is the whole
argument:

- *"I am bound by `skills.build`"* changes only if the role key is renamed — an event that would
  break every config in every consuming repo and could not pass unnoticed. Keeping it is free, and it
  is genuinely useful to a reader who opened the skill file first.
- *"I am / am not the default"* moved under 0193 and will move again. Every copy of it is a surface
  that a future flip has to find. 0193 had to sweep eight files for exactly this; the point of the
  rule is that the ninth and tenth never accumulate.

The convention's *Skill layer* and `README.md` remain the sole owners of which skill each role
resolves to by default. This is the same ownership boundary ADR-0012 draws elsewhere: one writer per
fact.

### Rejected alternatives

- **Say nothing about binding at all.** The strictest reading of the stub's open question, and the
  cost is real: it would also strip the `skills.build` mention from the body and raise pressure on the
  frontmatter `description:` fields, which are how a reader or a harness discovers what the skill is
  for. The drift hazard is the default claim, not the key; removing the key buys nothing and loses
  orientation.
- **Keep the default claim but pin it with a guard** (the way change 0111 pins its four surfaces).
  Correct-by-construction, but it adds a pinned surface to keep in sync, which is the exact posture
  change 0154 argues against: *"prefer removal plus a pointer to the owning contract over adding
  another pinned surface, since removal makes drift impossible rather than merely detected."*
- **Kill this change and fold it into #0154.** The overlap is real — 0194 is a strict instance of
  0154's class. Rejected because 0154 is itself un-groomed and unscheduled, so folding would ship the
  known-stale line indefinitely, and because 0194 settles a *rule* that 0154's per-site sweep can then
  apply rather than re-litigate.

### The one guard, and why it is not the rejected pinning

Task 3 adds a **negative** guard: no docket-owned role skill body asserts default status. This is not
the pinning rejected above. A pinning guard asserts that a copy *matches* the source — it keeps the
copy alive and adds a second thing to update. A negative guard asserts that the copy *does not
exist*, so it can never go stale by construction; the only way to redden it is to reintroduce the
duplication the rule forbids. It is cheap, and without it the rule is a sentence in a spec that the
next role skill's author will not read.

Per [`guard-remedy-must-not-teach-the-evasion`](../../changes/learnings/guard-remedy-must-not-teach-the-evasion.md)
and [`mirrored-guard-enforces-its-own-property`](../../changes/learnings/mirrored-guard-enforces-its-own-property.md),
the guard ships with a **non-vacuity anchor** and a remedy string that names the rule rather than the
escape.

## Design

### Task 1 — `skills/docket-build/SKILL.md`

Replace the opening sentence so the paragraph leads with the role and its binding, and drops the
comparison entirely. The rest of the paragraph (already-running-inside-Step-5, read the plan, route,
dispatch, escalate, gate, stop) is unchanged and still carries the operational contrast with SDD
implicitly — the body already says "no per-task review", which is the substantive difference.

Target shape (exact wording is the builder's, these are the constraints):

- First clause names the role and `skills.build`.
- No occurrence of `superpowers:subagent-driven-development` in the body's self-description. Existing
  operational references elsewhere in the file, if any, are unaffected — the rule is about the skill
  describing *itself*.
- No word to the effect of *default*, *alternative*, *opt-in*, or *instead of* applied to this skill's
  own binding.

### Task 2 — `skills/docket-brainstorm/SKILL.md`

Same edit, and this is the case that makes it a rule rather than a fix: the sentence is **accurate
today**. It is removed anyway, because a claim that happens to be true is still a copy of a fact the
convention owns, and truth-at-time-of-writing is exactly the property 0193's eight-file sweep proved
worthless.

- `## Overview`: drop "is an opt-in alternative to the built-in `superpowers:brainstorming` role" and
  open on what the flow *is* — the docket-owned brainstorm role, bindable via `skills.brainstorm`,
  keeping the ADR-0006 dialogue boundary while adding a pinned consultant author/auditor.
- Frontmatter `description:`: drop the trailing "in place of the default `superpowers:brainstorming`"
  clause; keep "Bindable via `skills: brainstorm:`". This is in scope precisely because it names
  *another* role's default status, which is the fact most likely to move.
- Do **not** touch `README.md`'s brainstorm section — it correctly owns and states the opt-in posture,
  and `tests/test_consultant_brainstorm.sh:27` reads it there.

### Task 3 — the negative guard

Add to the test suite (the natural home is `tests/test_skill_contracts.sh` if one exists, otherwise a
small dedicated file; the builder picks by what the tree actually has at build time — the plan must
not assume a filename this spec cannot see post-0193):

- **Assert:** for each of `skills/docket-build/SKILL.md`, `skills/docket-review/SKILL.md`,
  `skills/docket-brainstorm/SKILL.md`, the file contains no self-describing default claim — matched
  as a `superpowers:` reference co-occurring with an *alternative / default / instead of / opt-in*
  word on the same line.
- **Non-vacuity anchor:** assert the three files exist and are non-empty, and that the matcher does
  fire on a synthetic fixture line carrying the forbidden shape. Without the second half, a typo in
  the pattern makes the guard permanently green — the inversion
  [`mirrored-guard-enforces-its-own-property`](../../changes/learnings/mirrored-guard-enforces-its-own-property.md)
  warns about.
- **Remedy string:** points at the convention's *Skill layer* as the owner of role defaults and states
  the rule ("a role skill body names its binding, never its default status"). It must **not** say
  "add your file to the exemption list" or "update the expected count".
- **Deliberate limitation, named in the guard's own header comment:** it is line-scoped and
  vocabulary-scoped, so a default claim spread across two lines or phrased without any of the anchor
  words escapes it. That is acceptable — the guard's job is to catch the recurrence of the exact
  construct being removed, not to prove the absence of all possible paraphrases.

### Task 4 — write the rule down where it is owned

Add one bullet to the convention's *Skill layer* section (`skills/docket-convention/SKILL.md`) stating
the rule, so it applies to the next docket-owned role skill without anyone rediscovering it:

> **Role skill self-description.** A docket-owned role skill body names its role and its
> `skills.<role>` binding key; it never states whether that binding is the shipped default. Defaults
> are owned by this section's role table and by `README.md`.

This is a **single-home** addition, not a restatement: the *Skill layer* is already the owner of role
binding, and no other file will carry a copy of this rule.

## Consequences

- The two role skills stop disagreeing with `docket-review`, which already conformed by accident.
- A future default flip touches the convention's role table, `README.md`, `docket-config.sh`, and
  `.docket.example.yml` — and nothing under `skills/docket-*/SKILL.md`. 0193's sweep gets shorter.
- `docket-brainstorm` loses a true and mildly useful sentence. A reader who wants to know whether it
  is on by default now has to reach `README.md` or the convention. That is the intended trade: one
  extra hop for the reader, one fewer surface for the maintainer.
- The rule is now enforceable against new role skills, which matters more than the two edits: docket
  has added a docket-owned role roughly once per major change cycle (0167, 0170, the consultant flow),
  and each one wrote this sentence unprompted.
- **No ADR.** This is a documentation-ownership rule that follows directly from ADR-0012's
  one-writer-per-fact posture and lives in the convention; it reverses no decision and adds no
  architectural surface. If the build turns up a genuine non-obvious judgment (most plausibly in the
  guard's scoping), `docket-implement-next` step 6's ADR dispatch is the right place for it.

## Out of scope

- Any behavior change inside `docket-build`, `docket-review`, or `docket-brainstorm`.
- Re-litigating 0193's default flip, or 0193's own judgment that this line was out of *its* scope —
  that judgment is why this change exists and is not being second-guessed here.
- The wider `skills/`-tree restatement audit (#0154). This change settles the rule for the *role
  self-description* construct only; check-id lists, exit codes, flag lists, and count restatements
  remain 0154's.
- `README.md`'s brainstorm/build/review sections and the convention's role table — those are the
  owners, and 0193 already swept the build and review ones.
- Historical records: archived changes, plans, specs, and results are immutable and keep the old
  framing.

### Reconcile confirmation (2026-08-02, against merged `origin/main`)

The *Verified inventory* re-ran green after 0193 merged — both sentences are still present and
still the only two occurrences of the construct under `skills/`; `docket-review/SKILL.md` still
carries none. `README.md:688` still owns the brainstorm opt-in prose that
`tests/test_consultant_brainstorm.sh` reads, and no test greps either sentence being deleted.

Two build-time facts the spec left open, now settled: **`tests/test_skill_contracts.sh` does not
exist**, so Task 3 ships a new dedicated test file; and the suite is a bare `tests/test_*.sh` glob
(no registry file), so a new test needs no registration.

## Reconcile notes for the builder

- **Land after #0193 merges.** This change's premise is that `docket-build` and `docket-review` are
  the shipped defaults; that is only true on `main` once PR #152 lands. `depends_on: [193]` is set for
  this reason.
- Re-run the *Verified inventory* greps at reconcile. 0193 rewrites prose in `README.md` and
  `skills/docket-implement-next/SKILL.md` and shifts line numbers; per ADR-0054 nothing here anchors on
  a line number, but the test-grep result must be re-confirmed rather than trusted from this document
  ([`verify-the-claim`](../../changes/learnings/verify-the-claim.md)).
- Before deleting either sentence, re-run the test-suite grep for the exact prose being removed — not
  for the fact it restates. That is the standing rule from
  [`restatement-accumulates-its-own-guards`](../../changes/learnings/restatement-accumulates-its-own-guards.md),
  and grooming's clean result does not exempt the build from repeating it against the merged tree.
