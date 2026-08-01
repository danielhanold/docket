---
id: 184
slug: four-tier-build-profile-ladder
title: Four-tier build profile ladder — low/medium/high/max replaces economy/standard/premium
status: in-progress
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
results:
trivial: false
auto_groomable:
branch: feat/four-tier-build-profile-ladder
claimed_at: 2026-08-01T12:21:05Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-01-four-tier-build-profiles-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-four-tier-build-profiles-design.md) |
| Plan | [2026-08-01-four-tier-build-profile-ladder.md](https://github.com/danielhanold/docket/blob/feat/four-tier-build-profile-ladder/docs/superpowers/plans/2026-08-01-four-tier-build-profile-ladder.md) |
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

Replace the three build profiles with four — `low` / `medium` / `high` / `max` — as a
clean break (old names invalid, no aliases):

- **Routing:** `max` is reachable only by a two-item rubric (unresolved architecture,
  irreversible data changes), a plan override, or escalation from `high`; the other
  current premium triggers demote to `high`; `medium` stays the default; `low` keeps
  the positively-established bar.
- **Ladders:** escalation stays one-rung-once (`low→medium→high→max→halt`); the
  integration-repair ladder becomes `high → max`, preserving today's effective repair
  strength.
- **Pins:** compressed — `max` inherits today's premium pin on every harness, each
  rung below is at or below today's cost, and the default tier (`medium`) drops a
  notch (the bulk of the savings). Claude `build-low` ships Sonnet/low, with Haiku
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
