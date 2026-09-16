---
id: 430
slug: 'native-codex-acceptance-for-durable-review-evidence'
title: 'Native Codex acceptance for durable review evidence'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-09-16'
updated: '2026-09-16'
depends_on: []
stacked_on:
related: [425]
discovered_from: []
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

Verify native planner, scoped worker, and reviewer dispatch after repairing the evidence.record file-resource handoff, through a real PR in danielhanold/docket.

## What changes

On the dedicated acceptance/native-evidence-20260916 integration branch, add Double() int beside the prepared internal/nativeacceptance Value fixture and cover it with a red/green test. Exercise durable evidence, results attachment, and the implemented transition.

## Out of scope

Do not merge the acceptance PR or modify unrelated changes. This is a branch-scoped acceptance fixture, not a change to main's production behavior.
