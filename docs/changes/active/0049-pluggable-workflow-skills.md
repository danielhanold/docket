---
id: 49
slug: pluggable-workflow-skills
title: Pluggable workflow skills — make superpowers invocations configurable, with auto fallback
status: implemented
priority: medium
created: 2026-07-08
updated: 2026-07-09
depends_on: []
related: [44, 16]
adrs: [18]
spec: docs/superpowers/specs/2026-07-08-pluggable-workflow-skills-design.md
plan: docs/superpowers/plans/2026-07-09-pluggable-workflow-skills.md
results: docs/results/2026-07-09-pluggable-workflow-skills-results.md
trivial: false
auto_groomable:
branch: feat/pluggable-workflow-skills
pr: https://github.com/danielhanold/docket/pull/58
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-08-pluggable-workflow-skills-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-08-pluggable-workflow-skills-design.md) |
| Plan | [2026-07-09-pluggable-workflow-skills.md](https://github.com/danielhanold/docket/blob/feat/pluggable-workflow-skills/docs/superpowers/plans/2026-07-09-pluggable-workflow-skills.md) |
| Results | [2026-07-09-pluggable-workflow-skills-results.md](https://github.com/danielhanold/docket/blob/feat/pluggable-workflow-skills/docs/results/2026-07-09-pluggable-workflow-skills-results.md) |
| PR | [#58](https://github.com/danielhanold/docket/pull/58) |
| ADRs | [ADR-0018](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0018-pluggable-skills-passthrough-degrade.md) |
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

### 2026-07-09 — reconcile (build-time, autonomous)

Verified against current `origin/main` (`4f9f071`) and recently-merged changes 0045–0048.

- **Invocation points unchanged.** All five hard-coded superpowers invocations sit exactly where
  the spec's table places them — brainstorm in `docket-new-change` step 2 and `docket-groom-next`;
  plan / build / review / finish in `docket-implement-next` steps 4 / 5 / 6 / 7; finish also in
  `docket-finalize-change`'s non-standard close-out. The prose-only mentions (the `do NOT continue
  to writing-plans` guards, "re-brainstorming is a human act handled by `superpowers:brainstorming`")
  remain out of scope, as the change body states.
- **Recent merges are orthogonal.** 0045 / 0046 / 0048 (multi-harness + harness-first agent-model
  generation) and 0047 (README discoverability) all touch the **agent-model** axis — which model
  each docket subagent runs at — not the **skill-invocation** axis this change adds. No overlap with
  the touched surfaces.
- **Harness-first `agents:` shape considered; skills held flat.** 0046 reshaped `agents:` into a
  harness-first map (`default:` + per-harness keys) because model IDs are harness-specific. Skill
  availability is a different axis — a per-machine property already handled by the spec's
  degrade-to-auto + warn rule — so a flat role-keyed `skills:` map stays correct: a Cursor /
  no-superpowers machine simply degrades an unavailable skill to `auto`. Harness-keyed skills remain
  out of scope (matches the spec's rejected "per-skill + step keys" alternative — deeper schema, no
  identified need).
- **#0044 guard still valid.** 0044 (configurable SDD build models) remains `proposed`; its
  `build.implementer` / `build.reviewer` knobs stay inert unless `skills.build` resolves to SDD,
  exactly as the spec's relationship section records.

No scope, body, or spec changes required — the design is current as drafted.
