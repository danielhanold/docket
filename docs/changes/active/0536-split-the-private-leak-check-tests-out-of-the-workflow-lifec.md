---
id: 536
slug: 'split-the-private-leak-check-tests-out-of-the-workflow-lifec'
title: 'Split the private-mode tests out of the workflow lifecycle shard'
status: 'implemented'
priority: 'low'
type: 'fix'
created: '2026-10-06'
updated: '2026-10-16'
depends_on: []
stacked_on:
related: [527, 528, 273, 531, 532]
discovered_from: [532]
adrs: []
spec: 'docs/superpowers/specs/2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec-design.md'
plan: 'docs/superpowers/plans/2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec.md'
results: 'docs/results/2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/split-the-private-leak-check-tests-out-of-the-workflow-lifec'
pr: 'https://github.com/danielhanold/docket/pull/420'
blocked_by:
reconciled: true
claimed_at: '2026-10-16T15:24:38Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec-design.md](../../superpowers/specs/2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec-design.md) |
| Plan | [2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec.md](../../superpowers/plans/2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec.md) |
| Results | [2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec-results.md](../../results/2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec-results.md) |
<!-- docket:artifacts:end -->

## Why

Private-visibility work grew the `tests/test_go_integration_app_workflowlifecycle.sh` shard from a 35s budget ceiling to 80s in three steps (change 531's private end-to-end acceptance test, then change 532's six leak-check tests), raising its `tests/runtime-budgets.tsv` row each time instead of re-shaping the file. A quiet serial run on 2026-10-16 measured 83s, already over the row. The suite's convention is to split an oversized shard so its ceiling keeps meaning something, as 0242 and 0434 did.

## What changes

Rename the seven private-mode integration tests (the end-to-end `…PrivateInitToImplemented` test and the six leak-check tests, the shared-mode negative included) onto one new prefix, `TestIntegrationPrivateMode`. Then select them from a new declarations-only shard, `tests/test_go_integration_app_privatemode.sh`, which the completeness contract discovers on its own. Give the new shard a budget row, and lower the lifecycle shard's row, with both numbers taken from fresh serial measurements under the table's own seeding rule. The rename table, the prefix constraint, and the verification steps are in the spec.

## Out of scope

Changing what any moved test asserts. The concurrency and reposetup shards' budgets (#527). Re-seeding or host-relativizing the budget table (#273). Detecting concurrent suites in the solo re-check (#528). Editing point-in-time records that name the old shard or tests.

## Reconcile log

### 2026-10-16

2026-10-16 — Re-checked against origin/main a0bd49b. All seven tests still carry their old names (workflow_private_integration_test.go, leak_check_private_integration_test.go), the lifecycle shard still selects prefix TestIntegrationWorkflowLifecycle at an 80s row, and no live SHARD_PREFIX collides with TestIntegrationPrivateMode in either direction (nearest: TestIntegrationPrivateInstructions, TestIntegrationRepoVisibilityPrivate). Related #527/#528 still proposed, #273 deferred; none overlaps. Scope unchanged.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: 7cdaca68007781067caeefdb91562fe5097492f3
ran_at:   2026-10-16T15:39:10Z
<!-- docket:build-evidence:end -->
