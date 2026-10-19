<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0567 — Clean up stacked children when their stack lands, and finalize a child after its parent](../changes/archive/2026-10-19-0567-cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea.md)**
<!-- docket:backlink:end -->

# Clean up stacked children when their stack lands, and finalize a child after its parent — Results

**Human action:** Nothing is required to merge. One part of the original problem, finalizing a child whose parent landed first, is still blocked by a separate rule in the parent's closeout. Read the first item under *Known issues and follow-ups* and decide whether to capture it as a new change.

## Outcome

Before this change, when the root of a stack of changes landed on `main`, every child in the stack kept its worktree folder, its local branch, and its remote branch forever. Cleanup refused the children because their pull requests had merged into the parent's branch, not `main`.

Now:

- **Finalize and the sweep clean up every carried child right after the root**, in the same run. Finalize's step 10 runs cleanup for the root and then for each id the root's closeout carried. The sweep does the same after each closeout and reports one cleanup result per child.
- **Cleanup proves a child's work reached `main` before deleting anything.** It walks up the stack (each link's pull request merged into its parent's branch, ending at the root that merged into `main`) and checks that the child's content is on `main`, either by commit ancestry or by exact content at the root's merge. Closeout and cleanup share the same walk and the same content check, so the two can't drift apart. If the relationship can't be proven, cleanup refuses as before (`destination-mismatch`). If the relationship holds but the work can't be found on `main`, it keeps everything and reports a new pending reason, `carry-unproven`, without suggesting a manual delete. A probe that errors also keeps everything.
- **A finished worktree stays its change's own after its base moves.** Docket used to treat a stacked child's folder as someone else's once the child's base changed from its parent's branch to `main`. A finished or cleaned folder is now recognized by its repository, change, slug, branch, and path. A folder whose creation was interrupted still has to match its base. Recorded as ADR-0152.
- **Leftovers already in users' repositories get cleaned.** The sweep no longer skips a stacked child's leftover branch, so the first full sweep after upgrading cleans existing leftovers.

Departure from the design: the second goal, finalizing a child whose parent landed first, works only once the parent is `done`. Docket can't get the parent to `done` on its own today; see the first known issue.

## Human actions and testing

### Optional — watch a real stack get cleaned up

This walkthrough shows the main new behavior in a real repository. The automated end-to-end tests already cover it with a fake GitHub.

Prerequisites: a scratch repository set up with docket, and two changes B stacked on A (`stacked_on:`), both built to open PRs.

1. Merge B's PR into A's branch and run `docket finalize closeout --id <B>`.
   Expected: B is `stacked-merged`, and its worktree folder and branches still exist.
2. Finalize A through to merge and closeout.
   Expected: the closeout result's `carried_ids` lists B.
3. Run `docket finalize cleanup --id <A>`, then `docket finalize cleanup --id <B>`.
   Expected: both report `cleaned`. B's folder under `.worktrees/` is gone, `git branch --list <B's branch>` prints nothing, and `git ls-remote origin <B's branch>` prints nothing.

Cleanup: delete the scratch repository.

## Verification performed

- Full suite (`go run ./cmd/docket development test`), run through the gate driver on the final head: 91 of 91 files passed. No budget breach lines. The report lists the end-to-end and cleanup shards as parallel-sensitive screening notes.
- New tests cover each part: the ownership rule for each workspace phase (unit and real git), the shared stack walk (domain), the shared content check (unit), carried-child cleanup for two- and three-level stacks rebase-merged on real git, with the fail-closed cases (content missing, ancestor probe error, preservation probe error, destination matching no parent, killed link, moved branch, open stacked PR), sweep ordering and dedup, and end-to-end CLI runs: two-level stack cleaned by finalize, three-level stack cleaned by one sweep, a leftover cleaned by a sweep, and a parent-landed-first child.
- Mutation checks: each one went red when its guard was removed. Restoring the old base comparison, skipping the content check (red in both closeout and cleanup tests), dropping the root-ancestry check, skipping the containment proof, removing the sweep dedup, and putting back the carried-descendant sweep exception.
- Whole-branch review (deep tier): 5 findings (1 important, 4 minor). The 4 minors were fixed in-branch. The important one is a blocker outside this branch, reported below. The full table is in the PR body.
- Backlog check ran over 23 proposed and deferred changes.

## Known issues and follow-ups

- **A parent that lands before its stacked child can't be closed out (confirmed).** When a parent's PR merges to `main` while a child is still open, with the child's PR retargeted to `main` through `finalize retarget-children`, the parent's closeout refuses with `children-retarget-required`. Closeout requires every stacked child to be `stacked-merged` into the parent, and it counts a retargeted child too. So the parent never becomes `done`, the child's base never moves to `main`, and the child-after-parent flow can't finish through docket. The end-to-end test for that flow hand-writes the parent's `done` record and then drives the child's rebase, publish, merge, closeout, and cleanup through the CLI. Everything this change owns works there, but the user-facing flow is still blocked. Workaround: none inside docket. Suggested next action: decide whether a retargeted child, one whose PR targets the integration branch, should be excluded from the parent's carry set. Also check whether the parent's closeout still refuses after the child itself merges to `main`. No existing change fits (checked 23); a human captures it as a new change, related to #567.
- **The cleanup integration shard runs well over its time budget when run alone (confirmed).** `tests/test_go_integration_app_cleanup.sh` has a 40s budget and took 82–86s when run alone. About 23s of that comes from this change's carried-stack fixtures. The rest was already there. The end-to-end shard (30s budget) took about 35–40s alone. The suite report flags neither as a breach. Both show up only as parallel-sensitive screening notes. Suggested next action: trim or parallelize the cleanup shard's fixtures, or re-baseline its budget. This is this branch's own test cost, so no backlog match.
- **In some sweeps a carried child's cleanup is dispatched twice (confirmed, harmless).** If a child closes out (`stacked-merged`) earlier in the same sweep that also closes out its root, the child gets a cleanup right after its own closeout, which keeps everything, and another after the root's. Only the later result is reported, so each child shows one outcome. The extra dispatch costs one cleanup observation. This is this branch's own behavior, so no backlog match.
