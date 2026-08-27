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

This change switches `source-gate` from `ubuntu-24.04` to the standard `macos-15` arm64 runner, the
platform family the suite is built for, and explicitly provisions the modern Bash runtime the suite
requires. The suite then runs on the OS and shell it was written and measured on and goes green.
Real Linux coverage is unaffected: the workflow's `package` and `smoke` jobs already build and smoke
the candidate on all four native tuples (Darwin/Linux × amd64/arm64), so nothing about Linux release
behavior stops being exercised — only the *suite gate* moves to where the suite is valid.

## Motivating outcome

On a pull request against `main`, the `release-candidate` workflow's `source-gate` job completes
**green**: the tree-cleanliness and build-identity steps pass, the embedded-asset drift check
passes, and `scripts/run-tests.sh` reports `SUITE … failed=0` with the same file set that passes on
a maintainer's local macOS checkout. Before the suite starts, the job proves that
`DOCKET_BASH_PATH` names GNU Bash 4.3 or newer. Pull requests that change `tests/**` or the
`.docket.yml` suite configuration trigger the workflow, just as changes to `scripts/**` already do.
Budget screening findings appear in the job summary, and an authoritative serial-confirmed breach
cannot leave the job green. The job's outputs (`commit`, `short`, `epoch`) are unchanged in shape and
continue to feed the downstream `package` and `smoke` jobs, which keep their existing runners. The
overall workflow conclusion for a clean candidate is success rather than failure.

The gzip downloader-sandbox defect that this investigation also surfaced (the Section B+ PATH
sandbox omitted `gzip`, so GNU `tar -z` failed on Linux) is **already fixed inside change 0317** and
is not part of this change.

## Design

### The change

In `.github/workflows/release-candidate.yml`, the `source-gate` job's `runs-on:` moves from
`ubuntu-24.04` to `macos-15`. The source-gate sequence gains one modern-Bash setup step and updates
its budget-report classification; checkout, Go setup, clean-tree/identity resolution, embedded-asset
drift, and resolved-suite semantics otherwise stay unchanged.

The `package`, `smoke`, and `summary` jobs are **not** retargeted. `package`/`summary` stay on
`ubuntu-24.04`; `smoke` keeps its native-tuple runner matrix. Only the suite gate — the job whose
correctness depends on the suite's authored platform — moves.

### Pull-request trigger coverage

Add `tests/**` and `.docket.yml` to the workflow's `pull_request.paths` list. The source gate executes
the test corpus and reads `finalize.test_command` from `.docket.yml`; a pull request that changes
either input must not bypass the job. The existing path filters remain unchanged otherwise.

### Bash on the runner

The suite is `#!/usr/bin/env bash` and needs GNU Bash 4.3 or newer (`wait -n`); GitHub's macOS image
documents only the Apple-provided Bash 3.2 at `/bin/bash`. After Go setup, add an explicit suite-Bash
step that:

1. installs the Homebrew `bash` formula when it is absent;
2. resolves the formula's absolute `bin/bash` path through `brew --prefix bash`;
3. executes that binary and refuses to continue unless its version is at least 4.3; and
4. writes the absolute path to `DOCKET_BASH_PATH` through `GITHUB_ENV` for subsequent steps.

The resolved-suite step keeps reading `finalize.test_command` from `.docket.yml`. Its initial
platform `bash` invocation may still be Bash 3.2: `scripts/run-tests.sh` already detects that case
and re-executes itself through `DOCKET_BASH_PATH`, then uses the same configured runtime for every
test file. The other source-gate run blocks remain compatible with the platform shell. This makes
the runtime selection explicit and testable instead of treating a green run as the first version
probe.

### Budgets

`tests/runtime-budgets.tsv` ceilings are measured on `darwin/arm64`; standard `macos-15` runners are
arm64, so the numbers are in the right regime. A CI machine slower than a maintainer's laptop may
emit `BUDGET WATCH:` or `PARALLEL-SENSITIVE:` screening lines. Those remain non-fatal because a
parallel wall-clock number is machine-dependent, but the suite step collects them into a clearly
labeled job-summary warning.

The existing workflow searches only for `OVER BUDGET:`, which misses the default parallel runner's
current vocabulary. Replace that stale classifier with the complete report contract:

- `BUDGET WATCH:` and `PARALLEL-SENSITIVE:` are summarized as screening findings and do not change
  the suite exit status.
- `OVER BUDGET:` and `SERIAL CONFIRMED OVER BUDGET:` are summarized as authoritative findings. If
  the suite itself was otherwise green, their presence makes the source-gate step fail.

No budget rows or thresholds are changed for CI hardware, and the workflow does not opt into
`--strict-budget`.

### Regression guards

Extend `tests/test_release_package.sh`, which already owns the release-candidate workflow contract,
with guards derived from the `source-gate` job and trigger blocks. They prove that:

- `source-gate` uses the standard arm64 `macos-15` label while the other jobs retain their existing
  runner assignments;
- the suite-Bash step exports an absolute `DOCKET_BASH_PATH` only after verifying GNU Bash 4.3+;
- `tests/**` and `.docket.yml` both trigger the workflow; and
- the suite summary recognizes the screening and authoritative budget-report shapes above.

Mutation-test each guard: changing `source-gate` back to Ubuntu, removing the Bash export/version
check, removing either trigger, or reverting the budget classifier must redden the owning test. The
guards key on the syntactic job/step/block shapes rather than matching an unrelated occurrence of
the same literal elsewhere in the workflow.

## Acceptance

- `tests/test_release_package.sh` passes with the revised workflow, and each specified mutation
  makes the corresponding guard fail.
- A pull request containing the workflow change triggers `release-candidate`; the source-gate log
  identifies GNU Bash 4.3+ through `DOCKET_BASH_PATH` and the complete resolved suite reports
  `failed=0`.
- Screening budget findings, if any, appear in the job summary without failing the job; an
  authoritative budget finding cannot produce a green source gate.
- `package`, all four native `smoke` legs, and `summary` retain their runner assignments and finish
  green for the same candidate.

## Considered alternatives

- **Make the whole suite Linux-portable (OS-gate each failing test).** Rejected as the primary
  approach: it is the largest effort, several failures are macOS-only by nature (the `brew` remedy
  test, the mode/umask test), and change 0318's hard cutover already removes production Bash and
  Bash-only tests — so a chunk of the ported work would be deleted shortly after. Genuine
  cross-platform CI can be revisited later on its own merits; it is not what a gate fix should carry.
- **Restrict `source-gate` to a Linux-portable subset on ubuntu.** Rejected: partial coverage that
  needs a maintained allow/deny list and silently lets a macOS-only regression ride through the gate.
- **Use `macos-14`.** Rejected because that image is already in its retirement window and is
  scheduled to become unsupported shortly after this change. The workflow already uses and has
  validated the standard arm64 `macos-15` label in its smoke matrix.
- **Use `macos-latest`.** Rejected because the moving label can change OS generations independently
  of this repository. Pinning `macos-15` keeps the suite environment reviewable while avoiding the
  near-term `macos-14` retirement.

## Relationship to change 0318

0318 (hard Go cutover) removes production Bash and Bash-only tests, which will delete several of the
currently-failing files (`test_bash_runtime_install`, the retired-mechanism `test_ensure_*`). This
change should land **before** 0318 so the release-candidate gate is already meaningful when the
cutover happens. Moving the runner is orthogonal to and forward-compatible with that deletion: a
smaller post-0318 suite still runs correctly on `macos-15`; the explicit Bash setup can be removed
alongside the Bash-only suite when 0318 proves it is no longer required.

## Out of scope

- Any change to `package`, `smoke`, or `summary` job runners or logic.
- A general cross-platform (Linux) port of `scripts/run-tests.sh` or its test files.
- Retuning `tests/runtime-budgets.tsv` for CI hardware.
- Persisting advisory budget-state history across ephemeral GitHub runners.
- The 0317 gzip downloader-sandbox fix (already landed in 0317).
- 0318's Bash removal and cutover.

## Risks

- **Homebrew availability.** The suite now depends on the runner's documented Homebrew installation
  to provide modern Bash. The setup step fails before the suite with the resolved path and version
  visible when Homebrew or the formula is unavailable; it never silently falls back to Bash 3.2.
- **macOS runner capacity.** `source-gate` occupies a standard macOS runner for roughly six minutes,
  which may queue differently from Ubuntu. Standard hosted runners remain unbilled for this public
  repository; making the repository private would reintroduce GitHub's higher macOS minute rate.
