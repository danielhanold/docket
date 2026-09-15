---
id: 429
slug: 'fix-stacked-change-validation-after-a-manual-parent-rebase'
title: 'Fix stacked-change validation after a manual parent rebase'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-15'
updated: '2026-09-15'
depends_on: []
stacked_on:
related: [298, 316, 327]
discovered_from: []
adrs: [92]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md) |
<!-- docket:artifacts:end -->

## Why

Manually rebasing a branch that carries stacked changes onto the integration branch can leave valid stacks unable to finalize. Historical parent and child commit IDs change, but Docket continues treating those old IDs as required evidence.

Reported instance 1: a stacked PR was rebased or updated onto a newer parent. Its branch existed locally and remotely, and GitHub considered the PR mergeable with green CI. Local finalize nevertheless reported branch-missing because the workspace still recorded its original parent tip, which was no longer an ancestor of the current PR head. The recorded creation base is not a durable branch-identity test after a legitimate rebase. The implemented change had no usable path to reconcile that stale workspace binding and finish locally.

Reported instance 2: feature/eks-consumers was rebased onto master after children 0001–0003 and 0005–0011 had merged into the stack. Those historical GitHub merge-result SHAs were no longer ancestors of the stack tip. The exact-blob fallback also failed because later children legitimately evolved shared files, including Chart.yaml, chart tests, values, and helpers. The preservation check blocked publish, merge, and root archive, leaving children stacked-merged with their remote branches retained. Attaching the old SHAs with synthetic merge commits is not an acceptable fix: that repository permits only rebase-and-merge.

These are downstream incident IDs, not references to this repository's change IDs. Change 0327 supplies the current child-preservation guard; this fix addresses its false refusal after a legitimate parent rebase without undoing its protection against lost child work.

## What changes

Correct only the validation that falsely blocks existing stacked changes after a manual parent rebase onto the configured integration branch (main or master), including a subsequent child PR rebase/update onto the refreshed parent.

- Validate the recorded branch and current PR/workspace identity without requiring the original workspace base commit to remain an ancestor forever. Make the existing local finalize path usable for affected implemented changes; report a present-but-stale or mismatched workspace accurately instead of calling its branch missing.
- Let stack-preservation validation recognize retained child work across the reported history rewrite when later stacked children have legitimately modified the same files. Historical merge-SHA ancestry and equality with each child's old whole-file blobs must not be the only successful proof for this case.
- Apply the same narrowly scoped correction at the existing enforcement points for rebase, publish, merge, and root closeout. Successful closeout should use the existing lifecycle and cleanup behavior; an obsolete ancestry requirement alone must not strand children in stacked-merged.
- Add reproducible Git regression fixtures for both incidents, including overlapping child edits and a repository that permits only rebase-and-merge. Verify that local finalize can progress against the current parent and that the carried stack can reach normal root closeout without synthetic merge commits or hand-edited metadata.
- Preserve negative coverage for a wrong branch/worktree, stale or mismatched PR head, actually dropped child work, and unresolved Git evidence. A green CI or merged PR metadata alone does not establish preservation.

Design constraint: select the smallest demonstrable correction in the existing workspace and preservation checks. Establish how the reported retained-history case is proven before treating this proposal as build-ready; do not claim arbitrary semantic equivalence of rewritten code.

## Out of scope

- General stack management, automatic restacking, arbitrary history repair, or a new recovery framework.
- Disabling preservation checks, accepting delivery from metadata/CI alone, or adding a force/skip-proof override.
- Synthetic merge commits, rewriting user history as a workaround, or changing repository merge policy.
- Generic semantic-equivalence detection, configurable proof strategies, new infrastructure for hypothetical rewrite cases, or unrelated refactoring.
- Broad workspace rebinding/cleanup redesign, reopening historical done records, or repairing either downstream incident as part of this change.
- Implementing the fix in this capture task.
