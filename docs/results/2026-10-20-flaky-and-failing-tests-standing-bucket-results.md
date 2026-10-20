<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0569 — Fix the flaky run-cancel worktree test](../changes/archive/2026-10-20-0569-flaky-and-failing-tests-standing-bucket.md)**
<!-- docket:backlink:end -->

# Fix the flaky run-cancel worktree test — results

**Human action:** None needed. This is a test-only change. The forced-race proof, the repeated runs, and the green full suite cover it.

## Outcome

`TestIntegrationRunCancelFreesWorktreeForNextStart` now waits until the launcher has released its launch claim before it cancels. Its one cancel therefore always reports `cancelled`. The new helper `awaitLaunchSettled` polls the production read-only launch census until it reports `run-live:<drive>` and no `claim-busy:<drive>`. It uses a 30-second deadline and polls every 10 ms, with no fixed sleep. Every existing assert is unchanged, and no production file changed.

## Verification performed

- The race was forced on demand. A temporary `time.Sleep(400 * time.Millisecond)` went in right before the non-deferred `claim.close()` in `(*Driver).launchScopeless`. It was never staged or committed, and the file was restored from a backup. With the delay, the unchanged test failed with `disposition = "cancellation-pending", want cancelled (findings=[claim-busy:78562525ec5ecc0495a0d9fb927b9d7b])`.
- With the same delay, the fixed test passed in 0.86s, and the cancel still reported `replacement-stopped:<drive>`.
- With the delay removed, `git diff --stat` showed only `internal/app/runtracker_cancel_integration_test.go` changed (24 lines added). The fixed test then passed 50 of 50 runs (`-count=50`, 24s).
- These tests are built only with `-tags integration`. The plan's suggested commands left the tag out, which runs nothing, so every run above used the tag.
- Sweep for tests with the same shape: grep over all `*_test.go` for `awaitRunPhase`, `runCancel(`, `ReconcileRunLaunches`, `VerdictRunLaunches` and `"running"`. It found 159 hits in 24 files, and none had the same shape. The only other `awaitRunPhase(..., "running")` caller is `TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled`, which waits for `Start` to return (`awaitStart`) before it cancels. The other cancels and census reads work on seeded records, use fakes, or come after a synchronous start.
- Full suite at the build gate (`go run ./cmd/docket development test`): green at b5e6d4883. The budget report printed only BUDGET WATCH screening lines for parallel overruns, which need no action.
- Review: one standard-tier whole-branch review, with no findings.
