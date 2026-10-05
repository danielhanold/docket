---
id: 523
slug: 'repository-check-reports-a-behind-only-docket-copy-as-diverg'
title: 'repository check reports a behind-only .docket copy as diverged after repair'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [366, 511]
discovered_from: [511]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

After `docket repository repair --yes` pushes its fix, the local `.docket` metadata worktree is only behind `origin/docket`; it has no commits of its own. `docket repository check` still reports it as diverged and tells the user to reconcile with a human. A plain fast-forward would fix it, so the message sends the user to a manual reconcile that isn't needed. Change 511 found this while proving the Bash-to-Go upgrade guide. `docs/release/upgrading-from-bash.md` works around it by running `repository prepare` again right after `repair`.

Repair is not the only trigger. During the alpha.1 acceptance (change 0366) the same state appeared twice after `docket-finalize-change` closed out a change. It happened in the fixture repository and again in this repository: the local `.docket` copy was clean and one commit behind (`0 ahead, 1 behind`). Both times `check` reported `repository_state: conflict` with two errors, `metadata-worktree-dirty` and `local-metadata-diverged`. "Dirty" is wrong too, since the worktree had no changes. `repository prepare` fast-forwarded it and `check` went healthy. Any typed operation that pushes to `docket` without advancing the persistent `.docket` worktree can leave this state behind.

## What changes

Make `repository check` tell a behind-only local metadata copy apart from a truly diverged or dirty one. Give the behind case a remedy that fits it (re-run `repository prepare`, or fast-forward) instead of a human reconcile. It should also stop raising `metadata-worktree-dirty` for a clean worktree that is only behind.

Making `repository repair` leave the local copy current after its push would fix only the repair trigger, not the finalize one. Grooming should weigh that, or make every pushing operation leave the local copy current. Once the fix lands, decide whether the upgrade guide's extra `prepare` step can go.

## Out of scope

Changing how a truly diverged `.docket` copy is handled. Other `repository check` findings.
