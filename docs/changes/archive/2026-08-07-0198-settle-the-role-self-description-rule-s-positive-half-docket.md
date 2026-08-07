---
id: 198
slug: settle-the-role-self-description-rule-s-positive-half-docket
title: Settle the role-self-description rule's positive half — docket-review names no skills.review binding
status: killed
priority: medium
type: docs
created: 2026-08-03
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [194]
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

Change 0194 added a rule to the docket-convention *Skill layer*: a role skill body names its role
and its `skills.<role>` binding key, never whether that binding is the shipped default. The
whole-branch review found the rule's **positive half ships already violated**.

`skills/docket-review/SKILL.md` opens with "docket's review role" but contains no occurrence of
`skills.review` anywhere in the file. 0194's premise that docket-review "already conformed" holds
only for the *negative* half (it asserts no default status). The guard 0194 shipped
(`tests/test_role_skill_self_description.sh`) tests absence only, so the positive obligation is a
normative sentence with neither a conforming instance in one of the three files it governs nor an
assert behind it.

A rule stated in the convention that one of its own governed files does not follow is worse than no
rule: the next author reads the bullet, looks at docket-review for the house pattern, and copies the
non-conforming shape.

## What changes

Pick one and make the tree self-consistent:

- **Enforce it** — add `bound by \`skills.review\`` to `skills/docket-review/SKILL.md`'s opening
  paragraph, and add a positive assert to `tests/test_role_skill_self_description.sh`
  (`grep -qF "skills.$role"` per role skill). That assert doubles as a stronger non-vacuity anchor
  than the current name-presence check, which is a second reason to prefer this route.
- **Or soften it** — reword the convention bullet to state only the prohibition it actually
  enforces, dropping the positive obligation.

Check `skills/docket-build/SKILL.md` and `skills/docket-brainstorm/SKILL.md` against whichever half
survives; 0194 gave both of those an explicit binding mention, so they conform today.

## Out of scope

- Re-litigating 0194's deletion of the default-status claims, or its budget raise.
- The guard's other limitations (co-occurrence scoping, hardcoded population) — separate follow-up.

## Why killed

Consolidated into #0248 at the 2026-08-07 backlog triage: explicit sibling pair with #0199 (each named the other), same guard file; enforce-the-positive-half is the default taken.
