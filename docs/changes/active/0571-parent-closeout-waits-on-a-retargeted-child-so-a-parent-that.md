---
id: 571
slug: 'parent-closeout-waits-on-a-retargeted-child-so-a-parent-that'
title: 'Close out a landed stack parent without waiting on children that never merged into it'
status: 'implemented'
priority: 'medium'
type: 'fix'
deliverable: 'pr'
created: '2026-10-19'
updated: '2026-10-20'
depends_on: []
stacked_on:
related: [567]
discovered_from: [567]
adrs: [92, 156]
spec: 'docs/superpowers/specs/2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that-design.md'
plan: 'docs/superpowers/plans/2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that.md'
results: 'docs/results/2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/parent-closeout-waits-on-a-retargeted-child-so-a-parent-that'
pr: 'https://github.com/danielhanold/docket/pull/437'
blocked_by:
reconciled: true
claimed_at: '2026-10-20T02:23:06Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that-design.md](../../superpowers/specs/2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that-design.md) |
| Plan | [2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that.md](../../superpowers/plans/2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that.md) |
| Results | [2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that-results.md](../../results/2026-10-20-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that-results.md) |
| ADRs | [ADR-0092](../../adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0156](../../adrs/0156-a-stack-parent-carries-exactly-the-children-whose-prs-merged.md) |
<!-- docket:artifacts:end -->

## Why

When a stacked parent's PR merges to main, its closeout refuses unless every change stacked on it has already merged into the parent's branch. So the parent stays `implemented` forever if any stacked child is in another state: its PR was moved to main before the parent landed, it is only groomed or still being built, it was killed, or it already landed on main by itself. The sweep retries the parent every run and gets the same refusal.

The merge step, and the check of what a parent carries that rebase, publish, and merge run, already ignore those children. Closeout is the one step that disagrees. Found while building #567 (PR #433): its end-to-end test for the parent-landed-first flow writes the parent's done record by hand to get past this.

## What changes

- A landed parent's closeout takes to done exactly the children whose PRs merged into it, down the chain, once each child's work is proven on main. No other child blocks it.
- A child whose PR merged into the parent's branch but whose own closeout never ran goes to done with the parent, after the same proof, instead of holding the parent until its own closeout runs (which can never happen once the parent's branch is deleted).
- Rebase, publish, merge, and closeout share one definition of what a parent carries.
- Closeout's `children-retarget-required` result goes away, with the docs and skill text that describe it.
- #567's end-to-end test uses the real parent closeout instead of a hand-written done record.

## Out of scope

How a child's PR is retargeted. Cleanup of carried children's branches and folders (#567). Letting implement-next's bounded closeout close out a stack ancestor to unblock a named stacked child. The killed-parent policy.

## Reconcile log

### 2026-10-20

Reconciled against origin/main 0ea3b6a86. The spec (groomed 2026-10-19) still matches the code: DeriveRootCloseoutSet, DeriveCarriedSet, CloseoutDispChildrenRetargetRequired, and archiveAsDoneOnOrigin are all present as described. #567 is done. No scope change.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: 7c4f0dc7d77eb41b56675c7180698c89ed3a9e32
ran_at:   2026-10-20T03:07:44Z
<!-- docket:build-evidence:end -->
