---
id: 156
slug: 'a-stack-parent-carries-exactly-the-children-whose-prs-merged'
title: 'A stack parent carries exactly the children whose PRs merged into it'
status: 'Accepted'
date: '2026-10-20'
supersedes: []
reverses: []
relates_to: [92]
change: 571
---

## Context

Root closeout of a landed stack parent (DeriveRootCloseoutSet) required every descendant to be stacked-merged and refused with children-retarget-required otherwise, so a parent stayed implemented forever when a child was retargeted to main, unbuilt, killed, already done, or merged into the parent by hand without its own closeout running (which can never run once the parent branch is deleted). Rebase/publish/merge's carry proof (DeriveCarriedSet) already ignored non-merged children; closeout disagreed. Found while building #567.

## Decision

A parent carries exactly the children whose live PR merged into the parent's recorded branch: status stacked-merged, or implemented with its PR merged into that branch, walking down only through carried children. Each carried child's work is proven in Git on the integration branch (proveCarriedContent) before any write; root closeout takes carried children to done in one all-or-nothing transaction. No other child blocks the parent. One rule (DeriveCarriedSet; DeriveRootCloseoutSet is a thin wrapper keeping the root ancestry-cycle refusal) serves rebase, publish, merge, and closeout. A stacked-merged child with mismatched or unknown facts still refuses fail-closed. GitHub is queried only for implemented or stacked-merged descendants. Closeout's children-retarget-required disposition is retired (cleanup's same-named disposition stays).

## Consequences

Parents reach done; a by-hand-merged child closes with its parent even if the parent branch was deleted; rebase/publish/merge now also prove an implemented-but-merged child's work survives. Cost: an implemented child whose PR is not merged into the parent is the child's own closeout's job.

## Alternatives considered

Keep requiring all descendants stacked-merged: parents stay stuck forever. Require a human retarget or kill first: manual toil with no safety gain. Carry any merged implemented child regardless of destination: would claim work merged elsewhere.
