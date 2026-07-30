---
id: 169
slug: codex-profile-routed-build-support
title: Codex support for profile-routed Docket builds
status: proposed
priority: medium
type: feat
created: 2026-07-30
updated: 2026-07-30
depends_on: [167]
related: [78, 79]
discovered_from: [167]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

Change 0167 intentionally supports Claude Code first. Codex uses TOML agent profiles,
`model_reasoning_effort`, and different native dispatch semantics; Claude model identifiers and
frontmatter cannot serve as a portable default.

## What changes

Design and implement Codex-native `economy`, `standard`, and `premium` build profiles, connect them
to Codex task dispatch, and validate explicit overrides, automatic routing, and one-step
escalation end to end.

## Out of scope

- Changing the shared task-worker contract established by change 0167.
- Cursor support or replacement of the whole-branch review skill.

## Open questions

- Which Codex models and reasoning-effort levels should ship for each profile?
- Should native Codex dispatch and Claude-parent runner delegation share one adapter contract?
