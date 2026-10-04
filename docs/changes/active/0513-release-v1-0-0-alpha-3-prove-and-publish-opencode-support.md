---
id: 513
slug: 'release-v1-0-0-alpha-3-prove-and-publish-opencode-support'
title: 'Release v1.0.0-alpha.3: prove and publish OpenCode support'
status: 'proposed'
priority: 'high'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: [512]
stacked_on:
related: [366, 511, 512]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

This is the third step of the one-harness-at-a-time pre-release plan: Claude Code in alpha.1 (change 0366), Cursor in alpha.2 (change 0512), then OpenCode here. Until alpha.3 ships, OpenCode users have no tested release and no upgrade steps for their OpenCode files.

## What changes

- **Upgrade guide.** Add the OpenCode section to the Bash upgrade guide from change 0511, and add the OpenCode assertions to its test. The saved v0.9.2 and v0.9.3 cases already hold the OpenCode files.
- **Human test.** Run the full lifecycle in a fresh OpenCode process, as 0366 does for Claude Code.
- **Release.** Package, verify and publish `v1.0.0-alpha.3` with the same lighter protocol alpha.2 settles.

## Out of scope

- Codex (paused).
- Re-proving Claude Code or Cursor beyond what the shared protocol already re-runs.
