<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0480 — Report killed changes truthfully in finalize cleanup](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-01-0480-check-finalize-cleanup-s-not-final-message-for-killed-change.md)**
<!-- docket:backlink:end -->
# Report killed changes truthfully in finalize cleanup — Results

**Human action:** None required to merge. If you have an old killed change whose worktree or branch was left behind, you still have to remove it by hand until change 0483 lands.

## Outcome

Before this change, `docket finalize cleanup --id <id>` on a killed change was refused with `invalid-state`. That was misleading: `killed` is a final status. The close-out docs also told kill-path callers that cleanup would prune the killed change's worktree and branch, and it never did. A reconcile-kill that followed the docs therefore reported a failure after the kill had already archived the record, and the resources stayed behind without any explanation.

Cleanup now treats a killed change the same way it already treated a stacked-merged one: as a deliberate retention. It returns `no-op` with disposition `retained` and the new reason `killed-retained`, plus a message saying the workspace and branches are kept. Nothing is deleted. `close-out.md` step 4 and the reconcile-kill paragraph in `edge-paths.md` (with their embedded copies) now describe this outcome and point to change 0483 for real cleanup.

Departures from the spec:
- The behaviour test is `TestIntegrationFinalizeCleanupKilledRetained` in the integration-tagged `finalize_cleanup_integration_test.go`. The spec put it in the unit test file, but the stacked-merged fixture it said to copy needs real Git. Only the wire-spelling pin is in the unit file.
- Before the fix, a killed change was actually refused with reason `branch-missing`, not the `not-final` the spec quoted. Killed records carry no `branch:`, so the aborted-rebase scratch check in the default branch refuses before the not-final refusal is reached. Both were false refusals, and the fix covers both.
- I added guard rows to `internal/repoguard/prose_contracts_test.go`. They require the new kill-path wording and reject the old pruning claims.

## Verification performed

- Each task was test-driven. For the code task the test first failed to compile, then failed for the behavioural reason (`branch-missing`), then passed. Removing the new `killed` case made the test fail again.
- For the docs guard, the prose-contract test failed on all five intended phrases before the edit and passed after it. Re-applying the old wording and swapping the reason spelling made it fail again.
- `go run ./cmd/genassets -check` is clean, and the embedded copies match their sources byte for byte. `edge-paths.md` stays inside its size budget at 118 lines and 1551 words.
- Running the real binary against archived killed change 0028, `go run ./cmd/docket finalize cleanup --id 28 --json` returned `no-op` / `retained` / `killed-retained`.
- The full-suite build gate (`go run ./cmd/docket development test`) is recorded in the PR's build-evidence block.

- The whole-branch review (standard tier) found no blockers and two minor issues. Both were fixed in commit `5ea2a6edc`. The close-out docs now say how to find a killed change's leftover branch and worktree, since the kill clears `branch:` from the record. The prose guard now also rejects the broader phrase "prunes any feature worktree", and a mutation test confirmed it catches that phrase.

## Known issues and follow-ups

### A killed change's worktree and branches are still not removed

This happens when you kill a change that already had a workspace or branch, such as a reconcile-kill of a resumed in-progress change. Cleanup now says honestly that the resources are retained, but it still leaves them on disk and on the remote. The workaround is to remove them by hand. The kill clears `branch:` from the archived record, so the branch is the pre-kill value (`<type>/<slug>` by default), and `git worktree list` shows the worktree. This is confirmed and intended for this change. The next step is change 0483, which adds real killed-change cleanup.
