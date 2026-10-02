---
id: 491
slug: 'stop-fencing-gate-admission-on-the-run-id-keep-the-run-track'
title: 'Stop fencing gate admission on the run id; keep the run tracker for attribution only'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: [490]
stacked_on:
related: [375, 422, 435, 437, 441, 463, 467]
discovered_from: []
adrs: [111, 118, 124, 128]
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
| ADRs | [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0124](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md), [ADR-0128](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0128-resume-arms-mint-an-arm-time-epoch-that-run-cancel-can-cance.md) |
<!-- docket:artifacts:end -->

## Why

The run tracker does two jobs.

Its core guards against unreliable child reports: the run key, binding the dispatch's run context to its claim, retry-once accounting, and observe mode. At worst it produces misleading verdicts; it has caused no run halts.

Its second job is fencing gate admission. Changes 0375, 0437, and 0467 (ADR-0124/0128) added it:

- The run id is stamped on worktree slots and scopes.
- `runLaunchGate` re-checks it at start, revalidation, recovery, and takeover.
- `run.verdict` carries a closeout step that exists because slots owned by a run block finalize's gate.

That coupling causes stuck builds whenever a token goes missing on the way:

- `runLaunchGate` refuses with `stale-run-id` when the run record's worktree is empty, wrong, or unbound, not only when the run was superseded. A claim made without `--run-context` never binds the worktree, so every later gate start is refused, and `run.cancel` refuses `claim-unconfirmed`.
- A slot keeps the run id it was stamped with. Any start without that id is refused `stale-run-id`. That includes fix-loop workers, a start without a scope, and finalize before the closeout runs.
- Every `stale-run-id` refusal gives the same next action, "the run was superseded by a resume" (`fenceNextAction` in `internal/app/gate_drive.go`), even when the real cause is an unbound worktree or a missing run id. Agents are sent hunting for a resume that never happened.

The fix chain: 0375 → 0435 → 0437 → 0441 → 0463 → 0467.

Once dependency 0490 replaces the stored slot with a lock held by the running process, the run id has nothing durable left to fence.

## What changes

- **Remove the fence.** Drop run-id checks from gate admission and launch: `runLaunchGate`, stamping the run id on slots, the `stale-run-id` refusal, `settleStaleReleasedRun`, and the revocation checks at launch and recovery. Also drop the `run.verdict` closeout steps that exist only to release slots.
  - **Already done by 0489:** the revocation check at takeover is gone. 0489 (PR #365) deleted `Takeover`'s run-revocation check, `SetRunRevokedResolver`/`runRevokedResolver`, and the recovery scope's `RunID`; the outer recovery scope never carried a run id. Don't re-plan it here — see 0489's spec, "Effect on follow-ups".
- **Simpler cancel.** `run.cancel` becomes: mark the run cancelled, then stop the run's registered supervisor process groups. No admission fence.
- **Keep the core.** Keep attribution, retry accounting, observe mode, the `## Run halted` marker, and resume admission (one live run per worktree), as far as none of them depends on the slot.
- **Decide on `--run-id`.** Either keep `--run-id` on `gate.drive.start` as a locator for evidence and attribution, or drop it. Then update the run-tracker blocks in CLAUDE.md and AGENTS.md, cursor-rules, and the skills to match.
- **ADRs and related work.** Supersede or amend ADR-0118/0124/0128 as needed. Re-check 0422 (blocked: retries over-counted) against the slimmer verdict path.

Accepted loss: an agent left over from a cancelled run could still start a test suite in that worktree. The worktree lock from 0490 still prevents two suites from running at once.

## Out of scope

- Retiring the run tracker itself, or its attribution and retry model.
- Changing the `run-*` report vocabulary beyond lines that exist only for slot fencing.
- The workflow-mutation fence (`admitWorkflowMutation` over metadata transactions and PR/workspace publish), unless grooming shows it depends on the slot.

## Open questions

### A never-launched drive blocks a successful run's closeout (from 0490 review finding F3)

0490's deep review found this (finding F3, confirmed). It was deliberately not fixed in 0490 (PR #366); 0490's results file records it under "Known issues and follow-ups" and hands it to 0491. Nothing else tracks it, so it has to be decided while grooming this change. Code references below are to 0490's branch.

**How it happens**

1. A tracked `gate.drive.start` admits a drive. The drive record is written with the run's context hash and an admission token (or, for the single relaunch, a reservation), but the supervisor has not been started yet.
2. The CLI is killed in that window (Ctrl-C, coordinator interrupt, crash). No supervisor ever starts, so no worktree lock is taken and the worktree itself is free. The drive record stays in its reserved, never-launched state.
3. The build otherwise finishes, and the coordinator runs `run.verdict <key>`. The success closeout (`completeSuccessfulRun`, `internal/app/runtracker_complete.go`) moves the run `active → completing` and walks its drives with `ObserveRunLaunches` (`internal/gatedrive/reconcile.go`). That walk only observes. A drive proven never-launched (`reconcileFirstLaunch` for a first launch, `reconcileReservation` for a reserved relaunch) yields the finding `launch-pending:<drive>` and is never settled.
4. The verdict prints `run-stop <key> run-tracker-unavailable completion-unaccounted` with that finding. The run stays durably `completing`, and every repeat of the verdict gives the same answer.

**What the coordinator sees.** A `run-stop`, which forbids re-dispatch, on a run whose work is actually done. The finding names the drive but not the remedy, and nothing in CLAUDE.md, AGENTS.md or the skills tells an operator what to do.

**The only remedy today** is `run.cancel --key <key> --run-id <id> --reason <why>`. Cancel's walk (`ReconcileRunLaunches`) settles the never-launched drive HALTED `run-cancelled` (`settleNeverLaunchedFirstLaunch` / `settleNeverLaunchedCancelled`). The cost is that a successful build ends up recorded as **cancelled**, not complete.

**Why 0490 left it.** Every clean fix either changes the verdict's `run-*` report lines or changes what the success closeout is allowed to do. Both were outside 0490's scope.

**To decide while grooming:**

- **Does the success closeout survive 0491?** "What changes" drops the closeout steps "that exist only to release slots". After 0490 the closeout writes no worktree record. What is left is the success fence that releases the run's hold on the workflow-mutation fence (`admitWorkflowMutation`), which "Out of scope" excludes unless it depends on the slot. If the closeout, or its launch census, goes away, this problem goes with it; say so explicitly. If it stays, the problem stays and needs one of the fixes below.
- **If it stays, pick a fix:**
  - **(a) Let the success closeout settle a proven never-launched drive** under the held per-drive claim, the same way cancel already does. A proven never-launched drive is not running, and once settled it can never run, so it has no bearing on whether the run succeeded. The report lines stay as they are. Open points: the terminal label (it must not be `run-cancelled`), and that this breaks the closeout's "observation only" rule in the `runtracker_complete.go` header comment, which the ADR-0124 line of decisions rests on. `internal/gatedrive/reconcile_test.go` pins today's behaviour (observe mode leaves the never-launched drive `launch-pending` with its record untouched) and would flip.
  - **(b) Keep the closeout observation-only, but name the remedy.** When every finding is `launch-pending`, the `completion-unaccounted` line or its next action names `run.cancel`. This changes the report vocabulary, so the second "Out of scope" bullet would have to be widened. It still ends a good build as cancelled.
  - **(c) Leave the behaviour and document the manual remedy** in the run-tracker blocks of CLAUDE.md and AGENTS.md. This is the cheapest option and has the same cancelled-not-complete cost.
- **`resolution-unresolved:<drive>` stays fail-closed** under any option, because a reservation that can't be proven either way might be running. Confirm that.
- **Regression test:** admit a tracked drive, kill before launch, then run the keyed verdict, and assert the chosen outcome end-to-end through `run.verdict`, not only at the `gatedrive` layer.

Not part of this note: the sibling gap 0490's results file pointed at 0492, now its own change, 0493. Cancel clears a relaunch that halted without attaching using only the first run's directory, so a replacement supervisor that came up anyway is not stopped.
