<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0323 — docket uninstall and version-tree collection for the Go installer](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0323-docket-uninstall-and-version-tree-collection-for-the-go-inst.md)**
<!-- docket:backlink:end -->
# docket uninstall and version-tree collection for the Go installer — Results

## Outcome

Implementation is paused during Task 8. Tasks 1 through 7 are committed on the feature branch, with Task 7 committed as `2056ea7c34a4f09eebde78606b9215a19c3de043` and published to its remote. The Task 8 worker confirmed the RED state and added implementation edits, but returned `BLOCKED` while its focused gate drive was waiting without a valid handoff token. Parent takeover of the same drive produced terminal `HALTED` with cause `handoff-outstanding`. No Task 8 commit was created; the worker's uncommitted edits remain preserved in the feature worktree for inspection and a later resume.

## Verification performed

- `workspace.inspect` reported the owned feature workspace ready at `626a1f1aa5d825e17e2f54809ffdfdea3f1cc8db`.
- Tasks 1 through 6 were committed and verified on the feature branch:
  - Task 1: `15332684a5604709763232f15d57cefa9bc14da6` — empty and active ownership-state validation; focused validation passed.
  - Task 2: `6f6e53f444256642dce9ee8b1ef0efcffa0c6f63` — version manifests; focused version/manifest tests and legacy-root mutation checks passed.
  - Task 3: `c53ba024969f105a1c9c96388f82a15bb564b2ae` — reference derivation and containment; focused and mutation checks passed.
  - Task 4: `0ce89b63ab3e701247bb5873642ba2c9a8b18bf1` — collection journal and roots; interruption and mutation checks passed.
  - Task 5: `0506ed2bf47a33b5f1050539496e5f28fc30f69c` — collection, quarantine, deletion, and dry-run lock; focused tests and deletion mutation checks passed.
  - Task 6: `acede6e9f166b0920f3cd58a712c74f8af8af617` — transactional uninstall and recovery; focused matrix, `./internal/install`, and mutation checks passed.
- Task 7 left nine tracked files modified and uncommitted; `git diff --check` was clean. The edits include the temporary removal of the `uninstall` capability annotation in `internal/cli/root.go` and are intentionally preserved.
- Scope takeover returned drive `8c8311c8b894668958aa7c8d301102c0` with terminal `PASSED`; its focused command completed with exit code 0 but matched no tests.
- No Task 1 files were modified and no Task 1 commit was created.
- The Task 8 worker's uncommitted changes are limited to `internal/app/install.go`, `internal/install/devmode.go`, `internal/install/service.go`, `internal/install/service_test.go`, and `internal/install/uninstall.go`; they were not adopted or committed by the parent.
- Task 8 focused gate drive `d8a602fc627bf9a08159251f694d11de` returned `WAITING`; its worker return omitted the required single-use handoff token. Parent takeover of scope `d079e194c2c103481482387451ab0e05` returned `HALTED` with cause `handoff-outstanding`.

## Findings and limitations

### Focused gate credentials were not captured

The dispatched standard worker returned `BLOCKED` before TDD because its gate-start wrapper used a reserved zsh variable and therefore did not capture the required JSON response. The scope was safely taken over and its already-started drive was not rerun. A fresh resumed run needs the worker-side gate-start capture corrected before implementation can continue.

### Resume attempt — Task 1 gate request rejected

The resumed standard worker used a newly prepared scope with the supplied gate context. Its focused RED drive reached the expected pre-implementation failure, but the focused GREEN gate-start request was rejected twice by the native driver as `invalid-request` before any GREEN command launched. No task commit was created. The worker left only the four assigned Task 1 files modified and unstaged for inspection on a later resume; no gate process remained live.

### Resume attempt — worker contract unavailable

The standard feature-worker entry was refused before launch because the installed
`docket-build-task` contract differs from the source contract at
`/Users/homer/.agents/skills/docket-build-task/SKILL.md`. No worker or focused gate started;
the four owned Task 1 edits remain preserved and uncommitted for a later resume. The required
remedy is to run `docket install` and start a fresh session before dispatching the worker again.

### Resume attempt — Task 7 mutation gate response unavailable

The standard worker completed its Task 7 RED/GREEN work, but the final required mutation-test gate invocation returned no JSON drive identity or owner generation while its test process remained live. The parent takeover of scope `41b036303bbb63d3bba029247ef61b5f` advanced drive `505277339b6d5b203a18447822bbfe1f` to terminal outcome `FAILED`. No Task 7 commit was created, and all nine uncommitted Task 7 files remain available for human inspection. A fresh run needs a valid native gate response before Task 7 can be accepted.

### Resume attempt — Task 8 focused gate handoff unavailable

The premium Task 8 worker confirmed the expected RED tests and added the shared post-commit collection implementation plus tests, but its native focused gate returned `WAITING` and the worker returned without the required single-use handoff token. The parent takeover of the prepared scope returned `HALTED` with cause `handoff-outstanding`. The Task 8 edits remain uncommitted and preserved; no replacement worker or rerun was started.
