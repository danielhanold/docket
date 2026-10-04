---
id: 512
slug: 'release-v1-0-0-alpha-2-prove-and-publish-cursor-support'
title: 'Release v1.0.0-alpha.2: prove and publish Cursor support'
status: 'proposed'
priority: 'high'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: [366]
stacked_on:
related: [366, 511]
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

The human decided to grow the Go pre-releases one harness at a time:

- `v1.0.0-alpha.1` (change 0366) is human-tested on Claude Code only.
- Cursor comes next, in alpha.2.
- OpenCode follows in alpha.3.

Until alpha.2 ships, Cursor users have no tested release and no upgrade steps for their Cursor files.

## What changes

- **Upgrade guide.** Add the Cursor section to the Bash upgrade guide from change 0511, and add the Cursor assertions to its test. The saved v0.9.2 and v0.9.3 cases already hold the Cursor files.
- **Human test.** Run the full lifecycle in a fresh Cursor process (IDE), as 0366 does for Claude Code.
- **Release.** Package, verify and publish `v1.0.0-alpha.2` with a lighter version of 0366's protocol: the new harness row plus the release basics. Settle the exact trimmed protocol at grooming.

## Out of scope

- OpenCode (alpha.3).
- Codex (paused).
- Re-proving Claude Code beyond what the shared protocol already re-runs.
