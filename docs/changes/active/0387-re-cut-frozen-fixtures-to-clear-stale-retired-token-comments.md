---
id: 387
slug: 're-cut-frozen-fixtures-to-clear-stale-retired-token-comments'
title: 'Re-cut frozen fixtures to clear stale retired-token comments in harness-defaults and .docket.yml'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: []
discovered_from: [370]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
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

After change 0370 retired the Bash facade, comments at agents/harness-defaults.yml:7 and .docket.yml:20 still describe now-retired tokens. They cannot be truthfully corrected in isolation because both lines are byte-pinned to frozen versioned fixtures; an honest fix needs a coordinated fixture re-cut so the pinned bytes and the live comments stay in agreement.

## What changes

Coordinate a versioned re-cut of the frozen fixtures that pin agents/harness-defaults.yml and .docket.yml so the stale retired-token comments can be corrected without breaking the byte-pinned fixture assertions, then update the comments to reflect the post-0370 vocabulary.

## Out of scope

Changing the resolved default values themselves; any behavior change to config resolution.
