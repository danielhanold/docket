---
id: 439
slug: 'leaked-worktree-gate-admission-slot-stuck-in-executing-block'
title: 'Leaked worktree gate-admission slot stuck in "executing" blocks finalize.rebase with a swallowed unavailable error'
status: 'done'
priority: 'medium'
type: 'fix'
created: '2026-09-19'
updated: '2026-09-20'
depends_on: []
stacked_on:
related: [368, 375, 428, 435, 437]
discovered_from: []
adrs: [87, 95, 118, 120]
spec: 'docs/superpowers/specs/2026-09-20-leaked-worktree-gate-admission-slot-stuck-in-executing-block-design.md'
plan: 'docs/superpowers/plans/2026-09-20-leaked-worktree-gate-admission-slot-diagnostics.md'
results: 'docs/results/2026-09-20-leaked-worktree-gate-admission-slot-stuck-in-executing-block-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/leaked-worktree-gate-admission-slot-stuck-in-executing-block'
pr: 'https://github.com/danielhanold/docket/pull/317'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-20-leaked-worktree-gate-admission-slot-stuck-in-executing-block-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-20-leaked-worktree-gate-admission-slot-stuck-in-executing-block-design.md) |
| Plan | [2026-09-20-leaked-worktree-gate-admission-slot-diagnostics.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-09-20-leaked-worktree-gate-admission-slot-diagnostics.md) |
| Results | [2026-09-20-leaked-worktree-gate-admission-slot-stuck-in-executing-block-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-09-20-leaked-worktree-gate-admission-slot-stuck-in-executing-block-results.md) |
| ADRs | [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0120](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0120-historical-gate-drive-schemas-are-assessed-never-executed.md) |
<!-- docket:artifacts:end -->

## Why

During finalize of change 0368 (PR #313, 2026-09-19), a completed test run still occupied its worktree admission slot. Finalize repeatedly reported only an unavailable halt, leaving the operator to discover that stopping the already-completed run released the slot.

Tracing the implementation and ADR-0118 corrects the initial leak hypothesis: raw gate launches deliberately retain their slot until explicit stop proves teardown, and they do not create a drive document. The reported successful stop is consistent with that existing lifecycle. The concrete defect is that finalize drops the typed admission refusal and the busy message assumes the occupying process is still running. Historical-drive cleanup does not inventory current admission slots, so its zero-blocker result does not establish that the worktree is free.

The existing stop machinery already provides recovery. The change should make the blocking run and applicable remedy discoverable without private-record inspection or new recovery policy. The original incident has not been independently replayed; the linked spec requires a behavioral regression of the supported raw-launch sequence.

## What changes

- Preserve the actual admission refusal through finalize's JSON and human reporting, identifying the occupying run or drive with existing credential-free locator conventions.
- Explain slot occupancy accurately and provide applicable guidance through existing observe, stop, continuation, or run-cancellation operations. For a confirmed standalone raw run, identify its recorded run directory and explain how explicit stop settles its slot, including after completion.
- Keep diagnostic facts tied to the incumbent that caused the refusal. Missing or ambiguous identity must not produce guessed commands, leaked credentials, or permission to retry automatically.
- Clarify the existing boundaries between current admission slots, process recovery, and historical-drive cleanup.
- Add focused regression coverage for completed and live raw runs, driven and epoch-owned incumbents, uncertain identity, diagnostic propagation, and safe reuse after the existing stop remedy.

The linked spec records the implementation trace, prior changes and ADRs, alternatives, and acceptance criteria. Changes 0435 and 0437 are merged and remain the owners of the separate cancelled-epoch retirement and admission repairs.

## Out of scope

Automatic slot release, changing admission or teardown policy, expanding history cleanup into current-slot recovery, and redesigning cancellation or epoch fences. No new CLI command, configuration, persistent schema, lifecycle state, liveness implementation, daemon, retry layer, or architecture decision. Rebase-receipt recovery remains change 0438; implementation and implementation planning are separate work.

## Reconcile log

### 2026-09-20

2026-09-20: Reconciled against current reality. Spec baseline (main @ 30dcb069) equals current main HEAD, so the traced code is unchanged: internal/gatedrive/admission.go reserveWorktreeExecution, internal/app/gate.go GateLaunch/releaseRawSlotForStop/incumbentLocator/GateRecover, internal/app/gate_drive.go mapDriveResult/ownershipNextAction, and internal/app/finalize_rebase.go mapDriveOutcome/LocalGateResult/GateReport/HumanText all present as described. Related changes 0435 and 0437 are done (cancelled/revoked-epoch repairs), leaving standalone raw-slot recovery untouched — this change's diagnostic scope stands. Adjacent 0438 (rebase-receipt recovery) remains separate. No scope change; design remains valid as groomed.
