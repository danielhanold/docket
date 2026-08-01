---
id: 184
slug: four-tier-build-profile-ladder
title: Four-tier build profile ladder — low/medium/high/max replaces economy/standard/premium
status: proposed
priority: medium
type: feat
created: 2026-08-01
updated: 2026-08-01
depends_on: []
related: [44, 167, 168, 169]
discovered_from: []
adrs: [15, 16, 63]
spec: docs/superpowers/specs/2026-08-01-four-tier-build-profiles-design.md
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
| Spec | [2026-08-01-four-tier-build-profiles-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-four-tier-build-profiles-design.md) |
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
