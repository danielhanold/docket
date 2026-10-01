---
id: 483
slug: 'clean-up-a-killed-change-s-workspace-in-finalize-cleanup'
title: 'Clean up a killed change''s workspace in finalize cleanup'
status: 'proposed'
priority: 'low'
type: 'feat'
created: '2026-10-01'
updated: '2026-10-01'
depends_on: []
stacked_on:
related: [480]
discovered_from: [480]
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

`finalize cleanup` only tears down resources for `done` changes. A killed change (most realistically an `in-progress` change killed by the implementer's reconcile pass after its workspace or branch already exists) has its feature worktree and branch left behind with no automated removal. Change 0480 makes cleanup report this truthfully (`no-op` / `retained` / reason `killed-retained`) and corrects the close-out docs that promised pruning, but the resources still accumulate until a human removes them by hand.

## What changes

Teach `finalize cleanup` to handle a `killed` change: remove its feature workspace when ownership is provable and the workspace is clean, and retain any feature branch whose work never merged (the behaviour `close-out.md` step 4 originally described). Retire the `killed-retained` interim reason once real cleanup replaces it, and update the close-out kill-path docs to match.

## Out of scope

Deleting unmerged feature branches or remote refs of a killed change. Any change to the `done` or `stacked-merged` cleanup paths.
