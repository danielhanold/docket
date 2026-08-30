---
id: 373
slug: 'harden-integration-race-test-isolation-under-parallel-load'
title: 'Harden integration/race test isolation under parallel load'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-08-30'
updated: '2026-08-30'
depends_on: []
stacked_on:
related: []
discovered_from: [371]
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

During change 0371's build, three separate full-suite gate runs each reddened on a *different* unrelated test under parallel load, and each one serial-confirmed green in isolation:

1. a process-group liveness check,
2. the `internal/app` package, and
3. a `t.TempDir()` cleanup race in `internal/gitcli`.

These are flaky under the gate's parallel execution, not real defects — but each one halts an otherwise-green run and forces a serial re-confirmation by hand. As the suite grows, non-deterministic reds under parallel load erode trust in the gate and cost real time on every affected run.

## What changes

Investigate and harden integration/race test isolation so the gate suite is deterministic under parallel execution — no unrelated test reds purely as an artifact of concurrent scheduling. Scope of the fix (per-test isolation, resource sandboxing, reduced shared state, or selective serialization of the offenders) is to be settled at brainstorm time.

## Out of scope

The 0371 change itself (already merged/implemented). Any product behavior change — this is test-infrastructure hardening only.

## Open questions

- Are the three observed offenders symptoms of one shared-resource root cause or three independent isolation gaps?
- Should the fix be per-test isolation, or should specific tests be marked serial at the gate?
