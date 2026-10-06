---
id: 529
slug: 'repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc'
title: 'Repoint a merged PR''s change backlink when the change is archived'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-06'
depends_on: []
stacked_on:
related: [337, 417, 530]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc-design.md'
plan: 'docs/superpowers/plans/2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc.md'
results: 'docs/results/2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-06T00:51:00Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc-design.md) |
| Plan | [2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc.md](https://github.com/danielhanold/docket/blob/fix/repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc/docs/superpowers/plans/2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc.md) |
| Results | [2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc-results.md](https://github.com/danielhanold/docket/blob/fix/repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc/docs/results/2026-10-05-repoint-a-merged-pr-s-change-backlink-when-the-change-is-arc-results.md) |
<!-- docket:artifacts:end -->

## Why

Every PR docket opens starts with a `↩ Change NNNN — title` link to the change record at `docs/changes/active/<id>-<slug>.md` on the `docket` branch. When the change closes out, the record moves to `docs/changes/archive/<date>-<id>-<slug>.md`, and nothing updates the PR description. Every merged PR's link to its change is therefore dead. Example: PR #250 links to `active/0363-…`, but change 0363 was archived on 2026-08-29.

Roughly 250 merged PRs in this repository carry a dead link today, and every future merge adds one more.

## What changes

- When close-out archives a change, it edits the merged PR's description so the backlink points at the archive path. This is a GitHub edit, not a git push. A failed edit is retried by the maintenance sweep, the same way other best-effort close-out legs are.
- A one-time repair, previewed first and run only after a human confirms, fixes the existing merged PRs whose backlink still points at an `active/` path that no longer exists.
- Living docs describing the PR backlink are corrected only where they would otherwise be wrong.

## Out of scope

- Moving plan, results, or build evidence (a separate change in this series).
- Changing the backlink's wording or adding new links to the PR description.
- PRs of changes killed before they merged.
- Private-visibility repositories, whose PRs carry no backlink at all (a separate change in this series).

## Reconcile log

### 2026-10-06

2026-10-06: Reconciled against current main (6b02ad8bb). The code sites the spec cites (assemblePRBody in internal/app/pr_publish.go, runCloseoutBacklinkLeg in internal/app/finalize_closeout.go, sweepAssessBacklinkLeg in internal/app/maintenance_assess.go, EnsurePullRequest in internal/githubcli/ensure.go) all still exist as described. Related #530 is still proposed, so the integration-branch backlink legs remain in place. No scope change.
