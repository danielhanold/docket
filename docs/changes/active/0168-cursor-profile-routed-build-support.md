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
related: [135, 142]
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

Change 0167 intentionally supports Claude Code first. Cursor has different nested-dispatch
mechanics and encodes effort in its model identifier, so reusing Claude profile wrappers would
either misroute work or silently lose the intended cost tier.

## What changes

Design and implement Cursor-native `economy`, `standard`, and `premium` build profiles, route them
through Cursor's supported agent-dispatch surface, and validate explicit overrides, automatic
routing, and one-step escalation end to end.

## Out of scope

- Changing the shared task-worker contract established by change 0167.
- Codex support or replacement of the whole-branch review skill.

## Open questions

- Which Cursor model identifiers should ship for the three profiles?
- How should Cursor's model-embedded effort values map to the three Docket profile names?
