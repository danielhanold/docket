---
id: 486
slug: 'run-plan-mutation-checks-outside-a-gate-drive-not-by-editing'
title: 'Run plan mutation checks outside a gate drive, not by editing the tree under it'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-10-01'
updated: '2026-10-01'
depends_on: []
stacked_on:
related: [479]
discovered_from: [479]
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

Change 0479's plan has Task 1's mutation checks edit internal/testsupport/unfiltered.go in place while a gate drive is running. The gate driver halts any drive whose worktree changes mid-run (`worktree-changed`), and the scope then refuses every further start (`predecessor-not-reusable` / `scope-second-live-drive`). The Task 1 worker returned BLOCKED, halting the whole run (run a2572af3c302e0122465fb8658349e54), and Task 2's wiring mutation would hit the same wall. The guard rule 'a guard is code: mutation-test it' is correct; the way plans prescribe it is incompatible with the driver's worktree-stability contract. Any plan that mutation-tests a guard will repeat this.

## What changes

Make the plan-writing and build guidance prescribe mutation checks that never edit the live feature worktree while a drive runs: run them before or between drives (not inside one), or against a throwaway copy of the tree, restoring byte-identical and verifying the restore. Trace first how docket-plan-writer, docket-build-task and the conventions currently word mutation checks, and extend that existing wording rather than adding a new mechanism. Decide, from the trace, whether the driver should also offer a sanctioned way to recover a scope halted by `worktree-changed` without a human `change.resume-halted --acknowledge-quiescent`, or whether prevention in the plan guidance is enough (YAGNI: prefer prevention).

## Out of scope

Changing the gate driver's worktree-changed detection itself. Reworking change 0479's own mutation checks beyond what its resume needs (handled when 0479 is resumed). Any change to the AGENTS.md mutation-testing rule beyond clarifying how to run it.
