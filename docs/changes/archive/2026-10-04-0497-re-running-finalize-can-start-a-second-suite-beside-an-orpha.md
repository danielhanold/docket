---
id: 497
slug: 're-running-finalize-can-start-a-second-suite-beside-an-orpha'
title: 'Re-running finalize can start a second suite beside an orphaned one'
status: 'done'
priority: 'medium'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [490, 492, 493]
discovered_from: [493]
adrs: [132, 134, 135]
spec: 'docs/superpowers/specs/2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha-design.md'
plan: 'docs/superpowers/plans/2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha.md'
results: 'docs/results/2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/re-running-finalize-can-start-a-second-suite-beside-an-orpha'
pr: 'https://github.com/danielhanold/docket/pull/379'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha-design.md](../../superpowers/specs/2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha-design.md) |
| Plan | [2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha.md) |
| Results | [2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-10-04-re-running-finalize-can-start-a-second-suite-beside-an-orpha-results.md) |
| ADRs | [ADR-0132](../../adrs/0132-worktree-admission-is-a-supervisor-held-kernel-lock.md), [ADR-0134](../../adrs/0134-a-dead-supervisor-s-suite-counts-as-gone-only-when-its-proce.md), [ADR-0135](../../adrs/0135-gate-drives-never-relaunch-automatically.md) |
<!-- docket:artifacts:end -->

## Why

When a gate's supervisor dies alone (SIGKILL or a crash), its test suite keeps running in the supervisor's process group, but the worktree lock frees at once. Since 0493 (ADR-0135) the drive halts and a human re-runs the workflow; a re-run right away starts a second suite in the same checkout beside the leftover one, so the run can go slow, time out, or go red from contention, and in finalize a red feeds integration repair.

Grooming traced the stub's idea (check 0492's launch census at the next gate start and refuse) and found it cannot work as written. Finalize deletes the halted gate's run folder, including the record of its process group, in the same call that reports the halt, and the census never sees finalize drives at all. The halt is the one moment docket can still see the leftover, and today finalize's halt message does not even say the supervisor died. Build gates keep their run folder, but nothing tells the human either.

None of the 237 gate drives since 2026-09-29 had a supervisor death, so the fix is visibility only: no new refusal or block, in line with ADR-0134's rejection of making cancel wait for a leftover.

## What changes

Warn at the death halt, using the leftover check 0492 already built:

- **The gate driver checks at the halt.** When a drive halts because its supervisor died, it runs the read-only leftover check on that run. Only a clear "leftover" answer adds the informational `tree-survives:<drive>:<pgid>` finding to the halted drive, stored with the halt. Any other answer leaves the halt exactly as today. Outcome and cause never change.
- **Finalize says so.** The finding goes into the gate report's existing `teardown_finding` field, and the halt message names the supervisor death and, when a suite is still running, tells the human to wait until `pgrep -lg <pgid>` prints nothing before re-running finalize. Disposition, reason, and result are unchanged; the existing relay carries the message to the PR comment and `## Finalize blocked`.
- **Build relays it.** One sentence of `docket-build` skill text copies a halted drive's finding into the halt report, so it reaches `## Run halted`.
- **Docs and decision record.** The glossary's `tree-survives` entry covers halts too; finalize's gate-failure reference gets one sentence; ADR-0134 gets a dated Update note. No new ADR.

Failure posture: no new halt, block, refusal, or signal. A pinned, mutation-checked test asserts that every probe answer yields the same halt outcome, cause, and finalize result; only the finding and message differ.

## Out of scope

- Refusing or delaying a gate start while a leftover runs (rejected at grooming: a new block path).
- A `tree-survives` line from `run.start --resume` (rejected at grooming: its reader is the coordinator agent).
- Keeping a halted gate's run folder on disk.
- Stopping or signalling a leftover suite.
- `evidence.recertify`'s reporting (it drives every slice in one process, so a dead supervisor always reads unclear).
- Raw `gate.launch` runs, and 0492's accepted gaps 2 and 4.
- Process leaks inside individual tests (`t.Cleanup` hygiene).

## Reconcile log

### 2026-10-04

2026-10-04: Reconciled against origin/main ecbb32f19. The spec (groomed today) still matches current code: `ProbeLeftover` exists with the launch census as its only caller (`internal/gatedrive/reconcile.go`), `driveSlice`'s death branch calls `proveNoTreeSurvives` without probing the group, finalize removes the halted run root unless `haltedRunRootHoldsUnexitedRun`, and `mapDriveHaltCause` maps `supervisor-died` to `unavailable`. No intervening change touched this area. Scope unchanged.
