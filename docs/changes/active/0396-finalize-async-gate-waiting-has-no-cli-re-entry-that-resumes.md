---
id: 396
slug: 'finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes'
title: 'finalize async gate WAITING has no CLI re-entry that resumes the same drive'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-09-02'
updated: '2026-09-02'
depends_on: []
stacked_on:
related: [342, 364, 375]
discovered_from: [364]
adrs: [98, 105]
spec: 'docs/superpowers/specs/2026-09-02-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes-design.md'
plan: 'docs/superpowers/plans/2026-09-02-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-02T11:05:11Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-02-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-02-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes-design.md) |
| Plan | [2026-09-02-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-02-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes.md) |
| ADRs | [ADR-0098](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0098-structured-gate-waiting-and-ownership-handoff.md), [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md) |
<!-- docket:artifacts:end -->

## Why

During finalize of change 0364, the async local gate returned `disposition: waiting` from `finalize.rebase-continue` with a continuation (drive id + owner generation). There is no CLI path that both advances THAT drive to terminal and records finalize evidence through the finalize seam:

- `finalize.rebase` / `finalize.rebase-continue` never expose the continuation, so re-entering `finalize rebase` reaches the gate seam with an empty continuation, mints a fresh run root, and calls the driver's `Start` every time — a NEW drive re-running the whole suite instead of resuming the live one. Repeated re-entry never converges and leaves orphaned detached-supervisor runs under `$TMPDIR/docket-finalize-gate-*`.
- The generic `docket gate drive advance` does advance the same drive, but it is not the finalize seam, so a PASSED there mints no finalize evidence block; evidence has to be recovered out-of-band via `evidence.record --run <raw run dir>`.

The `docket-finalize-change` skill has no `waiting` route in its step-3 disposition table at all, which is why the hand-stitched workaround grew. Net effect: every finalize whose gate takes more than one slice either burns extra full suite runs or is stitched by hand, and it is easy to get wrong (observed live on 0364).

The application layer already carries the resume path (`FinalizeRebaseRequest.Continuation` → recovery path → seam `Advance`); it is unreachable only because the CLI never populates it, and it cannot be populated keylessly because the owner generation is a caller-held secret by design (ADR-0098).

The stub also reported the `finalize.rebase` JSON truncating the attempt token (8-char vs the receipt's 12-char base suffix), causing an `attempt-token-mismatch` on continue. Grooming found no truncation in the code (`newRebaseAttempt` mints 12 hex characters and every result copies it verbatim); the claim is recorded as unverified in the spec and is settled by a round-trip test at build time.

## What changes

Persist the finalize local gate's continuation in the owned rebase receipt, so the WAITING re-entry is the identical `finalize.rebase` invocation (same `--id --version --head`) with no new operation and no new flag. `rebase-receipt.json` gains a `gate_drive_id` / `gate_owner_generation` pair, written on WAITING and cleared in the same call that maps any terminal (passed, failed, halted). The receipt-recovery path reads the pair and advances the SAME drive; on PASSED the finalize seam mints the evidence block and returns `rebased`/`unchanged` as today. The owner generation becomes receipt-private: the `waiting` document keeps `gate.continuation.drive_id` and drops `generation`. `FinalizeRebaseRequest.Continuation` is retired. The `docket-finalize-change` skill gains an explicit `waiting` route bound to re-running `finalize.rebase` (never `gate drive advance`). A short ADR refines ADR-0098 for finalize. The attempt-token claim is verified by a CLI round-trip test and fixed only if it reproduces. Design detail is in the linked spec.

## Out of scope

- Gate supervisor/driver mechanics (`gate.drive.*`, `internal/gatedrive`) — including #0375 (`gate drive start` idempotency), a sibling this design sidesteps rather than depends on.
- The two-agent resolver/repair flow, and build's gate.
- Cleaning up run roots orphaned by earlier finalize runs.
- The repair path's post-repair re-gate, which today uses raw `gate.launch`/`gate.observe` instead of the driver (at odds with ADR-0098) — discovered work, to be captured as its own change.

## Reconcile log

### 2026-09-02

2026-09-02 — Reconciled against current reality. All symbols the spec relies on exist as described in internal/app/finalize_rebase.go (FinalizeRebaseRequest.Continuation, GateContinuation, composeLocalGate(...cont GateContinuation), recoverFromReceipt, mapContinuedRebase, newRebaseAttempt) and internal/workspace/rebasereceipt.go (RebaseReceipt, validateRebaseReceipt, WriteRebaseReceipt). ADR-0098 present. newRebaseAttempt mints `<stamp>-<baseHead[:12]>` (12 hex) — confirming the §6 attempt-token-truncation claim is NOT reproduced in the minting code; it stays unverified and is settled by a CLI round-trip test at build time (fix only if it reddens). The docket-finalize-change skill has no `waiting` route today (only an unrelated 'waiting for the safety-net sweep' phrase in its description) — the gap the spec fills. Related: 342/364 done, 375 (gate drive start idempotency) still proposed and correctly treated as a sidestepped sibling, not a dependency. No scope change; design holds as written. Discovered work to report (already flagged out-of-scope in the spec): the repair path's post-repair re-gate uses raw gate.launch/gate.observe instead of the driver, at odds with ADR-0098 — capture as its own change.
