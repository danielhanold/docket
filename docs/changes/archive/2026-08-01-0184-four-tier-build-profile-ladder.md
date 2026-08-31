---
id: 184
slug: four-tier-build-profile-ladder
title: Four-tier build profile ladder — economy/standard/premium/max
status: done
priority: high
type: feat
created: 2026-08-01
updated: 2026-08-01
depends_on: []
related: [44, 167, 168, 169]
discovered_from: []
adrs: [15, 16, 63]
spec: docs/superpowers/specs/2026-08-01-four-tier-build-profiles-design.md
plan: docs/superpowers/plans/2026-08-01-four-tier-build-profile-ladder.md
results: docs/results/2026-08-01-four-tier-build-profile-ladder-results.md
trivial: false
auto_groomable:
branch: feat/four-tier-build-profile-ladder
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/147
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-01-four-tier-build-profiles-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-four-tier-build-profiles-design.md) |
| Plan | [2026-08-01-four-tier-build-profile-ladder.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-01-four-tier-build-profile-ladder.md) |
| Results | [2026-08-01-four-tier-build-profile-ladder-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-01-four-tier-build-profile-ladder-results.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md) |
<!-- docket:artifacts:end -->

## Why

docket-build's three profiles put too much work on the top tier. The premium
named-risk triggers (auth, migrations, concurrency, release infra, unresolved
architecture) sweep ordinary risk-adjacent work into the most expensive profile, and
every failed `standard` task escalates straight to the top — so the tier meant for
extreme cases runs routinely. Structurally, three levels leave no room for both a
routine-safe "risky work" tier and a genuinely rare top tier. Separately, the Claude
harness pins all three profiles to the same model, so `economy` never delivered a
truly cheap floor.

## What changes

Add a fourth build profile above the existing three, keeping their names:
`economy` / `standard` / `premium` / `max`.

> **Naming, settled late.** This spec was built as `low` / `medium` / `high` / `max` and renamed
> before merge. Two objections: the names collided with the `effort:` vocabulary in
> `agents/harness-defaults.yml`, where the two ladders deliberately disagree row by row
> (`build-low` at effort `xhigh` on Codex), and none of them identified which rung was the
> default. Rungs 1–3 revert to their pre-0184 names; `max` — the rung this change adds — is
> unaffected. Structure, routing, and pins below are unchanged by the rename.

- **Routing:** `max` is reachable only by a two-item rubric (unresolved architecture,
  irreversible data changes), a plan override, or escalation from `premium`; the other
  current top-rung triggers demote to `premium`; `standard` stays the default; `economy`
  keeps the positively-established bar.
- **Ladders:** escalation stays one-rung-once (`economy→standard→premium→max→halt`); the
  integration-repair ladder becomes `premium → max`, preserving today's effective repair
  strength.
- **Pins:** compressed — `max` inherits today's top-rung pin on every harness, each
  rung below is at or below today's cost, and the default tier (`standard`) drops a
  notch (the bulk of the savings). Claude `build-economy` ships Sonnet/low, with Haiku
  documented as the cost-aggressive user-layer override.
- **Surface:** rename/add the wrapper agents (thirteen wrappers, four build workers),
  4-row build sets in all three `harness-defaults.yml` harness blocks, and update the
  profile language in docket-build, docket-build-task, and docket-convention.

Full rubric, ladders, pin tables, ripple list, and compatibility posture are in the
linked spec.

## Out of scope

- Changes to the docket-build-task worker contract beyond profile-name references.
- Routing telemetry / cost accounting.
- Pin changes for non-build agents.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-01 — reconciled, no design drift

The change was groomed the same day it is being built, so the spec's premises were
re-verified rather than revised. Scope is unchanged; the spec needed no edit.

- **All four `related` changes are terminal.** 0044 (configurable build model), 0167
  (lean profile-routed build), 0168 (cursor support), and 0169 (codex support) are
  archived `done`. Nothing in flight overlaps this change, and the three-profile system
  this replaces is fully landed on `main`.
- **Shipped pins match the spec's "today's pin" claims exactly**, so the compression
  table is buildable as written: claude `build-economy/standard/premium` =
  opus-5 at low/medium/high; cursor = grok-4.5-medium / grok-4.5-high /
  claude-opus-5-high; codex = luna/xhigh, terra/high, sol/medium. Every "today's X pin"
  annotation in the spec's pin table verified against `agents/harness-defaults.yml` on
  `origin/main`.
- **Ripple list extended with concrete files the spec named only by role.** Its
  "dispatch-rule generation — verify no hardcoded profile names remain" resolves to
  committed per-agent fragments: `cursor-rules/dispatch/docket-build-{economy,standard,premium}.md`
  (rename three, add a fourth) plus `cursor-rules/dispatch.head.md`. Live prose sites
  beyond the spec's list: `README.md` (two wrapper-count sentences),
  `docs/cursor/validation.md` (three sites, including an explicit-routing walkthrough
  that names `economy`), `docs/codex/setup.md` (twelve-agent claim),
  `.docket.example.yml` (three shipped blocks mirrored value for value), and the test
  suite (`test_docket_build.sh`, `test_harness_defaults.sh`, `test_sync_agents.sh`,
  `test_sync_agents_cursor.sh`, `test_docket_example_yml.sh`).
- **Historical records are explicitly out of scope for rewriting** — archived change
  files, plans, results, prior specs, and ADR-0063/0064 keep their original
  economy/standard/premium prose. They record what was true when written; only live
  surfaces are renamed.
- **Coupling noted, no action:** stub #183 (cursor-dispatch head ships a stale unpinned
  claim) touches `cursor-rules/dispatch.head.md`, which this change also edits. #183 is
  needs-brainstorm and unclaimed, so there is no concurrent-edit risk; whichever lands
  second reconciles against the other.
- **Auto-capture:** enabled, nothing minted at this pass — every follow-up this reconcile
  surfaced is already inside the change's own scope or already tracked as #183.
