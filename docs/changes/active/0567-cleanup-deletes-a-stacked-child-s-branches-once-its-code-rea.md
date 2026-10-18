---
id: 567
slug: 'cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea'
title: 'Clean up stacked children when their stack lands, and finalize a child after its parent'
status: 'in-progress'
priority: 'high'
type: 'fix'
deliverable: 'pr'
created: '2026-10-17'
updated: '2026-10-18'
depends_on: []
stacked_on:
related: [562]
discovered_from: [562]
adrs: [92, 35]
spec: 'docs/superpowers/specs/2026-10-18-cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-10-18T23:44:55Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-18-cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea-design.md](../../superpowers/specs/2026-10-18-cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea-design.md) |
| ADRs | [ADR-0092](../../adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0035](../../adrs/0035-cleanup-teardown-fail-closed.md) |
<!-- docket:artifacts:end -->

## Why

Docket users stack changes in their own repositories, and finishing a stack is broken in two ways.

First, when a stack's root lands on main, none of its children get cleaned up: each child keeps its worktree folder, its local branch, and its remote branch forever. Finalize and the sweep clean up only the root. When cleanup does run for a child, it refuses, because the child's PR merged into its parent's branch rather than main. Even past that refusal, the child's folder looks like it belongs to someone else: docket compares the branch the folder was cut from (the parent's) with the base it now expects (main), and they no longer match. On top of that, the usual "is the merge on main" proof can't pass for a child, because docket's rebase merges put rewritten copies of the child's commits on main, not the same commits.

Second, the same folder comparison likely stops docket from finalizing a child whose parent landed on main first, after finalize moved the child's PR to main.

Found while grooming #562, which deliberately left stacked children alone.

## What changes

- A finished worktree folder is recognized as its change's own even when the branch it was cut from is no longer its base. A half-created folder (creation interrupted) still has to match.
- Cleanup proves a stacked child's work reached main through its stack, using the same check closeout already runs before marking the child done (shared, not copied). Then it removes the child's folder and both branches under the usual safety checks. If the proof fails, it keeps everything and says so, with no "delete it by hand" advice.
- Finalize and the sweep clean up each child the root carried right after the root itself.
- The sweep treats a stacked child's leftover branch like any other leftover, so leftovers already in users' repositories are cleaned on the first full sweep after upgrade.
- A child whose parent landed first can be finalized normally.

## Out of scope

How stacked children merge or become done, and the rule that a child's branches are kept until its root lands. Ordinary (unstacked) cleanup, including removing a clean folder when the branch is kept. Killed changes' branches and worktrees. The killed-parent policy.
