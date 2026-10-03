<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0495 — Drop startedRunResult's unreachable empty-input check](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0495-drop-startedrunresult-s-unreachable-empty-input-check.md)**
<!-- docket:backlink:end -->
# Drop startedRunResult's unreachable empty-input check — Results

**Human action:** None needed. This change only removes dead code and the two test assertions that called it. `run.start` behaves exactly as before.

## Outcome

`startedRunResult` builds the `run-started <key> <run-context>` line that `run.start` prints. It used to start with a check that returned `run-untracked mint-failed` when the key or run context was empty. Since change 0491, both of its callers refuse an empty run context (as `run-untracked scope-failed`) before anything is minted, and the key always comes from a successful mint, so that check could never run. It is now removed.

Two comments that described the old check now point at the callers' refusal before the mint instead: the comment on `startedRunResult` and the one on `RunStartResult.HumanText`. Two test assertions that called the helper with blank inputs are gone. The test that still checks the started result's fields is renamed `TestIntegrationRunStartStartedResultFields`, so its name no longer claims a refusal it doesn't exercise. Nothing departs from the agreed design.

## Verification performed

- Mutation probe of the guard that protects this behavior. Removing the empty-run-context refusal in `RunStart` turned `TestIntegrationRunStartEmptyRunContextMintsNothing` red: the fresh-start and no-run-record-resume subcases reported `mint-failed` and minted a run directory. Removing it separately in `armResumeReplacement` turned the cancelled-replacement-resume subcase red. Both mutations were restored before the commit.
- Focused tests passed: the `TestRunStartResult*` unit tests and the `TestIntegrationRunStart*` integration tests (`-count=1`).
- The full-suite build gate runs on the branch head before the PR opens; its evidence appears in the PR body.
