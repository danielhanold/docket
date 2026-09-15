---
id: 429
slug: 'fix-stacked-change-validation-after-a-manual-parent-rebase'
title: 'Fix stacked-change validation after a manual parent rebase'
status: 'implemented'
priority: 'high'
type: 'fix'
created: '2026-09-15'
updated: '2026-09-15'
depends_on: []
stacked_on:
related: [298, 316, 327]
discovered_from: []
adrs: [92]
spec: 'docs/superpowers/specs/2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase-design.md'
plan: 'docs/superpowers/plans/2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase.md'
results: 'docs/results/2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/fix-stacked-change-validation-after-a-manual-parent-rebase'
pr: 'https://github.com/danielhanold/docket/pull/304'
blocked_by:
reconciled: true
claimed_at: '2026-09-15T20:40:59Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase-design.md) |
| Plan | [2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase.md](https://github.com/danielhanold/docket/blob/fix/fix-stacked-change-validation-after-a-manual-parent-rebase/docs/superpowers/plans/2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase.md) |
| Results | [2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase-results.md](https://github.com/danielhanold/docket/blob/fix/fix-stacked-change-validation-after-a-manual-parent-rebase/docs/results/2026-09-15-fix-stacked-change-validation-after-a-manual-parent-rebase-results.md) |
| ADRs | [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md) |
<!-- docket:artifacts:end -->

## Why

Manually rebasing a branch that carries stacked changes onto the integration branch can leave valid stacks unable to finalize. Historical parent and child commit IDs change, but Docket continues treating those old IDs as required evidence.

Reported instance 1: a stacked PR was rebased or updated onto a newer parent. Its branch existed locally and remotely, and GitHub considered the PR mergeable with green CI. Local finalize nevertheless reported branch-missing because the workspace still recorded its original parent tip, which was no longer an ancestor of the current PR head. The recorded creation base is not a durable branch-identity test after a legitimate rebase. The implemented change had no usable path to reconcile that stale workspace binding and finish locally.

Reported instance 2: feature/eks-consumers was rebased onto master after children 0001–0003 and 0005–0011 had merged into the stack. Those historical GitHub merge-result SHAs were no longer ancestors of the stack tip. The exact-blob fallback also failed because later children legitimately evolved shared files, including Chart.yaml, chart tests, values, and helpers. The preservation check blocked publish, merge, and root archive, leaving children stacked-merged with their remote branches retained. Attaching the old SHAs with synthetic merge commits is not an acceptable fix: that repository permits only rebase-and-merge.

These are downstream incident IDs, not references to this repository's change IDs. Change 0327 supplies the current child-preservation guard; this fix addresses its false refusal after a legitimate parent rebase without undoing its protection against lost child work.

## What changes

Correct the two checks that falsely reject an otherwise valid stack after its parent is manually rebased.

- For an already-created, owned workspace, validate its current manifest/registration/branch/head identity without requiring its original creation base to remain ancestral. Apply that same correction to inspection, publication, and otherwise-eligible cleanup. Keep unfinished-allocation ancestry protection and all PR/head/lease checks.
- Extend the existing child-preservation check to accept the child's complete exact recorded delta at one commit reachable from the pinned target, so later stacked children may evolve shared files without invalidating historical inclusion. Retain original-ancestry and exact-at-tip proofs. Never assemble a proof from entries spread across different commits.
- Reuse the corrected primitive at all existing rebase, publish, merge, and stacked/root-closeout gates. Keep the lifecycle, root-merge target, atomic archive, and branch-retention policy unchanged.
- Prove both reported regression shapes and continued refusal of dropped child work, wrong/stale workspace identity, unrelated history, and incomplete observations.

The linked spec defines the minimal algorithm and tests. Support is limited to rewrites retaining an exact matching snapshot; arbitrary conflict resolutions or squashes that erase all such snapshots remain unproven.

## Out of scope

- Automatic restacking, generic recovery/rebinding, remote/local synchronization, or downstream incident repair.
- Force/skip-proof overrides, metadata-only delivery claims, new configuration, new persistence, or proof infrastructure.
- Semantic equivalence or automatic acceptance of every squash/conflict resolution.
- Synthetic merge commits, merge-policy changes, broader cleanup or branch-retention changes, and unrelated refactoring.
- Rewriting frozen historical records or implementing the fix during grooming.

## Reconcile log

### 2026-09-15

2026-09-15: Reconciled against current main. Confirmed the two target checks still exist as the spec describes: ProvePreserved lives in internal/gitcli/preservecommit.go (with callers in internal/app/finalize_preservation.go and finalize_closeout.go), and the ready-workspace original-base ancestry check lives in internal/app/workspace_ops.go (mirrored in inspection, publication, and cleanup verification). Change 0327's child-preservation guard is the current guard to extend without weakening. No scope change: fix is well-bounded and depends on nothing unmerged. Related 0298/0316/0327 and ADR-0092 remain accurate. Proceeding to build as specified.
