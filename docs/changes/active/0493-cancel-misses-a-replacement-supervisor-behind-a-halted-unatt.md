---
id: 493
slug: 'cancel-misses-a-replacement-supervisor-behind-a-halted-unatt'
title: 'Retire the automatic gate relaunch'
status: 'implemented'
priority: 'medium'
type: 'fix'
created: '2026-10-02'
updated: '2026-10-03'
depends_on: [490]
stacked_on:
related: [491, 492]
discovered_from: [490]
adrs: [98, 107, 132, 135]
spec: 'docs/superpowers/specs/2026-10-02-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt-design.md'
plan: 'docs/superpowers/plans/2026-10-03-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt.md'
results: 'docs/results/2026-10-03-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/cancel-misses-a-replacement-supervisor-behind-a-halted-unatt'
pr: 'https://github.com/danielhanold/docket/pull/370'
blocked_by:
reconciled: true
claimed_at: '2026-10-03T09:17:38Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-02-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-02-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt-design.md) |
| Plan | [2026-10-03-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt.md](https://github.com/danielhanold/docket/blob/fix/cancel-misses-a-replacement-supervisor-behind-a-halted-unatt/docs/superpowers/plans/2026-10-03-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt.md) |
| Results | [2026-10-03-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt-results.md](https://github.com/danielhanold/docket/blob/fix/cancel-misses-a-replacement-supervisor-behind-a-halted-unatt/docs/results/2026-10-03-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt-results.md) |
| ADRs | [ADR-0098](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0098-structured-gate-waiting-and-ownership-handoff.md), [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0132](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0132-worktree-admission-is-a-supervisor-held-kernel-lock.md), [ADR-0135](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0135-gate-drives-never-relaunch-automatically.md) |
<!-- docket:artifacts:end -->

## Why

A gate drive whose supervisor dies mid-run can earn one automatic relaunch, if it opted in with `IdempotentSuiteGate`. Only finalize's local gate opts in, and `evidence.recertify` shares that gate. The relaunch has a large crash-window apparatus behind it: a reservation, a token, crash recovery, a lock re-take, and an attach step.

This change was captured to fix a gap in that apparatus, found while fixing 0490's review findings. A relaunch whose launch response is lost halts without attaching its run dir, and the run census (`run.cancel` and `run.verdict`'s success closeout) would then miss a replacement supervisor. Grooming found that no production run can reach the gap: relaunching drives never carry a run context, so the census never sees them. ADR-0132 problem fact 6 already records this. The relaunch has also never fired on this machine: none of 216 drive records since 2026-09-29 relaunched.

The 2026-10-02 backlog review chose to **retire the relaunch** rather than harden around it. Retiring it removes this change's whole bug class, removes 0492's item 3 (`proveNoTreeSurvives` trusting `vanished` before a relaunch), and simplifies the driver. The cost is small: a rare supervisor death in finalize halts, and a human re-runs finalize, which re-runs the suite anyway.

**This change must add no other risk.** Build gates behave exactly as today apart from a renamed halt cause, and the spec's risk analysis checks every point.

## What changes

- **A supervisor death always halts.** Every gate drive halts when its supervisor dies, with a new cause, `supervisor-died`. Build drives already halt here, as `not-idempotent`; only the token is renamed. Finalize and recertify halt where they used to relaunch, and finalize reports `gate-halted`.
- **The relaunch machinery is deleted:** the reservation and token, crash recovery, the lock re-take and attach, the relaunch-only halt causes, the `IdempotentSuiteGate` opt-in and its `--idempotent-suite-gate` flag, and the relaunch fields in the drive record.
- **The census handles at most one launch.** With one launch per drive, the census needs no relaunch branch, and 0493's original gap becomes impossible by construction.
- **Old records still load.** Unknown fields are ignored and there is no schema bump. The per-drive claim keeps its `relaunch.lock` file name on disk. A one-line pre-install check confirms no drive was left mid-relaunch.
- **Docs and decision record.** Prose in docket-build, finalize's gate-failure reference, and the glossary is updated. A new ADR records the retirement, and ADR-0132 gets an Update note.
- **Tests.** Relaunch tests are rewritten as halt tests, and mutation checks are added. Every other test passes unmodified.

## Out of scope

- Suite teardown after a supervisor is gone: process groups outliving it, KILL escalation, and graceful-stop timing. That is 0492's items 1, 2, and 4. Item 3 is dropped by this change.
- Any automatic re-run in finalize after a `supervisor-died` halt. That would reintroduce a relaunch.
- A never-launched drive blocking a successful run's closeout (0490 review finding F3, recorded under 0491).
- The run id and its fences (0491).
- The worktree lock and its holder model (0490, ADR-0132), apart from deleting the relaunch's lock re-take.
- Deleting `relaunch.lock` files or the old relaunch fields from records already on disk.

## Reconcile log

### 2026-10-03

### 2026-10-03

- Dependency 0490 is done; related 0491 (run id retired) and 0492 (tree-survives census finding) both merged to `main` after the spec was written against `756fea9`. 22 commits touched `internal/gatedrive` since (driver.go, reconcile.go, test files reshaped; `driver_runfence_test.go`, `run_launch_gate_test.go`, `launch_sites_guard_test.go` deleted).
- The design still holds: every relaunch site the spec names is still present on `main` (`RelaunchReserved`, `reconcileReservation`, `settleNeverLaunchedCancelled`, `PriorRawRunDir`, `IdempotentSuiteGate`, the CLI flag, `PriorHolder`). Scope unchanged.
- Fold-in: `settleNeverLaunched` uses `errRelaunchRaceLost` as its CAS-lost sentinel; when that sentinel is deleted the census needs its own neutral sentinel. 0492 shipped without its item 3, consistent with this change. The test-site list must be re-derived from a grep of current `main`, not the spec's `756fea9` list.

### 2026-10-03

### 2026-10-03 (ADR)

- Recorded ADR-0135, "Gate drives never relaunch automatically", and added an Update note to ADR-0132.
