<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0507 — Fix the observe-test hang and bring two test files back under their time limits](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0507-flaky-tests-track-and-stabilize-intermittent-suite-failures.md)**
<!-- docket:backlink:end -->
# Fix the observe-test hang and bring two test files back under their time limits — Results

**Human action:** One decision is needed before merge: whether `tests/test_go_finalize_e2e.sh` keeps its 30s budget row or moves to 35s (see the Important item below). Nothing else needs a human.

## Outcome

This change targeted four suite problems. Each one now has a recorded outcome.

- **The `TestObserveRunningThenTerminal` hang was not reproduced.** The test was looped 2,000 times under CPU stress (22 busy processes on 11 CPUs, load up to 53) and 2,000 more times next to a looping `go test -race ./internal/...`. There were no hangs and no other failures. So the root cause is still open. Instead, the test no longer discards two errors: a failed manifest read or a failed kill now fails the test immediately with the error. When the unchanged 30s wait expires, the failure message reports what was stuck: the last `Observe` state and error, whether the killed process group still has live members, and whether the run's live lock is still held. The 30s deadline was not changed.
- **`tests/test_go_integration_app_merge.sh` dropped from 42.6s to 10.0s** (quiet solo medians). Its 12 merge tests now run in parallel. They could not do so before because their shared fixture set an environment variable per test (`t.Setenv`), which Go refuses in parallel tests. The fixture now accepts a planning-node builder, and the merge tests use one that points the global config directory at an isolated empty directory once per process, the same way the e2e tests already do. A new test pins that the merge tests never read the developer's own config. Its budget row was lowered from 30s to 15s by the table's own sizing rule, so the guard catches future growth again.
- **`tests/test_go_finalize_e2e.sh`: Part C's 25s target is not clearly met, and no code was cut.** Two quiet-protocol medians on the identical tree came out 28.2s and 24.3s, and the build gate's own solo re-check recorded 23s. The 24.3s figure is machine noise, not the result of a cut. Each candidate cut was measured and rejected: vet is cached (no gain), the fake `gh` build costs about 0.2s, and per-test setup is not shareable. Almost all the time (about 27s) is one test, `TestE2EStack`. The 30s row was left unchanged; whether to apply the spec's fallback (35s) is left to a human.
- **`tests/test_go_race.sh` measured 44.6s** against its 60s row. The earlier 117s and 94s readings were taken while another suite was running on the same machine; they do not reflect growth.

## Human actions and testing

### Important — decide the e2e file's budget row

The spec says a file that cannot reach 25s gets its row re-set by the sizing rule, with the evidence argued. This file's measurements straddle the line (23s, 24.3s, 28.2s), so the build left the row at 30s rather than pick one reading. If skipped, the row stays at 30s and a moderately loaded run may report a screening line for this file.

1. Read the C-before / C-after numbers and the rejected levers in the commit message of `4e370c355` (`git log -1 4e370c355`).
   Expected: medians 28.2s and 24.3s on an identical tree, test run about 27s of it.
2. Either accept 30s as is, or edit the `tests/test_go_finalize_e2e.sh` row in `tests/runtime-budgets.tsv` to 35 (ceil5(28.2) + 5) on this branch before merging.
   Expected: if edited, `go test ./internal/suiterunner/` still passes.

## Verification performed

- Every timing was taken with the spec's protocol: from the feature worktree, with a private HOME/TMPDIR/XDG_CONFIG_HOME, the concurrency variables unset, one discarded warm-up run, then three runs with the median reported, each starting on a quiet machine (no other suite running, 1-minute load below the CPU count).
- New diagnostics for the observe test: unit tests for the stuck-run report, plus two mutation checks. When the group probe was forced to report absent, the report test went red. When a bad process group was signalled, the observe test failed in 0.05s, naming the error.
- Merge shard: the full shard, its `-race` run, the sibling rebase/evidence/state/ops integration shards that share the fixture, the integration contract, and the repository guards all passed. The new config-isolation test went red when its `os.Setenv` was removed.
- E2E completeness assert: a narrowed `-run` filter turned the file red, as designed.
- The hang reproduction was narrower than the spec allowed: the plan capped it at 2,000 iterations per profile (4,000 total, about 5 minutes of test time, against the spec's 5,000 iterations / one hour), and the second load profile was a looping `go test -race ./internal/...`, not a full suite run from another worktree.
- Build gate budget report (full suite, green): two `PARALLEL-SENSITIVE:` lines and no `BUDGET WATCH:` or `SERIAL CONFIRMED OVER BUDGET:` lines. `test_go_finalize_e2e.sh` took 98s under `-j11` with a last solo measurement of 23s, and `test_go_race.sh` took 222s under `-j11` with a last solo of 46s. Both solo numbers agree with this change's quiet measurements (24.3–28.2s and 44.6s), so these are parallel contention, not over-budget files. The merge file drew no line at its old 30s row. A second full-suite run after the row was lowered to 15s gave the same two `PARALLEL-SENSITIVE:` lines (97s and 216s) plus one `BUDGET WATCH:` for the merge file: 42s under `-j11`, parallel-overrun streak 1/5. That is a screening line under full parallel load; the file's quiet solo runs are 9–10.5s, well inside 15s. If the streak climbs, the runner's serial re-check decides whether it is real.
- Whole-branch review (deep tier): 4 findings (2 important, 2 minor). The merge row was lowered to 15s in-branch; the other three were results-file corrections, made here. Full table in the PR body.

## Known issues and follow-ups

### The observe-test hang's root cause is still unknown

If the hang returns, the gate goes red on `TestObserveRunningThenTerminal` after 30s, as before. The difference is that the failure message now says what was stuck. Impact: an occasional false red on a gate; a re-run passes. Confirmed open (not reproduced in 4,000 loaded iterations). Next action: read the next sighting's stuck-run report against the reproduction record above and fix from there.

### `test_go_finalize_e2e.sh` sits on its 25s target

Measured medians on the same tree ranged from 23s to 28.2s depending on load, so a moderately loaded run can approach the 30s row. Nearly all of it is `TestE2EStack`. A further cut would need a cloned fixture snapshot that rewrites absolute paths, which the build judged risky. Suspected future screening noise, not a confirmed breach. Next action: the row decision above.

### The e2e completeness assert misses a renamed test

If a `TestE2E*` function is renamed to lower case, it drops out of both the declared count and the ran count, so the file stays green while the test silently stops running. This gap predates this change. Confirmed by mutation. Next action: a small guard fix. No existing change fits (checked 20) — capture a new change.

### A non-canonical TMPDIR breaks the no-git guard tests

When `TMPDIR` ends in `/`, a private temp dir made from it contains `//`. `TestNoGitGuardShadowsGitOnPath` compares paths as strings and then fails in several packages. Seen only in a hand-run measurement harness, not in gates. Next action: clean or resolve the path before comparing. No existing change fits (checked 20) — capture a new change.

### Spec-listed discovered work

The spec named two items for a human to capture: the runner's solo re-check cannot see another worktree's suite (so it certifies contended readings), and `TestProcessExitSitesAreAllowlisted` walks into `.worktrees/` when run from the primary checkout, giving a false red there. Both are unchanged by this branch. Contention-aware re-check: Related to #273 (host-relative budgets, deferred), which addresses a different cause — capture a new change and list #273 under `related:`. Exit-site guard: No existing change fits (checked 20) — capture a new change.
