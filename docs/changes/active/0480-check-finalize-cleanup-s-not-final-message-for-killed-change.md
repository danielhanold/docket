---
id: 480
slug: 'check-finalize-cleanup-s-not-final-message-for-killed-change'
title: 'Check finalize cleanup''s not-final message for killed changes'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-09-30'
updated: '2026-09-30'
depends_on: []
stacked_on:
related: []
discovered_from: [474]
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

Change 0474 renamed "terminal" to "final" across the lifecycle vocabulary. During that work, `finalize cleanup`'s default branch was found to emit a "not final" message that was carried over unchanged from the old "not terminal" wording. If a killed change can reach that default branch, the message is inaccurate, because a killed change is in a final status. Nobody has traced whether that path is reachable.

## What changes

Trace `finalize cleanup`'s default branch to confirm whether a killed change (or any final-status change) can reach it. If it can, correct the message so it describes the real condition. If it cannot, record that and close the stub without a code change.

## Out of scope

Any wider rewording of lifecycle messages beyond this one branch. Changes to which statuses `finalize cleanup` accepts.
