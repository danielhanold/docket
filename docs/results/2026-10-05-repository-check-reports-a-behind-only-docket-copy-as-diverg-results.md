<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0523 — Treat a behind-only .docket copy as healthy and make prepare fast-forward it in place](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0523-repository-check-reports-a-behind-only-docket-copy-as-diverg.md)**
<!-- docket:backlink:end -->
# Treat a behind-only .docket copy as healthy and make prepare fast-forward it in place — Results

**Human action:** No action is required before merge. One optional walkthrough is below if you want to watch the user-visible fix on a real checkout.

## Outcome

Before this change, `docket repository check` reported a clean `.docket` folder that was only behind `origin/docket` as both dirty and diverged, and told the user to reconcile it with a human. That is the normal state after any typed metadata write. Now:

- **Check classifies the local copy by ancestry, not tip equality.** Behind-only is healthy. Ahead gets its own `local-metadata-ahead` finding. Diverged keeps `local-metadata-diverged`. An unprovable relationship gets `local-metadata-sync-unverified`. The ancestry probe ignores replace refs and grafts. Check and prepare compute the relationship through one shared function, so their verdicts and refusal texts match. A parity test pins this.
- **Prepare fast-forwards `.docket` in place.** It no longer removes and re-adds the worktree. The move is a compare-and-swap with repository hooks forced off: it refuses rather than overwrite a local edit, an untracked file in the way, or an unfinished Git operation, and it rolls the branch back if the checkout step fails. As decided during the build, an ignored file at a path the target newly tracks is overwritten, which is Git's normal checkout rule (spec §3 property 4).
- **The interrupted case gets a safe remedy.** If prepare is killed mid-update, the next check or prepare names the state and tells the user to run `git -C .docket reset --merge HEAD`. It no longer suggests committing, which would have recorded a revert of the remote metadata.
- **Attach checks freshness.** When `.docket` is missing, prepare attaches at the current tip and then fast-forwards it in place. Git's own checked-out, rebase, and bisect guard decides whether the branch is held elsewhere. A held branch gets the routed `docket-worktree-ambiguous-registration` refusal. If the holder is `.docket`'s own stale registration, the remedy names `git worktree prune`. The attach runs with repository hooks off.
- **Check treats an unfinished Git operation as dirty, and splits the primary-checkout finding by relationship.** A primary checkout that is ahead is no longer reported as behind.
- The upgrade guide no longer tells users to re-run `prepare` as a workaround.

One departure from the plan: the plan's `AdvanceBranchChecked`, which moved the branch before attaching, was replaced during the fix pass by attach-then-fast-forward, and the primitive was deleted.

## Human actions and testing

### Optional — watch check accept a behind-only `.docket`

This exercises behavior the integration tests already cover. It shows the fix on a real checkout.

Prerequisites: a docket-managed repository with this branch's binary installed (`go run ./cmd/docket development install --source <repo>`), and a clean `.docket`.

1. From another clone or a second terminal, make any typed metadata write that pushes to `origin/docket`, then run `git -C <repo> fetch origin`.
   Expected: `git -C <repo>/.docket status` shows the branch is behind `origin/docket` and nothing to commit.
2. Run `docket repository check --repo-dir <repo>`.
   Expected: no `metadata-worktree-dirty` and no `local-metadata-diverged` finding.
3. Run `docket repository prepare --repo-dir <repo> --json`.
   Expected: `applied`. `.docket` is now at the remote tip, and the directory was not removed and re-created (its inode is unchanged: compare `ls -id <repo>/.docket` before and after).

## Verification performed

- Every plan task was test-driven by its worker (RED first, then GREEN) with focused tests. Each guard added on this branch was mutation-tested: an 8-row mutation matrix plus per-fix probes, and every mutation turned its test red.
- The build gate ran the full suite (`go run ./cmd/docket development test`) green before review. The suite runs again on this final head before the PR opens; that evidence goes in the PR body.
- Budget: the gitcli repo shard went over budget solo (36s against 30s). Following the split-not-bump rule, its scenarios were split into a new `gitcli_branchtip` shard (9s, budget 15s). The `repoinplaceff` shard runs 24–28s solo against its 30s ceiling. The gate's budget report showed only PARALLEL-SENSITIVE lines for unrelated shards.
- Whole-branch review (deep tier): 7 findings (1 important, 6 minor), all fixed in-branch. The full disposition table is in the PR body.

## Known issues and follow-ups

### The `repoinplaceff` test shard has little budget headroom

It runs in 24–28s solo against a 30s ceiling. A heavily loaded machine may print a `BUDGET WATCH` line for it. That line is informational and does not fail the run. This is confirmed by measurement and is in scope. Next action: put the next test for this area in a sibling shard rather than this one.

### A branch moved during attach leaves `.docket` attached at the moved tip

If another writer moves the local `docket` branch between prepare's routing and its attach, the compare-and-swap refuses. `.docket` stays attached, hooks off, at the moved tip, with nothing lost, and the next run classifies it as ahead, behind, or diverged. This is a designed limitation, not a defect. No action is needed.
