---
id: 193
slug: default-build-review-roles-to-docket-owned-skills
title: Default the build and review roles to docket-build and docket-review
status: proposed
priority: medium
type: chore
created: 2026-08-02
updated: 2026-08-02
depends_on: []
related: [167, 170]
discovered_from: [170]
adrs: [63, 66]
spec:
plan:
results:
trivial: true
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
| ADRs | [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md) |
<!-- docket:artifacts:end -->

## Why

Changes 0167 and 0170 built docket's own build and review roles — `docket-build` (profile-routed
per-task workers) and `docket-review` (one bounded read-only whole-branch reviewer behind three
pinned rungs) — but shipped them as opt-in. The built-in defaults in `docket-config.sh` still
resolve `skills.build` to `superpowers:subagent-driven-development` and `skills.review` to
`superpowers:requesting-code-review`; docket's own repo picks up the new roles only because
`.docket.yml` overrides them.

Both roles have now been exercised enough on this repo to be trusted as the shipped default. Every
consuming repo should get them without opting in, and docket's own override should go away so the
repo genuinely dogfoods the default rather than a pin over it.

## What changes

- Flip the two built-in defaults in `scripts/docket-config.sh`: `SKILL_BUILD=docket-build`,
  `SKILL_REVIEW=docket-review`.
- Remove the now-redundant `skills:` block from this repo's `.docket.yml` (and its explanatory
  comment), so docket runs on the shipped defaults.
- Update the live documentation that names the old defaults: `.docket.example.yml`, `README.md`,
  `scripts/docket-config.md`, and the *Skill layer* role table in `skills/docket-convention/SKILL.md`
  plus any restatement in `skills/docket-implement-next/SKILL.md`.
- Update the affected tests — `tests/test_docket_config.sh` (default-resolution assertions),
  `tests/test_docket_example_yml.sh` (mirror/drift guards), and any default-naming assertions in
  `tests/test_docket_build.sh` / `tests/test_docket_review.sh`.
- Note the reversal of intent against ADR-0063 and ADR-0066 only if their text asserts the
  opt-in-not-default posture; if so, record the flip per the ADR rules rather than editing them.

## Out of scope

- Any behavior change inside `docket-build` or `docket-review` themselves — this is a default flip,
  not a redesign of either role.
- The other three roles (`brainstorm`, `plan`, `finish`) keep their superpowers defaults.
- Historical records (archived changes, plans, specs, results, existing ADR bodies) are immutable
  and are not rewritten to match the new default.

## Open questions

- Non-Claude harnesses: `docket-build`/`docket-review` dispatch profile/rung subagents, so verify a
  Codex/Cursor-shaped install still resolves sensibly on the new default (the Tier-C
  authorized-or-halt posture should cover it, but confirm the docs say so).

## Reconcile log
