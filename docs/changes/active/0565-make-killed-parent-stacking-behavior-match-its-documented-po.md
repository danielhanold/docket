---
id: 565
slug: 'make-killed-parent-stacking-behavior-match-its-documented-po'
title: 'Make killed-parent stacking behavior match its documented policy'
status: 'proposed'
priority: 'low'
type: 'fix'
deliverable: 'pr'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [563]
discovered_from: [563]
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

The stacked-changes reference (*When a parent is killed*) says that killing a stack parent flips every open descendant, proposed included, to `blocked` with `blocked_by: stack parent #A killed — re-scope, re-parent, or kill`, and that stacked-merged descendants are blocked with their branches preserved. The code does not do this: `change.kill` calls `domain.Kill`, and `domain.KillStackParent` — which implements exactly that descendant blocking — is never called from the app layer. So a killed parent's descendants silently stay in their current status; a proposed child just shows `parent-killed` as its effective base and never becomes build-ready, with nothing telling a human it needs a decision. The same doc paragraph also says the flip is "a human-directed edit, never an automatic sweep", which contradicts the "flip to blocked" description above it. Found by the builder check while grooming change 563.

## What changes

Decide which behavior is intended and make the code and the doc agree: either wire `KillStackParent` into the kill path (so killing a parent blocks its open descendants in the same transaction), or drop the blocking from the policy and document how a human finds and handles a killed parent's descendants (for a proposed descendant, change 563 adds re-parent / un-stack through `change.groom`). If descendants are blocked, decide how a human re-parents or un-stacks a blocked descendant, since `change.groom` only accepts proposed changes.

## Out of scope

How `change.groom` sets or clears `stacked_on` (change 563). How stacks resolve their effective base.
