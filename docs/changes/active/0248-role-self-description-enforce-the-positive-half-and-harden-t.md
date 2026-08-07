---
id: 248
slug: role-self-description-enforce-the-positive-half-and-harden-t
title: 'Role self-description: enforce the positive half and harden the guard'
status: proposed
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [198, 199]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Consolidates #0198 and #0199 (2026-08-07 triage): an explicit sibling pair — each named the other in its Out of scope — both editing `tests/test_role_skill_self_description.sh` around 0194's role-self-description rule.

Verified 2026-08-07:

- **The positive half is unenforced and violated (#0198).** `skills/docket-review/SKILL.md` contains zero occurrences of `skills.review`, violating the convention rule that a role skill body names its binding key. The guard has only two positive anchors (`[ -s "$f" ]` and skill-name presence) plus the absence assert — no `grep -qF "skills.$role"` exists. #0198's own analysis argues for **enforce** over soften (a stronger non-vacuity anchor); that is the default this change takes.
- **Three guard gaps (#0199), all verbatim:** (1) co-occurrence gap — `claim_hits()` requires a `superpowers:` token AND a default-vocabulary word on one line, so a violation phrased across lines passes; the header discloses LINE-SCOPED and VOCABULARY-SCOPED but not the AND. (2) `ROLE_SKILLS="docket-build docket-review docket-brainstorm"` is a hardcoded population with no completeness anchor — a fourth role skill is silently unguarded; `tests/test_skill_size_budgets.sh` already solved this shape by auto-discovery over `skills/**/*.md`. (3) The bare `default` matcher is broad enough to false-positive on an operational sentence; the fire/ignore probe set doesn't cover it.

## What changes

- Add the binding-key mention to `skills/docket-review/SKILL.md` (and any other role skill missing it) per the rule's positive half; add the positive assert.
- Close or disclose the three guard gaps: derive the population by auto-discovery, disclose or fix the co-occurrence scope, tighten the `default` matcher with fire/ignore probes.

## Out of scope

- Softening the 0194 convention rule (rejected — enforce).
- New role skills.
