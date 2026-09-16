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
stacked_on: 425
related: [425]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-09-16-native-codex-acceptance-for-durable-review-evidence-design.md'
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
| Spec | [2026-09-16-native-codex-acceptance-for-durable-review-evidence-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-16-native-codex-acceptance-for-durable-review-evidence-design.md) |
<!-- docket:artifacts:end -->

## Why

Verify native planner, scoped worker, and reviewer dispatch after repairing the evidence.record file-resource handoff, through a real PR in danielhanold/docket.

## What changes

Stack on change 425 and add a small internal/nativeacceptance package with tested Value and Double functions. Exercise the real native planner, scoped worker, durable evidence resource, native reviewer, full configured suites, results attachment, and a PR in danielhanold/docket targeting the published 425 branch. The acceptance/native-evidence-20260916 checkout supplies launch resources only.

## Out of scope

Do not merge the acceptance PR or modify unrelated changes. This is a branch-scoped acceptance fixture, not a change to main's production behavior.
