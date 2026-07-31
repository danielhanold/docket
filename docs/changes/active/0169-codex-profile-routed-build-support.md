---
id: 169
slug: codex-profile-routed-build-support
title: Codex support for profile-routed Docket builds
status: in-progress
priority: medium
type: feat
created: 2026-07-30
updated: 2026-07-31
depends_on: [167, 168]
related: [77, 78, 79]
discovered_from: [167]
adrs: [36, 37, 38, 63, 64]
spec: docs/superpowers/specs/2026-07-31-codex-profile-routed-build-support-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/codex-profile-routed-build-support
claimed_at: 2026-07-31T18:04:32Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-31-codex-profile-routed-build-support-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-31-codex-profile-routed-build-support-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0037](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0037-runner-delegation-explicit-runner-field.md), [ADR-0038](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0038-runner-shim-wrapper-single-dispatch-chokepoint.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0064](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md) |
<!-- docket:artifacts:end -->

## Why

Change 0167 shipped Docket's profile-routed build under Claude first. Change 0168 then moved all
shipped agent defaults into the harness-indexed `agents/harness-defaults.yml` sidecar and made each
shipped harness complete across all twelve wrappers. Codex already has native TOML generation and
dispatch, but it is the remaining known harness with no shipped block, so every Codex wrapper is
honestly unpinned today.

The missing work is now narrow: supply one complete, validated Codex mapping, certify the three
build profiles under the native harness, and flip the guards and documentation that deliberately
encode the current unpinned state.

## What changes

- Add a complete twelve-agent `codex:` sidecar block. Promote the existing nine illustrative
  mappings unchanged and ship the selected build profiles: Luna/xhigh for economy, Terra/high for
  standard, and Sol/medium for premium.
- Add Codex to the shipped-harness completeness gate and promote `.docket.example.yml`'s Codex
  block from a doubly commented illustration to a singly commented exact mirror.
- Reuse the existing Codex TOML emitter and native named-agent dispatch. Keep whole-run runner
  delegation separate and prove shipped native defaults never leak into runner flags.
- Replace the pre-0169 TOML absence assertions with shipped-value assertions, extend the derived
  mirror/round-trip guards, and update maintained Codex and build-profile documentation.
- Certify all three named profile dispatches in a real Codex session. Automatic classification and
  single escalation remain hermetically covered, with the live-observation waiver recorded in the
  results artifact.

## Out of scope

- Changing the shared task-worker contract established by change 0167.
- Adding a Codex-specific controller branch or invoking `codex exec` once per task.
- Cursor support (change 0168) or replacement of the whole-branch review skill.
- Revisiting ADR-0064's sidecar design. This change is a consumer of it: it adds a harness block
  and satisfies the existing completeness rule. If Codex needs a shape the sidecar cannot express,
  that is a new ADR, not an edit to that one.

## Reconcile log
