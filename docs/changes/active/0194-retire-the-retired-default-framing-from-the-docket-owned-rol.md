---
id: 194
slug: retire-the-retired-default-framing-from-the-docket-owned-rol
title: Retire the retired-default framing from the docket-owned role skill bodies
status: proposed
priority: medium
type: docs
created: 2026-08-02
updated: 2026-08-02
depends_on: [193]
related: [154, 193]
discovered_from: [193]
adrs: []
spec: docs/superpowers/specs/2026-08-02-retire-the-retired-default-framing-from-the-docket-owned-rol-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-02-retire-the-retired-default-framing-from-the-docket-owned-rol-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-02-retire-the-retired-default-framing-from-the-docket-owned-rol-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0193 made `docket-build` and `docket-review` the shipped defaults for the `build` and
`review` roles and swept the config, README, convention, and implement-next prose accordingly. It
deliberately scoped OUT the role skills' own bodies, so `skills/docket-build/SKILL.md` still opens
by introducing itself as "The lean alternative to `superpowers:subagent-driven-development`" — a
framing that describes the retired opt-in posture rather than the shipped default it now is.

Surfaced by the whole-branch review of 0193 as a `minor` finding. It is not a behavior claim and
nothing breaks, but it leaves the two role skills disagreeing in tone: `skills/docket-review/SKILL.md`
carries no equivalent "alternative to" line, so a reader comparing them gets an inconsistent story
about which engine the project actually ships.

Grooming settled the underlying rule rather than just the one sentence, because the sentence keeps
getting written: every docket-owned role skill so far has introduced itself by its binding posture,
and 0193 had to sweep eight files when that posture moved.

## What changes

**The rule** — a docket-owned role skill body may name its role and the `skills.<role>` key that
binds it, but never whether that binding is the shipped default, and never positions itself as an
"alternative" to another role skill. The binding key is stable; the default is not, and every copy
of it is a surface the next flip has to find. Defaults stay owned by the convention's *Skill layer*
role table and `README.md`.

- Reword the opening of `skills/docket-build/SKILL.md` to lead with the role and `skills.build`,
  dropping the "lean alternative to `superpowers:subagent-driven-development`" comparison.
- Apply the same edit to `skills/docket-brainstorm/SKILL.md` — both its `## Overview` opening and the
  "in place of the default `superpowers:brainstorming`" clause in its frontmatter description. This
  claim is *accurate today*; it goes anyway, because truth-at-time-of-writing is the property 0193
  proved worthless. `skills/docket-review/SKILL.md` already conforms and is not touched.
- Add a **negative** guard: no docket-owned role skill body asserts default status. Ships with a
  non-vacuity anchor and a remedy naming the rule, and names its own line-scoped limitation.
- Record the rule as one bullet in the convention's *Skill layer* — its single home, not a copy.

The two frontmatter descriptions on `docket-build` and `docket-review` already follow the rule and
stay as they are. Grooming verified that no test greps either removed sentence, so the deletion is a
one-file edit each; the build re-runs that grep against the post-0193 tree regardless.

## Out of scope

- Any behavior change inside `docket-build`, `docket-review`, or `docket-brainstorm`.
- Re-litigating the 0193 default flip, or 0193's judgment that this line was out of *its* scope.
- The wider `skills/`-tree restatement audit (#0154) — check-id lists, exit codes, flag lists, and
  count restatements stay there. This change settles the role-self-description construct only.
- `README.md` and the convention's role table, which are the owners of default status.

## Notes

`depends_on: [193]` — the premise that `docket-build` and `docket-review` *are* the defaults only
holds on `main` once PR #152 merges.
