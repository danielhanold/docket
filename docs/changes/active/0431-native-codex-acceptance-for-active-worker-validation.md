---
id: 431
slug: 'native-codex-acceptance-for-active-worker-validation'
title: 'Native Codex acceptance for active worker validation'
status: 'in-progress'
priority: 'low'
type: 'chore'
created: '2026-09-16'
updated: '2026-09-16'
depends_on: []
stacked_on: 425
related: [425, 430]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md'
plan: 'docs/superpowers/plans/2026-09-16-native-codex-acceptance-for-active-worker-validation-plan.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/native-codex-acceptance-for-active-worker-validation'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-16T14:58:03Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md) |
| Plan | [2026-09-16-native-codex-acceptance-for-active-worker-validation-plan.md](https://github.com/danielhanold/docket/blob/chore/native-codex-acceptance-for-active-worker-validation/docs/superpowers/plans/2026-09-16-native-codex-acceptance-for-active-worker-validation-plan.md) |
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

## Run halted

### 2026-09-16

Native economy worker task-1 passed immutable continuation input checks but its first required RED gate.drive.start was rejected with invalid-input/stale-run-epoch: an in-flight run owns the feature worktree despite replacement epoch d9cdc98d9fe81183d0b04f7377623bf2. No RED test ran, no implementation files were created, and no commit exists. The inherited untracked internal/nativeacceptance/value_test.go remains byte-identical at SHA-256 f0ce49d43b268ba06bb285505f18fb0a4bee8ba60ffec7e8e032faa19144726e. Evidence: resume-431-prepare/control/receipts/431-worker-task-1-red.stdout. Resolve or cancel the still-owning run through the gate facade before another attributed resume; do not start a replacement worker.
