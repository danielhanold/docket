<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0482 — Finish 0469's leftover "repair" (relink) and "Step 0" (startup check) wording](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-01-0482-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r.md)**
<!-- docket:backlink:end -->
# Finish 0469's leftover "repair" (relink) and "Step 0" (startup check) wording — Results

**Human action:** None required. This change only rewords messages, help text, comments and test wording; reading the PR diff is enough.

## Outcome

Change 0469 renamed two Docket concepts but left their old spellings in place. In some places "repair" still named the `change relink` operation, and "Step-0" / "Step 0" still named the shared startup check (`repository prepare`). This change finishes both renames:

- **User-visible text.** Three `change relink` messages now say "relink" instead of "repair". The `change relink --id` flag help now reads `change id to relink (required)`, and the `repository prepare` help ends in "(the startup check)" instead of "(Step 0)".
- **Go comments and tests.** Comments, test messages, panic strings, three guard fixtures and one test closure (`repair` is now `relink`) use the new names.

Every other meaning is unchanged: finalize's and build's repair, the board and ADR-index "repair notice", the retirement guards that must spell the old token, and implement-next's own "Step 0" step label. No flag, wire token, schema key or behavior changed.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed through the build gate at the code head 891058f60. The run printed several `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` screening lines for long-running integration scripts under parallel load, and no `SERIAL CONFIRMED OVER BUDGET:` line. This branch only touches comments and strings, so those timings are not caused by it.
- Each task ran focused build, vet (including `-tags integration`) and package tests, plus closing greps, through the gate driver. All passed.
- The whole-branch review (standard tier) returned no findings. It confirmed that every site in the spec's inventory changed, and that the closing greps for "repair", "Step-0" and "Step 0" now hit only the meanings the spec keeps, plus frozen testdata.

- A later full-suite run, at the head that added only this results file, went red once. `tests/test_go_race.sh` hit its 8-minute `-race` backstop in `internal/repoguard` while the machine's load average was 45-69, with another change's gate running at the same time. That gate hit the same backstop in the same package. A repair worker found nothing to change on this branch. The re-run of that same head passed.

## Known issues and follow-ups

### `test_go_race` is serially confirmed over its time budget (not caused by this branch)

The suite reports `SERIAL CONFIRMED OVER BUDGET: tests/test_go_race.sh — 94s solo; solo threshold 90s`. When the machine is busy, the race lane can exceed its 8-minute backstop and fail the gate, as happened once here. This is confirmed. Recent solo timings have grown from 32s to 94s, in step with rows added on `main` to the retired-vocabulary seal (`TestRetiredVocabularySeal`). This branch changes only comments and test wording. Workaround: re-run the gate when the machine is less loaded. Suggested next action: open a change on `main` that makes the seal's per-line scan cheaper (for example, pre-filter each line before running the per-row regexes) or re-baselines the budget row. Do not raise the race timeout.

### Two comment lines are slightly wider than their neighbours

`internal/app/repoprepare_integration_test.go` and `tests/test_go_integration_app_repoprepare.sh` each have one header comment line that is now a few characters longer than the lines around it. Re-wrapping would have touched the whole paragraph. This is cosmetic only and has no functional effect. Suggested action: none, or re-wrap whenever those files are next edited.
