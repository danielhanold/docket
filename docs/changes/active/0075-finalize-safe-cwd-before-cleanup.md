---
id: 75
slug: finalize-safe-cwd-before-cleanup
title: Finalize from a durable checkout — don't run cleanup while CWD is the feature worktree
status: proposed
priority: medium
created: 2026-07-14
updated: 2026-07-14
depends_on: []
related: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

When `docket-finalize-change` runs while the agent's CWD / Cursor workspace is the feature worktree under `.worktrees/<slug>`, step 4 (`cleanup-feature-branch`) correctly removes that worktree — and thereby deletes the agent's working directory mid-run. Cleanup is already in finalize scope (terminal close-out + provenance guard under `.worktrees/` only); the gap is operational posture: finalize should perform merge/metadata/cleanup from a durable checkout, not from inside the feature worktree about to be removed.

## What changes

Ensure finalize's merge, metadata updates, and feature-branch cleanup run from a durable checkout — the primary integration checkout or the `.docket/` metadata worktree — so removing `.worktrees/<slug>` cannot yank the agent's CWD out from under the run.

## Out of scope

- Changing whether cleanup is part of finalize (it already is)
- Broadening cleanup beyond the `.worktrees/` provenance guard
- Redesigning the finalize consent / merge-authorization model

## Open questions

- Should the skill refuse to start (or self-`cd`) when CWD is under `.worktrees/`, or always switch to a known durable root before step 4?
- Preferred durable root: primary integration checkout vs `.docket/` metadata worktree?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
