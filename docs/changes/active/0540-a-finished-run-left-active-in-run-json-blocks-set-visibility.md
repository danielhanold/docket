---
id: 540
slug: 'a-finished-run-left-active-in-run-json-blocks-set-visibility'
title: 'A finished run left active in run.json blocks set-visibility and cannot be cancelled'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-08'
updated: '2026-10-08'
depends_on: []
stacked_on:
related: [541, 345, 532]
discovered_from: []
adrs: [124, 128]
spec: 'docs/superpowers/specs/2026-10-08-a-finished-run-left-active-in-run-json-blocks-set-visibility-design.md'
plan: 'docs/superpowers/plans/2026-10-08-a-finished-run-left-active-in-run-json-blocks-set-visibility.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/a-finished-run-left-active-in-run-json-blocks-set-visibility'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-08T01:49:43Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-08-a-finished-run-left-active-in-run-json-blocks-set-visibility-design.md](../../superpowers/specs/2026-10-08-a-finished-run-left-active-in-run-json-blocks-set-visibility-design.md) |
| Plan | [2026-10-08-a-finished-run-left-active-in-run-json-blocks-set-visibility.md](../../superpowers/plans/2026-10-08-a-finished-run-left-active-in-run-json-blocks-set-visibility.md) |
| ADRs | [ADR-0124](../../adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md), [ADR-0128](../../adrs/0128-resume-arms-mint-an-arm-time-epoch-that-run-cancel-can-cance.md) |
<!-- docket:artifacts:end -->

## Why

A run whose tracker verdict is already final can still leave its `run.json` at `active`. Only a keyed `run-done … run-complete` verdict closes `run.json`. Every other terminal verdict writes `terminal: true` into `record.json` and leaves `run.json` `active`. The repository-wide live-run check reads `run.json`, so `docket repository set-visibility` refused with `invalid-state (needs-review): a run or gate is still live` and named `docket run cancel` as the remedy. For a run that claimed nothing (`run-done <key> no-attributable-claim`), cancel always refuses with `claim-unconfirmed`. Neither `docket run verdict <key>` nor `--unattributed` moves `run.json`, so no documented command can settle the run, and the only way out was hand-editing `run.json`.

This is not rare. At grooming on 2026-10-07 this machine held three stranded `active` runs, and a test repository held a fourth that had been hand-edited. Two of those runs did build their change but claimed without `--run-context` (#541), so every such attribution failure also strands a live-looking run.

## What changes

- Every keyed `run-done` verdict (`run-complete`, `no-attributable-claim`, `run-unclaimed`) retires the run through the existing observation-only closeout, so `run.json` ends `completed`. Report lines are unchanged on success. A blocked closeout reports `run-stop … run-tracker-unavailable completion-unaccounted` and is replayed by repeating the verdict.
- Halted, stop, retry, and continue verdicts keep today's behavior. Observe mode stays read-only, and a cancelling, cancelled, or superseded run is never relabelled.
- `set-visibility`'s live-run list names a remedy that works: the cancel command when `run cancel` would accept the run, `docket run verdict <key>` when it would refuse, and the by-hand note for a `cancelling` run that cancel cannot settle.
- Record an ADR that extends ADR-0124 Rule 1 to every keyed `run-done` verdict.
- Regression tests reproduce the incident (terminal `record.json`, `active` `run.json`, no claim) and guard each path. Existing stranded runs are cleared by re-running `docket run verdict <key>`, with no migration.

The linked spec holds the evidence, the per-verdict table, the remedy matrix, and the test list.

## Out of scope

Why attribution fails: the child claiming without `--run-context` (#541) and command-launched dispatch (#345). Changing `run cancel`'s ownership proofs (ADR-0128). How halted and stop verdicts treat `run.json`. Any new run state, schema change, or change to the run tracker's lock and generation scheme.

## Reconcile log

### 2026-10-08

Reconciled against origin/main 1ace73350. The spec's code citations still hold: RunVerdict only routes VerdictRunComplete through runTrackerCompleteRun; VerdictRunUnclaimed and runTrackerOwnershipDone persist record.json only; liveRunsUnder hard-codes runCancelCommand for active/completing. No related change (#541, #345) has landed, and no archived change touched this path since #532. Scope unchanged.
