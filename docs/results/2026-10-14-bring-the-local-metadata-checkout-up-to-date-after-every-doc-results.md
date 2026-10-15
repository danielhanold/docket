<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0549 — Bring the local metadata checkout up to date after every docket write](../changes/active/0549-bring-the-local-metadata-checkout-up-to-date-after-every-doc.md)**
<!-- docket:backlink:end -->

# Bring the local metadata checkout up to date after every docket write — Results

**Human action:** None required. One optional walkthrough is listed if you want to see the private-mode board refresh with your own eyes.

## Outcome

Before this change, every docket write (create, claim, finalize, kill, and the rest) pushed its commit to the metadata remote but left the local metadata checkout on disk behind until the next `repository prepare`. In a private repository that checkout is the only place a human reads `BOARD.md`, so the board looked stale after finalize.

Now every typed write, and `repository repair`, moves the local metadata checkout (shared `.docket/` or private `checkouts/<clone-id>/`) forward to the commit it just pushed, when that is safe. It is best-effort and silent: a checkout with uncommitted edits or untracked files, its own commits, a different branch, or one already moved further is left alone, and the next `prepare` reports it as before.

Departures from the spec:

- The follow requires a clean checkout before moving it. Git's `read-tree -u -m` would otherwise carry unrelated local edits across the move, which breaks the spec's "left alone" rule. This matches `prepare`'s own rule.
- The follow reads the branch tip before the clean check, so two writes landing in one clone at once can only skip, never corrupt the checkout. The reasoning is in the function's doc comment.
- `repository repair` no longer prints "pending local sync: re-run `docket repository prepare`" (and its JSON `pending_local` stays empty) when the follow already brought the checkout up to date. It still prints it when the checkout could not be moved. The spec had listed output changes as a non-goal; review flagged the line as now false, so it was fixed in-branch.
- The upgrade guide (`docs/release/upgrading-from-bash.md`) and its guide-driven test now describe repair leaving `.docket` at the repaired commit.

## Human actions and testing

### Optional — watch a private repository's board refresh after a write

Shows the user-visible effect in the layout this mostly benefits. Automated tests cover it (shared and private fixtures).

Prerequisites: the rebuilt `docket` binary from this branch, and a private test repository (for example `dckt-test`).

1. Run `docket open board --print` in the private repository and note the path it prints.
   Expected: a path under `~/.local/share/dckt/<owner>-<repo>/checkouts/<clone-id>/`.
2. Create a throwaway change: `docket change create` with any title.
   Expected: the command reports applied.
3. Without running any other docket command, open the `BOARD.md` at the path from step 1.
   Expected: the new change is already listed.

Cleanup: kill the throwaway change with `docket change kill`.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the final head: 87/87 files. The first full run went red in the bash-upgrade guide test because repair now moves `.docket`; a repair task updated the test and guide, and the next run was green.
- New tests cover: the engine hook running once on applied/already-applied and never otherwise; the follow's leave-alone cases (dirty, untracked, interrupted fast-forward, other ref, unregistered clone, ahead/diverged); typed writes in shared and private fixtures; repair following the checkout and keeping the pending-local line when the checkout is dirty. Workers mutation-tested each guard, including removing the `newPlanningDepsOver` wiring.
- Whole-branch review (deep tier): 2 findings (1 important, 1 minor), both fixed in-branch; full table in the PR body.
- Budget report: the new shard `tests/test_go_integration_app_metadatafollow.sh` has a 40s budget measured under heavy machine load. The final run showed only BUDGET WATCH lines for unrelated shards (release, toolchain); there were no serial-confirmed breaches.
