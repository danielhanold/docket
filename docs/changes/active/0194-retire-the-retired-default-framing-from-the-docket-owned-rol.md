---
id: 194
slug: retire-the-retired-default-framing-from-the-docket-owned-rol
title: Retire the retired-default framing from the docket-owned role skill bodies
status: proposed
priority: medium
type: docs
created: 2026-08-02
updated: 2026-08-02
depends_on: []
related: []
discovered_from: [193]
adrs: []
spec:
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

## What changes

- Reword the opening of `skills/docket-build/SKILL.md` so it names the role and its default status
  first and keeps the superpowers comparison as a contrast rather than an identity.
- Sweep the other docket-owned role skill bodies for the same retired framing and settle whether a
  role skill should assert its own binding posture at all — a self-description that restates a
  default is one more copy to keep in sync when the default moves again, which is exactly what 0193
  had to sweep.
- If the answer is that role skills should NOT restate their binding, remove the claim rather than
  correcting it, and check whether any test greps the sentence before deleting it.

## Out of scope

- Any behavior change inside `docket-build` or `docket-review`.
- Re-litigating the 0193 default flip itself.

## Open questions

- Should a role skill body state which config key binds it and whether it is the default, or is that
  strictly the convention's and README's job? The 0193 sweep is evidence for the latter.
