---
id: 168
slug: cursor-profile-routed-build-support
title: Cursor support for profile-routed Docket builds
status: proposed
priority: medium
type: feat
created: 2026-07-30
updated: 2026-07-30
depends_on: [167]
related: [135, 142, 164, 169]
discovered_from: [167]
adrs: [15, 16, 60, 63]
spec: docs/superpowers/specs/2026-07-30-cursor-profile-routed-build-support-design.md
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
| Spec | [2026-07-30-cursor-profile-routed-build-support-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-30-cursor-profile-routed-build-support-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md) |
<!-- docket:artifacts:end -->

## Why

Change 0167 shipped Docket's lean, profile-routed build with Claude defaults only. Cursor can
translate and dispatch the workers, but without native defaults it inherits a foreign model ID or
requires every user to supply the same overrides. The wrapper sources also double as the Claude
default store, so adding one-off Cursor exceptions would leave shipped defaults split across two
mechanisms.

## What changes

- Add one harness-indexed shipped-default sidecar: complete for all twelve Claude agents and sparse
  for Cursor's three build workers. Make the wrapper sources behavior-only templates.
- Ship Cursor build mappings for `economy` (`cursor-grok-4.5-medium`), `standard`
  (`cursor-grok-4.5-high`), and `premium` (`claude-opus-5-high`), each with its effort already
  encoded in the Cursor model ID.
- Preserve field-level user overrides and keep shipped native defaults out of delegated
  child-runner flags.
- Make unsupported harness/agent combinations inherit their own harness default rather than a
  Claude source value; change 0169 will add Codex defaults to the same sidecar.
- Certify explicit routing, automatic routing, and one bounded escalation in the Cursor IDE.
- Make the README skill catalog complete under a count-free `## Skills` heading and update the
  build documentation for shipped Claude and Cursor support.

## Out of scope

- Changing the shared task-worker contract established by change 0167.
- Shipping defaults for Cursor's other nine agents or any Codex agents.
- Runtime model discovery, Cursor CLI certification, or replacement of the whole-branch review
  skill.
