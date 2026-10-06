<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0533 — Switch a repository between shared and private visibility](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0533-switch-a-repository-between-shared-and-private-visibility.md)**
<!-- docket:backlink:end -->

# Switch a repository between shared and private visibility: design

Change #533, groomed interactively on 2026-10-05. Last in the private-visibility series (#529–#533). It depends on #530 (relative links, artifacts on the metadata branch) and #531 (private mode). It is best built after #532.

## Summary

A repository's visibility is chosen at `init` and is then a fact about the repository's state. Editing config never moves a repository (#531). This change adds the one supported way to move an existing repository between modes, keeping every record:

```
docket repository set-visibility <shared|private>
    [--yes] [--metadata-remote <url>] [--delete-shared-branch] [--remove-shared-files] [--repo-dir <dir>]
```

## Evidence gathered at grooming

- **The pattern to follow** is `docket repository migrate` (`internal/app/repository_migrate.go`):
  - two-pass authorization: a preview, then `--yes` pinned to the previewed source revision (`ExpectedSource`; contended if it moved);
  - phase routing that resumes an interrupted run from durable state (`migrateRoute`);
  - receipts in commit trailers;
  - no force-push, no foreign-ref deletion, and no rollback of a published branch.
- **Records survive a branch rename unchanged.** Metadata commit receipts don't encode the branch name (a `Docket-Result` trailer decoded at grooming holds only id, op, path and slug), so the identical commits can be published under another branch name with every claim receipt and idempotency replay intact. After #530, links between metadata files are relative, so no record needs rewriting.
- **Live runs** are visible in the state folder: active run-tracker keys, cancellation-pending runs, gate drives, and worktree locks.
- **No workflow gate covers the config gap.** Ordinary operations refuse only a legacy layout (`operationalRefusal`). A `needs-review` repository (an uncommitted `.docket.yml`) is admitted. If a repository switched to shared before its repository-identity keys reached the committed `.docket.yml`, workflows would silently run on defaults (for example `integration_branch` falling back to `auto`). The switch itself must prevent that window.

## Decisions (settled with the human)

1. **A dedicated command**, not an option on `migrate`: commands are named for their job.
2. **Preview, then `--yes`,** pinned to the previewed state. The switch is resumable, refuses while any run is live, and is idempotent.
3. **The identical metadata history is published** under the target branch name. It is never re-seeded or squashed.
4. **Shared → private:**
   - `origin/docket` stays unless `--delete-shared-branch` is given, and is deleted only after verifying that the bare remote holds that exact tip.
   - Files committed on the integration branch stay unless `--remove-shared-files` is given, which produces uncommitted edits for the user's PR.
   - Plan and results files merged into the integration branch before #530 always stay.
5. **Private → shared** refuses if `origin` already has a `docket` branch. Two backlogs are never merged. The bare remote is kept as a backup.
6. **Config alignment.** The switch rewrites `visibility` in the local file it manages, so file and state agree.
7. **A second clone** re-running the switch performs only its local phases.
8. **Sparse documentation:** command help, plus one line under `visibility` in `.docket.example.yml`.

## Design

### Preconditions (both directions)

- The repository is set up: not fresh, not legacy, no migration incomplete.
- No live run. Active run-tracker keys, cancellation-pending runs, gate drives, and worktree locks are each named in the refusal, with the `run.cancel` remedy where applicable.
- The metadata worktree is clean, and its local branch equals the resolved remote's tip.
- If the target equals the current mode, the switch only aligns the local file's `visibility` value. Otherwise it is a no-op.

### Preview

The preview lists every phase below that will run and every flag's effect, and pins the metadata tip plus the relevant `origin` refs (`docket`, default branch). It also warns about the following, where they apply:

- **Going private:** teammates' later writes to `origin/docket` will not be seen. If `--delete-shared-branch` is given, existing PR backlinks into `origin/docket` will stop resolving.
- **Any direction:** open PRs whose descriptions carry the other mode's text are listed, but not edited.

### Shared → private phases

1. **Bare remote.**
   - Create it at the default store path (#531), or use `--metadata-remote <url>`.
   - If it already holds `dckt`, adopt it only when its tip is equal to or an ancestor of `origin/docket`'s tip. Otherwise refuse.
2. **Publish.** Push `origin/docket`'s tip to the bare remote as `refs/heads/dckt`, with a create-only or fast-forward lease.
3. **Remote and branch.** Add the `dckt` remote, and create local `dckt` at the same commit.
4. **Config.**
   - Fold the committed `.docket.yml` (as read from `origin`'s default branch) and `.docket.local.yml` into `.git/dckt/config.yml`, with `visibility: private`.
   - Agent model and effort pins are never copied, because they are global-only.
   - Remove `.docket.local.yml`.
5. **State folder.** Rename `<git-common-dir>/docket/` to `<git-common-dir>/dckt/`. Refuse if the target exists and is non-empty.
6. **Ignore entries.** Write the `.git/info/exclude` block.
7. **Metadata worktree.**
   - `git worktree move` it from `<primary>/.docket` to the private checkout path.
   - Switch it to `dckt`, re-scope its hooks setting, and delete the local `docket` branch.
8. **Agent instructions.** When `agent_harnesses` is set, write the dispatch block into the private instructions file `<git-common-dir>/dckt/AGENTS.md` (#532). The repository's committed AGENTS.md keeps loading as before, promoted lessons included. Only docket's managed block is removed from it, and only with `--remove-shared-files`.
9. **`--delete-shared-branch`** (optional).
   - Verify that the bare `dckt` tip equals `origin/docket`'s tip.
   - Delete `origin`'s `docket` with an exact-tip lease.
   - On any mismatch, keep the branch and report.
10. **`--remove-shared-files`** (optional). Produce uncommitted edits that:
   - delete `.docket.yml`;
   - strip the managed `.gitignore` block;
   - strip the AGENTS.md and CLAUDE.md dispatch blocks;
   - delete `.cursor/rules/docket-dispatch.mdc`.

   The user commits them through a normal PR. The switch never pushes to the integration branch.

### Private → shared phases

1. **Refuse if `origin` already has `docket`.**
2. **Repository-identity keys.** If `.git/dckt/config.yml` sets any repository-identity key (`integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`), write `.docket.yml` carrying them as an uncommitted edit and stop with the remedy: commit and push it to the default branch, then re-run. The re-run verifies that the committed file carries those values. This keeps workflows from running on defaults after the switch without adding a workflow gate.
3. **Publish.** Push the bare remote's `dckt` tip to `origin` as `refs/heads/docket`, with a create-only lease.
4. **Branch and worktree.**
   - Create local `docket` at the same commit.
   - `git worktree move` the metadata worktree back to `<primary>/.docket` and switch it to `docket`.
   - Remove the `dckt` remote and the local `dckt` branch.
5. **Agent instructions.**
   - When `agent_harnesses` is set, write the repository-level dispatch block (AGENTS.md, CLAUDE.md, and the Cursor rule, exactly as `install`'s repository phase would) as **uncommitted edits** for review.
   - Promoted lessons held in the private instructions file are **listed** in the output for the human to move into the committed AGENTS.md. Promotion is always a human act.
   - Then remove the private instructions file and the private Cursor rule file.
6. **State folder.** Rename `<git-common-dir>/dckt/` to `<git-common-dir>/docket/`. The private instructions file was already removed in phase 5, so it never lands under the shared state folder.
7. **Config split.**
   - Personal keys go to `.docket.local.yml`.
   - `visibility: shared` plus any remaining repository keys go to the `.docket.yml` uncommitted edit.
   - The managed `.gitignore` block becomes an uncommitted edit.
   - The `.git/info/exclude` block is removed.
8. **Backup.** The bare remote is kept, and its path is printed.

### Resume and receipts

Every phase is idempotent, and its completion is readable from state (refs on each remote, folder names, worktree location, config files). An interrupted switch is resumed by re-running the same command, which routes to the first incomplete phase, the same way `migrateRoute` does. A second clone of an already-switched repository finds the remote phases complete and runs only its local ones.

### Catalog

`repository.set-visibility`, with effects `local-write`, `metadata-write`, and `external-write` (pushes and the optional deletion on `origin`).

## Acceptance criteria

1. **Round trip.** shared → private → shared preserves the metadata tip commit id and every file byte for byte. A claim receipt and an idempotent create replay recorded before the switch replay correctly after it.
2. **Interruption.** Killing the switch after each phase, then re-running, completes it with no duplicate pushes and no orphaned state.
3. **Refusals:**
   - a live run;
   - a dirty or diverged metadata worktree;
   - `origin` already having `docket` on the way to shared;
   - a non-ancestor bare remote on the way to private;
   - repository-identity keys not yet committed on the way to shared.
4. **`--delete-shared-branch`** deletes only on an exact tip match. On a mismatch it keeps the branch and reports.
5. **`--remove-shared-files`** produces uncommitted edits only, and nothing is pushed to the integration branch.
6. **Second clone.** It switches with local phases only.
7. **Private result is clean.** After switching to private, #531's private-layout checks pass: nothing docket-named in the root, `dckt` naming, and the worktree outside the clone.
8. **Instructions follow the switch.**
   - After switching to private, `docket instructions` prints the dispatch block.
   - After switching back to shared, it prints nothing, the repository-level block exists as an uncommitted edit, and any promoted lessons from the private file are listed.

## ADRs expected

A visibility switch preserves records by publishing the identical metadata history under the target branch name. It never re-seeds and never rewrites records. Relates to ADR-0099 and ADR-0001.

## Out of scope

- Merging two backlogs.
- Removing plan and results files merged before #530.
- Rewriting already-pushed history or PR descriptions.
- Guide or concept pages about the switch.
