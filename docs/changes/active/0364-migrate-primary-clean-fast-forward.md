---
id: 364
slug: migrate-primary-clean-fast-forward
title: 'Advance the primary in place on migrate via a gitcli clean-fast-forward primitive'
status: 'in-progress'
priority: medium
type: feat
created: 2026-08-28
updated: '2026-08-30'
depends_on: [352]
stacked_on:
related: [352]
discovered_from: [352]
adrs: []
spec: docs/superpowers/specs/2026-08-28-migrate-primary-clean-fast-forward-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'feat/migrate-primary-clean-fast-forward'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-01T10:32:16Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-migrate-primary-clean-fast-forward-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-migrate-primary-clean-fast-forward-design.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository migrate` (change 0352) publishes the metadata and integration tips to the remote
durably, then finishes the local side. Its final step — bringing the primary worktree in line with
the migrated integration branch — is deliberately **not** performed in place. Instead it returns a
`pending_local` remedy string telling the human to run
`git -C <primary> merge --ff-only <remote>/<branch>` themselves, and marks the migration
`needs-review`.

The code says why in its own doc comment: a clean-fast-forward working-tree primitive is
intentionally outside the migrate service's Git surface, because `internal/gitcli` has no such
primitive and change 0352 added none. On the happy path — primary still parked at the pinned source
commit, working tree clean — that hand-off is pure friction: the advance is a guaranteed clean
fast-forward migrate could perform itself. Surfaced as a recorded follow-up when 0352 landed
(PR #249).

## What changes

- Add a clean-fast-forward primitive to `internal/gitcli` that advances a worktree's checked-out
  branch to a target ref via `git -C <worktree> merge --ff-only <target>`, refusing (without
  mutating anything) both a non-fast-forward and a dirty working tree. `IsAncestor` already exists as
  the ancestry building block.
- Wire migrate's local-finish happy path (primary still at the pinned source) to attempt the
  in-place advance; on success emit **no** primary-sync `pending_local` line, so a routine migration
  reports `healthy`.
- On any refusal or error, fall back to exactly today's remedy string — behavior is unchanged in
  every case the advance can't be performed cleanly.

## Out of scope

- The `moved` (reconcile) branch — a primary that moved past the pinned source stays a
  `git pull --rebase` remedy string, untouched.
- Any caller other than migrate; `init` and `check` are unchanged.
- Auto-stashing/committing or otherwise resolving a dirty tree — a dirty tree is a refusal, never a
  mutation of the user's uncommitted work.
- Rolling back or re-erroring the migration on a failed local advance: the remote migration is
  already durable and is never undone by a local-finish failure.

## Open questions

- Exact method name, and whether the dirty-vs-non-fast-forward distinction is worth an explicit
  `IsAncestor` pre-check for a more precise error kind, or whether one refusal kind suffices. Either
  satisfies the contract; the implementer picks per the package's conventions.

## Reconcile log

### 2026-08-30

### 2026-08-30

Verified against current `main` after claiming:

- Dependency #352 is archived as `done`, and #363 is also done; the repository migration service and the single metadata topology it consumes are both present in the current tree.
- The proposed `internal/gitcli` primitive is still absent, while `IsAncestor`, the existing typed failure vocabulary, and the integration-test partition are available for the implementation.
- `migrateLocalFinish` still calls `migratePrimarySyncRemedy`; the not-moved path still emits the manual `git -C <primary> merge --ff-only <remote>/<branch>` remedy, and the moved path still emits the `pull --rebase` remedy. No other change has absorbed this work.

The scope, linked spec, dependency, related, and discovery relations remain accurate. No proposal or spec-section edits are required.

