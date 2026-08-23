---
id: 339
slug: retire-the-gate-run-sh-launch-liveness-stop-facade-now-that
title: 'Retire the gate-run.sh launch/liveness/stop facade now that the native Go-v1 gate is canonical (collapse the shared docket-liveness.sh seam with runner-dispatch.sh)'
status: 'in-progress'
priority: medium
type: refactor
created: 2026-08-22
updated: '2026-08-23'
depends_on: [338]
stacked_on:
related: [284, 314]
discovered_from: [338]
adrs: []
spec: docs/superpowers/specs/2026-08-23-retire-gate-run-facade-design.md
plan: 'docs/superpowers/plans/2026-08-23-retire-gate-run-facade.md'
results:
trivial: false
auto_groomable:
branch: 'feat/retire-the-gate-run-sh-launch-liveness-stop-facade-now-that'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-23T20:48:32Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-23-retire-gate-run-facade-design.md` |
| Plan | `docs/superpowers/plans/2026-08-23-retire-gate-run-facade.md` |
<!-- docket:artifacts:end -->

## Why

**Discovered from change 0338** (the gate-observe two-serialization seam). 0338 converges the gate's
`observe` **output format** on native protocol-v1 JSON and retires the shell `state=<name>` text
contract, but it deliberately draws its boundary at the observe *format* and the one caller loop.
It leaves a larger cleanup unaddressed and explicitly out of its scope: the `gate-run.sh` shell
facade still exists as a second implementation of gate launch/observe/stop alongside the landed Go-v1
native gate (`docket gate launch|observe|stop`, `internal/app/gate.go`).

The facade is not only an observe-format emitter. It also carries the **detached-launch and
liveness-checking machinery** it shares with `runner-dispatch.sh` through the extracted predicate in
`scripts/lib/docket-liveness.sh` (`docket_group_alive_and_ours`). So once 0338 lands and the observe
format is JSON, `gate-run.sh` and the native Go gate are two launch/liveness paths for one job,
reconciled by convention rather than by a single implementation. That duplication is exactly the
shape that drifted before (the liveness predicate had already diverged between `gate-run.sh` and
`runner-dispatch.sh` on the empty-token conjunct, which is why change 0284 extracted the shared lib).

## What changes

Fully retire the `gate-run.sh` facade — the native Go-v1 gate is the sole launch/liveness/stop
supervisor (design settled 2026-08-23; detail in the linked spec):

- Delete `scripts/gate-run.sh` + `scripts/gate-run.md`; drop the `WRAPPED_OPS` entry. No wrapper,
  no shim — the 0338 posture, applied to the remaining verbs.
- Migrate `docket-build`'s gate posture (the last skill-level caller) to
  `docket gate launch`/`stop` and the protocol-v1 JSON vocabulary.
- Move the orphaned caller guidance (canonical loop, state/retryability vocabulary, per-platform
  capability note) into `skills/docket-build/references/gate-execution.md`, with a recorded
  evidence-carryover note for the per-harness verdicts.
- Keep `scripts/lib/docket-liveness.sh` with `runner-dispatch.sh` as sole consumer; rewrite the
  ownership prose only. Migrating `runner-dispatch.sh` natively is out of scope (future change).
- Tests: delete the facade's tests after sorting subject-mechanics guards (die) from
  posture-prose guards (move to `test_gate_execution_posture.sh`).

**Sequenced after 0338** (done) — the observe format converged on JSON there. Boundary: the
launch/liveness/stop facade seam and its shared lib, NOT the observe-format seam 0338 owns.

## Reconcile log

### 2026-08-23

2026-08-23 — Reconciled against origin/main @ 38d9206f. Dependency 0338 (observe→JSON convergence) is `done`: `gate-run --observe` already refuses with a pointer to the native `docket gate observe --json`, so the launch/liveness/stop facade seam this change targets is exactly the residual 0338 left out of scope. Confirmed current reality still matches the settled spec: `scripts/gate-run.sh` + `scripts/gate-run.md` exist on origin/main and `gate-run` is in `WRAPPED_OPS` (`scripts/docket.sh`); the native Go-v1 gate (`docket gate launch|observe|stop`) is canonical; `skills/docket-build/SKILL.md` is the last skill-level caller of `gate-run --launch`/`--stop`; `skills/docket-build/references/gate-execution.md` exists as the destination for the orphaned caller guidance; `scripts/lib/docket-liveness.sh` is shared by `gate-run.sh` and `runner-dispatch.sh` (the latter its future sole consumer); the facade tests `tests/test_gate_run.sh` and `tests/test_gate_run_stop.sh` and the posture test `tests/test_gate_execution_posture.sh` all exist. Maintained-source `gate-run` references to sweep at build time (derived by whole-repo grep, per the house rule): `scripts/docket.md`, `scripts/docket.sh`, `scripts/lib/docket-liveness.sh`, `scripts/runner-dispatch.md`, `scripts/runner-dispatch.sh`, `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-execution.md`; frozen records (archived changes, results, merged plans, prior specs, Accepted ADRs) stay untouched. No scope, relation, or design changes required — proceeding to plan and build as specified.

## What happened

The plan (`docs/superpowers/plans/2026-08-23-retire-gate-run-facade.md`) is built on a green-per-commit-boundary architecture with deliberate transient duplication. Task 1 ("Move the orphaned caller guidance into gate-execution.md") edits `skills/docket-build/references/gate-execution.md` — Step 2 removes the `scripts/gate-run.md` reference from the capability-5 parenthetical, and Step 4 deletes the `docket.sh gate-run` facade sentence from the "One mitigation…" paragraph. But `tests/test_gate_execution_posture.sh` asserts on exactly those two edited blocks:

- assert ~590-591: `grep -qF -- "gate-run.md" <<<"$cap5_blk"` (block sliced from gate-execution.md's `## The six required capabilities`) — reddened by Task 1 Step 2.
- assert ~605-606: `grep -qE "docket\.sh gate-run" <<<"$mitigation_blk"` (the "One mitigation…" paragraph) — reddened by Task 1 Step 4.

The retarget of those two asserts is assigned to **Task 4** (plan lines 139-140), which the plan folds into **Task 2's** later commit. So Task 1's commit boundary cannot be green in isolation, contradicting Task 1 Step 6's "Expected: all PASS" and the plan's own green-per-commit invariant. The plan author annotated the transient posture-test red for Task 2 ("The posture test is expected RED here") but did not notice Task 1's gate-execution.md edits trigger the same red one commit earlier.

The Task 1 worker (docket-build-standard) produced correct content (fence byte-identical to gate-run.md's; state vocabulary read from `internal/app/gate.go`; Setsid clause quoted from `internal/process/launch.go`; genassets regenerated cleanly) and, per contract, returned BLOCKED without committing rather than fudging its Step 6 gate.

## Why the build role halted rather than proceeding

Per the docket-build contract this is a planning defect at a task/commit boundary, which is explicitly "not something to re-cut here"; a worker BLOCKED on contradictory requirements is a halting condition. The three available repairs are all outside the build role's authority: (a) fold Tasks 1+2+4 into one commit; (b) move the two posture-test asserts (591, 606) into Task 1's Files/scope so the retarget lands with the content move; or (c) reclassify the posture test as an expected-red neighbor for Task 1 (drop it from Task 1 Step 6's gate), accepting transient red until Task 2's commit — consistent with docket-build running the full suite only once at the end. Committing the worker's uncommitted edits, or re-dispatching Task 1 to repair its own return, are both forbidden by the contract.

## State left behind (for the resume path)

- Feature branch `feat/retire-the-gate-run-sh-launch-liveness-stop-facade-now-that` @ `bb66cfd0` — only the plan commit; no build commits landed.
- The Task 1 worker's correct content edits are **uncommitted** in the feature worktree (`skills/docket-build/references/gate-execution.md`, `internal/assets/embedded/manifest.json`, `internal/assets/embedded/tree/skills/docket-build/references/gate-execution.md`). They were not adopted or committed by this run. A resume may keep them (re-verify first) or discard and re-run Task 1 under a corrected plan.
- Claim, reconcile (reconciled: true), and the attached plan are all landed on the metadata branch.

## Suggested human action

Amend the plan to remove the Task 1 commit-boundary contradiction (option a, b, or c above — b is the smallest edit: add `tests/test_gate_execution_posture.sh` to Task 1's Files and move the assert-591/606 retargets from Task 4 into Task 1), then resume change 0339 by id via the halted-resume path.
