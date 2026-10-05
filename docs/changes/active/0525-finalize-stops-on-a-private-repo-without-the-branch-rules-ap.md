---
id: 525
slug: 'finalize-stops-on-a-private-repo-without-the-branch-rules-ap'
title: 'Finalize stops on a private repo without the branch-rules API, and leaves half-removed workspaces'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [366, 316, 483]
discovered_from: [366]
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

The alpha.1 acceptance (change 0366) hit two finalize failures.

1. **Branch-rules API refusal.** The fixture was a private repository on a GitHub plan that doesn't serve the branch-rules API. Finalize's branch-protection check got an error back and stopped before merging. Its only remedies were: make the repository public, upgrade the plan, or merge on GitHub by hand and re-run finalize. A first-time user with a private repository on a free plan would hit the same stop. Docket's direction is that new checks default to visibility-only rather than blocking. Here, an unanswerable probe blocks the merge outright.
2. **Half-finished cleanup that can't finish.** On this repository, `finalize cleanup` for 0366 returned `pending` / `workspace-blocked`. The workspace directory held an untracked `.DS_Store`, which Finder writes when a folder is browsed. Cleanup had already removed Git's worktree registration and most of the files. It left 36 files and a `.git` pointer to a registration that no longer existed. Re-running cleanup returned `workspace-blocked` again, because the remnant can never look like a clean checkout. The directory and the local and remote branches had to be deleted by hand.

## What changes

- **Branch-rules probe.** When the branch-rules or protection probe can't be answered (not available on the plan, or forbidden), finalize should not stop. It reports the probe as unknown, visibly in its result and the closeout notes, and proceeds with the merge. A probe that answers and actually forbids the merge still stops. Grooming settles exactly which HTTP and API responses count as "unanswerable".
- **Cleanup.** Decide what cleanup does with untracked, ignorable OS files such as `.DS_Store`, either treating them as clean or removing them. Cleanup should also not end up half-done: either check before removing anything, or recognize and finish its own remnant on a re-run.

## Out of scope

Other finalize gates. Changing what a real branch-protection rule allows. Workspaces with uncommitted tracked changes, which must still block.
