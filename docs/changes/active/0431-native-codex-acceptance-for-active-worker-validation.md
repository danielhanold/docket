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
results: 'docs/results/2026-09-16-native-codex-acceptance-for-active-worker-validation-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/native-codex-acceptance-for-active-worker-validation'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-17T11:55:29Z'
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

## Run halted

### 2026-09-17

The required native docket-review-standard review completed against certified feature head cfa62e0b3fc1ac773ea16e3e2c48cd232063290e and returned one blocker: internal/gatedrive/admission.go invokes replacement(stored.Record.RunEpochID, rec.RunEpochID, canonical, ""), so a verified replacement epoch cannot reclaim a released worktree slot because its attributed change ID is omitted. The prescribed repair is to pass rec.ChangeID and add end-to-end replacement-admission coverage. This acceptance run is constrained to the pinned e8b53918 executable and manual configuration changes to existing 431 docket files; repairing and rebuilding the runtime is outside that authority. The blocker is therefore durably halted without results publication, final certification, PR publication, or implemented transition.
