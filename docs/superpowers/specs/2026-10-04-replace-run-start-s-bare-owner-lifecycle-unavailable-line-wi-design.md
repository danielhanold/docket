<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0501 — Replace run.start's bare owner-lifecycle-unavailable line with a plain stop note](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0501-replace-run-start-s-bare-owner-lifecycle-unavailable-line-wi.md)**
<!-- docket:backlink:end -->

# Replace run.start's bare owner-lifecycle-unavailable line with a plain stop note — design

## Problem

Every started run prints two lines:

```
run-started <key> <run-context>
owner-lifecycle-unavailable
```

`startedRunResult` (internal/app/runtracker_start.go) sets `OwnerLifecycle: ReasonOwnerLifecycleUnavailable` on every started run, and `RunStartResult.HumanText` appends it as a bare second line. A bare `something-unavailable` token reads like a reason code for a failure, and since it never varies it tells the reader nothing about the run at hand. Coordinator agents then repeat it in every dispatch report, because the run-tracker rule says `run.start` "reports the honest owner-lifecycle caveat". The human reads that as a bug.

The caveat itself is correct and stays (change 0375, ADR-0118): on the default dispatch routes nothing tells the run tracker that the dispatching session ended, so a run stays active until `run.cancel` runs. Its printed wording is too absolute, though. On the Codex route, implement-next calls `run.start` and then `agent.enter --run-key`, which sets up the death guardian (`agent_guardian.go`). `run.start` cannot know which route follows it, so whatever it prints must be true on both.

## Design

1. **Printed line.** When `OwnerLifecycle` is set, `RunStartResult.HumanText` appends this second line instead of the bare token:

   ```
   note: closing this session may not stop this run. To stop it: docket run cancel --key <key> --reason <why>
   ```

   `<key>` is the run's own key (`r.Key`), filled in. `<why>` stays a literal placeholder. The line is one line, so the started report remains exactly two lines. "May not" (rather than "won't") keeps the note true on the Codex route. A human-facing `docket run cancel --key …` spelling has precedent in `resumeIncumbentRemedy`. The `run-started <key> <run-context>` first line is unchanged.

2. **JSON unchanged.** The `owner_lifecycle` field keeps the value `owner-lifecycle-unavailable`, and `ReasonOwnerLifecycleUnavailable` keeps its name and value. The printed note stays gated on that field being set, so text and JSON cannot drift apart.

3. **Run-tracker rule.** In `cursor-rules/run-tracker.md` (section "Stopping a dispatched run"), replace "and `run.start` says so (it reports the honest owner-lifecycle caveat)" with wording that says `run.start` prints a one-line stop note, and add: that note is for you, not a finding — do not repeat it in your dispatch report; bring up `run.cancel` only when the human asks how to stop a run or a run actually needs stopping. Regenerate the embedded copy (`go generate ./internal/assets`) and refresh this repo's `docket:dispatch` managed blocks in AGENTS.md and CLAUDE.md through docket's own generation path, never by hand.

4. **Comments.** Update the doc comments on `ReasonOwnerLifecycleUnavailable`, `RunStartResult.OwnerLifecycle`, `HumanText`, and the step (7) comment in `RunStart` so they describe the printed note rather than "the token".

## Testing

- Update the exact-match `HumanText` assertions in `runtracker_start_integration_test.go` (two sites) and `change_integration_test.go`, and the `strings.Contains` check in `runtracker_run_record_integration_test.go`, to the new line.
- Add a test that the note carries the run's own key (`--key <res.Key>`), and a test that the bare `owner-lifecycle-unavailable` token no longer appears in `HumanText`, while the JSON field still carries it.
- Mutation-test both new assertions (per AGENTS.md "Guards and tests"): revert `HumanText` to the bare token and confirm they redden.
- Full suite at the build gate.

## Out of scope

- Route awareness in `run.start`, or `agent.enter` reporting its guardian.
- Changing or removing the JSON field or the constant.
- Rewording the rule's "there is no automatic Stop button" heading, which is also inexact on the Codex route.
