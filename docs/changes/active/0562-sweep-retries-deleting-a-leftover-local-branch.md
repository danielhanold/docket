---
id: 562
slug: 'sweep-retries-deleting-a-leftover-local-branch'
title: 'Sweep retries deleting a leftover local branch'
status: 'implemented'
priority: 'low'
type: 'fix'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [567, 390]
discovered_from: [559]
adrs: []
spec: 'docs/superpowers/specs/2026-10-17-sweep-retries-deleting-a-leftover-local-branch-design.md'
plan: 'docs/superpowers/plans/2026-10-17-sweep-retries-deleting-a-leftover-local-branch.md'
results: 'docs/results/2026-10-17-sweep-retries-deleting-a-leftover-local-branch-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/sweep-retries-deleting-a-leftover-local-branch'
pr: 'https://github.com/danielhanold/docket/pull/429'
blocked_by:
reconciled: true
claimed_at: '2026-10-17T22:30:06Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-17-sweep-retries-deleting-a-leftover-local-branch-design.md](../../superpowers/specs/2026-10-17-sweep-retries-deleting-a-leftover-local-branch-design.md) |
| Plan | [2026-10-17-sweep-retries-deleting-a-leftover-local-branch.md](../../superpowers/plans/2026-10-17-sweep-retries-deleting-a-leftover-local-branch.md) |
| Results | [2026-10-17-sweep-retries-deleting-a-leftover-local-branch-results.md](../../results/2026-10-17-sweep-retries-deleting-a-leftover-local-branch-results.md) |
<!-- docket:artifacts:end -->

## Why

When cleanup removes a done change's worktree but then fails to delete its local branch, the full maintenance sweep never tries again. Before re-running cleanup for an old done change, the sweep checks only whether some worktree still has the branch checked out, never whether the branch still exists. So a leftover branch with no worktree reads as "nothing to do" on every sweep. Change 559 found this for results-artifact changes, and PR changes have the same gap. The original sweep design (390) asked for the existence check, but only the remote half was built.

Separately, when cleanup keeps a branch on purpose because deleting it could lose work, its message names no branch and gives no next step, and the sweep's summary shows only counts, so nobody sees it.

## What changes

- The full sweep reads one list of local branches per run and re-runs the normal cleanup for any done change whose recorded branch still exists, for both PR and results-artifact changes. If that list can't be read, the branch's state is unknown, never clean.
- When cleanup keeps a branch on purpose (it moved off the merged commit, it has commits not on main, it is checked out elsewhere, or the remote copy moved after the merge), the warning names the exact branch and says to review it and delete it by hand, with the git command. If a live change now uses the same branch name, the warning drops the delete advice and says so.
- Those warnings appear in the human-readable output of both `finalize cleanup` and the sweep, not only in JSON. Temporary failures keep today's quiet retry wording.
- The docs say again that the sweep retries a leftover local branch.

A branch kept on purpose is retried on every full sweep, and its warning repeats until a human deletes it.

## Out of scope

Changing when closeout first attempts cleanup, or any cleanup safety check. Branches of killed or stacked-merged changes. Stacked children whose PR merged into their parent's branch: they keep today's behavior here, and #567 designs deleting their branches. The implementation-scope sweep (preflight).

## Reconcile log

### 2026-10-17

2026-10-17 — Reconciled against main a52c2d9c9 (results-artifact deliverable just landed via #559). The spec-cited symbols still exist as described: sweepAssessLocalRefLeg (worktree-only check), gatherSweepSharedFacts (remoteHeadsErr), sweepRunCleanup, finalizeCleanupLocalRefNoPR. #567 is still proposed, so carried descendants stay excluded as the spec says. No scope change.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: dc4705ace9941296b52055652303300544939f09
ran_at:   2026-10-18T14:49:07Z
<!-- docket:build-evidence:end -->
