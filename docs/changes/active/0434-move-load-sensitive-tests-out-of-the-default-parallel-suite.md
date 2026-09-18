---
id: 434
slug: 'move-load-sensitive-tests-out-of-the-default-parallel-suite'
title: 'Move load-sensitive tests out of the default parallel suite lane'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-09-18'
updated: '2026-09-18'
depends_on: []
stacked_on:
related: []
discovered_from: [411]
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

Change 0411's build burned 2 of its 4 build.max_attempts on false reds: `internal/app` integration buckets hit Go's 10m per-package timeout (`panic: test timed out after 10m0s`) under -j11 parallel load (~742s wall vs ~300s on passing runs), yet passed serially (`go test -tags integration -p 1 -timeout 30m -run ^TestIntegrationFinalizeRebase ./internal/app` = 66s). With the budget exhausted, the final head could only be certified from an earlier passing run. CI on PR #311 (run 35371390505, job 105685942540, release-candidate source-gate, SUITE files=49 passed=47 failed=2 wall=781s) went red on a second load-sensitive case: `internal/gatedrive` `TestConcurrentSameOwnerAdvanceRelaunchesOnce/terminal_relaunch_winner_fails` (`driver_concurrency_test.go`: `advance 1 returned an error: gatedrive: drive already terminal`), in both test_go_race and test_go_toolchain. 0411 does not touch gatedrive, and the test passes on main locally (-count=3). Environmental false reds use up retry budget and can block implement-next/finalize on unrelated diffs.

## What changes

- Measure which suite files/packages are contention-sensitive (the `internal/app` integration buckets under `tests/test_go_integration_app_*.sh`, and the go_race/go_toolchain packages that run gatedrive) using the runner's budget report and serial confirmation.
- Move the confirmed offenders from the default parallel lane to the serial lane (`mode` column in `tests/runtime-budgets.tsv`), or split them so no one package gets near Go's per-package timeout under load; raise the per-package `-timeout` where a split isn't enough.
- Root-cause `TestConcurrentSameOwnerAdvanceRelaunchesOnce/terminal_relaunch_winner_fails`: find out whether the test has a timing race (a fix to the test) or the driver's terminal-relaunch path has a real race (a fix to the product). Moving it to another lane alone isn't enough.
- Record the resulting wall-clock change for the whole suite and confirm the budget report has no new `SERIAL CONFIRMED OVER BUDGET:` lines.

## Out of scope

- Changing build.max_attempts or the gate's retry accounting.
- Weakening or deleting assertions to make tests pass.
- The pre-existing gofmt drift found on main during 0411 (separate follow-up).
