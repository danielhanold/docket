<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0536 — Split the private-mode tests out of the workflow lifecycle shard](../changes/active/0536-split-the-private-leak-check-tests-out-of-the-workflow-lifec.md)**
<!-- docket:backlink:end -->

# Split the private-mode tests out of the workflow lifecycle shard — results

**Human action:** None needed beyond normal PR review; the change only moves seven existing tests into their own shard and re-seeds two budget rows.

## Outcome

The seven private-mode integration tests (the private init-to-implemented end-to-end test and the six leak-check tests, including the shared-mode negative) were renamed onto the `TestIntegrationPrivateMode` prefix and now run from a new declarations-only shard, `tests/test_go_integration_app_privatemode.sh`. The workflow lifecycle shard keeps its other 21 tests. No test assertion changed.

Budget rows in `tests/runtime-budgets.tsv`, from quiet serial measurements under the table's seeding rule (round up to the next 5s, add 5s):

| Shard | Serial readings | Old row | New row |
|---|---|---|---|
| `test_go_integration_app_privatemode.sh` | 39.70s, 41.32s | — | 50 |
| `test_go_integration_app_workflowlifecycle.sh` | 36.64s, 36.65s | 80 | 45 |

## Verification performed

- The new shard ran 7 tests and the lifecycle shard ran 21, all passing; `go test -tags integration -list '^TestIntegrationWorkflowLifecycle(Private|SharedPublish)' ./internal/app` lists nothing.
- `tests/test_go_integration_contract.sh` and `TestRuntimeBudgetsCorrespondence` pass.
- Serial timings were taken only after confirming no other worktree's suite was running.
- The full suite (`go run ./cmd/docket development test`) passed at the branch head with no `BUDGET WATCH` or `SERIAL CONFIRMED OVER BUDGET` line. Inside that parallel run the two shards took 81s and 72s wall time; the budget table measures serial ceilings, and the suite raised no budget finding for either shard.
- Review: standard-tier whole-branch review returned no findings.
