---
id: 388
slug: 'reimplement-post-merge-fast-forward-integration-branch-sync'
title: 'Reimplement post-merge fast-forward integration-branch sync as a native Go verb'
status: 'proposed'
priority: 'medium'
type: 'feat'
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

Change 0370 deleted the orphaned sync-integration-branch.sh and corrected the convention prose, removing the post-merge fast-forward integration-branch sync capability with no native Go replacement. This was a working best-effort capability (FF-only sync of the integration branch after a merge lands) dropped only because it had no Go home, not because it was unwanted; without it the integration branch is not kept in sync after merges in Go v1.

## What changes

Design and implement a native Go docket verb that performs the post-merge best-effort FF-only integration-branch sync the deleted sync-integration-branch.sh provided (no-op in main-mode and on any non-FF/dirty/feature-branch tree), and wire it into the merge sites that previously called the script.

## Out of scope

Restoring the Bash script; any non-FF merge strategy; terminal publication (deferred from Go v1).
