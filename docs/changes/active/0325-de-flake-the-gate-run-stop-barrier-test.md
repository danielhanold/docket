---
id: 325
slug: de-flake-the-gate-run-stop-barrier-test
title: 'De-flake the gate-run --stop barrier test'
status: proposed
priority: medium
type: fix
created: 2026-08-17
updated: 2026-08-17
depends_on: []
stacked_on:
related: []
discovered_from: [312]
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

**Trigger** — surfaced at change 0312's merge gate (PR #214). `tests/test_gate_run_stop.sh` — a timing-sensitive `--stop` barrier test of `scripts/gate-run.sh`, code change 0312 never touched — reddened under a saturated parallel suite run and was green both in isolation and in the final `-j 3` run.

**Opportunity** — the test asserts a barrier outcome by racing wall clock against the runner's own scheduling, so its verdict depends on how hard the machine is contending with itself rather than on the behavior under test. Rewriting it to synchronize on an observable marker (a file, a fd, a process state) instead of elapsed time would make it a real gate at any parallelism.

**Independent value** — `gate-run.sh` and this test both predate 0312 and are untouched by it; the flake stands with 0312 reverted, and every future merge gate pays for it as an intermittent red that costs a differential re-run to disprove.

**Boundary** — the `--stop` barrier test and whatever minimal observability hook `gate-run.sh` needs to be synchronized on deterministically. It does not change `gate-run.sh`'s behavior, does not touch the per-file wall-clock budget table, and does not attempt a general de-flaking pass over other timing-sensitive tests.

**Reason for deferral** — 0312 is a Go-side planning-mutations slice with no relationship to the gate runner; fixing an unrelated baseline shell test on that branch would expand its scope and put an untested-by-that-change area into its diff.
