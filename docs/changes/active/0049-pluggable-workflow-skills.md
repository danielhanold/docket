---
id: 49
slug: pluggable-workflow-skills
title: Pluggable workflow skills — make superpowers invocations configurable, with auto fallback
status: in-progress
priority: medium
created: 2026-07-08
updated: 2026-07-09
depends_on: []
related: [44, 16]
adrs: []
spec: docs/superpowers/specs/2026-07-08-pluggable-workflow-skills-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/pluggable-workflow-skills
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-08-pluggable-workflow-skills-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-08-pluggable-workflow-skills-design.md) |
<!-- docket:artifacts:end -->

## Why

docket hard-codes five superpowers skill invocations — brainstorm
(`superpowers:brainstorming`), plan (`superpowers:writing-plans`), build
(`superpowers:subagent-driven-development`), review (`superpowers:requesting-code-review`), and
finish (`superpowers:finishing-a-development-branch`) — across `docket-new-change`,
`docket-groom-next`, `docket-implement-next`, and `docket-finalize-change`. A human who does not
want or does not have the superpowers plugin cannot use docket at all; SDD in particular is
heavyweight subagent machinery some users don't need. There is no way to substitute a different
skill, or no skill, at any of these points.

## What changes

A role-keyed **`skills:` map in `.docket.yml`** covering the five pluggable invocation points
(`brainstorm`, `plan`, `build`, `review`, `finish`); full detail in the linked spec:

- Each value is a **skill name passed verbatim** to the Skill tool (unvalidated passthrough, the
  ADR-0015 philosophy) or the sentinel **`auto`** — no skill; the running agent performs the step
  inline at its own model. Unset keys default to today's superpowers skills (absent config is
  byte-identical to current behavior).
- **Auto fallbacks define the final artifact only** (spec file / plan file / executed plan /
  pre-PR review / open-PR-and-stop) — never the method.
- **Missing skill ⇒ degrade to auto + prominent warning**, so a repo without superpowers works
  with zero config.
- Resolution rides `docket-config.sh --export` (five new `SKILL_*` variables); the convention
  gains a "Skill layer" section; the four invoking skill bodies switch to the resolved skill.

## Out of scope

- Renaming `docs/superpowers/…` spec/plan paths — they stay as-is.
- #0044's SDD-internal model knobs (`build.implementer`/`build.reviewer`) — separate change; they
  simply become inert unless `skills.build` resolves to SDD.
- Per-invocation-site config (e.g. different brainstorm skills for new-change vs groom-next).
- Prose-reference mentions of superpowers skills (auto-groom's "do NOT invoke", status's borrowed
  provenance guard, groom-next's "do NOT continue to writing-plans") — not invocations, untouched.

## Open questions

- Availability probe for the missing-skill rule (Skill-tool listing vs attempt-and-catch) —
  decide against harness behavior at build.
- Whether generated agent wrapper directives need wording changes (expected no-op; verify).
- One ADR likely (passthrough + degrade-to-auto posture) — decided at build.

## Reconcile log
