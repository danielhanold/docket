<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0562 — Sweep retries deleting a leftover local branch](../changes/archive/2026-10-18-0562-sweep-retries-deleting-a-leftover-local-branch.md)**
<!-- docket:backlink:end -->

# Sweep retries deleting a leftover local branch — Results

**Human action:** None required to merge. One optional walkthrough is below for anyone who wants to see the new sweep warning by hand.

## Outcome

Before this change, if `finalize cleanup` removed a done change's worktree but failed to delete its local branch, the full maintenance sweep never tried again: it only checked whether a worktree still had the branch checked out, never whether the branch existed.

Now the full sweep reads the list of local branches once per run (one `git for-each-ref refs/heads/` call) and re-runs the normal cleanup for any done change whose recorded branch is still there, for both PR changes and results-artifact changes. If that list cannot be read, the sweep reports the change as unknown, never as clean.

Three kinds of record are deliberately not retried, so the sweep never dispatches a cleanup that would refuse or do harm:

- a stacked change merged into its parent's branch (its cleanup would refuse; #567 owns deleting those branches),
- a change whose workspace is blocked or unknown (cleanup stops before the branch step),
- a change whose branch name a live change now records. This last one was added during review: without it, the sweep could have deleted a live change's branch on the results-artifact path.

When cleanup keeps a branch on purpose (it moved off the merged commit, has commits not on main, is checked out in another worktree, or the remote copy moved after the merge), the warning now names the branch and gives the delete command, for example `local branch feat/foo was kept: it has commits that are not on main. Review it, then delete it by hand (git branch -D feat/foo).` If a live change uses that name, the warning says so and gives no delete advice. These warnings now print in the human output of both `finalize cleanup` and `maintenance sweep` (one `warning: change #N: …` line each) and repeat on every full sweep until the branch is gone. Temporary failures keep their old quiet retry wording.

The close-out reference and the glossary say again that the sweep retries a leftover local branch, and name the exceptions.

## Human actions and testing

### Optional — see a kept-branch warning in a real sweep

This exercises behavior the integration tests already cover (`TestIntegrationFinalizeCleanupLeftoverBranchFullSweep`). Useful if you want to see the wording yourself.

Prerequisites: a scratch repository set up with docket, with one change that is `done` and fully cleaned up, its branch `feat/<slug>` merged into main.

1. Recreate the branch and add a commit on it: `git branch feat/<slug> main && git checkout feat/<slug> && git commit --allow-empty -m "after merge" && git checkout main`.
2. Run `docket maintenance sweep --scope full`.
   Expected: a line `warning: change #<id>: local branch feat/<slug> was kept: it has commits that are not on main. Review it, then delete it by hand (git branch -D feat/<slug>).`
3. Run the same command again.
   Expected: the same warning appears again.

Cleanup: `git branch -D feat/<slug>`, then sweep once more; expected: no warning.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the final head 5d1f3fb88. Attempt 1 failed on the skill-size budget for `close-out.md` (the new doc text pushed it over; slimmed, ceiling not raised). Attempt 2 failed on `TestIntegrationRunCancelFreesWorktreeForNextStart` (`claim-busy`, a run-cancel timing test this change does not touch); it passed 3 of 3 in isolation and the next full run was green. No repair commit was made for that flake.
- Budget report: `BUDGET WATCH` lines only (no `SERIAL CONFIRMED OVER BUDGET`), including `test_go_integration_app_cleanup.sh` at 112s under -j11 (streak 2/5); this change added integration tests to that shard. Other watch lines were in shards this change did not touch, and concurrent suites from other worktrees were running.
- Mutation checks: removing the branch-presence lookup turned the unit and integration leftover-branch tests red; defeating the stacked, blocked-workspace, and reused-name exclusions each turned their tests red.
- The request-budget tests now assert exactly one local-branch listing per full sweep and none in the implementation-scope sweep.
- Whole-branch review (deep tier): 2 minor findings, both fixed in-branch (reused branch name could be deleted by a sweep; docs overstated "every done change"). Full table in the PR body.

## Known issues and follow-ups

### Flaky run-cancel integration test

`TestIntegrationRunCancelFreesWorktreeForNextStart` failed once in a full parallel run with `cancellation-pending` and a `claim-busy` finding, then passed 3 of 3 alone. Impact: a build or finalize gate can go red for no reason and cost an attempt. Suspected (seen once), unrelated to this change. Workaround: re-run the gate. Suggested next action: capture a change to make the test wait for the claim to clear. No existing change fits (checked 24); capture a new change.

### `close-out.md` is at its word budget

The convention's close-out reference now sits exactly at its 1704-word ceiling. Any future addition must trim the file or lower the ceiling in the same diff. No action needed now.

### Contradictory sentence in `close-out.md` step 1

A build worker noticed that step 1's done-path sentence says the step-2 re-render and step-4 board pass "still run" as no-ops, while steps 2 and 4 say no separate pass runs. It reads as a contradiction; left alone because it is outside this change. Suggested next action: a one-line docs fix. No existing change fits (checked 24); capture a new change (or fold it into the next edit to that file).

### Stacked children's leftover branches

A stacked change merged into its parent's branch still keeps its branches with no warning, as designed here. Fits #567 ("Cleanup deletes a stacked child's branches once its code reaches main"), which already covers it; #567 is a needs-grooming proposed change, so edit it through `docket-groom-next 567` only if you want this case named explicitly.
