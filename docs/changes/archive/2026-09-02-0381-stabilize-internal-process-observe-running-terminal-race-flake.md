---
id: 381
slug: 'stabilize-internal-process-observe-running-terminal-race-flake'
title: 'Stabilize internal/process TestObserveRunningThenTerminal parallel-load -race flake'
status: 'killed'
priority: medium
type: fix
created: '2026-08-30'
updated: '2026-09-02'
depends_on: []
stacked_on:
related: []
discovered_from: [378, 397]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`internal/process` `TestObserveRunningThenTerminal` is green in isolation but reddens under full
parallel suite load with `-race`, hitting a 30s ceiling. It surfaced twice during change 0378's
build/review gates and cleared on re-run each time, so it is a wall-clock/scheduling flake under
contention rather than a defect in 0378 (whose packages it does not touch). An intermittently red
test at the build gate erodes trust in the gate and forces manual serial re-confirmation, so it is
worth stabilizing on its own.

## What changes

Root-cause the parallel-load timing sensitivity (contention on the observed process/terminal
transition under `-race`) and make the test deterministic under full-suite parallelism — by
tightening synchronization, relaxing a too-tight wall-clock ceiling, or isolating the test's
timing assumptions — without weakening what it verifies.

## Out of scope

- The 0378 metadata-ownership follow-ups (SHA-256 width fix; descendant-receipt fixture).
- Broader suite parallelism/timeout retuning beyond this one test (cf. the retired internal/app
  timeout work) unless reconcile finds a shared root cause.

## Open questions

- Is the flake a genuine data-race the `-race` detector is catching, or purely a wall-clock timeout
  under scheduling contention? Reconcile should confirm before choosing the fix shape.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

## Why killed

Duplicate of change 373 (Harden integration/race test isolation under parallel load). 381's `internal/process` `TestObserveRunningThenTerminal` parallel-load `-race` flake is sighting 1 in 373's problem set, and 373's open question about a shared root cause resolves to yes: the flake is covered by 373's runner-level load bound (section 1) and shared real-process fixture (section 2). Folded into 373 and killed here during 373's reconcile pass so the fix is structural, not a per-test workaround.
