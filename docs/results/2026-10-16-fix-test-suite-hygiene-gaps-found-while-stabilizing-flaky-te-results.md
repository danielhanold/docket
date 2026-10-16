<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0527 — Fix test-suite hygiene gaps found while stabilizing flaky tests](../changes/active/0527-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te.md)**
<!-- docket:backlink:end -->

# Fix test-suite hygiene gaps found while stabilizing flaky tests — Results

**Human action:** Assessment pending — the build is green; whole-branch review has not run yet.

## Outcome

Build complete (7 plan tasks, one commit each); the full suite passed at the build head. Review pending.

- A new repo guard flags any test-shaped function (`*testing.T`/`B`/`F`) that `go test` will never run because of its name and that nothing calls.
- The no-git test guard stores a cleaned temp path, so an unclean `TMPDIR` (for example `/tmp//`) no longer breaks `TestNoGitGuardShadowsGitOnPath`.
- The codex client now waits (bounded at 2 s) for a crashing app-server's stderr before reporting a read error, so the server's last message is no longer lost behind a bare `EOF`. The test's fake server now says why it exits.
- `TestProcessExitSitesAreAllowlisted` uses the shared maintained-file walk and no longer trips on feature worktrees.
- Four gate-driver tests now hold their child on a go-signal file instead of a timer.
- `TestRecoverMarksCleanlyAbandonedOwnedRun` kills the supervisor before the group and has no retry loop.
- 23 private reposetup tests moved to a new shard (`TestIntegrationPrivateRepoSetup` prefix); budget rows: concurrency 45→35, reposetup 35→30, new private shard 40.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the build head.
- Guard mutation-tested: a lower-cased real `TestE2E…` function turned it red; reverted.
- `TMPDIR=/tmp//` run of the four `TestNoGitGuardShadowsGitOnPath` wrappers passed.
- New codex client test failed 5/5 against the old code; passes 20/20 under `-race`.
- Converted gate-driver tests passed `-count=20`; `child-signal-death` passed 40/40 under two background `dd`+`sync` loops.
- `TestRecoverMarksCleanlyAbandonedOwnedRun` passed `-count=200` under three concurrent `go test ./internal/...` loops (load average about 5.8 on 11 CPUs).
- Item 7 (`TestObserveRunningThenTerminal` supervisor-first kill) confirmed present on `main` by content.
- Serial shard timings: concurrency 28.7 s, reposetup 24.3 s, reposetup_private 33.5 s.
