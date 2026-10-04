---
id: 523
slug: 'repository-check-reports-a-behind-only-docket-copy-as-diverg'
title: 'repository check reports a behind-only .docket copy as diverged after repair'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [511]
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

## What changes

Make `repository check` tell a behind-only local metadata copy apart from a truly diverged one, and give the behind case a remedy that fits it (re-run `repository prepare`, or fast-forward) instead of a human reconcile. Alternatively, have `repository repair` leave the local copy current after its push, so `check` sees nothing wrong. Grooming picks one. Once the fix lands, decide whether the upgrade guide's extra `prepare` step can go.

## Out of scope

Changing how a truly diverged `.docket` copy is handled. Other `repository check` findings.
