---
id: 533
slug: 'switch-a-repository-between-shared-and-private-visibility'
title: 'Switch a repository between shared and private visibility'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-05'
updated: '2026-10-07'
depends_on: [530, 531, 535]
stacked_on:
related: [532, 534, 352, 363]
discovered_from: []
adrs: [1, 99]
spec: 'docs/superpowers/specs/2026-10-05-switch-a-repository-between-shared-and-private-visibility-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-switch-a-repository-between-shared-and-private-visibility-design.md](../../superpowers/specs/2026-10-05-switch-a-repository-between-shared-and-private-visibility-design.md) |
| ADRs | [ADR-0001](../../adrs/0001-docket-metadata-branch-model.md), [ADR-0099](../../adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Once private visibility exists, a repository's mode is chosen at `init` and is then fixed. Editing the config never moves a repository, because switching moves real data: the metadata branch history, the state folder, and the metadata worktree. A user who starts privately and later gets a team on board, or who needs to pull docket out of a shared repository, has no supported way to switch. Every record must survive the switch: change files, specs, plans, results, ADRs, learnings, and the commit history that carries claim and idempotency receipts.

## What changes

- A dedicated command, `docket repository set-visibility <shared|private>`. It previews its plan, applies it with `--yes` pinned to what the preview showed, is resumable after interruption, and refuses while any run is live.
- It pushes the identical metadata history under the other branch name. Receipts don't record the branch name, and links are relative, so no record is rewritten.
- To private:
  - creates or adopts the bare remote
  - folds the repo config into `.git/dckt/config.yml`
  - renames the state folder
  - moves ignore entries into `.git/info/exclude`
  - moves the metadata worktree out of the clone
  - optional `--delete-shared-branch` deletes `origin/docket` only after verifying the bare remote holds the same tip
  - optional `--remove-shared-files` removes `.docket.yml` and the managed blocks in one local commit
- To shared:
  - refuses if `origin` already has a `docket` branch
  - publishes the history to `origin` as `docket`
  - writes `.docket.yml`, the `.gitignore` block, and the dispatch blocks in one local commit for review
  - restores the shared layout
  - keeps the bare remote as a backup
- Integration-branch commits use fixed messages, so what stays visible in the repository's history is never typed by hand:
  - going private: `Remove docket configurations from repository`
  - going shared: `Add docket configurations to repository`
  - no message ever says "shared", "public", "private", or "visibility"
  - each commit holds only the switch's own files, and the switch never pushes it
- It rewrites `visibility` in the local file it manages, so file and state agree. Re-running it in a second clone only updates that clone.

## Out of scope

- Merging two separate backlogs: a private repository whose `origin` already has a `docket` branch.
- Removing plan or results files that were merged into `main` before plans and results moved to the metadata branch.
- Rewriting already-pushed history or PR descriptions.
- Guide or concept pages describing the switch. Documentation is command help, skills, and `.docket.example.yml` only.

## Open questions

- **Old spec backlinks are still absolute links (found while building #530, 2026-10-06).** The spec says that after #530, links between metadata files are relative, so no record needs rewriting. That holds for `## Artifacts` blocks once `docket repository repair` has run. It does not hold for the backlink block at the top of spec files groomed before #530: those still carry full `https://github.com/<owner>/<repo>/blob/docket/...` links, and `repository repair` does not rewrite them (it re-renders only `## Artifacts` blocks, the board, and the ADR index). This change's own spec is one of them. After a switch to private with `--delete-shared-branch`, those links stop resolving. Settle at build: the suggested route is to extend `repository repair` to re-stamp spec backlinks as relative links, and have the `set-visibility` preview list any absolute same-branch links that remain.
