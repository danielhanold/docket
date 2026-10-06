<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0529 — Repoint a merged PR's change backlink when the change is archived](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0529-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc.md)**
<!-- docket:backlink:end -->
# Repoint a merged PR's change backlink when the change is archived — Results

**Human action:** Before merging, know that the first full maintenance sweep after merge will repoint about 168 already-merged PR descriptions on its own, with no preview. If you want to review them first, run the `docket repository repair --pr-backlinks` preview right after merge, before any full sweep.

## Outcome

Before this change, every PR docket opened linked to its change record at `docs/changes/active/...` on the `docket` branch. Close-out moves the record to `docs/changes/archive/...`, so every merged PR's change link went dead.

Now, when close-out archives a merged change, it rewrites only the docket-owned backlink block in that PR's description so it points at the archived record. Authored text in the description is left byte-for-byte as it was. A PR with no backlink block (written by hand) is left alone. The edit is best effort: if GitHub refuses or the result is unknown, close-out still succeeds and reports a pending warning, and the next cleanup or full maintenance sweep retries it.

The full maintenance sweep applies the same retry to every archived change whose PR backlink still names a stale `active/` path, so it also fixes PRs merged before this change, without a preview. A new, explicit `docket repository repair --pr-backlinks` lists every merged PR whose backlink still names a path that no longer exists, with the current and corrected link, and with `--yes` fixes them one at a time. A PR that can't be read or written is reported and skipped. Plain `repository repair`, `repository check`, and `repository prepare` still never read PR descriptions.

Departures from the spec:

- `finalize.closeout` now declares the `external-write` effect (it edits GitHub). The spec only named this for `repository.repair`.
- `--pr-backlinks` runs only the PR repair, not the usual metadata repairs; plain `repository repair` still does those.
- The full maintenance sweep's retry also covers legacy PRs, so it does the bulk repair itself; `repository repair --pr-backlinks` is the previewed way to do it all at once.
- Every full-scope maintenance sweep now reads the descriptions of done changes' PRs in batches of 25 (roughly ceil(done/25) GitHub GraphQL calls, about 10 for this repository today). The implementation-scope sweep reads none.

## Human actions and testing

### Important — decide how the ~168 legacy PR links get fixed

About 168 already-merged PRs link to a change record that no longer exists. The first full maintenance sweep after merge (for example a `docket-status` refresh) will edit all of them automatically. If you would rather preview the list first, do the steps below right after merge and before any full sweep; if you are happy with the automatic fix, you can skip them.

Prerequisites: this change merged and the installed `docket` binary rebuilt from `main`; `gh` authenticated with write access to the repository.

1. Run `docket repository repair --pr-backlinks --repo-dir /Users/homer/dev/docket`.
   Expected: a preview listing each PR with its current `active/` link and the corrected `archive/` link, and a note that confirmation is required. A read-only preview during the build planned 168 PRs, 0 unreadable.
2. Spot-check two or three listed PRs on GitHub: the corrected link should open the archived change record.
3. Run the same command with `--yes`.
   Expected: per-PR outcomes, all repointed (or individually reported if one fails).
4. Run step 1 again.
   Expected: nothing left to repair.

This edits PR descriptions on GitHub; there is no undo command, though each PR's description history on GitHub keeps the prior text.

### Optional — watch the first real close-out

The automated tests use a fake GitHub. After merge, the next finalized change's PR description should link to `docs/changes/archive/...` once close-out runs. If close-out reports a `pr-backlink-pending` warning instead, GitHub may be storing the edited description differently from what docket sent (see Known issues).

## Verification performed

- A real, read-only `gh api graphql` probe confirmed that merged PR #250 is `MERGED` with `viewerCanUpdate: true` (gh 2.101.0), so a merged PR's description is editable. No GitHub write was made against a real PR during the build.
- Each plan task ran focused tests red then green; mutation probes reddened the guards for the close-out leg (acceptance criterion 5), the cleanup retry, the sweep's unknown-body handling, the sweep reader's identity memo, the edit's revision check, and the "no GitHub reads without `--pr-backlinks`" guard (acceptance criterion 4).
- A read-only preview of `repository repair --pr-backlinks` against this repository planned 168 PRs, 0 unreadable; `--yes` was not run.
- Bundle regeneration was checked with `genassets -check`; the living-docs, comment-anchor and skill-size guards pass.

- Whole-branch review (deep tier): 6 findings (1 blocker, 1 important, 4 minor), all fixed in-branch; full table in the PR body.
- The final full-suite gate passed; its budget report carried screening-only `PARALLEL-SENSITIVE` / `BUDGET WATCH` lines (finalize e2e, app merge, race tests) and no serial-confirmed breach.

## Known issues and follow-ups

### GitHub may normalize an edited description

When docket edits a PR description it re-reads it to confirm. If GitHub stores the text with different bytes (for example, changed line endings), the re-read won't match and docket reports the leg as pending, retrying on each sweep. This is suspected, not observed; the tests use a fake GitHub. Watch the first real close-out after merge. No backlog follow-up needed unless it happens.

### Full sweeps now make more GitHub calls

Every full-scope maintenance sweep reads done changes' PR descriptions in batches, about 10 extra GraphQL calls today and growing with the archive. This is by design so the sweep can retry failed repoints. If it becomes a cost problem, the sweep could skip records already confirmed repointed.

### The first full sweep edits many PRs at once

Because the sweep retries any archived change with a stale link, its first run after merge makes about 168 cleanup passes and GitHub edits. This was a review blocker, resolved by keeping the spec's sweep rule and documenting it. Each edit only rewrites the docket-owned link block. To control timing, run the repair preview first (see Human actions).

### Older PRs without a backlink block are not touched

The preview found 168 PRs to fix rather than the roughly 250 estimated at grooming. Older merged PRs that predate the backlink block have nothing to repoint and are deliberately left alone, matching the spec.
