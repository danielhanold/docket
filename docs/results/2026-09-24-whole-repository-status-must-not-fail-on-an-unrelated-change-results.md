<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0454 — Whole-repository status must not fail on an unrelated change's invalid branch name](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-25-0454-whole-repository-status-must-not-fail-on-an-unrelated-change.md)**
<!-- docket:backlink:end -->
# Whole-repository status must not fail on an unrelated change's invalid branch name — Results

**Human action:** No human action is needed before merge. The optional walkthrough below shows the new finding in real `docket status` output, if you want to see it.

## Outcome

Before this change, `docket status` failed completely with an `external-failed` error when any change that another change stacks on recorded a `branch:` value that is not a valid git branch name, for example `feat/a..parent`. Two other reads failed the same way: implement-next's startup preflight (`maintenance.preflight`) and automatic change selection (`context.implementation` with no id). So one broken record hid the whole backlog, including the broken record itself.

Now:

- All three reads complete. A branch name that cannot exist in git is treated as absent and is never fetched. A change stacked on such a parent is not build-ready and shows the existing `stack-base-unresolved` state.
- `docket status` reports one error-severity `branch-malformed` finding on each displayed active change with an invalid branch name. The rest of the backlog still renders. The preflight envelope forwards the finding, and the preflight verdict still depends only on the sweep, as it did before.
- The finding's remedy fits the record's state. If the record has a parseable `pr:`, the remedy is a filled-in `docket change repair-identity --adopt-pr-head` command. Otherwise it says to correct `branch:` on the `docket` branch by hand and then run `docket repository migrate`.
- Docket's local branch-name check now follows git's own `check-ref-format` rules. It also rejects control characters, `~ ^ : ? [`, and a trailing `.`. Names like these used to reach `git fetch` and fail there. The repair path's check (`recordedBranch`) uses the same rule, so `repair-identity` can repair a name like `feat/a:b`.
- A well-formed branch name whose fetch fails for a real reason, such as network or auth, still fails the whole read. Named operations are unchanged.

The build matches the design with one small difference. The design's example said automatic selection picks change 0007. In the test fixture it picks 0006, which is the first healthy candidate by id, and the test pins that.

## Human actions and testing

### Optional — see the finding in real status output

This covers the same ground as `TestStatusBranchMalformed*`. It just shows the new finding in real `docket status` output.

Prerequisites: a scratch clone of a docket-managed repo, not your working repo.

1. On the `docket` branch of the scratch clone, set the `branch:` of an active change to `feat/a..bad`, and set another change's `stacked_on:` to point at it. Commit.
   Expected: the commit succeeds.
2. Run `docket status --json`.
   Expected: `result` is `applied`. `findings` has an entry with `code: branch-malformed`, `severity: error`, and `field: branch` for that change. The stacked child's readiness is `stack-base-unresolved`.

Cleanup: delete the scratch clone.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed on the build head through the build gate. The run printed `BUDGET WATCH` lines only, all on long-running integration and race scripts this change did not touch.
- Every task was mutation-tested: removing the filter, the predicate, the remedy branch, or the `recordedBranch` delegation turns its tests red.
- The whole-branch review (standard rung) found nothing.
