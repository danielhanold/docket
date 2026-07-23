---
id: 119
slug: scope-the-metadata-worktree-git-commit-calls-to-the-paths-th
title: Scope the metadata-worktree git commit calls to the paths they own
status: proposed
priority: medium
created: 2026-07-21
updated: 2026-07-21
depends_on: []
related: []
discovered_from: [83]
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
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`scripts/docket-status.sh`'s `## Artifacts` commit calls a pathspec-less `git commit` inside the
**shared** `.docket` metadata worktree. That worktree is shared with concurrent autonomous loops, so
a pathspec-less commit sweeps up whatever another agent happens to have staged at that instant —
committing someone else's in-flight work under this run's message, on this run's push.

Change #0083 hit the same class from the other side (its CAS retry wedged the shared worktree by
`die`-ing mid-rebase) and adopted the stricter `git commit … -- <path>` idiom for its own new mark
path. The older call site was left as-is and noted in the #0083 results as "not filed, noted."

This is the `no-checkout-in-shared-worktree` / `cas-re-read-fresh-origin` family: operations in
`.docket` must scope themselves to the paths they own, because the tree is not exclusively theirs.

## What changes

- Audit `scripts/docket-status.sh` for pathspec-less `git commit` / `git add -A` style calls in the
  metadata worktree; scope each to the paths that call site owns.
- Sweep the other in-repo scripts that commit in `.docket` for the same shape.
- Add a guard keyed on the *shape* (a `git commit` in a metadata-worktree code path with no `--`
  pathspec), mutation-tested per `guards-are-code` — not an enumerated list of call sites.

## Out of scope

- The feature-branch commit paths, which run in per-change worktrees that are not shared.
- Any change to what the artifacts commit actually contains.

## Open questions

- Is a shape-keyed guard tractable here, or does the dynamic construction of these calls force a
  call-site-pinned audit instead?
- Are there call sites where a pathspec is genuinely impossible (an unknown-ahead-of-time file set)?
  If so, they need an explicit clean-tree precondition rather than a pathspec.
