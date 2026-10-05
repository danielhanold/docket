---
id: 507
slug: 'flaky-tests-track-and-stabilize-intermittent-suite-failures'
title: 'Fix the observe-test hang and bring two test files back under their time limits'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [381, 273, 373]
discovered_from: [504, 506, 520, 517]
adrs: [108]
spec: 'docs/superpowers/specs/2026-10-05-flaky-tests-track-and-stabilize-intermittent-suite-failures-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-flaky-tests-track-and-stabilize-intermittent-suite-failures-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-05-flaky-tests-track-and-stabilize-intermittent-suite-failures-design.md) |
| ADRs | [ADR-0108](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md) |
<!-- docket:artifacts:end -->

## Why

The full suite sometimes goes red on a test that passes on a plain re-run. Gates also sometimes report a test file as officially over its time limit when it is not. Both cost gate re-runs, follow-up notes and grooming time, and both can be mistaken for a regression.

Grooming (2026-10-05) traced the four cases this change was opened for:

- `internal/process` `TestObserveRunningThenTerminal` normally takes about 0.06s. In change 0504's final gate it waited its full 30s for a killed process to read as gone, so something got stuck; the transition was not merely slow. It had happened twice before (change 0378, recorded in killed #381). Change #373's load cap did not stop it.
- Three test files were reported over their official line (1.5 times their time limit): `test_go_finalize_e2e.sh`, `test_go_integration_app_merge.sh` and `test_go_race.sh`. Measured alone on a quiet machine they come in at 36s, 41s and 54s, all under that line. The high readings were most likely taken while another change's suite was running on the same machine. But the first two are still over their own 30s limits, so they have no headroom left.

## What changes

One PR that fixes the current list, then closes. A future flake or slow file becomes its own change.

- **The hang.** Reproduce it under load with the race detector. If it reproduces, fix the real cause and prove the fix with a clean streak under the same load. If it does not reproduce within a bounded effort, stop the test from ignoring two errors (the kill result and the run-record read), and make the 30s timeout report what was stuck. Never raise the 30s, add retries, or loosen assertions.
- **`test_go_integration_app_merge.sh` and `test_go_finalize_e2e.sh`.** Cut work until each measures 25s or less alone on a quiet machine, so its 30s limit is honest. For the merge file, that most likely means running its 12 tests in parallel. A limit is raised only if the work cannot be cut, with the evidence argued in the PR.
- **`test_go_race.sh`.** No code work; record its quiet measurement.

Detail, measurement protocol and acceptance criteria are in the linked spec.

## Out of scope

- Retry-on-failure machinery, and any change to the gate or budget policy (thresholds, exit codes, the screen-then-confirm regime).
- Making the runner's solo re-check notice another suite running on the same machine. That is a separate change for a human to capture.
- Flakes and slow files not named here.
- #273's host-relative budgets.
