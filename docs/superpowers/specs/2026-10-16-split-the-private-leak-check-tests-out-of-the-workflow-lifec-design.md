<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0536 — Split the private-mode tests out of the workflow lifecycle shard](../../changes/active/0536-split-the-private-leak-check-tests-out-of-the-workflow-lifec.md)**
<!-- docket:backlink:end -->

# Split the private-mode tests out of the workflow lifecycle shard — design

Change #536, groomed 2026-10-16.

## Problem

`tests/test_go_integration_app_workflowlifecycle.sh` selects every `internal/app` integration
test whose name starts with `TestIntegrationWorkflowLifecycle`. Private-visibility work grew it
from a 35s ceiling to 80s in three steps, each raising the `tests/runtime-budgets.tsv` row
instead of re-shaping the file:

| Step | Added | Row |
|---|---|---|
| private-visibility acceptance (change 530) | `…PrivateInitToImplemented` | 35 → 55 |
| feature-push leak check (change 532) | five `…PrivateLeakCheck*` tests + `…SharedPublishRunsNoLeakCheck` | 55 → 70 |
| PR-write leak check (change 532) | `…PrivateLeakCheckGatesPR` | 70 → 80 |

A quiet serial run on 2026-10-16 measured **83s wall** for the shard, already above its 80s row.
Per-test sums from that run:

- the seven private-mode tests: **42.2s** (six leak-check tests 31.6s, the end-to-end test 10.6s)
- the remaining 21 lifecycle tests: **37.4s**

The suite's convention (`tests/README.md`, "If that shard has no room … start a new one") is to
split an oversized shard so that its ceiling keeps meaning something, not to keep raising it.

## Design

Move all seven private-mode tests into one new shard, keyed on one new prefix,
`TestIntegrationPrivateMode`. One literal prefix per shard is required:
`tests/test_go_integration_contract.sh` check (4) matches each tagged test against each runner's
prefix as a literal glob (`case "$t" in "$rx"*)`), so a regex alternation in `SHARD_PREFIX` would
break the exactly-one-runner check.

### Renames (functions only; files stay where they are)

| Current name | New name |
|---|---|
| `TestIntegrationWorkflowLifecyclePrivateInitToImplemented` | `TestIntegrationPrivateModeInitToImplemented` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckBlocksSeededLeaks` | `TestIntegrationPrivateModeLeakCheckBlocksSeededLeaks` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckMatchWordOff` | `TestIntegrationPrivateModeLeakCheckMatchWordOff` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckUnverifiedPushesNothing` | `TestIntegrationPrivateModeLeakCheckUnverifiedPushesNothing` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckScansBranchName` | `TestIntegrationPrivateModeLeakCheckScansBranchName` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckGatesPR` | `TestIntegrationPrivateModeLeakCheckGatesPR` |
| `TestIntegrationWorkflowLifecycleSharedPublishRunsNoLeakCheck` | `TestIntegrationPrivateModeLeakCheckSkipsSharedPublish` |

The last one is the shared-mode negative ("the private-mode leak check never runs on a shared
publish"); its new name says that, and it still starts with the shard prefix.

The new prefix collides with no existing shard prefix in either direction
(`TestIntegrationPrivateInstructions`, `TestIntegrationRepoVisibilityPrivate`, and the rest).
The implementer re-checks this against the live set of `SHARD_PREFIX` declarations rather than
trusting this list.

Update the doc comment above each renamed function, and the file-header paragraphs of
`internal/app/workflow_private_integration_test.go` and
`internal/app/leak_check_private_integration_test.go`, which both currently describe themselves as
slices "of the workflow lifecycle shard (prefix TestIntegrationWorkflowLifecycle)". Both should
name the new shard and prefix instead.

### New shard script

`tests/test_go_integration_app_privatemode.sh`, a declarations-only runner copied from the
lifecycle shard's shape: `SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationPrivateMode"`,
`SHARD_MODE="normal"`, then `shard_inspect_maybe` / `run_integration_shard` from
`tests/lib/go-integration-shard.sh`. Its header comment says what it holds and that it was split out
of the lifecycle shard. The completeness contract discovers runners structurally, so
`tests/test_go_integration_contract.sh` needs no edit: it will pick up the new runner and prove
that every renamed test matches exactly one runner.

### Budget rows

Both rows come from a fresh **serial** measurement of each shard after the move, using the table's
own seeding rule from its header comment: round the measured wall time up to the next multiple of
5, add a 5s margin, minimum 10s. Neither number is fixed in advance. The 2026-10-16 sums predict
roughly 50 for the lifecycle shard and 55 for the new one. Measure from a quiet machine; another
worktree's suite inflates a solo reading. If a reading looks off, re-measure it once.

- Add `tests/test_go_integration_app_privatemode.sh <N> parallel` beside the other app shards.
- Lower `tests/test_go_integration_app_workflowlifecycle.sh` from 80 to its measured value.

`TestRuntimeBudgetsCorrespondence` enforces that the new file has a row.

## Verification

- `tests/test_go_integration_contract.sh` passes: every tagged test, the seven renamed ones
  included, matches exactly one runner, and the new runner selects at least one test.
- Both shards pass on their own, and the shard runner's per-shard pass count shows 7 tests in the
  new shard and 21 in the lifecycle shard.
- `go test -tags integration -list '^TestIntegrationWorkflowLifecycle(Private|SharedPublish)'
  ./internal/app` lists nothing.
- The whole suite (`build.test_command`) is green, and its budget report shows no
  `BUDGET WATCH:` or `SERIAL CONFIRMED OVER BUDGET:` line for either shard.

## Out of scope

- Changing what any of the seven tests asserts, or speeding them up.
- Re-seeding or host-relativizing the rest of the budget table (#273).
- Detecting concurrent suites in the solo re-check (#528).
- Editing point-in-time records that name the old shard or tests (release run logs, archived
  changes, results files).
