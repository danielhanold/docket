---
id: 500
slug: 'committed-ignore-invalid-hint-for-an-absent-gitignore-names'
title: 'committed-ignore-invalid remedies print the paste-ready managed block'
status: 'in-progress'
priority: 'low'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [418, 352, 57]
discovered_from: [496]
adrs: []
spec: 'docs/superpowers/specs/2026-10-04-committed-ignore-invalid-hint-for-an-absent-gitignore-names-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/committed-ignore-invalid-hint-for-an-absent-gitignore-names'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-04T06:04:56Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-committed-ignore-invalid-hint-for-an-absent-gitignore-names-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-committed-ignore-invalid-hint-for-an-absent-gitignore-names-design.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository check` reports `committed-ignore-invalid` when the committed `.gitignore` on the integration branch lacks docket's managed ignore block. When the file is missing entirely, the remedy says to re-run `docket repository migrate`. Since change 0496, `migrate` is a no-op on a migrated repository and points at `docket repository repair`, which does not write `.gitignore` either. A human following the hint runs two commands that do nothing. No command writes the block on a migrated repository, so the remedy that works is the by-hand one.

The by-hand remedies share a second gap. Every `committed-ignore-invalid` variant tells the human to write "the managed block" or "the canonical representation", but none shows it. The marker lines have an exact spelling nobody can guess, and the block is documented only in docket's own source and ADR-0020.

The 0496 build surfaced this. It retargeted every mechanically-repairable remedy to `repository repair`, but its guard only covers repairable findings, so this remedy was never checked.

## What changes

Every `committed-ignore-invalid` remedy ends with the exact managed block, ready to paste: markers and all entries, taken from the same canonical source that `init` writes and `check` validates. Each case keeps its own instruction (add the file, append, replace the legacy block, fix the markers first, restore missing entries, rewrite). The missing-file remedy stops naming `migrate`.

A guard covers every variant. Each remedy carries the canonical block on its own lines, and none names `repository migrate`. `repository check`'s text output prints the block flush left so it pastes cleanly.

## Out of scope

Teaching `repository repair` or `repository init` to write `.gitignore`. Neither can write the integration working tree on a migrated repository, and printing the block is the cheapest remedy that works in every state.

The other `migrate` remedies in health.go (`local-metadata-missing`, `docket-worktree-missing`, the legacy and half-migrated findings) stay, because they remain valid. Also unchanged: `committed-ignore-unverified` (an unreadable blob), `status`'s indented rendering of findings on the legacy refusal path, and documenting the block in user-facing docs.

## Reconcile log

### 2026-10-04

Traced against main 20bc0a36a: `committedIgnoreFinding` in internal/reposetup/health.go still carries the seven remedies the spec describes, the FileAbsent one still names `docket repository migrate`, and `IgnoreDefect` still ends at `IgnoreDefectUnreadable`. `GitignoreBlock()` is unchanged. No related change altered this surface since grooming; scope stands as specified.
