---
id: 142
slug: 'private-visibility-keeps-the-single-metadata-layout-and-vari'
title: 'Private visibility keeps the single metadata layout and varies only where the metadata branch is published and how per-repo paths are spelled'
status: 'Accepted'
date: '2026-10-06'
supersedes: []
reverses: []
relates_to: [1, 99, 19]
change: 531
---

## Context

Some host repositories forbid a `docket` branch or docket-named files in the repository root, yet one user must still be able to run docket on them privately. ADR-0099 made the orphan metadata branch plus metadata worktree plus lease-push compare-and-swap the sole metadata topology; ADR-0019 made the repository-identity keys per-repo-only so every clone agrees on them; ADR-0001 established the metadata-branch model. A private mode must fit inside these decisions rather than fork them.

## Decision

Private mode keeps ADR-0099's single topology unchanged: an orphan metadata branch, a metadata worktree, and lease-push CAS as the writer lock. What varies is only where the metadata branch is published and how per-repo paths are spelled:

- The metadata branch is published to a bare repository on the user's machine, `${XDG_DATA_HOME:-~/.local/share}/dckt/<owner>-<repo>/remote.git`, or to any URL given via `--metadata-remote`, under the git remote `dckt`.
- The branch, the state folder (`.git/dckt/`), and the exclude markers are named `dckt`.
- The metadata worktree lives outside the clone, under `.../checkouts/<clone-id>/`.

Mode is decided from repository state (`.git/dckt/` exists), never from config. `visibility: shared|private` is an ordinary layered config key that only steers `init`; a repo-level file whose value disagrees with the repository's actual mode is a report-only finding.

Private repositories read configuration from `.git/dckt/config.yml` plus the global layer only (never `.docket.yml`), and honor the repository-identity keys there, because a single private clone has no cross-clone agreement to protect. This narrows ADR-0019 for private mode only. Having both `.docket.local.yml` and `.git/dckt/config.yml` present is a refusal.

## Consequences

Four resolved values (metadata remote, metadata branch, state folder, metadata worktree) replace the hard-coded spellings throughout the code. Code-side remote work (feature branches, PRs, integration fetches) stays on `origin`. Skills key on the prepare-context values (`metadata_worktree_path`, `metadata_remote`, `metadata_tracking_ref`) rather than literal `.docket`/`origin/docket` spellings. The writer lock, transactions, and derived views keep working unchanged because the topology is the same. Cost: a private repo's metadata is only as durable and shareable as the chosen remote; the default local bare repo is single-machine.

## Alternatives considered

- Keep the metadata branch purely local with no remote: rejected, it loses the lease-push writer lock and would need a new local CAS mechanism.
- A `metadata_remote` config key: rejected, the `dckt` remote's URL already is the setting; a key would duplicate it.
- Special single-layer scoping for `visibility`: rejected, config keys must behave uniformly across layers.
