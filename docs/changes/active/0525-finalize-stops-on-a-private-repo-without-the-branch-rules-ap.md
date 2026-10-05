---
id: 525
slug: 'finalize-stops-on-a-private-repo-without-the-branch-rules-ap'
title: 'Finalize stops on a private repo without the branch-rules API, and leaves half-removed workspaces'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [366, 316, 483, 336]
discovered_from: [366]
adrs: [35]
spec: 'docs/superpowers/specs/2026-10-05-finalize-stops-on-a-private-repo-without-the-branch-rules-ap-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/finalize-stops-on-a-private-repo-without-the-branch-rules-ap'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-05T10:42:14Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-finalize-stops-on-a-private-repo-without-the-branch-rules-ap-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-05-finalize-stops-on-a-private-repo-without-the-branch-rules-ap-design.md) |
| ADRs | [ADR-0035](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0035-cleanup-teardown-fail-closed.md) |
<!-- docket:artifacts:end -->

## Why

The alpha.1 acceptance (change 0366) hit two finalize failures. Tracing showed both had a different cause than first reported.

1. **The merge-method picker stops on a plan without branch rules.** Before merging, finalize reads the repository's allowed merge methods and the base branch's rules to pick rebase, merge commit, or squash (change 0336). On a private repository whose GitHub plan has no branch rules, the rules request returns HTTP 403, "Upgrade to GitHub Pro or make this repository public to enable this feature." 0336 counts any failed read as unknown, so finalize stopped and the human merged by hand. That 403 is really an answer: no rule can restrict the merge on that plan.
2. **A removal Git started is never finished.** `git worktree remove` passed its own clean check, then failed partway through deleting the folder. Finder writing a `.DS_Store` into it during the delete is the likely cause. Git still removed its registration. Docket read the error as "Git refused" and left its manifest at ready, so every re-run found an unregistered path and stopped as `workspace-blocked`. Because the workspace step never finished, both feature branches were kept too. The folder and both branches had to be deleted by hand. `.DS_Store` itself was gitignored and was not the blocker.

A blocked cleanup also doesn't say what blocked it, which made this hard to diagnose.

## What changes

- **Merge picker.** Treat GitHub's plan-gate 403 as "this branch has no rules": pick the method from the repository settings, merge, and record a visible `branch-rules-unavailable` note in the merge result and the closeout notes. Every other failed rules read still stops as unknown.
- **Cleanup.** When `git worktree remove` fails after Git has already removed its registration, finish deleting the leftover folder, mark the workspace cleaned, and delete the branches as usual. If the folder still can't be deleted, report its path in a `workspace-remnant` warning instead of blocking. A new ADR records this narrow exception to "never delete a folder by path", related to ADR-0035.
- **Visibility.** The `workspace-blocked` warning names the paths or reasons that blocked it.

## Out of scope

Other finalize gates, and what a real branch rule allows. Any other unreadable branch-rules response. Ignoring or deleting OS files such as `.DS_Store`. A durable "removing" manifest phase. Workspaces with uncommitted tracked changes, which must still block.

## Reconcile log

### 2026-10-05

Reconciled against origin/main f5fef87be. probeBranchMergeRules (internal/githubcli/mergemethod.go), cleanupReady/RemoveWorktreeClean and CleanupResult.BlockedBy (internal/workspace/cleanup.go), and finalizeCleanupWorkspace's workspace-blocked finding (internal/app/finalize_cleanup.go) are unchanged since grooming; related changes 366/316/336 are done and 483 killed, none of which altered this ground. Scope and spec stand as written.
