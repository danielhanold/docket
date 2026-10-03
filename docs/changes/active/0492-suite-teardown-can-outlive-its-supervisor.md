---
id: 492
slug: 'suite-teardown-can-outlive-its-supervisor'
title: 'Suite teardown can outlive its supervisor'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-02'
updated: '2026-10-03'
depends_on: [490]
stacked_on:
related: [375, 491, 493]
discovered_from: [490]
adrs: [95, 132]
spec: 'docs/superpowers/specs/2026-10-02-suite-teardown-can-outlive-its-supervisor-design.md'
plan: 'docs/superpowers/plans/2026-10-03-suite-teardown-can-outlive-its-supervisor.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/suite-teardown-can-outlive-its-supervisor'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-03T06:12:08Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-02-suite-teardown-can-outlive-its-supervisor-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-02-suite-teardown-can-outlive-its-supervisor-design.md) |
| Plan | [2026-10-03-suite-teardown-can-outlive-its-supervisor.md](https://github.com/danielhanold/docket/blob/fix/suite-teardown-can-outlive-its-supervisor/docs/superpowers/plans/2026-10-03-suite-teardown-can-outlive-its-supervisor.md) |
| ADRs | [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0132](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0132-worktree-admission-is-a-supervisor-held-kernel-lock.md) |
<!-- docket:artifacts:end -->

## Why

A gate's test suite runs as a process tree: supervisor → `go run` → test runner → one test target per process group. Docket decides "the suite is gone" by looking only at the supervisor. Grooming 0490 found four ways the suite can keep running after docket thinks it stopped; 0490's ADR (ADR-0132) records them as accepted losses.

Tracing them at this groom:

1. **A supervisor killed alone (SIGKILL or a crash) leaves its suite running.** `run.cancel` reports `cancelled` and the worktree reads as free. This is the gap this change fixes.
2. **A KILL escalation can leave test targets running**, but only when the runner is stuck for more than 10s after a stop's TERM; the runner's own 5s grace normally finishes first.
3. **Finalize's single automatic relaunch trusts a dead supervisor.** The 2026-10-02 backlog review retargeted 0493 to retire that relaunch, which removes this gap.
4. **A graceful stop frees the worktree a moment before teardown ends**, usually milliseconds.

Nothing like this has happened in production. The harm is a false `cancelled`, CPU contention from an overlapping suite, and leftover processes that finish on their own.

## What changes

Teach docket to notice when a crashed gate's suite is still running, and say so, without killing anything:

- **A read-only leftover check** in the process layer: once a run's supervisor has exited, its suite counts as gone only when the supervisor's process group is empty. A populated group whose supervisor pid is gone is a **leftover**; anything ambiguous (a zombie supervisor, a reused process number, a probe error) is **unclear** and keeps today's behavior.
- **Cancel and the success closeout report a leftover** with an informational `tree-survives:<drive>:<pgid>` finding. The cancel disposition and the closeout verdict never change.
- **Docket never signals on this evidence.** A new ADR records the rule; ADR-0132 gets a dated Update note.

Failure posture: no new halt and no new block. The finding is information only, pinned by a mutation-checked test. Gaps 2 and 4 stay documented as accepted.

## Out of scope

- Finalize's automatic relaunch and `proveNoTreeSurvives` (0493 retires the relaunch).
- Stopping a leftover suite, or making cancel wait for one.
- The worktree lock and its holder model (0490), and checks at gate start.
- The run id and its fences (0491).
- Process leaks inside individual tests (`t.Cleanup` hygiene).

## Reconcile log

### 2026-10-03

2026-10-03 — Reconciled against main 710637ff4 (0490 and 0491 landed). 0491 retired the run tracker's run id but the process layer's per-run-directory `run_id` (manifest/dir agreement in `Observe`) remains, so the spec's validation rule still holds. `supervisorGone` / `proveRunDirsGone` in `internal/gatedrive/reconcile.go` and `groupAlive`/`processAlive`/`recoverGroupProbe` in `internal/process` exist as the spec describes. 0493 (relaunch retirement) is still proposed and untouched here; build 0492 apart from it. No scope change.
