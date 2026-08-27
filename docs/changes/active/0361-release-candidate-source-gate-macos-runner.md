---
id: 361
slug: release-candidate-source-gate-macos-runner
title: 'Release-candidate source-gate green on a macOS runner'
status: 'in-progress'
priority: high
type: fix
created: 2026-08-27
updated: '2026-08-27'
depends_on: [317]
stacked_on:
related: [318, 322]
discovered_from: [317]
adrs: []
spec: docs/superpowers/specs/2026-08-27-release-candidate-source-gate-macos-runner-design.md
plan: 'docs/superpowers/plans/2026-08-27-release-candidate-source-gate-macos-runner.md'
results:
trivial: false
auto_groomable:
branch: 'fix/release-candidate-source-gate-macos-runner'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-27T23:32:38Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-27-release-candidate-source-gate-macos-runner-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-27-release-candidate-source-gate-macos-runner-design.md) |
| Plan | [2026-08-27-release-candidate-source-gate-macos-runner.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-27-release-candidate-source-gate-macos-runner.md) |
<!-- docket:artifacts:end -->

## Why

Change 0317's `release-candidate.yml` runs its `source-gate` job — clean tree, build identity,
embedded-asset drift, and the full `scripts/run-tests.sh` suite — on `ubuntu-24.04`. The docket
suite is authored and wall-clock-measured only on macOS (`darwin/arm64`); `source-gate` is the
first workflow to run it on Linux, and it fails on roughly seven macOS-only tests (the `brew`
remedy, mode/umask assertions, shell-rc writes, and three Go/budget-state files). The gate has never
been green since the PR opened, so every candidate carries a red check and the release-candidate
workflow's suite gate is meaningless. It needs to be a real signal before 0318's hard cutover.

## What changes

Move the `source-gate` job's runner from `ubuntu-24.04` to the standard arm64 `macos-15` runner — the
platform family the suite is built for — and explicitly install, version-check, and route the suite
through Homebrew Bash 4.3+. Add `tests/**` and `.docket.yml` to the pull-request triggers, surface the
suite runner's current budget-finding vocabulary, fail on an authoritative budget breach, and add
mutation-tested workflow guards. The `package`, `smoke`, and `summary` jobs keep their runners: real
Linux coverage still comes from the native-tuple matrix, which builds and smokes the candidate on
Darwin and Linux × amd64/arm64.

## Out of scope

Any change to the `package`/`smoke`/`summary` runners or logic; a general Linux port of
`run-tests.sh` or its tests; retuning or persisting `runtime-budgets.tsv` state for CI hardware; the
gzip downloader-sandbox fix (already landed in 0317); and 0318's Bash removal and cutover.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-27

Reconciled against current main at commit 6a9f1dec6ee1cc96d3d79adba4efe5079638e2b8 on 2026-08-27. Change 0317 is done and its release-candidate workflow is present; change 0318 remains proposed and still depends on 0317. The approved scope remains valid: source-gate runner/runtime, pull-request triggers, budget summary/failure classification, and tests/test_release_package.sh regression guards are still the outstanding work. The package, smoke, and summary jobs remain outside the change, as do the landed 0317 gzip fix and 0322 installer work. Relations remain depends_on [317], related [318, 322], discovered_from [317], with no ADRs or stack base. No adjacent follow-up is untracked; auto-capture is disabled.

## Run halted

### 2026-08-27

### 2026-08-27

All five plan tasks are built and committed on `fix/release-candidate-source-gate-macos-runner`
(HEAD `9ee1787d`, base `origin/main` @ `6a9f1dec`); the branch delta is exactly the three intended
files — `.github/workflows/release-candidate.yml`, `tests/test_release_package.sh`, and the plan.
Each task was TDD'd and its guards mutation-tested; the focused guard suite
`bash tests/test_release_package.sh` is green (exit 0, 0 NOT OK).

The run halts at the full-suite build gate. `scripts/run-tests.sh` reports
`SUITE files=149 passed=148 failed=1` with a single red file: `test_go_race`. Its failure is
`github.com/danielhanold/docket/internal/release  FAIL  600.647s` — the Go per-package 600s timeout
tripped under the suite's parallel `go test -race -count=1 ./...` load, not an assertion failure.

Serial confirmation (authoritative per AGENTS.md's budget/wall-clock discipline):
`go test -race -count=1 ./internal/release/ -timeout 30m` → `ok … 139.311s`. The package passes
comfortably when run alone; it only breaches the timeout under concurrent race-instrumented load.

This is an **environmental** blocker, not a defect in this change: change 0361 touches zero Go
files (only a workflow YAML and a shell test), so it cannot have caused an `internal/release` race
timeout. It is the same class of blocker as the retired `internal/app` 600s timeout (change 355,
fixed by change 0333 partitioning that package behind a build tag). The real fix is a **separate
change** that shards or build-tags `internal/release` so it stays under the per-package race
timeout inside the parallel suite — out of scope for 0361, whose scope is the release-candidate
`source-gate` job.

The build cannot mint a green build-evidence record from a red gate, and there is nothing in this
change's own diff to repair, so an in-branch integration-repair task would be futile and
scope-expanding. A human decides whether to (a) land an `internal/release` partition change first,
then resume 0361, or (b) accept the environment-explained red and merge on the serial-confirmed
green. The branch, lease, workspace, and its commits are preserved untouched for either path.
