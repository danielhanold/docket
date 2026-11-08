<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0514 — Retire the saved Bash upgrade test cases when stable v1.0.0 ships](../changes/archive/2026-11-08-0514-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0.md)**
<!-- docket:backlink:end -->

# Retire the saved Bash upgrade test cases — results

**Human action:** None needed. The change only deletes test code and fixtures, moves one test, and edits the upgrade guide's prose; the full suite passed and the review found nothing.

## Outcome

- Deleted `testdata/bash-upgrade/`, the `internal/bashupgrade/` package, the `tests/test_go_integration_bashupgrade.sh` shard, its `tests/runtime-budgets.tsv` row, and the bash-upgrade section of `testdata/README.md` (its intro now says "Two fixture tiers").
- Moved `TestAliasInvocationMatchesDocket` to `cmd/docket/alias_invocation_test.go`. It now uses the binary that `cmd/docket`'s `TestMain` already builds, so the move adds no build.
- `docs/release/upgrading-from-bash.md` no longer mentions the retired test: all 20 `upgrade-step` markers are gone, and each sentence about test coverage was deleted or reworded as the guide's own scope. That includes one sentence the spec's table did not list: "The saved installs cover `ci` only." in the §6 `finalize.gate` row. No step, command, or table cell changed apart from that one sentence.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed at the branch head.
- The moved alias test passes in `cmd/docket`. A mutation that reversed its comparison made it fail, and it passed again once restored.
- A whole-repo grep for `bashupgrade`, `bash-upgrade`, `BashUpgrade` and `upgrade-step` finds only point-in-time records: release decisions and candidate runs, archived specs, plans and results, and the frozen export corpus. These were left as written.
- The deep review tier ran (chosen because the diff is over 1500 lines, almost all deletions) and found nothing.

## Known issues and follow-ups

- **Unrelated budget breach in the suite report.** On this run, the budget runner printed `SERIAL CONFIRMED OVER BUDGET` for `tests/test_go_integration_app_cleanup.sh`: 82s on its own against a 60s ceiling. This branch does not touch that shard. Impact: the line is noise on every build until someone deals with it; it does not fail the suite. Status: confirmed on one run; it may come from a slower host rather than from code. Suggested next action: **Fits #273** (deferred: put runtime budgets on a host-relative basis), the closest match among the 18 proposed and deferred changes checked. Revive #273 with `change.revive`, then edit it as a proposed change. This breach strengthens the case for reviving it.
