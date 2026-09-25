<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0450 — Typed change.unblock operation to reverse change.block](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0450-typed-change-unblock-operation-to-reverse-change-block.md)**
<!-- docket:backlink:end -->
# Typed change.unblock operation to reverse change.block — Results

**Human action:** None required. The change is ready to merge. One optional walkthrough is below if you want to see the two new commands work on a scratch repository.

## Outcome

Docket could move a change to `blocked` (`docket change block`) or `deferred` (`docket change defer`), but there was no command to undo either. Getting a change out meant hand-editing its frontmatter and committing on the `docket` branch, which skipped the version check and left `BOARD.md` stale until someone ran `docket repository migrate`. This happened on change 0444.

Two new operations close that gap:

- `docket change unblock` (`change.unblock`) moves a `blocked` change back to `in-progress` and clears `blocked_by`.
- `docket change revive` (`change.revive`) moves a `deferred` change back to `proposed`.

Both work like `block` and `defer`: the request pins the record's exact version, and one metadata commit updates `updated:`, the `## Artifacts` block and the board together. A change in any other status is refused and nothing is written. Revive leaves the `## Why deferred` section, `branch:` and `claimed_at:` in place, as the spec required. A change that was halted and then blocked keeps its `## Run halted` section through unblock, so `change.resume-halted` works afterwards. That is the 0444 path, and it now has a regression test.

Both commands appear in the capability catalog and the schema registry. The docket-convention lifecycle rules now point to these commands instead of the hand edit.

There was one small departure from the plan: the new schema-registry rows sit in id-sorted position (after `change.resume-halted`), not next to block/defer, because a registry test requires sorted order.

## Human actions and testing

### Optional — Block, unblock, defer and revive a scratch change

Use this to see the new commands work end to end. Automated tests already cover this behavior.

Prerequisites: a throwaway repository initialized with docket, containing one `proposed` change that has `trivial: true`, and a docket binary built from this branch (`go build -o /tmp/docket ./cmd/docket`).

1. Claim the change with `docket change claim`, then block it with `docket change block` using a reason.
   Expected: the status is `blocked`, `blocked_by` holds your reason, and `BOARD.md` shows it as blocked.
2. Run `docket change unblock` with the change id, path and current version (from `docket status --json`) in the request file.
   Expected: the status is `in-progress`, `blocked_by` is empty, and one new commit on `docket` updated the board.
3. Defer the change with `docket change defer`, then run `docket change revive`.
   Expected: the status is `proposed`, and `## Why deferred` is still in the body.
4. Run `docket change unblock` on the now-`proposed` change.
   Expected: the command refuses with `invalid-state` and no commit is made.

Cleanup: delete the scratch repository.

## Verification performed

- Each of the four plan tasks ran focused tests through the gate driver: the lifecycle unit tests, the `internal/app` integration tests (`-tags integration`), the `internal/cli` and `internal/app` packages in full, and the `internal/assets` and `internal/repoguard` tests. All passed.
- Where it applied, each task showed a failing test first: the missing types, then the unregistered commands.
- Mutation check on the regression test: making `ChangeUnblock` call the revive transition caused the 0444 regression test to fail as intended. The mutation was reverted.
- The full build suite (`go run ./cmd/docket development test`) passed. Several integration and race test files printed `BUDGET WATCH` lines, meaning they ran longer than budget while tests ran in parallel. These are screening notes, not failures, and no serial run confirmed a breach.
- The whole-branch review returned one important finding. The spec asked for real-engine integration tests that check the board row for an applied unblock and revive, but the first applied tests used a fake engine. The finding was fixed in commit e5ba1ead: a new real-engine defer-then-revive test, plus checks of the commit subject and board row on the unblock step of the 0444 regression. A deliberately broken expectation made the new assertions fail, so they do catch mistakes. The full suite was then run again at the final head. The build evidence in the PR body records that run.
