---
id: 532
slug: 'private-visibility-keep-docket-out-of-prs-commits-and-shippe'
title: 'Implement private visibility for PRs, commits, and shipped files'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-10-05'
updated: '2026-10-06'
depends_on: [530, 531]
stacked_on:
related: [529, 533, 534, 535]
discovered_from: [531]
adrs: [36, 78]
spec: 'docs/superpowers/specs/2026-10-05-private-visibility-keep-docket-out-of-prs-commits-and-shippe-design.md'
plan: 'docs/superpowers/plans/2026-10-06-private-visibility-keep-docket-out-of-prs-commits-and-shippe.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/private-visibility-keep-docket-out-of-prs-commits-and-shippe'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-06T15:04:58Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-private-visibility-keep-docket-out-of-prs-commits-and-shippe-design.md](../../superpowers/specs/2026-10-05-private-visibility-keep-docket-out-of-prs-commits-and-shippe-design.md) |
| Plan | [2026-10-06-private-visibility-keep-docket-out-of-prs-commits-and-shippe.md](../../superpowers/plans/2026-10-06-private-visibility-keep-docket-out-of-prs-commits-and-shippe.md) |
| ADRs | [ADR-0036](../../adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0078](../../adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md) |
<!-- docket:artifacts:end -->

## Why

In a private-visibility repository, other people see the feature branches, PRs, commits, and code docket produces, and none of them use docket. Even with the metadata branch kept local, docket still leaves traces on shared surfaces:

- PR descriptions carry docket marker blocks and change lines.
- Finalize posts marker-bearing PR comments.
- Models habitually write change ids such as "(0507)" or "change 0612" into commit subjects and PR titles.
- Docket vocabulary can appear in the spec copy that ships with the PR.
- A repository-level instruction block or Cursor rule file names docket.

Once a leak is pushed it cannot be undone, and a rebase-merge would carry it into `main`.

## What changes

- PR descriptions in private repositories are plain prose, with no docket blocks, no `docket:backlink` block, and no change line. The backlink repointing that finalize close-out does after merge (`finalize.closeout`, `finalize.cleanup`, `maintenance.sweep`, `repository.repair --pr-backlinks`) skips private repositories. A finalize block is recorded on the change only, with no PR comment.
- Writing rules for grooming, the plan-writer, build and fix workers, and the PR author: no docket or `dckt` vocabulary and no change ids in specs, commits, code, or PR text.
- A leak check, in private repositories only, scans commit messages, added lines (the spec copy included), and the PR title and description for docket and `dckt` fingerprints. Matching the bare word "docket" can be switched off. The check **blocks** the feature-branch push and the PR create or edit, and the run halts with a report naming the commit or line. This is a deliberate exception to report-only checks, because a pushed leak is irreversible.
- `repository check` and `prepare` report a `docket` or `dckt` branch appearing on `origin` in a private repository. Report only.
- Split out to their own changes: agent instructions for private repositories (#535) and the `dckt` binary alias (#534).
- Finalize creates no `refs/docket/…` refs on the remote in private repositories (found during change 531's build, which left them in place). Keep any ref a private repository needs out of the docket-named ref namespace, or keep it local.
- The leak check and `repository check` also report a `refs/docket/…` ref on `origin` in a private repository. Report only.

## Out of scope

- Content of the metadata branch, which stays private and may use docket vocabulary freely.
- Rewriting history that is already pushed.
- Shared-mode PR descriptions and comments, which are unchanged.
- Scanning for references to the host repository's own ADRs or tickets.

## Reconcile log

### 2026-10-06

Reconciled against current main (dc3559810) after #530, #531, #534 landed. Anchors still hold: assemblePRBody in internal/app/pr_publish.go, finalizeBlockedCommentMarker in internal/app/finalize_block.go, workspace.Service.PublishHead in internal/workspace/publish.go, private findings in internal/app/repository_private_findings.go, and finalize rebase anchors under refs/docket/ (internal/gitcli/rebase.go ownedRefRequiredPrefix). No leak scanner exists yet. Scope unchanged.
