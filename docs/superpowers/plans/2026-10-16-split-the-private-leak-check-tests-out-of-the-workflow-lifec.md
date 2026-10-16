<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0536 — Split the private-mode tests out of the workflow lifecycle shard](../../changes/archive/2026-10-16-0536-split-the-private-leak-check-tests-out-of-the-workflow-lifec.md)**
<!-- docket:backlink:end -->

# Split the private-mode tests out of the workflow lifecycle shard — Implementation Plan

> **For agentic workers:** executed by `docket-build`, task by task, under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the seven private-mode integration tests out of `tests/test_go_integration_app_workflowlifecycle.sh` into a new shard, `tests/test_go_integration_app_privatemode.sh`, keyed on the new prefix `TestIntegrationPrivateMode`, and re-seed both shards' budget rows from fresh serial measurements.

**Architecture:** The rename is a pure function-name and doc-comment change in two existing test files. The new shard is a declarations-only runner with the same shape as every other `internal/app` shard. `tests/test_go_integration_contract.sh` discovers runners structurally and needs no edit. Both budget rows in `tests/runtime-budgets.tsv` come from measurement, not from prediction.

**Tech Stack:** Go integration tests (`//go:build integration`), Bash shard wrappers (`tests/lib/go-integration-shard.sh`), `tests/runtime-budgets.tsv`.

**Spec:** `docs/superpowers/specs/2026-10-16-split-the-private-leak-check-tests-out-of-the-workflow-lifec-design.md`

## Global Constraints

- One literal prefix per shard. `SHARD_PREFIX` must be the plain string `TestIntegrationPrivateMode`, with no regex alternation. Contract check (4) matches with `case "$t" in "$rx"*)`.
- No moved test changes what it asserts. Only the function names, their doc comments, and the two file-header paragraphs change.
- The files stay where they are: `internal/app/workflow_private_integration_test.go` and `internal/app/leak_check_private_integration_test.go`.
- Budget seeding rule, from the table's own header: take the measured serial wall time, round it up to a multiple of 5, add a 5s margin, and use at least 10s. Apply the rule to the **worst** standalone serial reading of each shard.
- Do not edit point-in-time records that name the old shard or tests: `docs/release/**`, older `docs/superpowers/plans/**` and `specs/**`, archived changes, and results files.
- Out of scope: the concurrency and reposetup budget rows (#527), re-seeding the rest of the table (#273), and concurrent-suite detection (#528).

## Review Focus

1. **Prefix collision.** If the new prefix were a prefix of another runner's `SHARD_PREFIX`, or the other way round, the contract check would report a test matched by two runners. Task 1 Step 1 re-checks this against the live set of prefixes.
2. **A stale old name left somewhere.** A comment, a `-run` string, or a current-behavior doc could still name a `TestIntegrationWorkflowLifecyclePrivate*` or `…SharedPublishRunsNoLeakCheck` test. Task 1 Step 6 runs a repo-wide grep and expects hits only in point-in-time records.
3. **A budget seeded from a contended reading.** Another worktree's suite inflates a solo measurement. Task 2 gates measurement on a quiet `ps` check and takes the worst of two serial readings.
4. **A lifecycle row left at 80.** The split only matters if the old ceiling drops. Task 2 lowers that row from its own measurement.
5. **A vacuous new shard.** The new shard must select and run exactly 7 tests, and the lifecycle shard must select and run exactly 21. Both shard runs in Task 1 assert the counts from their output.

---

### Task 1: Rename the seven tests and add the private-mode shard

**Files:**
- Create: `tests/test_go_integration_app_privatemode.sh` (mode 100755, like its sibling shards)
- Modify: `internal/app/workflow_private_integration_test.go` (header paragraph at lines 22–23; doc comment and func at 225–229)
- Modify: `internal/app/leak_check_private_integration_test.go` (header paragraph at lines 18–19; doc comments and funcs at 236–240, 367–370, 420–424, 439–443, 458–463, 533–536)
- Modify: `tests/runtime-budgets.tsv` (add a **provisional** row for the new shard; Task 2 replaces its value)

**Interfaces:**
- Produces: the shard file `tests/test_go_integration_app_privatemode.sh` with `SHARD_PREFIX="TestIntegrationPrivateMode"`, and the seven renamed test functions. Task 2 measures this shard.

- [ ] **Step 1: Re-check the prefix against the live runners**

Run:
```bash
grep -h 'SHARD_PREFIX=' tests/*.sh | sed 's/.*SHARD_PREFIX="\([^"]*\)".*/\1/' | sort -u | \
  awk -v p=TestIntegrationPrivateMode 'index($0,p)==1 || index(p,$0)==1'
```
Expected: no output. If it prints anything, stop and return BLOCKED with that line, because the prefix collides.

- [ ] **Step 2: Write the new shard runner (the failing test)**

Copy the structure of `tests/test_go_integration_app_workflowlifecycle.sh` exactly, with this header and declarations:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_privatemode.sh — Go integration shard (change 0536):
# the private-visibility tests — the init-to-implemented acceptance run and the
# leak-check tests (seeded leaks, match_word off, an unverified scan, the branch
# name, the PR write, and the shared-mode negative) — behind the `integration`
# build tag, prefix ^TestIntegrationPrivateMode. Split out of
# tests/test_go_integration_app_workflowlifecycle.sh. Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract
# is tests/test_go_integration_contract.sh.
```
Then use `SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationPrivateMode"`, `SHARD_MODE="normal"`, and the same `set -uo pipefail` / `REPO` / `assert` / source / `shard_inspect_maybe` / `run_integration_shard` / `exit "$fail"` body. Run `chmod +x`.

- [ ] **Step 3: Run it and confirm it fails**

Run: `bash tests/test_go_integration_app_privatemode.sh`
Expected: `NOT OK - prefix ^TestIntegrationPrivateMode selects at least one tagged test in ./internal/app` and a non-zero exit.

- [ ] **Step 4: Rename the seven functions and update their comments**

Rename each function and the first token of its doc comment:

| Current name | New name |
|---|---|
| `TestIntegrationWorkflowLifecyclePrivateInitToImplemented` | `TestIntegrationPrivateModeInitToImplemented` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckBlocksSeededLeaks` | `TestIntegrationPrivateModeLeakCheckBlocksSeededLeaks` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckMatchWordOff` | `TestIntegrationPrivateModeLeakCheckMatchWordOff` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckUnverifiedPushesNothing` | `TestIntegrationPrivateModeLeakCheckUnverifiedPushesNothing` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckScansBranchName` | `TestIntegrationPrivateModeLeakCheckScansBranchName` |
| `TestIntegrationWorkflowLifecyclePrivateLeakCheckGatesPR` | `TestIntegrationPrivateModeLeakCheckGatesPR` |
| `TestIntegrationWorkflowLifecycleSharedPublishRunsNoLeakCheck` | `TestIntegrationPrivateModeLeakCheckSkipsSharedPublish` |

Re-wrap any doc-comment line that the longer or shorter name pushes past about 80 columns. Leave the wording otherwise unchanged.

Rewrite the two file-header opening sentences so that they name the new shard and prefix. Keep the rest of each paragraph as it is:
- `workflow_private_integration_test.go`: "This file is the private-visibility acceptance slice of the private-mode shard (tests/test_go_integration_app_privatemode.sh, prefix TestIntegrationPrivateMode). It drives a …"
- `leak_check_private_integration_test.go`: "This file is the leak-check slice of the private-mode shard (tests/test_go_integration_app_privatemode.sh, prefix TestIntegrationPrivateMode). Over real repositories it proves that a …"

- [ ] **Step 5: Add a provisional budget row**

In `tests/runtime-budgets.tsv`, add the following line directly after the `tests/test_go_integration_app_workflowlifecycle.sh` row, with tabs as separators:
```
tests/test_go_integration_app_privatemode.sh	55	parallel
```
The 55 is the spec's prediction. It exists only so that `TestRuntimeBudgetsCorrespondence` holds at this commit, and Task 2 replaces it with a measured value. Leave the lifecycle row at 80 in this task.

- [ ] **Step 6: Verify the rename is complete and confined**

Run:
```bash
go test -tags integration -count=1 -list '^TestIntegrationWorkflowLifecycle(Private|SharedPublish)' ./internal/app | grep -c '^Test'
go test -tags integration -count=1 -list '^TestIntegrationPrivateMode' ./internal/app | grep -c '^Test'
go test -tags integration -count=1 -list '^TestIntegrationWorkflowLifecycle' ./internal/app | grep -c '^Test'
grep -rnE 'TestIntegrationWorkflowLifecycle(Private|SharedPublish)' --exclude-dir=.git --exclude-dir=.worktrees .
```
Expected: `0`, then `7`, then `21`. The grep may hit only point-in-time records: the 2026-10-16 spec for this change and any `docs/release/**` or older plan or spec. It must hit nothing under `internal/`, `tests/`, `cmd/`, `skills/`, or a current-behavior doc. If such a hit exists, update it.

- [ ] **Step 7: Run both shards, the contract, and the correspondence guard**

Run:
```bash
bash tests/test_go_integration_app_privatemode.sh
bash tests/test_go_integration_app_workflowlifecycle.sh
bash tests/test_go_integration_contract.sh
go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard
```
Expected:
- The new shard ends with `ok - every selected test actually ran and passed (7 declared)` and exits 0.
- The lifecycle shard ends with `ok - every selected test actually ran and passed (21 declared)` and exits 0.
- The contract prints no `NOT OK` line and exits 0.
- The repoguard test prints `ok`.

- [ ] **Step 8: Commit**

```bash
git add tests/test_go_integration_app_privatemode.sh tests/runtime-budgets.tsv \
  internal/app/workflow_private_integration_test.go internal/app/leak_check_private_integration_test.go
git commit -m "test: split the private-mode tests into their own integration shard"
```

---

### Task 2: Seed both budget rows from quiet serial measurements

**Files:**
- Modify: `tests/runtime-budgets.tsv` (the `…_privatemode.sh` row added in Task 1, and the `…_workflowlifecycle.sh` row, currently 80)

**Interfaces:**
- Consumes: `tests/test_go_integration_app_privatemode.sh` and the renamed tests from Task 1.

- [ ] **Step 1: Confirm the machine is quiet**

Run:
```bash
ps -axo pid,etime,command | grep -E 'docket-devtest|development test|go-integration|go test -tags integration' | grep -v grep
```
Expected: no output. If anything is listed, another worktree's suite is running. Wait for it to finish by polling this command with Monitor and an until-loop, then re-check. If the machine is still busy after 30 minutes, measure anyway and say so in the commit body, so that the rows can be re-measured later.

- [ ] **Step 2: Warm the build cache**

Run: `go test -tags integration -count=1 -run '^$' ./internal/app`
Expected: `ok` with no tests run. This keeps a cold compile out of the first reading.

- [ ] **Step 3: Measure each shard serially, twice**

Run these one at a time, never in parallel, and re-check Step 1 between them:
```bash
time bash tests/test_go_integration_app_privatemode.sh
time bash tests/test_go_integration_app_workflowlifecycle.sh
time bash tests/test_go_integration_app_privatemode.sh
time bash tests/test_go_integration_app_workflowlifecycle.sh
```
Expected: every run passes (7 and 21 declared). Record the `real` time of each run. If the two readings for one shard differ by more than about 20%, re-run Step 1 and measure that shard once more. Use the worst reading for each shard.

- [ ] **Step 4: Apply the seeding rule and write both rows**

For each shard, round its worst reading up to a multiple of 5, add 5, and use at least 10. For example, 47.3s rounds to 50 and becomes 55, and 45.0s stays 45 and becomes 50. Write the new shard's value over the provisional 55. Replace the lifecycle row's 80 with its own value. Keep both rows `parallel`. The spec predicts about 55 and about 50. A measured value well outside that range is a reading to repeat (Step 1, then Step 3), not one to adjust by hand.

- [ ] **Step 5: Verify the table**

Run: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard`
Expected: `ok`.

- [ ] **Step 6: Commit**

Put each shard's readings, the worst value used, and the computed row in the commit body:
```bash
git add tests/runtime-budgets.tsv
git commit -m "test: re-seed the lifecycle and private-mode shard budgets from serial runs" \
  -m "privatemode: <r1>s, <r2>s -> worst <w>s -> <N>
workflowlifecycle: <r1>s, <r2>s -> worst <w>s -> <M> (was 80)"
```

The final full-suite gate (`build.test_command`) must be green. Its budget report must show no `BUDGET WATCH:` and no `SERIAL CONFIRMED OVER BUDGET:` line for either shard. If such a line appears, record the margin in the results file as a number.
