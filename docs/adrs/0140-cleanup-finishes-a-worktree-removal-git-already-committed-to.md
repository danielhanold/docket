---
id: 140
slug: 'cleanup-finishes-a-worktree-removal-git-already-committed-to'
title: 'Cleanup finishes a worktree removal Git already committed to'
status: 'Accepted'
date: '2026-10-05'
supersedes: []
reverses: []
relates_to: [35]
change: 525
---

## Context

Finalize cleanup removes a ready feature worktree with a non-forcing `git worktree remove`. Git runs its own clean check, then deletes the tree; even if that delete fails (for example Finder writing `.DS_Store` mid-delete), Git still deletes the worktree's admin directory (its registration) and exits non-zero. Docket classified that exit as "git refused", left the manifest `ready`, and every re-run then blocked on a path that was no longer registered (seen in change 0366). The feature branches were retained too, so cleanup stayed stuck until a human hand-deleted the directory.

## Decision

When `RemoveWorktreeClean` fails with `KindCommandFailed`, Docket re-lists the worktrees and decides from what Git actually did:

- Listing error: the cleanup reports `failed`.
- Path still registered: Git refused before deleting anything. The cleanup is `blocked` and the tree is left untouched.
- Path no longer registered: Git passed its clean check and began deleting. Docket finishes the removal with `os.RemoveAll` on the manifest's recorded path (already proven by `ownsManifest` to be `<primary>/.worktrees/<slug>`), advances the manifest to `cleaned`, and returns `cleaned`. If `RemoveAll` itself fails, Docket still marks the manifest `cleaned` and reports the leftover path as a workspace-remnant warning.

This is the one case where Docket deletes a directory by path. It is a narrow exception to ADR-0035's fail-closed, never-half-destructive teardown: Docket only completes a destruction Git already committed to under the same clean proof, never starts one.

## Consequences

A removal interrupted by a stray OS write no longer needs hand cleanup, and the change's branches are cleaned in the same run. Known, accepted limit: if the Docket process dies between Git's failed removal and Docket's finish, the manifest stays `ready` with an unregistered path, which is today's stuck state and still needs a hand delete. No durable "removing" manifest phase is added to cover that window.

## Alternatives considered

- A durable `removing` manifest phase written before calling Git: rejected; the crash window it closes is too small to justify a new lifecycle state.
- Ignoring or deleting OS files such as `.DS_Store` before removal: rejected; no special-casing of particular filenames.
- Keep blocking, with a better message only: rejected; it leaves cleanup permanently stuck on a path Git has already unregistered.
