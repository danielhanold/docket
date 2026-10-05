<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0529 — Repoint a merged PR's change backlink when the change is archived](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0529-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc.md)**
<!-- docket:backlink:end -->

# Repoint a merged PR's change backlink when the change is archived: design

Change #529, groomed interactively on 2026-10-05. It is independent of the private-visibility series (#530–#533) it was found alongside, and it can land first.

## Summary

Every PR that docket publishes opens with a backlink line, `↩ Change NNNN — title`, pointing at the change record on the `docket` branch at `docs/changes/active/<id>-<slug>.md`. Close-out moves the record to `docs/changes/archive/<date>-<id>-<slug>.md`, but nothing updates the PR description. Every merged PR's link to its change is therefore a 404.

This change makes close-out repoint the link when it archives the change. It also adds a one-time, human-confirmed repair for the PRs that are already broken.

## Evidence gathered at grooming

- PR #250 (change 0363) links to `https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0363-remove-main-mode-compatibility-from-go-v1.md`. Change 0363 was archived on 2026-08-29 and now lives at `docs/changes/archive/2026-08-29-0363-…`, so the link resolves to nothing.
- The most recently merged PR (change 0507) links to `active/0507-…`. 0507 was archived on 2026-10-05.
- `origin/main` holds 374 plan files and 291 results files. Most merged PRs carry the same dead link.
- `pr.publish` builds the PR body in `assemblePRBody` (`internal/app/pr_publish.go`). It inserts or replaces the docket-owned backlink block, rendered by `render.BacklinkContent` against the change's path at publish time.
- Close-out's existing backlink work (`runCloseoutBacklinkLeg` / `backlinkLegRetarget` in `internal/app/finalize_closeout.go`, plus the maintenance-sweep assessment `sweepAssessBacklinkLeg` in `internal/app/maintenance_assess.go`) re-stamps backlinks inside the merged plan and results files on the integration branch. It never touches the PR body.
- The GitHub adapter's create-or-edit path (`githubcli.Client.EnsurePullRequest`, `internal/githubcli/ensure.go`) finds PRs by head among **open** PRs (`openPRs`). A merged PR therefore needs an edit addressed by PR number.

## Design

### 1. Close-out repoints the PR backlink

When `finalize.closeout` archives a change that has a `pr:` (every archiving disposition with a merged PR: `done-archived`, `root-archived`, and each stacked descendant promoted with it), it also does the following:

1. Renders the backlink block interior for the record's **archive** path, using the same renderer `pr.publish` uses.
2. Reads the PR body by number, replaces only the docket-owned backlink block, and leaves every authored byte unchanged. It uses the same document block-replace path as `assemblePRBody`.
3. Writes the body back with a GitHub edit addressed by PR number, then re-reads it to verify.

Rules:

- **Best-effort, never blocking.** A failed or unknown GitHub edit does not fail close-out. It becomes a pending warning, the same posture as the existing best-effort close-out legs, and the maintenance sweep retries it.
- **Idempotent.** A body whose block already points at the archive path is a no-op. A body with no backlink block (a PR a human wrote by hand) is left untouched and is never given one.
- **The sweep retries.** The sweep's leg assessment gains the PR-body leg for archived changes whose PR body still names an `active/` path. Reads use the sweep's existing batched PR reader.
- **The adapter gains an edit-by-number for a merged PR.** The plan's first task confirms with a real `gh` call that a merged PR's body is editable, and records the result in the results file.

This leg is separate from the integration-branch backlink legs. Change #530 retires those, but the PR-body leg stays.

### 2. One-time repair of existing PRs

`docket repository repair --pr-backlinks` finds every archived record with a `pr:` whose PR body's backlink block names a path that no longer exists on the metadata branch. Its preview lists each PR with its current and corrected link. `--yes` applies the same edit as section 1, one PR at a time, and reports per-PR outcomes. A second run finds nothing.

- It sits behind an explicit flag, so a routine `repository repair` stays local and never reads about 250 PR bodies. `repository check` and `prepare` never read PR bodies.
- `repository.repair`'s catalog effects gain `external-write`.
- A PR whose body can't be read or written is reported and skipped. It never aborts the batch.

### 3. Docs

Living docs that describe the PR backlink are corrected only where they would otherwise be wrong. They describe current behavior only, with no change citations, as required by `TestLivingDocsAlignment`.

## Acceptance criteria

1. After close-out of a merged change, the PR body's backlink block links to the archive path. Every authored byte outside the block is unchanged, and re-running close-out or the sweep is a no-op.
2. A simulated GitHub edit failure leaves close-out's disposition unchanged and emits a pending warning. The next sweep repoints the link.
3. `repository repair --pr-backlinks` previews exactly the merged PRs with a dead `active/` backlink. `--yes` fixes them, and a second run reports nothing to do.
4. A routine `repository repair` (no flag), `repository check`, and `repository prepare` make no GitHub PR-body reads (pinned by a test with a fake adapter that fails on any PR read).
5. Removing the PR-body leg from close-out turns a test red (mutation-tested).

## Out of scope

- Moving plans, results, or build evidence (#530).
- Changing the backlink wording or adding other links to the PR body.
- PRs of killed changes.
- Private-visibility repositories, whose PRs carry no backlink (#532).
