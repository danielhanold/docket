<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0465 — test_go_race times out on internal/app in CI (Go's 10m per-package limit)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0465-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa.md)**
<!-- docket:backlink:end -->
# test_go_race times out on internal/app in CI (Go's 10m per-package limit) — Results

**Human action:** Yes, after merge. Watch several release-candidate CI source-gate runs to confirm `test_go_race` is reliably green. Separately, decide whether to open a follow-up for the packages that now set the race gate's time (it still measures about 67s locally, over its 60s row).

## Outcome

The CI race gate (`tests/test_go_race.sh`, which runs `go test -race -count=1 ./...`) kept hitting Go's 10-minute per-package limit in `internal/app`. The cause was that about 337 of that package's default tests start a real `git` process, and together they took roughly 225s of the package's ~238s under the race detector.

What changed:

- **The fast test corpus can no longer run git.** A test-only guard (`internal/app/nogit_guard_test.go`, default build only) puts a fake `git` at the front of `PATH`. Any default `internal/app` test that reaches git fails, and so does the whole package, even when the test itself tolerated the error. Tagged builds (`integration`, `e2e`) install a no-op twin instead (`nogit_guard_off_test.go`).
- **All 337 offenders moved** behind `//go:build integration`, into 12 new plain (non-race) shard runners: gatecancel, gateverdict, gatefence, gatecompletion, gateepoch, gatearm, gatelifecycle, finalizeops, finalizerebaseops, evidence, recordops and contextprobe. Five tests that really race goroutines or processes went to the existing race shard (`TestRaceIntegrationAppConcurrency…`), each with a rationale comment.
- **Measured result:** `go test -race -count=1 ./internal/app/` went from about 238s to about 14s. The default `internal/app` run takes about 1.3s.
- **Readable timeout:** `tests/test_go_race.sh` now passes `-timeout 8m`, and it prints a named backstop line if any package overruns. The slowest package locally at a CI-like `-p 2` is `internal/repository/transaction` at 48.7s. The spec's data puts CI at about 2.4–3.8× slower than local, which projects that package to about 120–230s. 8m is at least twice that and still below Go's 10m default. A repoguard test pins this floor. An overrun now gives a clear failure instead of a 10-minute goroutine dump.
  - The first build used 4m. Review showed that 4m left too little CI headroom and could itself trip intermittently, so it was raised.
- **Budget alerts now work across worktrees.** The suite runner's budget-state key uses the repo-relative test path instead of the absolute one. Overruns now build up across `.worktrees/<slug>` checkouts, so the serial "SERIAL CONFIRMED OVER BUDGET" confirmation can finally fire. Existing local budget records are orphaned once, which is harmless because the store is advisory.

Departures from the spec:

- The guard excludes both `integration` and `e2e` builds, because `finalize_e2e_test.go` legitimately runs git.
- The guard also fails the package when a test swallowed the git error. Four such tolerant tests existed.
- The finalize-ops family is split across two shards, because one runner measured 56s, which would need a 65s row.

## Human actions and testing

### Important — Confirm CI's race gate is stably green after merge

The failure was intermittent and depended on the load of the shared 3-core macOS runner. A local measurement cannot prove the CI fix. If you skip this, the change is merged on local evidence alone.

Prerequisite: this PR is merged to `main`.

1. Open the GitHub Actions page for the release-candidate workflow and look at the next 3–5 source-gate runs, whether from new PRs or re-runs.
   Expected: every run shows `test_go_race` passing, with no `panic: test timed out after 10m0s`.
2. In each run's log, find the `test_go_race` elapsed time.
   Expected: well under the 8m `-timeout`, and far below the old 581–908s.
3. If a run fails with the new backstop line (a package ran past the 8m `-timeout`), look at which package it names. That package is the new hot spot.

### Optional — Watch the guard reject a new real-git test

This shows what a future contributor will see.

1. In a scratch checkout of this branch, add to any default-build `internal/app/*_test.go` file: `func TestTmpGit(t *testing.T) { _ = exec.Command("git", "--version").Run() }`. Import `os/exec` if the file does not already.
2. Run `go test -count=1 -run TestTmpGit ./internal/app/`.
   Expected: the package FAILs with the guard's diagnostic. It tells you to move the test behind `//go:build integration` (change 0465/0333), even though the test ignored the error.
3. Cleanup: delete the scratch test.

## Verification performed

- Every task was checked through the gate driver:
  - The default `internal/app` package is green with the guard on, and the guard reports zero attempts.
  - `tests/test_go_integration_contract.sh` is green: every moved test sits in exactly one shard and runs in the right race mode.
  - All 39 `tests/test_go_integration_app_*.sh` runners returned 0.
  - `go vet` is clean for the default, `integration` and `e2e` builds.
- **Guard mutation tests:**
  - Stripping the `PATH` prepend turned the guard's proving tests red.
  - Making the verdict ignore violations turned them red too.
  - Restoring the file byte-identical turned them green.
- **Budget-key mutation:** with the call site still keyed on the absolute path, a new test that runs two checkouts against one shared store failed. Changing the call site made it pass.
- **Timeout guard mutation** (the test also asserts the floor; against the old 4m it went red): a new `internal/repoguard` test runs a copy of `test_go_race.sh` against a fake `go`.
  - Removing `-timeout` turned it red.
  - Deleting the backstop assert turned it red.
  - Restoring turned it green.
- **Measurements:**
  - Race `internal/app` at 3 CPUs went from ~238s to 13.9s.
  - The whole-module race run at `-p 2` passed. Its slowest package was `internal/repository/transaction` at 48.7s.
- **Whole-branch review:** a deep review found 0 blockers, 1 important and 3 minors. It confirmed that no test was dropped: 337 moved, bodies byte-identical apart from renames, and helpers unchanged. All four findings were fixed in-branch:
  - the timeout raised to 8m (commit ed887967a);
  - the toolchain row reverted to 55;
  - the guard header's known limits documented;
  - four stale file-name comments corrected (commit 0361260bc).
- The full build-gate suite runs after this file is committed. Its result is in the PR's build-evidence block, not here.

## Known issues and follow-ups

### The race gate is still slightly over its 60s budget row

`tests/test_go_race.sh` measured 66.5–67.6s solo on a loaded developer machine, and its row stays capped at 60. `internal/app` is no longer the cause. The time now comes from other packages under `-race`:

- `internal/repository/transaction`: ~61s
- `internal/workspace`: ~57s
- `internal/gatedrive`: ~43s

The gate is nowhere near the 8m timeout or Go's 10m limit, so CI should pass. The suite's budget report will likely print `BUDGET WATCH` for this file, and, now that keys converge, eventually `SERIAL CONFIRMED OVER BUDGET`. This is confirmed locally. Suggested next action: open a follow-up change to partition or speed up those packages' slow tests. Do not raise the row.

### The toolchain gate is over budget on a cold test cache

With its test cache invalidated, `tests/test_go_toolchain.sh` measured 61.7–65.8s, against its unchanged 55 row. It runs `go test ./...` without `-count=1`, so warm runs take about 3s. Expect an occasional `BUDGET WATCH` line. The same follow-up as above would cover it.

### Three untouched shards measured over their rows under load

This is suspected, not confirmed. During the re-measure, on a machine with load around 3:

- `tests/test_go_integration_app_closeout.sh`: 64s (row 40)
- `tests/test_go_integration_app_rebaserecovery.sh`: 60s (row 40)
- `tests/test_go_integration_app_changeruntime.sh`: 40s (row 40)

This branch did not change them. Suggested next action: serial-confirm on an idle machine before acting.

### Guard covers PATH-resolved git only

The guard shadows `git` on `PATH`. It does not catch these routes:

- a default test that builds a client with an absolute git path (`gitcli.WithExecutable`);
- a test that replaces `PATH` wholesale;
- a detached child process that runs git after the test binary finishes.

No default test does any of these today, and the guard file's header documents the limits. This is a known limitation, not a defect. Optional next action: a static check banning those patterns in default `internal/app` test files.

### Pre-existing prefix overlap among repo integration shards

`TestIntegrationRepo` is a string prefix of other `TestIntegrationRepo*` shard prefixes. This predates this change and the contract still passes. It is only noted for whoever next touches those shards.
