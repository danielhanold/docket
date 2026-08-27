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
plan: 'docs/superpowers/plans/2026-08-27-partition-slow-go-integration-tests.md'
results:
trivial: false
auto_groomable:
branch: 'refactor/partition-internal-app-to-retire-the-race-gate-s-300s-ceilin'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-27T01:37:58Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-27-partition-slow-go-integration-tests-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-27-partition-slow-go-integration-tests-design.md) |
| Plan | [2026-08-27-partition-slow-go-integration-tests.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-27-partition-slow-go-integration-tests.md) |
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

## Reconcile log

### 2026-08-27

Reconciled at claim. The design (spec dated 2026-08-27, groomed the day before this build) remains fully current against the tree: change 0332's temporary machinery is present and unchanged in tests/runtime-budgets.tsv (the path-keyed 300s exemption for tests/test_go_race.sh, its RELIEF COUNTER A sub-ceiling, and the serial-identity coupling) and in tests/test_go_race.sh (still serial-pinned over the whole module). No `//go:build integration` files exist yet in internal/app, internal/githubcli, or internal/gitcli, and internal/app remains the dominant package (59 test files). Related-change states verified: 251 done; 273 and 280 proposed and explicitly out of scope; 357 in-progress and still gated on this partition (this change removes 357's structural blocker but does not depend on it, so depends_on stays empty). No ADRs are required at reconcile. Scope, out-of-scope, and all relations hold as authored; no section or relation edits needed.
