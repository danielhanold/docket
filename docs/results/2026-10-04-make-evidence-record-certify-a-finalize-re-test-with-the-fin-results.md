<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0517 — Make evidence.record certify a finalize re-test with the finalize gate settings](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0517-make-evidence-record-certify-a-finalize-re-test-with-the-fin.md)**
<!-- docket:backlink:end -->
# Make evidence.record certify a finalize re-test with the finalize gate settings — Results

**Human action:** No human action is required to merge. One optional walkthrough is below for anyone who wants to see the new `--owner` flag work end to end.

## Outcome

Before this change, whenever finalize's suite passed (after a rebase, or when a repaired head was re-tested), the evidence record was built from the `build.*` settings, not the finalize settings that actually ran. In a repo whose build and finalize settings differ, the record could say "skipped" when the finalize suite really ran. It could name the wrong test command. Or it could be refused outright and halt finalize even though the suite was green.

Now `docket evidence record` takes an optional `--owner build|finalize`:

- **Omitted or `build`:** exactly the old behavior.
- **`finalize`:** reads only `finalize.test_command`, so it never mints skipped evidence and always records the finalize command.
- **Any other value:** refused as `invalid-owner` before any configuration is read.

`finalize rebase`'s built-in gate now passes its own owner, so finalize passes are recorded with the finalize command, and `evidence recertify` still records as build. The finalize skill's repaired-head re-test now passes `--owner finalize`, and implement-next's description of where the recorded command comes from has been corrected. The evidence record format is unchanged, and the change follows the agreed design.

## Human actions and testing

### Optional — see the finalize owner in action

This exercises behavior the automated tests already cover, in a repo where the two commands differ.

1. In a scratch repo set up with `docket repository init`, set `build.test_command: "true"` and `finalize.test_command: "true # finalize"` in `.docket.yml` and push the change.
2. Run `docket gate drive start --owner finalize ... --json` against a feature worktree, advance it to `PASSED`, then run `docket evidence record --id <id> --run <raw_run_dir> --head <head> --owner finalize --json`.
   Expected: `outcome` is `green` and `command` is `true # finalize`.
3. Repeat step 2 with `--owner bogus`.
   Expected: the command is refused with reason `invalid-owner`.

Cleanup: delete the scratch repo.

## Verification performed

- Each task ran its focused tests red, then green, and was mutation-tested:
  - removing the owner switch turned the finalize-owner tests red
  - removing the owner hand-off in `mapTerminalDrive` turned the built-in gate test red
  - dropping the CLI flag turned the routing test red
  - stripping `--owner finalize` from each finalize skill file turned the new `TestAlignmentContracts` entry red
- Every fixture uses different build and finalize commands, so the old and new code produce different records.
- The full suite (`go run ./cmd/docket development test`) passed through the gate driver.
- Whole-branch review (standard tier): no findings.

## Known issues and follow-ups

### The race-test suite file ran over its wall-clock budget

During the full-suite gate, the budget report printed `SERIAL CONFIRMED OVER BUDGET` for `tests/test_go_race.sh`: 117 seconds solo against a 90-second threshold. It does not fail the run. The likely cause is that another change's suite was running on the same machine at the same time, which slows the solo measurement; this change does not touch the race tests. Suspected environmental, not confirmed. Suggested next action: re-measure on an idle machine. Backlog: Fits #273 (put runtime budgets on a host-relative basis) — #273 is a groomed or needs-grooming proposed change, so edit it through `docket-groom-next 273` if the overrun reproduces on an idle machine.

### Skill size ceilings were raised slightly

The added prose pushed three skill files a few bytes over their exact size ceilings in `internal/repoguard/budgets_test.go`. Each ceiling was raised to the new exact count with a note naming this change. There is no runtime impact. No action is needed beyond noticing it in review.
