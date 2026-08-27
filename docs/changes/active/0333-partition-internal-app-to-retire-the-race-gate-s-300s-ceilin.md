---
id: 333
slug: partition-internal-app-to-retire-the-race-gate-s-300s-ceilin
title: 'Partition slow Go integration tests and retire the race gate''s 300s ceiling exemption'
status: 'in-progress'
priority: high
type: refactor
created: 2026-08-20
updated: '2026-08-27'
depends_on: []
stacked_on:
related: [251, 273, 280, 357]
discovered_from: [332]
adrs: []
spec: docs/superpowers/specs/2026-08-27-partition-slow-go-integration-tests-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'refactor/partition-internal-app-to-retire-the-race-gate-s-300s-ceilin'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-27T01:20:23Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-27-partition-slow-go-integration-tests-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-27-partition-slow-go-integration-tests-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0332 collapsed the race shards into one serial `go test -race ./...` gate and granted it a
temporary 300-second exception to the test budget's hard 60-second row ceiling. The dominant cost
was real-Git and subprocess integration work inside `internal/app`, not race instrumentation, so
the default Go gate and race gate paid substantially the same slow package twice.

The structural problem has since become a build blocker. Change 0357's completed high-priority fix
cannot clear the full gate because `internal/app` now reaches Go's ten-minute per-package timeout.
Fresh 2026-08-27 measurements also show the original package boundary is too narrow:
`internal/githubcli` takes 129.43 seconds under `-race`, and `internal/gitcli` takes 91.23 seconds.
Both contain broad tails of real-process protocol tests rather than one removable hotspot.

This change separates slow integration coverage from the fast default corpus across all three
packages. It keeps every scenario mandatory, reserves race instrumentation for tests that exercise
real concurrent behavior, and restores every Go test entry to the ordinary sub-60-second budget
regime so the 300-second exemption can be deleted.

## What changes

Settled design (2026-08-27 interactive grooming; detail in the linked spec):

- Move slow real-Git, GitHub, subprocess, timeout, and end-to-end operation tests in
  `internal/app`, `internal/githubcli`, and `internal/gitcli` behind one `integration` build tag.
  Fast fake-backed tests stay visible to ordinary `go test ./...`.
- Give tagged tests structural feature prefixes and run them through mandatory, feature-based
  `tests/test_*.sh` shards. Shards run in the existing parallel lane, target 45–50 seconds of
  standalone wall clock, and keep every budget row at or below 60 seconds.
- Mark only concurrency-bearing integration tests with the `TestRaceIntegration...` convention;
  each carries a nearby rationale and runs exactly once in a dedicated race shard. Sequential
  subprocess drivers run exactly once without `-race`.
- Add a fail-closed contract that discovers tagged tests and live runner declarations, proves every
  test belongs to exactly one shard and the correct race mode, proves tagged tests do not leak into
  the default corpus, and vets the tagged corpus. Mutation-prove missing tag, missing runner,
  duplicate prefix, and wrong race-mode detection.
- Keep the existing `e2e`-tagged finalize matrix unchanged. Add `t.Parallel()` inside new shards
  only where an isolation audit proves the test owns its repositories, environment, and process
  resources.
- Return `tests/test_go_race.sh` to the parallel lane over the fast default corpus. Delete the
  300-second exception and its serial-coupling guards, then re-derive all affected budget rows,
  `EXPECTED_SERIAL`, and `EXPECTED_TOTAL` from post-partition measurements.

## Out of scope

- Production package or behavior changes, suite-scheduler changes, and Go timeout increases.
- Host-relative budget work from #0273 and shell-suite sharding from #0280.
- Reworking the existing finalize E2E gate.
- Resuming or finalizing #0357; this change removes its structural gate blocker, after which its
  existing branch can resume separately.
