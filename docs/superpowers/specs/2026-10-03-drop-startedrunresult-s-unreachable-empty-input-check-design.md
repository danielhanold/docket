<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0495 — Drop startedRunResult's unreachable empty-input check](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0495-drop-startedrunresult-s-unreachable-empty-input-check.md)**
<!-- docket:backlink:end -->

# Drop startedRunResult's unreachable empty-input check — design

## Problem

`startedRunResult` (`internal/app/runtracker_start.go`) builds the `run-started <key> <run-context>` result that `run.start` returns. It opens with a guard: when `key` or `runContext` is empty, it returns `run-untracked mint-failed` instead. Its own comment already calls the guard "unreachable defense in depth". Change 0491's review fix F2 (commit `c999aea8a`) moved the empty-run-context refusal ahead of the mint, and that turned the guard into dead code. Two tests keep it alive by calling the helper directly with blank arguments. 0491's results file lists it as a follow-up.

## What the trace found

1. **The key is never empty.** Both callers take the key from `MintRunTrackerRecord`. It returns a key only on success, built as `implement-next-<stamp>-<pid>-<hex>`. Every failure returns an error, and the caller returns `run-untracked mint-failed` before it formats anything. `MintRunRecord` runs next with the same key, and it would also reject an empty key through `runKeyDir`.
2. **The run context is never empty.** Both callers check `grant.ChildCapability == ""` right after `Prepare` and return `run-untracked scope-failed` before any mint. `RunStart` does this at its step 5, which covers a fresh start and a resume with no run record. `armResumeReplacement` does it for a cancelled-replacement resume. These are the only two production calls to `startedRunResult`.
3. **The pre-mint refusal is already tested on every path.** `TestIntegrationRunStartEmptyRunContextMintsNothing` (`internal/app/runtracker_start_integration_test.go`) runs all three scope-preparing paths with an empty child capability: fresh start, no-run-record resume, and cancelled-replacement resume. It asserts `run-untracked scope-failed`, no returned key, one `Prepare` call, no new run-key directory, and a predecessor left unsuperseded. Removing the guard loses no fail-closed coverage.
4. **`ReasonRunMintFailed` stays.** The real mint and bind failures in both callers still return it.
5. **Two comments lean on the guard.** `startedRunResult`'s comment describes the check. `RunStartResult.HumanText`'s comment says the started line "is always two tokens, because every started result carries both (startedRunResult)".
6. **No overlap with in-flight work.** 0491 is done. 0493, in progress, does not touch these files.

## Decision

### Code — `internal/app/runtracker_start.go`

- Delete the `if key == "" || runContext == ""` branch from `startedRunResult`. The function becomes its single `return newRunStartResult(...)`.
- Rewrite its comment. Callers pass a key from a successful mint and a run context already checked non-empty before the mint (the `scope-failed` refusal), so the positional line is always two tokens. Drop the "unreachable defense in depth" sentence.
- Reword `HumanText`'s comment so the two-token guarantee points at the callers' pre-mint `scope-failed` refusal, not at `startedRunResult`. Anchor on symbol names, never line numbers (ADR-0054).
- Leave the step-7 comment in `RunStart` alone. It already names the step-5 refusal.

### Tests

- `TestIntegrationRunStartStartedResultRequiresKeyAndContext` (`internal/app/runtracker_start_resume_integration_test.go`): delete the blank-input loop. Keep the started-case assertions (`Started`, `Result`, `Key`, `RunContext`, `Target`, `OwnerLifecycle`), because this is the only test that pins the started result's `Target`. Rename the test (suggested: `TestIntegrationRunStartStartedResultFields`) and rewrite its comment to say what it now checks. Keeping the old name would leave a test claiming a refusal it no longer exercises (learning `test-premise-deleted-not-regated`).
- `TestRunStartResultCarriesNoRunID` (`internal/app/runtracker_start_result_json_test.go`): delete the final `startedRunResult("", "ctx-token")` assertion. The `run_id` and two-token assertions stay, and its name and comment still describe it.
- Add no new test. Finding 3's test already covers the pre-mint refusal.

### Verification

- Before deleting the guard, mutation-check finding 3: temporarily remove the `grant.ChildCapability == ""` refusal in `RunStart`, confirm `TestIntegrationRunStartEmptyRunContextMintsNothing` goes red, then restore it.
- Run the whole suite at the build gate.

## Out of scope

- Any other change to `run.start` behavior, its refusal reasons, or the `run-started` report format.
- Reworking the refusal ordering that 0491 settled.
- Moving the renamed test out of its integration-tagged file.
- Editing 0491's merged plan or results file. Both are frozen build records.
