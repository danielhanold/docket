---
id: 431
slug: 'native-codex-acceptance-for-active-worker-validation'
title: 'Native Codex acceptance for active worker validation'
status: 'implemented'
priority: 'low'
type: 'chore'
created: '2026-09-16'
updated: '2026-09-17'
depends_on: []
stacked_on: 425
related: [425, 430]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md'
plan: 'docs/superpowers/plans/2026-09-16-native-codex-acceptance-for-active-worker-validation-plan.md'
results: 'docs/results/2026-09-16-native-codex-acceptance-for-active-worker-validation-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/native-codex-acceptance-for-active-worker-validation'
pr: 'https://github.com/danielhanold/docket/pull/309'
blocked_by:
reconciled: true
claimed_at: '2026-09-17T12:11:08Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md) |
| Plan | [2026-09-16-native-codex-acceptance-for-active-worker-validation-plan.md](https://github.com/danielhanold/docket/blob/chore/native-codex-acceptance-for-active-worker-validation/docs/superpowers/plans/2026-09-16-native-codex-acceptance-for-active-worker-validation-plan.md) |
| Results | [2026-09-16-native-codex-acceptance-for-active-worker-validation-results.md](https://github.com/danielhanold/docket/blob/chore/native-codex-acceptance-for-active-worker-validation/docs/results/2026-09-16-native-codex-acceptance-for-active-worker-validation-results.md) |
<!-- docket:artifacts:end -->

## Why

Exercise the repaired native worker active-input check and completion ordering after acceptance 430 halted. Preserve change 430 and its failed fixture as evidence.

## What changes

On the published change 425 base, create internal/nativeacceptance/value.go and value_test.go with tested Value and Double functions. Require native planner, scoped worker, reviewer, both full configured gates, durable review evidence, results attachment, and a real stacked PR in danielhanold/docket. Use the fresh acceptance-active-validation launch kit and its pinned candidate.

## Out of scope

Do not merge, resume or reset change 430, modify unrelated changes, or weaken scope/input validation.

## Reconcile log

### 2026-09-16

2026-09-16: Reconciled against the prepared acceptance checkout and the published 425 effective base. The requested two-file Go package remains absent from the base, the stacked base and PR base remain codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor, and the scope remains valid.

## Finalize blocked

### 2026-09-17 — attempt 20260917T124602Z-95660e8fee6e

<!-- attempt:20260917T124602Z-95660e8fee6e -->

- Reason: gate-halted
- Head: f6695e5f8fbb198ba9458d40cfeb215a2af928bc
- PR: #309
- Comment: https://github.com/danielhanold/docket/pull/309#issuecomment-5714587729

Remedy: Resolve the finalize gate halt, then rerun finalize for change 431 by explicit id.
