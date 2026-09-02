---
id: 373
slug: 'harden-integration-race-test-isolation-under-parallel-load'
title: 'Harden integration/race test isolation under parallel load'
status: 'in-progress'
priority: high
type: 'chore'
created: '2026-08-30'
updated: '2026-09-02'
depends_on: []
stacked_on:
related: [381, 333, 280, 296]
discovered_from: [371, 397]
adrs: []
spec: 'docs/superpowers/specs/2026-09-02-harden-integration-race-test-isolation-under-parallel-load-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/harden-integration-race-test-isolation-under-parallel-load'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-02T16:15:44Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-02-harden-integration-race-test-isolation-under-parallel-load-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-02-harden-integration-race-test-isolation-under-parallel-load-design.md) |
<!-- docket:artifacts:end -->

## Why

During change 0371's build, three separate full-suite gate runs each reddened on a *different* unrelated test under parallel load, and each one serial-confirmed green in isolation:

1. a process-group liveness check,
2. the `internal/app` package, and
3. a `t.TempDir()` cleanup race in `internal/gitcli`.

These are flaky under the gate's parallel execution, not real defects — but each one halts an otherwise-green run and forces a serial re-confirmation by hand. As the suite grows, non-deterministic reds under parallel load erode trust in the gate and cost real time on every affected run.

Change 0397's finalize gate (2026-09-02, PR #269) added a fourth sighting of the same class: `internal/repository/transaction` `TestKeyedCommitCarriesFiveTrailers/keyed` — the assertions passed and only Go's `t.TempDir()` teardown failed (`directory not empty`) under `-j11` parallel load. This confirms the `t.TempDir()` cleanup race is not confined to `internal/gitcli`; the offender set spans at least two packages, which is evidence for a shared-resource root cause over three independent gaps (see the first open question).

## What changes

Settled design (2026-09-02 interactive grooming; detail in the linked spec):

- Bound total gate load at the runner: `internal/suiterunner` exports one concurrency cap into each target's sandbox, and the Go wrappers (the shared `tests/lib/go-integration-shard.sh` helper plus the three whole-module wrappers) translate it into `go test -p` / `GOMAXPROCS`, so concurrent Go test packages stay at a measured small multiple of the CPU count instead of `-j × NumCPU`. Affected budget rows are re-seeded from post-fix measurements.
- A shared real-process test fixture replaces bare `t.TempDir()` in every package whose tests spawn real git or supervisors: draining cleanup with bounded retry, git background work (auto gc, detached gc, auto maintenance, fsmonitor) disabled per fixture and in the runner sandbox, and a per-fixture process-registry root instead of the shared `$TMPDIR`.
- A fail-closed, mutation-tested repoguard proves no real-process package calls bare `t.TempDir()`. No wrapper gets a `serial` pin.
- Stub 381 is folded in; the reconcile pass kills it as a duplicate on claim.
- Evidence: five consecutive green full-gate runs at one head, the measured multiplier, re-seeded rows, and the five known sightings as regression cases.

## Out of scope

The 0371 and 0397 changes themselves (merged). Any product behavior change — this is test-infrastructure hardening only. The budget registry mechanism beyond re-seeding affected rows. Sharding over-budget files (280, 296). Changing what any test asserts.

