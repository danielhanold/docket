<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0507 — Fix the observe-test hang and bring two test files back under their time limits](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0507-flaky-tests-track-and-stabilize-intermittent-suite-failures.md)**
<!-- docket:backlink:end -->
# Fix the observe-test hang and bring two test files back under their time limits — Results

**Human action:** Assessment pending — the whole-branch review and final gate have not run yet.

## Outcome

This change targeted four suite problems. Each one now has a recorded outcome.

- **The `TestObserveRunningThenTerminal` hang was not reproduced.** The test was looped 2,000 times under CPU stress (22 busy processes on 11 CPUs, load up to 53) and 2,000 more times next to a looping `go test -race ./internal/...`. There were no hangs and no other failures. So the root cause is still open. Instead, the test no longer discards two errors: a failed manifest read or a failed kill now fails the test immediately with the error. When the unchanged 30s wait expires, the failure message reports what was stuck: the last `Observe` state and error, whether the killed process group still has live members, and whether the run's live lock is still held. The 30s deadline was not changed.
- **`tests/test_go_integration_app_merge.sh` dropped from 42.6s to 10.0s** (quiet solo medians). Its 12 merge tests now run in parallel. They could not do so before because their shared fixture set an environment variable per test (`t.Setenv`), which Go refuses in parallel tests. The fixture now accepts a planning-node builder, and the merge tests use one that points the global config directory at an isolated empty directory once per process, the same way the e2e tests already do. A new test pins that the merge tests never read the developer's own config. The 30s budget row is unchanged and now has real headroom.
- **`tests/test_go_finalize_e2e.sh` measured 28.2s before and 24.3s after on an identical tree.** That is under the 25s target, but no code was cut. Each candidate cut was measured and rejected: vet is cached (no gain), the fake `gh` build costs about 0.2s, and per-test setup is not shareable. Almost all the time (about 27s) is one test, `TestE2EStack`, so the file sits on the 25s line and moves with machine load. The 30s row is unchanged.
- **`tests/test_go_race.sh` measured 44.6s** against its 60s row. The earlier 117s and 94s readings were taken while another suite was running on the same machine; they do not reflect growth.

## Verification performed

- Every timing was taken with the spec's protocol: from the feature worktree, with a private HOME/TMPDIR/XDG_CONFIG_HOME, the concurrency variables unset, one discarded warm-up run, then three runs with the median reported, each starting on a quiet machine (no other suite running, 1-minute load below the CPU count).
- New diagnostics for the observe test: unit tests for the stuck-run report, plus two mutation checks. When the group probe was forced to report absent, the report test went red. When a bad process group was signalled, the observe test failed in 0.05s, naming the error.
- Merge shard: the full shard, its `-race` run, the sibling rebase/evidence/state/ops integration shards that share the fixture, the integration contract, and the repository guards all passed. The new config-isolation test went red when its `os.Setenv` was removed.
- E2E completeness assert: a narrowed `-run` filter turned the file red, as designed.

## Known issues and follow-ups

### The observe-test hang's root cause is still unknown

If the hang returns, the gate goes red on `TestObserveRunningThenTerminal` after 30s, as before. The difference is that the failure message now says what was stuck. Impact: an occasional false red on a gate; a re-run passes. Confirmed open (not reproduced in 4,000 loaded iterations). Next action: read the next sighting's stuck-run report against the reproduction record above and fix from there.

### `test_go_finalize_e2e.sh` sits on its 25s target

Measured medians on the same tree ranged from 24.3s to 28.2s depending on load, so a moderately loaded run can approach the 30s row. Nearly all of it is `TestE2EStack`. A further cut would need a cloned fixture snapshot that rewrites absolute paths, which the build judged risky. Suspected future screening noise, not a confirmed breach.

### The e2e completeness assert misses a renamed test

If a `TestE2E*` function is renamed to lower case, it drops out of both the declared count and the ran count, so the file stays green while the test silently stops running. This gap predates this change. Confirmed by mutation. Next action: a small guard fix in a new change.

### A non-canonical TMPDIR breaks the no-git guard tests

When `TMPDIR` ends in `/`, a private temp dir made from it contains `//`. `TestNoGitGuardShadowsGitOnPath` compares paths as strings and then fails in several packages. Seen only in a hand-run measurement harness, not in gates. Next action: clean or resolve the path before comparing, in a new change.
