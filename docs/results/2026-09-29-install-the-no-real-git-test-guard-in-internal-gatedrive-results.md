<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0470 — Install the no-real-git test guard in internal/gatedrive](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-29-0470-install-the-no-real-git-test-guard-in-internal-gatedrive.md)**
<!-- docket:backlink:end -->
# Install the no-real-git test guard in internal/gatedrive — Results

**Human action:** None needed beyond the normal PR review. The change adds test wiring and updates comments; it does not change any shipped behavior.

## Outcome

Before this change, `internal/gatedrive` was the one package that change 0466 split into a git-free default test corpus and a tagged `integration` corpus without also installing the shared no-real-git guard. If a test that runs real `git` crept back into the default corpus, the only sign would have been a slower `test_go_race` budget row.

The default (untagged) gatedrive test binary now installs `testsupport.InstallNoGitGuard` from a new `TestMain` in `internal/gatedrive/main_test.go`. Any default-corpus test that runs `git` now fails the package and names the package and its integration shards (`tests/test_go_integration_gatedrive_*.sh`) as the place such tests belong. Three proving tests in `internal/gatedrive/nogit_guard_test.go` confirm the guard is installed in this package's binary.

One difference from the pattern 0466 used in `internal/workspace` and `internal/repository/transaction`: gatedrive already has a `TestMain` in its integration-tagged `supervisor_integration_test.go`. The new `main_test.go` therefore carries `//go:build !integration && !e2e`, so the two never compile into the same binary.

Four maintained-source comments that described gatedrive as deliberately unguarded were updated to match: `internal/gatedrive/supervisor_integration_test.go`, `internal/testsupport/nogit.go`, `tests/test_go_race.sh`, and `tests/test_go_integration_gatedrive_process.sh`. Budget rows are unchanged.

## Verification performed

- The proving tests pass, and the whole package passes both plain and under `-race`.
- `go vet` is clean under the default, `integration`, and `e2e` tags, which confirms there is no duplicate `TestMain`.
- Mutation check: with the guard install removed, all three proving tests failed. They passed again after the file was restored (restore checked with `cmp`).
- Straggler probe: a temporary default-build test that ran `git rev-parse HEAD` and swallowed the error still failed the package with the gatedrive guard diagnostic. The probe was then removed.
- No existing default-corpus gatedrive test was caught running git, so no test needed to move behind the `integration` tag.
- The full suite runs at the build gate on the head that contains this file. That evidence is recorded in the PR body, not here.
- Whole-branch review (standard rung) found one minor issue: a comment in `tests/test_go_integration_gatedrive_process.sh` wrapped unevenly. It was rewrapped in-branch with no change to the wording or code.

## Known issues and follow-ups

### `test_go_integration_app_closeout.sh` is over its solo time budget

The build-gate full suite passed, but its budget report showed `SERIAL CONFIRMED OVER BUDGET` for `tests/test_go_integration_app_closeout.sh`: 69s when run alone, against a 60s solo threshold (148s under parallel load). This does not fail the suite. It means that shard is slower than its budget row allows, which can make the suite noisier and slower over time. The finding is confirmed by the runner's serial re-measurement. It is unrelated to this change, which touches only `internal/gatedrive` tests, one `internal/testsupport` comment, and two shell-script headers. Suggested next action: a human decides whether to capture a change to re-measure or split that shard, or to adjust its budget row.
