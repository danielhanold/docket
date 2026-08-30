---
id: 375
slug: gate-drive-start-not-idempotent-spawns-concurrent-drive
title: '`docket gate drive start` is not idempotent — a re-run spawns a second concurrent drive'
status: proposed
priority: medium
type: fix
created: 2026-08-30
updated: 2026-08-30
depends_on: []
stacked_on:
related: [376]
discovered_from: [372]
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

During the change-0372 build, `docket gate drive start` was invoked a second time in the same
worktree — to recover the `drive_id`/`generation` (owner-gen) that the first call's human-readable
output had omitted (see #376). Rather than returning the existing drive or refusing, the second
`start` launched a **second concurrent drive** in the same worktree. The double-load then aggravated
the known `test_go_race` internal/app parallel-load flake, reddening a run that a clean single drive
passes. `start` acting as a spawn on every call — instead of a get-or-create / refuse-if-live — is a
foot-gun: the natural recovery action (re-run the command) is exactly the action that breaks the run.

## What changes

Make `docket gate drive start` safe to invoke more than once against the same worktree: a second
`start` while a drive is already live should either return the existing drive's identity
(idempotent get-or-create) or refuse loudly without launching anything — never silently start a
second concurrent drive. Design the precise contract (idempotent vs. refuse, and how a genuinely
stale/abandoned drive is superseded) during brainstorm.

## Out of scope

- The missing owner-gen in `start`'s human-readable output — tracked separately as #376 (the trigger
  that made the operator re-run `start` in the first place).
- Any change to the `test_go_race` / internal/app parallel-load flake itself.

## Open questions

- Idempotent get-or-create, or hard refuse-if-live? What does the caller actually need back?
- How is a genuinely dead/abandoned prior drive detected and superseded rather than blocking forever?
- Should concurrent-drive detection live in `start`, or in a worktree-level lock the drive acquires?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
