<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0466 — Bring test_go_race back under its 60s budget row (transaction, workspace, gatedrive)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0466-bring-test-go-race-back-under-its-60s-budget-row-transaction.md)**
<!-- docket:backlink:end -->
# Bring test_go_race back under its 60s budget row (transaction, workspace, gatedrive) — Results

**Human action:** No action is required before merge. The budget numbers below were taken on a busy machine (load 2–5), so one optional idle-machine re-measure is suggested if you want an extra margin check.

## Outcome

Before this change, `tests/test_go_race.sh` took about 66–67s against its 60s budget row, and `tests/test_go_toolchain.sh` took 62–66s on a cold test cache against its 55s row. Most of that time came from three packages that ran dozens of slow real-git or real-process tests one after another: `internal/repository/transaction`, `internal/workspace`, and `internal/gatedrive`.

Those tests now sit behind the `integration` build tag, in the same partition that changes 0333 and 0465 set up for `internal/app`. Eight new shard runners run them:

- transaction: `apply`, `recovery`, `race`
- workspace: `setup`, `lifecycle`, `race`
- gatedrive: `process`, `race`

Tests that exercise concurrency stay under `-race` in the `race` shards (`TestRaceIntegration…`). Everything else goes to `mode=normal` shards (`TestIntegration…`). The integration contract found the new packages on its own, with no allowlist edit.

The no-real-git guard from 0465 now lives in `internal/testsupport` as `InstallNoGitGuard`. It is installed in `internal/app` (unchanged behavior), `internal/repository/transaction`, and `internal/workspace`, so those default test corpora fail loudly if a real-git test is added to them again. `internal/gatedrive` does not get the guard, as the spec decided. Its budget row is what catches growth there.

The race and toolchain rows (60/55) and the 8m race backstop are unchanged.

One departure from the plan: the first version of `InstallNoGitGuard` called `os.Exit` from a library file, and the repo's exit-site guard (`TestProcessExitSitesAreAllowlisted`) rejected that. The full-suite gate caught it, and a repair commit fixed it. The helper now returns an error, and each `TestMain` does the exit itself. `internal/app` still prints the same diagnostic and uses the same exit code.

## Human actions and testing

### Optional — re-measure the two gates on an idle machine

Every measurement below was taken with a load average of 2–5, never on an idle machine. The margins are wide (about 19s and 21s), so this is not required. It confirms the numbers if you want certainty.

Prerequisites: a checkout of this branch, Go installed, and nothing else running.

1. Run `time bash tests/test_go_race.sh`.
   Expected: exit 0, and a real time well under 60s. This change measured 37–41s.
2. Run `go clean -testcache`, then `time bash tests/test_go_toolchain.sh`.
   Expected: exit 0, and a real time well under 55s. This change measured 32–34s.

## Verification performed

Both gates were measured serially, twice each, on the green tree:

| Gate | Row | Worse of two runs | Margin |
|---|---|---|---|
| `tests/test_go_race.sh` (solo) | 60s | 41.26s (other run 37.15s) | 18.74s |
| `tests/test_go_toolchain.sh` (cold test cache) | 55s | 34.08s (other run 32.49s) | 20.92s |

Default-corpus `-race` time per package, from the `GOMAXPROCS=2 -p 2` ranking:

| Package | Before (groom) | After |
|---|---|---|
| `internal/repository/transaction` | ~46s | 1.24s |
| `internal/workspace` | ~42s | 1.42s |
| `internal/gatedrive` | ~42s | 20.85s |

The slowest package in the race gate is now `internal/cli` (24.0s), and the `test_go_race.sh` header names it.

New shard rows. Each was sized from a timed solo run after a warm-up run, rounded up to the next 5s, plus 5s:

| Shard runner | Measured | Row | Margin |
|---|---|---|---|
| `test_go_integration_transaction_apply.sh` | 18.02s | 25 | 6.98s |
| `test_go_integration_transaction_recovery.sh` | 16.21s | 25 | 8.79s |
| `test_go_integration_transaction_race.sh` | 19.85s | 25 | 5.15s |
| `test_go_integration_workspace_setup.sh` | 17.68s | 25 | 7.32s |
| `test_go_integration_workspace_lifecycle.sh` | 22.01s | 30 | 7.99s |
| `test_go_integration_workspace_race.sh` | 2.30s | 10 | 7.70s |
| `test_go_integration_gatedrive_process.sh` | 6.68s | 15 | 8.32s |
| `test_go_integration_gatedrive_race.sh` | 6.16s | 15 | 8.84s |

The guard was mutation-tested in each guarded package:

- A throwaway default-build test that runs `git status` turned `internal/repository/transaction`, `internal/workspace`, and `internal/app` red, each with the guard diagnostic.
- Removing the `InstallNoGitGuard` call from `TestMain` turned the `TestNoGitGuard*` proving tests red.

Other checks:

- `tests/test_go_integration_contract.sh` passes.
- `go vet` is clean in both the default and the `integration` build for every touched package.
- The whole suite runs at the build gate. Its evidence goes in the PR body.

## Known issues and follow-ups

### `internal/gatedrive` is still the third-slowest package in the race gate

Its default corpus went from about 42s to about 21s under `-race`. It still has 265 fast unit tests, and it has no no-real-git guard (by design: its slow tests start processes, not git). This is confirmed, but it is not a problem today: the race gate has an 18.7s margin. If the race gate creeps toward 60s again, `internal/cli` (24.0s) and `internal/gatedrive` are the next candidates. No change exists for this yet.

### Plan's stale-reference grep always has one hit

Task 1's stale-reference search in the plan matches a comment in `internal/testsupport/nogit_install_off.go` that says where the file used to live. The comment is accurate, so it was kept. The plan is a frozen record, so this is only noted here.
