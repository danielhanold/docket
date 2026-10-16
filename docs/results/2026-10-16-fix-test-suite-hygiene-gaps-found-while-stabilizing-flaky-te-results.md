<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0527 — Fix test-suite hygiene gaps found while stabilizing flaky tests](../changes/active/0527-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te.md)**
<!-- docket:backlink:end -->

# Fix test-suite hygiene gaps found while stabilizing flaky tests — Results

**Human action:** None required to merge. The one production change (the codex client's bounded stderr wait) is covered by new tests; an optional check is listed below.

## Outcome

All eight items are done. Seven landed as code; item 7 was already on `main`.

- **Unrunnable tests are caught.** A new repo guard (`internal/repoguard/unrunnable_tests_test.go`) fails when a test-shaped function (`*testing.T`, `*testing.B` or `*testing.F`) has a name `go test` will never run and nothing in its directory calls it. It covers every `_test.go` file, build-tagged ones included. The tree has no such functions today.
- **`TMPDIR` spelling no longer matters.** The no-git test guard stores a cleaned temp path, so `TMPDIR=/tmp//` no longer fails `TestNoGitGuardShadowsGitOnPath`.
- **A crashing codex app-server's message is kept.** When the server closes stdout, the client now waits up to 2 seconds for it to exit, then puts its stderr and exit status in the error. Before, the error was a bare `EOF`. The test's fake server now writes one line to stderr naming the check that failed before each exit.
- **The exit-sites guard ignores feature worktrees.** `TestProcessExitSitesAreAllowlisted` now uses the shared maintained-file walk, so it passes from the main checkout while `.worktrees/` exists.
- **Gate-driver tests no longer race a timer.** Four tests hold their child process on a signal file the test creates, instead of a fixed delay. The child is alive at every point a test checks, however slow the disk.
- **The abandoned-run test kills in two steps.** `TestRecoverMarksCleanlyAbandonedOwnedRun` kills the supervisor first, then the process group. Its 3-attempt retry is gone, and if setup misses the abandoned shape the test now fails saying so. A sweep of `internal/process` found no other test that builds this shape with a single group kill.
- **Budget rows follow the split-don't-raise rule.** The 23 private-mode reposetup tests were renamed to the `TestIntegrationPrivateRepoSetup` prefix and moved into a new shard, `tests/test_go_integration_app_reposetup_private.sh`. Rows: concurrency 45 → 35, reposetup 35 → 30, new private shard 40.

## Human actions and testing

### Optional — see a crashing app-server's message

Shows the codex client change from the outside. The new unit tests already cover it.

Prerequisites: a checkout of this branch with Go installed.

1. Run `go test -count=1 -v ./internal/codexentry -run TestRecvReportsStderrOfServerThatClosedStdoutFirst`.
   Expected: PASS. The test's server writes `app-server boom: config unreadable` to stderr and exits with status 7, and the client's error contains both.
2. Run `go test -count=1 -v ./internal/codexentry -run TestRecvAndCloseDoNotHangOnLiveServer`.
   Expected: PASS within a few seconds. A server that closes stdout but keeps running does not hang the client.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed at the final head. Its budget report had no watch or over-budget lines.
- Guard: a test proved it reports a planted unreferenced function and skips a referenced one. Renaming a real `TestE2E…` function to lower case turned it red; the rename was reverted.
- `TMPDIR=/tmp//` runs of the four `TestNoGitGuardShadowsGitOnPath` wrappers passed.
- Codex client: the new test failed 5 of 5 runs against the old code. It passes, including 20 runs under `-race`. Making the grace timer unable to fire turns the "does not hang" test red.
- Gate driver: the converted tests passed `-count=20`. `child-signal-death` passed 40 of 40 under two background `dd` + `sync` loops. Those loops may load the disk less than the original failure (34 of 40), so the fix rests mainly on the construction argument. Removing the release step makes `child-signal-death` hang at WAITING, which shows the child really waits for the signal.
- `TestRecoverMarksCleanlyAbandonedOwnedRun` passed `-count=200` under three concurrent `go test ./internal/...` loops (load average about 5.8 on 11 CPUs).
- Item 7: `TestObserveRunningThenTerminal` on `main` kills `m.SupervisorPID` and waits for it to be reaped before killing the group.
- Shard split: 23 tests carry the new prefix, reposetup keeps 33, no old names remain, and `tests/test_go_integration_contract.sh` passes. Serial timings: concurrency 28.1 s and 28.7 s, reposetup 24.3 s, the new private shard 33.5 s.
- Whole-branch review (deep tier): 2 minor findings, both fixed in-branch; full table in the PR body.

## Known issues and follow-ups

- **Codex client can now report an error up to 2 seconds later.** When the app-server sends a malformed reply or closes stdout but keeps running, `Recv` waits up to 2 seconds for it to exit before returning the error. Before, it returned at once. Impact: a slower failure on a misbehaving server; normal operation is unaffected. Confirmed, by design. No action needed.
- **One concurrency timing was high.** The first serial run of the concurrency shard took 38.2 s, against 28.1 s and 28.7 s on two later runs. The row was set to 35 from the later runs; 38.2 s would have meant 45. Impact: if the shard really is that slow sometimes, it may show a budget watch line. Suspected one-off (it was the first run after the `internal/app` rebuild). Next step: if a watch line shows up, re-measure serially before changing the row.
