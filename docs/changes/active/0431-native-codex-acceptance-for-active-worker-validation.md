---
id: 431
slug: 'native-codex-acceptance-for-active-worker-validation'
title: 'Native Codex acceptance for active worker validation'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-09-16'
updated: '2026-09-16'
depends_on: []
stacked_on: 425
related: [425, 430]
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

Exercise the repaired native worker active-input check and completion ordering after acceptance 430 halted. Preserve change 430 and its failed fixture as evidence.

## What changes

On the published change 425 base, create internal/nativeacceptance/value.go and value_test.go with tested Value and Double functions. Require native planner, scoped worker, reviewer, both full configured gates, durable review evidence, results attachment, and a real stacked PR in danielhanold/docket. Use the fresh acceptance-active-validation launch kit and its pinned candidate.

## Out of scope

Do not merge, resume or reset change 430, modify unrelated changes, or weaken scope/input validation.
