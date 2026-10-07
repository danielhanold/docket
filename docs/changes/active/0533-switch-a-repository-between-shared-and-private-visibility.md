---
id: 533
slug: 'switch-a-repository-between-shared-and-private-visibility'
title: 'Switch a repository between shared and private visibility'
status: 'in-progress'
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
plan: 'docs/superpowers/plans/2026-10-07-switch-a-repository-between-shared-and-private-visibility.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/switch-a-repository-between-shared-and-private-visibility'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-07T10:59:10Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-switch-a-repository-between-shared-and-private-visibility-design.md](../../superpowers/specs/2026-10-05-switch-a-repository-between-shared-and-private-visibility-design.md) |
| Plan | [2026-10-07-switch-a-repository-between-shared-and-private-visibility.md](../../superpowers/plans/2026-10-07-switch-a-repository-between-shared-and-private-visibility.md) |
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
- `docket repository repair` also re-stamps the generated backlink block at the top of spec files as a relative link, the same way it already re-renders `## Artifacts` blocks. Spec files groomed before #530 still carry absolute `https://github.com/<owner>/<repo>/blob/docket/...` backlinks (288 of 395 specs in this repository at reconcile), and those stop resolving once `origin/docket` is deleted.
- The `set-visibility` preview lists, as a warning only (never a refusal), any metadata files that still carry absolute same-branch links, with `docket repository repair` as the remedy.

## Out of scope

- Merging two separate backlogs: a private repository whose `origin` already has a `docket` branch.
- Removing plan or results files that were merged into `main` before plans and results moved to the metadata branch.
- Rewriting already-pushed history or PR descriptions.
- Guide or concept pages describing the switch. Documentation is command help, skills, and `.docket.example.yml` only.

## Open questions


None open. The absolute spec-backlink question found while building #530 was settled at reconcile (2026-10-07): take the suggested route. `repository repair` re-stamps spec backlinks as relative links, and the `set-visibility` preview warns about any absolute same-branch links left. The warning is visibility-only, never a gate.

## Reconcile log

### 2026-10-07

2026-10-07 — Claimed by docket-implement-next. Dependencies #530, #531, #535 are done; related #532 and #534 are done. Traced current code: `repository set-visibility` does not exist yet (only the remedy text in `internal/app/repository_private_findings.go` names it); `internal/app/repository_migrate.go` is still the pattern to follow; `repository repair` re-renders `## Artifacts`, the board, and the ADR index, and repairs PR backlinks with `--pr-backlinks`, but never re-stamps spec-file backlinks. 288 of 395 specs on `docket` still carry absolute `blob/docket` backlinks. Settled the open question by taking its suggested route: extend `repository repair` to re-stamp spec backlinks as relative links, and have the `set-visibility` preview warn (never refuse) about absolute same-branch links left. Scope otherwise unchanged.
