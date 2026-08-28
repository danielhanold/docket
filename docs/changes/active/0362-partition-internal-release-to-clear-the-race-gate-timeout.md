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
plan:
results:
trivial: false
auto_groomable:
branch: 'refactor/partition-internal-release-to-clear-the-race-gate-timeout'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-28T01:06:07Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-partition-internal-release-integration-tests-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-partition-internal-release-integration-tests-design.md) |
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
