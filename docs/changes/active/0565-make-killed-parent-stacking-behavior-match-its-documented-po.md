---
id: 565
slug: 'make-killed-parent-stacking-behavior-match-its-documented-po'
title: 'Refuse a stack-parent kill that would strand built children'
status: 'in-progress'
priority: 'low'
type: 'fix'
deliverable: 'pr'
created: '2026-10-17'
updated: '2026-10-18'
depends_on: [568]
stacked_on:
related: [563, 568]
discovered_from: [563]
adrs: []
spec: 'docs/superpowers/specs/2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po-design.md'
plan: 'docs/superpowers/plans/2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po.md'
results: 'docs/results/2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/make-killed-parent-stacking-behavior-match-its-documented-po'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-18T18:51:09Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po-design.md](../../superpowers/specs/2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po-design.md) |
| Plan | [2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po.md](../../superpowers/plans/2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po.md) |
| Results | [2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po-results.md](../../results/2026-10-18-make-killed-parent-stacking-behavior-match-its-documented-po-results.md) |
<!-- docket:artifacts:end -->

## Why

The stacked-changes reference (*When a parent is killed*) says that killing a stack parent flips every open descendant, proposed included, to `blocked` with `blocked_by: stack parent #A killed — re-scope, re-parent, or kill`, and that stacked-merged descendants are blocked with their branches preserved. The code does not do this: `change.kill` calls `domain.Kill`, and `domain.KillStackParent` — which implements exactly that descendant blocking — is never called from the app layer. So a killed parent's descendants silently stay in their current status; a proposed child just shows `parent-killed` as its effective base and never becomes build-ready, with nothing telling a human it needs a decision. The same doc paragraph also says the flip is "a human-directed edit, never an automatic sweep", which contradicts the "flip to blocked" description above it. Found by the builder check while grooming change 563.

## What changes

Killing a change never changes its stacked descendants' status. `change.kill` instead refuses to kill a change that still has built stacked descendants, meaning ones that are `implemented` or `stacked-merged` and reached only through live changes. A kill would strand them with no way to finish or kill them. The refusal names those descendants, says to finish the change or pause it with `change.block`, and is printed in terminal output. On the board, a proposed child of a killed parent reads `stack parent #A killed — re-parent, un-stack, or kill` instead of `waiting on #A — stack base not built`. The unused `KillStackParent` blocking code is deleted, and the stacked-changes reference, the convention, and the glossary describe only this behavior, including each surviving descendant's typed exits. Builds after #568, which closes the deferral path and sets the refusal-message pattern this extends.

## Out of scope

How `change.groom` sets or clears `stacked_on`. How stacks resolve their effective base, and the readiness order. Exits for an `implemented` or `stacked-merged` child of an already-killed parent: the refusal stops that state from being created through docket. Widening `change.groom`, `change.kill`, or `change.unblock` to new source statuses. New skill prose.

## Reconcile log

### 2026-10-18

2026-10-18: #568 is done and landed on main (kill's applied HumanText now carries the branch-left-behind warning; lifecycle refusals print Detail["message"]). `KillStackParent`, `StackParentKilledReason`, `StackKillResult`, `DescendantOutcome` are still unused outside their tests; `changeKillOp.Plan` still calls `domain.Kill(c)`; the board's stack-base-unresolved arm still renders the single waiting cell. Spec holds unchanged; no scope adjustment.
