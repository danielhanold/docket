---
id: 362
slug: partition-internal-release-to-clear-the-race-gate-timeout
title: 'Partition internal/release integration tests to clear the race-gate per-package timeout'
status: proposed
priority: high
type: refactor
created: 2026-08-28
updated: 2026-08-28
depends_on: []
stacked_on:
related: [333, 361]
discovered_from: [361]
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

PM-altitude scope — settle the detail during grooming; model directly on change 0333's design:

- Move the slow real-tar/gzip/subprocess release tests (`package_test.go`, `archive_test.go`, and any
  other heavy cases) behind the existing `integration` build tag, keeping fast unit tests
  (`render_test.go`, `version_test.go`, `checksums_test.go` where light) visible to ordinary
  `go test ./...`.
- Give the tagged tests a structural feature prefix and route them through a mandatory, feature-based
  `tests/test_*.sh` shard in the existing parallel lane, budgeted at or under the 60-second row
  ceiling; re-derive the affected budget rows (`tests/runtime-budgets.tsv`), `EXPECTED_SERIAL`, and
  `EXPECTED_TOTAL` from post-partition measurements.
- Mark only genuinely concurrency-bearing tests with the `TestRaceIntegration…` convention (release
  packaging is expected to be sequential, so likely none) — each with a nearby rationale, run once in
  a dedicated race shard; sequential subprocess drivers run once without `-race`.
- Extend 0333's fail-closed partition contract to cover `internal/release`: prove every tagged test
  belongs to exactly one shard and the correct race mode, prove tagged tests do not leak into the
  default corpus, and mutation-prove the guards (missing tag, missing runner, duplicate prefix, wrong
  race-mode detection).
- Confirm `tests/test_go_race.sh`'s default `-race ./...` corpus drops back under budget once the
  `internal/release` tail is tagged out.

## Out of scope

- Production release code or behavior changes — `render.go` embeds, packaging/checksum logic, the
  `release-candidate.yml` workflow, and `scripts/release-smoke.sh` are untouched.
- Raising Go's per-package timeout globally, or any suite-scheduler / parallelism change.
- Re-partitioning `internal/app` / `internal/githubcli` / `internal/gitcli` — 0333 already did that.
- Resuming or finalizing change 0361: this change removes 0361's structural gate blocker, after which
  0361's existing halted branch resumes separately (`docket change resume-halted
  --acknowledge-quiescent`). No `depends_on` edge is asserted here — the relationship is coordination,
  not a build-readiness gate.

## Open questions

- Is the 139s→600s+ blow-up purely `-race` worker contention under parallel load, or does
  `internal/release` also carry a cold-compile tail worth measuring separately? Re-measure standalone
  and under concurrent suite load during grooming.
- Does 0333's shard/contract machinery generalize cleanly to a 4th package, or does `internal/release`
  need its own runner and budget rows?
- Does any release test genuinely exercise concurrency (warranting a `TestRaceIntegration` marker), or
  is the whole package sequential (→ no race-shard entry at all)?
- Does `tests/test_go_toolchain.sh`'s non-race `go test ./...` also breach on a cold cache, or is only
  the `-race` gate affected? Determines whether the toolchain budget row also needs re-derivation.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
