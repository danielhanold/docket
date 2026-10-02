---
id: 493
slug: 'cancel-misses-a-replacement-supervisor-behind-a-halted-unatt'
title: 'Cancel misses a replacement supervisor behind a halted, unattached relaunch'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: [490]
stacked_on:
related: [491, 492]
discovered_from: [490]
adrs: [132]
spec:
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
| ADRs | [ADR-0132](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0132-worktree-admission-is-a-supervisor-held-kernel-lock.md) |
<!-- docket:artifacts:end -->

## Why

This gap was found while fixing 0490's review findings (PR #366), and is recorded as suspected in 0490's results file under "Known issues and follow-ups". It is confirmed only by reading the code; nothing has reproduced it yet. Code references are to 0490's branch.

Finalize's single automatic relaunch reserves the relaunch (`RelaunchReserved`, with a `RelaunchToken`), takes the worktree lock, and calls `process.Launch`. When that call returns an error, the driver resolves the relaunch token. If the result is `unresolved` (or the resolve call itself fails), the drive is written HALTED `launch-unconfirmed` (the relaunch path in `internal/gatedrive/driver.go`; `haltReservedRelaunchCause`). Crash recovery through `recoverReservedRelaunch` ends the same way. In both cases a replacement supervisor may have started even though the launch reported an error: the launch response was lost, not the launch.

After that halt, the drive's record names only the first run. The replacement's run dir was never attached, so `RawRunDir` is still the first run's and `PriorRawRunDir` is empty. The relaunch token is kept, but nothing reads it again.

When `run.cancel` (`ReconcileRunLaunches`) or the success closeout (`ObserveRunLaunches`) accounts a HALTED drive, `reconcileHaltedDrive` goes to `proveRunDirsGone`, which checks only `RawRunDir` and `PriorRawRunDir`. Only a HALTED first launch with no attached run dir resolves its launch token (`resolveHaltedFirstLaunch`); a HALTED relaunch never resolves its relaunch token. Once the first run's supervisor is gone, the drive counts as settled.

The result:

- **Cancel can report `cancelled` while the replacement supervisor and its suite are still running.** This is the same failure class as 0490's review finding F1 (cancel reporting `cancelled` while a first launch whose launch response was lost may still be running). F1's fix covered the first launch, not the relaunch.
- **The success closeout can count the run as complete** while an unattributed suite is still running in its worktree.
- **The worktree stays busy.** The replacement holds the worktree lock, so the next gate start is refused `worktree-busy` until that suite finishes on its own, and nothing in docket names the process or can stop it. The lock still prevents two suites from running at once, so this is a cleanup and attribution gap, not a concurrency hole.

A `relaunch-failed` halt is probably safe. It is written when the lock could not be taken (no launch happened) or when the token resolved to something other than `identified`, such as `never-launched`. Grooming should confirm that every `relaunch-failed` path really proves that no launch happened.

## What changes

These are hypotheses for grooming to evaluate, not decisions:

- **Resolve the relaunch token for a HALTED drive.** When a HALTED drive still carries a reserved relaunch that never attached (`RelaunchReserved` with a `RelaunchToken`), `reconcileHaltedDrive` resolves that token, the way `resolveHaltedFirstLaunch` resolves a first launch's admission token, in addition to proving the recorded run dirs gone. The outcomes are the same as for a first launch: `identified` means the replacement's supervisor must be proven gone (stopped in cancel mode, `run-live` in observe mode); `never-launched` is settled; `unresolved` or a probe error stays pending and fails closed.
- **Reuse `reconcileReservation`'s token resolution** instead of adding a new predicate.
- **Check every halt cause on the relaunch path** (`launch-unconfirmed`, `relaunch-failed`, `worktree-busy`, a lost attach race) and state, for each one, whether a launch could have happened.
- **Regression tests at the `gatedrive` layer and through `run.cancel`:** a relaunch whose `Launch` errors and whose token resolves `identified` after the halt must be stopped by cancel, and must block the success closeout while it runs.

Any new finding token follows 0490's credential-free `<token>:<drive>` shape. Any new check states its failure posture up front, and prefers making the problem visible over halting a run.

## Out of scope

- Tearing down a suite after its supervisor is gone (process groups outliving the supervisor, KILL escalation, `proveNoTreeSurvives` trusting `vanished`). That is 0492.
- A never-launched drive blocking a successful run's closeout (0490 review finding F3). That is recorded under 0491's "Open questions".
- The run id and its fences (0491).
- The worktree lock and its holder model (0490, ADR-0132).

## Open questions

### Retarget: retire the automatic relaunch (direction from the 2026-10-02 backlog review)

Daniel chose to retire finalize's single automatic relaunch rather than fix cancel's accounting for it. The hypotheses under "What changes" are superseded by this direction.

- **Who uses it.** Only finalize's local gate, including its build recertify path, opts into the relaunch (`IdempotentSuiteGate: true` in `internal/app/finalize_rebase.go`). No other caller sets the flag.
- **What it removes.** This change's whole bug class, plus 0492's item 3 (`proveNoTreeSurvives` trusting `vanished` before a relaunch).
- **The cost.** When a finalize suite's supervisor dies mid-run, finalize halts with `gate-halted` instead of relaunching once, and a human re-runs finalize, which re-runs the suite anyway.

Grooming should:

- retitle this change to match;
- trace everything the relaunch carries: the reservation and token fields in the drive record, `recoverReservedRelaunch`, the `relaunch-*` halt causes, the `--idempotent-suite-gate` flag, and ADR-0098's "one permitted relaunch" (carried into ADR-0107);
- decide how a new ADR records the retirement.
