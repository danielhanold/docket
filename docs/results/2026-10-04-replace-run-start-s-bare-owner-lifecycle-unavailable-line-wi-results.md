<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0501 — Replace run.start's bare owner-lifecycle-unavailable line with a plain stop note](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0501-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi.md)**
<!-- docket:backlink:end -->
# Replace run.start's bare owner-lifecycle-unavailable line with a plain stop note — Results

**Human action:** None required to merge. After merge, rebuild the installed `docket` binary as usual; one optional check below confirms the new line by eye.

## Outcome

`run.start` used to print a bare `owner-lifecycle-unavailable` token as its second line on every start. It looked like an error code, and coordinator agents copied it into every dispatch report.

The second line now reads:

```
note: closing this session may not stop this run. To stop it: docket run cancel --key <key> --reason <why>
```

`<key>` is filled in with the run's own key. `<why>` stays a placeholder for the person to fill in. The first line (`run-started <key> <run-context>`) is unchanged. The JSON output is unchanged: `owner_lifecycle` still carries `owner-lifecycle-unavailable`, and the printed note is shown only when that field is set.

The run-tracker rule (`cursor-rules/run-tracker.md`, mirrored into the embedded assets and this repo's `AGENTS.md`/`CLAUDE.md` dispatch block) now says `run.start` prints a one-line stop note, and tells coordinators not to repeat it in dispatch reports. They should bring up `run.cancel` only when asked how to stop a run or when a run actually needs stopping. The dispatch block's word budget in `internal/repoguard/budgets_test.go` went from 897 to 932. That is still below the 1156-word ceiling.

## Human actions and testing

### Optional — see the new line

Do this after the post-merge binary rebuild. Use any repo where you are about to dispatch implement-next anyway, so the run you start is a real one.

1. Run `run.start` with `implement-next` as you normally would.
   Expected: two lines. The first is `run-started <key> <run-context>`. The second starts with `note: closing this session may not stop this run.` and ends with `docket run cancel --key <the same key> --reason <why>`.
2. If you don't dispatch after all, cancel the run with the command the note printed, replacing `<why>` with a reason.

## Verification performed

- New unit tests check three things: the note names the run's own key, the bare token is gone from the printed text while JSON still carries it, and the note appears only when the lifecycle field is set. Both new assertions were mutation-tested: putting back the bare token, and hard-coding `<key>`, each turned them red.
- The existing exact-match integration assertions were updated to the new line and pass.
- The dispatch-block guards (`TestCommittedCodexDispatchMatchesGenerator`, `TestDispatchBlockBudget`) went red after the rule edit and green after regeneration through docket's install path. The embedded-asset check (`genassets -check`) passes.
- Full suite: certified by the build gate at the head that carries this file. The evidence is in the PR body.
- Whole-branch review (standard tier): no findings.
