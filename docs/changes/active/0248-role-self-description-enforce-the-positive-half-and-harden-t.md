---
id: 248
slug: role-self-description-enforce-the-positive-half-and-harden-t
title: 'Role self-description: enforce the positive half and harden the guard'
status: proposed
priority: low
type: chore
created: 2026-08-07
updated: 2026-08-09
depends_on: []
related: [194]
discovered_from: [198, 199]
adrs: []
spec: docs/superpowers/specs/2026-08-07-role-self-description-enforce-the-positive-half-and-harden-t-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-role-self-description-enforce-the-positive-half-and-harden-t-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-role-self-description-enforce-the-positive-half-and-harden-t-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0198 and #0199 (2026-08-07 triage): an explicit sibling pair — each named the other in its Out of scope — both editing `tests/test_role_skill_self_description.sh` around 0194's role-self-description rule.

Verified 2026-08-07:

- **The positive half is unenforced and violated (#0198).** `skills/docket-review/SKILL.md` contains zero occurrences of `skills.review`, violating the convention rule that a role skill body names its binding key. The guard has only two positive anchors (`[ -s "$f" ]` and skill-name presence) plus the absence assert — no `grep -qF "skills.$role"` exists. #0198's own analysis argues for **enforce** over soften (a stronger non-vacuity anchor); that is the default this change takes.
- **Three guard gaps (#0199), all verbatim:** (1) co-occurrence gap — `claim_hits()` requires a `superpowers:` token AND a default-vocabulary word on one line, so a violation phrased across lines passes; the header discloses LINE-SCOPED and VOCABULARY-SCOPED but not the AND. (2) `ROLE_SKILLS="docket-build docket-review docket-brainstorm"` is a hardcoded population with no completeness anchor — a fourth role skill is silently unguarded; `tests/test_skill_size_budgets.sh` already solved this shape by auto-discovery over `skills/**/*.md`. (3) The bare `default` matcher is broad enough to false-positive on an operational sentence; the fire/ignore probe set doesn't cover it.

## What changes

Groomed 2026-08-07 (auto-groom; two critic passes, PASS on re-check). The settled design — detail in the linked spec:

- **Enforce the positive half**: add a house-patterned "bound by `skills.review`" clause to `skills/docket-review/SKILL.md`'s opening (docket-build and docket-brainstorm already conform), and add a per-role positive assert (`grep -qF "skills.$role"`) that doubles as a stronger non-vacuity anchor.
- **Derive the guard population** from the convention's five role nouns filtered by directory existence, double-anchored: a population floor (today's three role skills) plus a SET-EQUALITY anchor against the `SKILL_*` variables `scripts/docket-config.sh` emits, so an added sixth role reddens the guard.
- **Close the co-occurrence gap** with a second matcher (role-noun co-occurrence, hyphen-compound-excluding shape `\b$role role\b|docket-$role([^-]|$)`) with its own fire/ignore probes, and update the header's disclosed-limitations block honestly.
- **Tighten WORDS** to claim-shaped, modifier-tolerant phrasing (`the ([a-z]+ )?default|by default|instead of|alternative to|opt-in`) so operational "defaults to" sentences stop false-positiving while the house phrasing "the shipped default" still fires; new ignore probe added.

All matcher shapes were empirically verified against the current tree under both ugrep and `/usr/bin/grep` (zero hits on conforming bodies; probes fire/ignore as specified).

## Out of scope

- Softening the 0194 convention rule (rejected — enforce).
- New role skills.

## Open questions

- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Re-target: `skills/docket-review/SKILL.md` still has zero occurrences of `skills.review`, so the prose fix stands. The guard the spec hardens (`tests/test_role_skill_self_description.sh`, `claim_hits()`, `ROLE_SKILLS=`) is deleted; move the positive-half assert to `internal/repoguard/prose_contracts_test.go`, deriving role names from `internal/config`.

