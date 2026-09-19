---
id: 439
slug: 'leaked-worktree-gate-admission-slot-stuck-in-executing-block'
title: 'Leaked worktree gate-admission slot stuck in "executing" blocks finalize.rebase with a swallowed unavailable error'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-19'
updated: '2026-09-19'
depends_on: []
stacked_on:
related: [368, 435, 437]
discovered_from: []
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

During finalize of change 368 (PR #313, 2026-09-19), a prior finalize gate drive had launched a supervised suite run (scratchpad/gate-368/<id>, supervisor pid 50631) that ran the full suite and PASSED (52/52 files, exit 0), but its worktree admission slot (.git/docket/gate-admission/v1/<id>/record.json) leaked in state 'executing' with no drive doc persisted. reserveWorktreeExecution returns worktree-busy for an 'executing' slot without probing liveness, and mapDriveOutcome swallows that typed error into a generic 'unavailable', so finalize.rebase repeatedly returned blocked/gate-halted with halt_cause: unavailable and no actionable diagnosis. docket gate history cleanup --dry-run reported 0 blocking legacy drives (a false negative for this case). docket gate recover only marked the terminal run but left the slot 'executing'. The only thing that worked was docket gate stop <run-dir> against the already-terminal run, which proved teardown and released the slot to 'released'. This is a distinct failure mode from changes 437/435 (stale RunEpochID on a released slot after cancellation) — here the slot never reached 'released' at all despite the underlying run having already terminated successfully.

## What changes

Make a leaked 'executing' admission slot whose owning process/run has actually terminated (successfully or otherwise) detectable and recoverable without a human having to manually diagnose it via gate stop on a hunch. Candidates: probe liveness (not just presence) before returning worktree-busy for an 'executing' slot; stop swallowing the specific leaked-slot condition into a generic 'unavailable' halt_cause so the diagnostic points at the real cause; and/or make `docket gate history cleanup` detect this case instead of reporting 0 blocking drives.

## Out of scope

Redesigning the gate-admission slot lifecycle or state machine; changes 437/435's stale-RunEpochID-after-cancellation problem (separate, already tracked).
