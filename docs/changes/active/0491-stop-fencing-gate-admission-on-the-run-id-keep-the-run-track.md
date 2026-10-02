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
