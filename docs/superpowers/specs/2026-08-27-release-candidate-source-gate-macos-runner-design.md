<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0361 — Release-candidate source-gate green on a macOS runner](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0361-release-candidate-source-gate-macos-runner.md)**
<!-- docket:backlink:end -->

# Release-candidate source-gate green on a macOS runner

**Change:** 0361 · **Date:** 2026-08-27 · **Status:** Approved design

## Summary

Change 0317 added `.github/workflows/release-candidate.yml`. Its first job, `source-gate`, proves
the checkout is clean, resolves the build identity the candidate bundle is stamped with, checks the
embedded asset bundle for drift, and runs the resolved test suite (`finalize.test_command` →
`scripts/run-tests.sh`). It runs on `ubuntu-24.04`.

The docket suite is authored and wall-clock-measured exclusively on macOS (`darwin/arm64` — every
sizing note in `tests/runtime-budgets.tsv` records that machine). `source-gate` is the first
workflow to run it on Linux, and it fails: on `ubuntu-24.04` roughly seven files redden as
macOS-isms — `test_bash_runtime_install` (asserts a `brew install bash` remedy),
`test_board_refresh` (asserts mode `644` under a specific umask), `test_ensure_docket_env` and
`test_ensure_global_config` (write and re-read shell rc files), plus `test_go_toolchain`,
`test_go_race`, and `test_run_tests_budget_state`. The gate has never been green since the PR
opened; it is a red check on every candidate.

This change switches `source-gate` from `ubuntu-24.04` to a macOS runner (`macos-14`, arm64), the
platform the suite is built for. The suite then runs on the OS it was written and measured on and
goes green. Real Linux coverage is unaffected: the workflow's `package` and `smoke` jobs already
build and smoke the candidate on all four native tuples (Darwin/Linux × amd64/arm64), so nothing
about Linux release behavior stops being exercised — only the *suite gate* moves to where the suite
is valid.

## Motivating outcome

On a pull request against `main`, the `release-candidate` workflow's `source-gate` job completes
**green**: the tree-cleanliness and build-identity steps pass, the embedded-asset drift check
passes, and `scripts/run-tests.sh` reports `SUITE … failed=0` with the same file set that passes on
a maintainer's local macOS checkout. The job's outputs (`commit`, `short`, `epoch`) are unchanged in
shape and continue to feed the downstream `package` and `smoke` jobs, which keep their existing
runners. The overall workflow conclusion for a clean candidate is success rather than failure.

The gzip downloader-sandbox defect that this investigation also surfaced (the Section B+ PATH
sandbox omitted `gzip`, so GNU `tar -z` failed on Linux) is **already fixed inside change 0317** and
is not part of this change.

## Design

### The change

In `.github/workflows/release-candidate.yml`, the `source-gate` job's `runs-on:` moves from
`ubuntu-24.04` to `macos-14`. No step logic changes: checkout, Go setup, the clean-tree/identity
resolution, the embedded-asset drift check, and the resolved-suite run are all platform-neutral and
already run on macOS locally every day.

The `package`, `smoke`, and `summary` jobs are **not** retargeted. `package`/`summary` stay on
`ubuntu-24.04`; `smoke` keeps its native-tuple runner matrix. Only the suite gate — the job whose
correctness depends on the suite's authored platform — moves.

### Bash on the runner

The suite is `#!/usr/bin/env bash` and uses features beyond the bash 3.2 that ships at
`/bin/bash` on macOS. A maintainer's local runs use a modern Homebrew bash
(`/opt/homebrew/bin/bash`). The build must ensure the `macos-14` job resolves a bash new enough to
run the suite — either the newer bash already present on the GitHub macOS image's `PATH`, or an
explicit provisioning step — and the *motivating outcome* (a green `failed=0` run) is the
acceptance signal that it did. Pinning the exact mechanism is a build-time decision, not a spec
commitment.

### Budgets

`tests/runtime-budgets.tsv` ceilings are measured on `darwin/arm64`; `macos-14` runners are arm64,
so the numbers are in the right regime. A CI machine slower than a maintainer's laptop may emit
`BUDGET WATCH:` advisory lines — those are non-fatal by design (a parallel wall-clock number is
machine-dependent; only a `SERIAL CONFIRMED OVER BUDGET:` line is authoritative), so they do not
redden the gate and no budget numbers are retuned for CI.

## Considered alternatives

- **Make the whole suite Linux-portable (OS-gate each failing test).** Rejected as the primary
  approach: it is the largest effort, several failures are macOS-only by nature (the `brew` remedy
  test, the mode/umask test), and change 0318's hard cutover already removes production Bash and
  Bash-only tests — so a chunk of the ported work would be deleted shortly after. Genuine
  cross-platform CI can be revisited later on its own merits; it is not what a gate fix should carry.
- **Restrict `source-gate` to a Linux-portable subset on ubuntu.** Rejected: partial coverage that
  needs a maintained allow/deny list and silently lets a macOS-only regression ride through the gate.

## Relationship to change 0318

0318 (hard Go cutover) removes production Bash and Bash-only tests, which will delete several of the
currently-failing files (`test_bash_runtime_install`, the retired-mechanism `test_ensure_*`). This
change should land **before** 0318 so the release-candidate gate is already meaningful when the
cutover happens. Moving the runner is orthogonal to and forward-compatible with that deletion: a
smaller post-0318 suite still runs correctly on `macos-14`.

## Out of scope

- Any change to `package`, `smoke`, or `summary` job runners or logic.
- A general cross-platform (Linux) port of `scripts/run-tests.sh` or its test files.
- Retuning `tests/runtime-budgets.tsv` for CI hardware.
- The 0317 gzip downloader-sandbox fix (already landed in 0317).
- 0318's Bash removal and cutover.

## Risks

- **Runner bash version.** If the `macos-14` image's default `PATH` bash is too old, the suite needs
  an explicit modern-bash step; the green-run acceptance signal catches this at build time.
- **macOS runner minutes.** macOS runners cost more than Linux; `source-gate` runs the full suite
  (~6 min wall). Accepted as the price of a valid gate; the other three jobs stay on cheaper runners.
