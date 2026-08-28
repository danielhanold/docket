---
id: 362
slug: partition-internal-release-to-clear-the-race-gate-timeout
title: 'Partition internal/release integration tests to clear the race-gate per-package timeout'
status: 'in-progress'
priority: high
type: refactor
created: 2026-08-28
updated: '2026-08-28'
depends_on: []
stacked_on:
related: [333, 361]
discovered_from: [361]
adrs: []
spec: docs/superpowers/specs/2026-08-28-partition-internal-release-integration-tests-design.md
plan: 'docs/superpowers/plans/2026-08-28-partition-internal-release-integration-tests.md'
results:
trivial: false
auto_groomable:
branch: 'refactor/partition-internal-release-to-clear-the-race-gate-timeout'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-28T02:35:59Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-partition-internal-release-integration-tests-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-partition-internal-release-integration-tests-design.md) |
| Plan | [2026-08-28-partition-internal-release-integration-tests.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-28-partition-internal-release-integration-tests.md) |
<!-- docket:artifacts:end -->

## Why

`internal/release` now blows Go's 600-second per-package timeout under the suite's parallel
`go test -race -count=1 ./...` load, reddening `tests/test_go_race.sh` (and, on a cold cache,
threatening `tests/test_go_toolchain.sh`'s `go test ./...`). Because the build gate runs the whole
suite, this reds **any** change — even a markdown-only or config-only diff — regardless of whether
it touches Go at all.

This is the same environmental wall-clock blocker class that change **0333** retired for
`internal/app` (and `internal/githubcli` / `internal/gitcli`). 0333 moved those packages' slow
real-subprocess corpus behind the `integration` build tag so the default `-race ./...` corpus drops
their tail — but `internal/release` was **left untagged**, so its packaging tests still run inside
the default race corpus and now dominate it.

**Concrete evidence — change 0361's halted run (2026-08-27):**
- Full-suite build gate: `SUITE files=149 passed=148 failed=1`; the single red file was
  `test_go_race`.
- Serial confirmation in isolation: `go test -race ./internal/release/ -timeout 30m` →
  **`ok … 139.311s`**. The package is healthy alone; it only breaches the 600s ceiling under
  concurrent, race-instrumented suite load (GOMAXPROCS-wide race workers starving each other).
- Change 0361 touches **zero Go files**, so it provably did not cause this — yet the red gate blocked
  0361's own finalize, forcing a `halted` disposition. This change removes that structural blocker.

**Cost driver (why the package is slow under `-race`):** release packaging is genuine
subprocess/filesystem integration work, not race-sensitive logic. Signals across the 5 test files:
- `package_test.go` — `exec.Command` (real subprocess) + `archive/tar` + `compress/gzip` + `t.TempDir`
- `archive_test.go` — `archive/tar` + `compress/gzip` + `t.TempDir`
- `checksums_test.go` / `version_test.go` — `t.TempDir` (lighter)
- `render_test.go` — pure, fast (no I/O signals)

This is exactly the shape 0333 moved behind the tag: heavy real-tar/gzip/subprocess tests that
belong in a dedicated feature shard, not the fast default `-race` corpus.

Prior instance of this class (now retired): the `internal/app` 600s timeout, hit on change 355,
fixed by 0333. See the design/plan on change 0333 for the reference implementation and the
fail-closed tag-partition contract it introduced.

## What changes

Settled design (2026-08-28 interactive grooming; detail in the linked spec):

- Move `package_test.go` and `archive_test.go` wholesale behind the existing `integration` build
  tag. Keep render, version, and checksum tests in the default corpus.
- Rename the moved tests under one `TestIntegrationRelease...` family and run them initially through
  one mandatory sequential, non-race integration shard. The current release tests exercise no
  concurrent protocol, so none warrants `TestRaceIntegration...` or a race shard.
- Generalize 0333's fail-closed partition contract to discover tagged packages and live runner
  declarations structurally instead of extending its hand-maintained package list. Prove exact
  one-runner coverage, correct race mode, no default-corpus leakage, non-empty runners, and tagged
  vet coverage in both directions; mutation-prove the guards.
- Preserve every existing test scenario and prove the before/after test-name mapping exactly.
- Measure the release shard, default race gate, and toolchain gate three times with a fresh Go build
  cache on an otherwise idle supported macOS host. Target 45–50 seconds and enforce a hard 60-second
  ceiling; split any affected runner that lacks headroom rather than raising a timeout or budget.
- For any package-level split, derive the package set from `go list ./...` and prove the shards are a
  disjoint exact cover. Re-derive runtime rows, `EXPECTED_SERIAL`, and `EXPECTED_TOTAL`, then verify
  the complete parallel suite.

## Out of scope

- Production release code or behavior changes — `render.go` embeds, packaging/checksum logic, the
  `release-candidate.yml` workflow, and `scripts/release-smoke.sh` are untouched.
- Raising Go's per-package timeout globally or accepting a larger-than-60-second runtime row.
- Re-partitioning `internal/app` / `internal/githubcli` / `internal/gitcli` — 0333 already did that.
- Resuming or finalizing change 0361: this change removes 0361's structural gate blocker, after which
  0361's existing halted branch resumes separately (`docket change resume-halted
  --acknowledge-quiescent`). No `depends_on` edge is asserted here — the relationship is coordination,
  not a build-readiness gate.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-28

2026-08-28 (UTC) — Reconciled at claim. Verified current reality on origin/main: change 0333's integration-tag partition machinery is present and in force (tests/test_go_integration_contract.sh plus ~17 tests/test_go_integration_*.sh runners covering internal/app, internal/gitcli, internal/githubcli), and internal/release remains entirely untagged (archive_test.go, checksums_test.go, package_test.go, render_test.go, version_test.go are all default-corpus *_test.go with no //go:build integration files). This confirms the spec's premise: internal/release's slow real-tar/gzip/subprocess corpus still runs inside the default `go test -race ./...` gate and drives the per-package 600s timeout the spec targets. No design invalidation and no scope adjustment: package_test.go + archive_test.go move behind the integration tag under a TestIntegrationRelease... family with one sequential non-race runner (tests/test_go_integration_release.sh); render/version/checksums stay in the default corpus; the contract is generalized to discover tagged packages/runners structurally rather than by allowlist; all affected runtime rows are re-measured from a fresh Go build cache under the 60s ceiling. Relations unchanged: related [333, 361], discovered_from [361]; no depends_on edge on 361 (coordination only — 0361 resumes separately after this lands). A pre-existing scan of internal/release/*_test.go for goroutines/sync/t.Parallel()/process-lifecycle coordination will be confirmed at plan/build time; grooming found none, so no race integration shard is planned unless build-time inspection surfaces a concrete concurrent protocol. No auto-capture (AUTO_CAPTURE_ENABLED=false).
