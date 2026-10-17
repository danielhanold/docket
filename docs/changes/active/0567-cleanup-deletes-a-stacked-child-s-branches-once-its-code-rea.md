---
id: 567
slug: 'cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea'
title: 'Cleanup deletes a stacked child''s branches once its code reaches main'
status: 'proposed'
priority: 'low'
type: 'fix'
deliverable: 'pr'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [562]
discovered_from: [562]
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

In the normal stacked flow, a child's PR merges into its parent's branch. When the parent then lands on main, both changes become done, but cleanup never deletes the child's local or remote branch. Cleanup only deletes branches whose PR it can prove merged into the integration branch, so for the child it always refuses with `destination-mismatch` and keeps both branches. GitHub's "automatically delete head branches" setting is off for this repo, so the remote branch stays too. Nothing has failed, but every stacked child leaves two branches behind forever, and the sweep quietly re-tries the child's cleanup while its remote branch exists. Found while grooming #562, which deliberately leaves these records alone.

## What changes

Let cleanup delete a carried stacked child's local and remote feature branches once it can prove the child's code reached the integration branch through its parent (for example, the child's merged head or merge commit is contained in the integration tip after the root lands), under the same exact-tip and exact-lease safety rules as an ordinary done change. Once cleanup can do that, the sweep should treat a carried child's leftover branch like any other leftover.

## Out of scope

Ordinary (unstacked) done changes, which #562 covers. Killed changes' branches. Changing how stacked children merge or are promoted.
