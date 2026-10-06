---
id: 536
slug: 'split-the-private-leak-check-tests-out-of-the-workflow-lifec'
title: 'Split the private leak-check tests out of the workflow lifecycle shard'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-06'
updated: '2026-10-06'
depends_on: []
stacked_on:
related: [528, 273]
discovered_from: [532]
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

Change 532 added six private-mode leak-check integration tests under the `TestIntegrationWorkflowLifecycle` prefix. The `tests/test_go_integration_app_workflowlifecycle.sh` shard grew from about 55s to about 71s measured serially, and 532 raised its row in `tests/runtime-budgets.tsv` from 55 to 80 to absorb that. The suite's convention is that a ceiling moves when a file is re-shaped, not when it grows: an oversized shard is split (as 0242 and 0434 did) so the budget keeps meaning something.

## What changes

Move the private leak-check tests (`TestIntegrationWorkflowLifecyclePrivateLeakCheck*` and `TestIntegrationWorkflowLifecycleSharedPublishRunsNoLeakCheck`) to their own test-name prefix and a new Go integration shard script beside the existing ones, declared through `tests/lib/go-integration-shard.sh` and covered by `tests/test_go_integration_contract.sh`. Give the new shard its own budget row, and return the lifecycle shard's row to 55s after re-measuring it serially.

## Out of scope

Re-seeding or host-relativizing the budget table (#273). Detecting concurrent suites in the solo re-check (#528). Changing what the leak-check tests assert.
