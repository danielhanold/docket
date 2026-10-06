---
id: 531
slug: 'private-visibility-keep-the-metadata-branch-on-a-local-remot'
title: 'Keep the metadata branch on a local remote with neutral naming'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-10-05'
updated: '2026-10-06'
depends_on: []
stacked_on:
related: [352, 363, 530, 532, 533]
discovered_from: []
adrs: [1, 19, 20, 25, 34, 89, 99]
spec: 'docs/superpowers/specs/2026-10-05-private-visibility-keep-the-metadata-branch-on-a-local-remot-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/private-visibility-keep-the-metadata-branch-on-a-local-remot'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-06T10:05:32Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-private-visibility-keep-the-metadata-branch-on-a-local-remot-design.md](../../superpowers/specs/2026-10-05-private-visibility-keep-the-metadata-branch-on-a-local-remot-design.md) |
| ADRs | [ADR-0001](../../adrs/0001-docket-metadata-branch-model.md), [ADR-0019](../../adrs/0019-global-config-fence-classification.md), [ADR-0020](../../adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0025](../../adrs/0025-docket-worktrees-disable-git-hooks.md), [ADR-0034](../../adrs/0034-repo-root-anchored-to-main-worktree.md), [ADR-0089](../../adrs/0089-shared-metadata-worktree-contention-survivable-not-impossible.md), [ADR-0099](../../adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Some host repositories don't allow a `docket` branch, root-level tool files, or any docket-specific content. One user should still be able to run docket there, privately, on their own machine. Today that is impossible:

- The metadata branch is always pushed to `origin`. The remote name is hard-coded at roughly 40 metadata call sites.
- The config file, the `.docket/` worktree, the managed `.gitignore` block, and the per-repo state folder all sit in or under the repository with docket-named paths.
- Repository-identity keys can only come from a committed `.docket.yml`.

Keeping the metadata branch purely local and never pushing it would remove docket's only writer lock. Concurrent metadata writes are serialized by the rejected lease push, so that approach is out. Pushing to a bare repository on the same machine keeps that lock intact.

## What changes

- A new ordinary config key, `visibility: shared | private` (default `shared`), settable in any layer with normal precedence. `docket repository init` sets a repository up in the effective mode; `--private`, `--shared`, and `--metadata-remote <url>` override it.
- A repository's actual mode is its state: a `.git/dckt/` folder means private. A repo-level config file that disagrees is reported by `repository check`. Report only; it never switches a repository.
- In private mode, every per-repo name docket writes uses `dckt`:
  - state folder and config: `.git/dckt/`, with `config.yml` inside
  - metadata branch and git remote: `dckt`
  - bare remote: `~/.local/share/dckt/<owner>-<repo>/remote.git`
  - metadata worktree: `…/checkouts/<clone-id>/`, outside the clone
  - ignore entries: a `# dckt:start` block in `.git/info/exclude`
- The metadata remote, metadata branch name, state folder, and metadata worktree path become resolved values instead of hard-coded ones. Code-side remote work stays on `origin`.
- Private repositories read config from `.git/dckt/config.yml` plus the global layer, never a committed `.docket.yml`, and honor repository-identity keys there.
- `repository check` and `repository prepare` accept the private layout. Skill prose stops hard-coding `.docket/` and `origin/docket`.
- Documentation of the private feature lives only in `.docket.example.yml`, command help, and skills.

## Out of scope

- Keeping docket out of PRs, commits, and code: writing rules and the leak check (a separate change in this series).
- Switching an existing repository between modes (a separate change in this series).
- Renaming the `docket` binary, its install paths, the global config path, agent files, or the `Docket-*` trailers on metadata commits.
- Feature branch naming templates, fork-based workflows, and repositories without PR permissions.
- Guide or concept pages describing private visibility.

## Reconcile log

### 2026-10-06

Reconciled against main 37f3803 after #530 landed (done). Design still holds: originRemote has 56 non-test uses, MetadataBranchName 43, and about 21 Join(..., "docket") state-folder sites remain hard-coded, so the resolver work is still needed. #530 already made metadata file links relative; spec section 6 is unchanged. #532/#533 remain proposed and depend on this change. No scope change.
