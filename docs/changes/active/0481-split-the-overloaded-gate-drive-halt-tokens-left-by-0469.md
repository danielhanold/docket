---
id: 481
slug: 'split-the-overloaded-gate-drive-halt-tokens-left-by-0469'
title: 'Split the overloaded gate-drive halt tokens left by 0469'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-10-01'
updated: '2026-10-01'
depends_on: [469]
stacked_on:
related: []
discovered_from: [469]
adrs: [129]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0469 renamed two gate-drive halt tokens, and each new name now covers one condition it does not describe. `worktree-changed` (formerly `identity-mismatch`) also fires when a takeover finds its recorded scope (branch, worktree, or change) no longer matches in `internal/gatedrive/takeover.go`, where the worktree itself did not change. `launch-unconfirmed` also fires when `resolveDriveRun` loses its worktree-slot link. Behavior is correct; the token misleads whoever reads the halt and picks a remedy. Deferred from PR #358 review finding 1, and first flagged during 0469's initial run.

## What changes

Give each mis-covered condition its own halt token: a scope-mismatch token for the takeover case and a separate token for the lost worktree-slot link in `resolveDriveRun`. Record the new tokens with an `## Update` to ADR-0129. Update the emitting sites, the verdict/report consumers, the operator-facing prose that maps tokens to remedies, and the tests that assert them.

## Out of scope

Changing when the gate drive halts or how it recovers; any other ADR-0129 rename rows; the leftover 0469 wording ("repair" in `change relink`, "Step 0"), tracked separately.
