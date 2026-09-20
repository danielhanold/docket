<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0439 — Leaked worktree gate-admission slot stuck in "executing" blocks finalize.rebase with a swallowed unavailable error](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-20-0439-leaked-worktree-gate-admission-slot-stuck-in-executing-block.md)**
<!-- docket:backlink:end -->

# Change 0439 — Explain occupied worktree gate slots and expose existing recovery

Approved design. Implementation and implementation planning are separate from grooming.

## Goal

When finalize cannot start its local gate because a worktree admission slot is occupied, report the actual refusal, identify the incumbent, and explain the existing recovery route. A completed raw run must be recoverable through the documented `gate.stop` operation without inspecting or editing Docket's private JSON records.

## Evidence and corrected diagnosis

Design baseline: main at `30dcb069f6cf5653250b3ffb63ad5f3a0b75516a`.

- `internal/gatedrive/admission.go`, `reserveWorktreeExecution`: the epoch fence precedes the slot-state switch. Reserved, executing and stopping slots refuse with `worktree-busy`; unresolved slots refuse with `unresolved-execution`. This is deliberate admission policy, not evidence the incumbent process is alive.
- `internal/app/gate.go`, `GateLaunch`: a raw launch reserves and confirms a slot without creating a drive document. `releaseRawSlotForStop` releases only a matching raw slot after the existing stop outcome proves teardown. It explicitly excludes driven slots. `GateRecover` classifies process records but does not release admission slots.
- `internal/app/gate_test.go`, `TestGateLaunchSecondRefusedWhileFirstLives` and `TestGateStopReleasesRawSlot`: the current tests explicitly require a completed raw run to retain its slot until stop, and demonstrate that stop of an already-completed run releases it.
- `internal/app/gate_drive.go`, `mapDriveResult` and `ownershipNextAction`: the typed refusal survives here, but a busy message incorrectly equates slot occupancy with a running process. `incumbentLocator` in `gate.go` already projects safe drive/run identifiers for raw-launch refusals; the driven-start refusal lacks that incumbent context.
- `internal/app/finalize_rebase.go`, `mapDriveOutcome`: any result without a drive document becomes a bare unavailable halt. `LocalGateResult`, `GateReport`, the halted composition branch and `FinalizeRebaseResult.HumanText` do not preserve the underlying diagnostic.
- Change 0375 and ADR-0118 deliberately require explicit stop for raw-slot teardown. Change 0428 and ADR-0120 restrict history cleanup to historical drive assessment; zero historical blockers is not a declaration that the current worktree slot is free.
- Changes 0435 and 0437 are done. They repair cancelled-epoch retirement and revoked-epoch admission; neither changes standalone raw recovery. Change 0368 is done and supplied the incident; adjacent 0438 concerns rebase receipt recovery and remains separate.

The incident report says stop released an executing slot with no drive document. On the traced implementation that is consistent with a raw launch, not proof of a leaked driven gate. Original incident records have not been independently replayed. The supported raw-launch lifecycle reproduces the relevant state by construction; implementation must cover that sequence with a behavioral regression before claiming a repair. If original records instead establish a driven orphan, report that discrepancy rather than broadening recovery authority under this spec.

Relevant learnings: groomed root causes are hypotheses; probe errors are not absence; printed remedies must be valid in the state that produced them. ADR-0087 and ADR-0095 remain governing process-evidence constraints.

## Design

### Reuse the current admission diagnostic path

Enrich the existing typed admission refusal with a small credential-free projection of the incumbent already read under the admission lock. Reuse the existing `incumbent-drive:<id>` / `incumbent-run:<id>` locator convention and `GateDriveResult` reason, message, stage and locator fields. Use stage `worktree-admission` for current-slot refusals; keep existing legacy-inventory diagnostics distinct.

Project only validated identity and the facts needed for the applicable remedy: raw versus driven ownership, whether an epoch owns the slot, and the exact recorded raw-run directory where that route is valid. Do not expose reservation tokens, owner generations, capabilities, command arguments, environment, or arbitrary record contents. Reuse/refactor the incumbent projection already used by raw launch rather than adding a separate inventory. Diagnostic facts come from the refusal's incumbent snapshot; a later changed slot must never be represented as its cause.

For a confirmed standalone raw incumbent, explain that the slot remains occupied until explicit teardown, provide `gate.observe` for its recorded run directory, and name `gate.stop` as the release route. State that stopping a still-running run is cancellation; an already-completed run can be stopped to settle its slot. Do not declare that every terminal state guarantees release: the existing stop operation decides whether teardown is proven. Render paths as safely quoted command operands in human guidance, using existing formatting conventions.

For driven or epoch-owned incumbents, preserve the owning drive/parent continuation or explicit run-cancellation guidance. Never suggest raw stop as a way to release a driven slot, fabricate missing credentials, or recommend an old epoch as a bypass. Missing/ambiguous identity yields an honest bounded diagnostic without a guessed command. Replace the blanket claim that busy means currently running with slot-occupancy wording. Diagnosis performs no process signaling, slot release, or launch retry.

### Carry the refusal through finalize

Carry the existing bounded reason, message, stage and locator through `LocalGateResult` into optional diagnostic fields on `GateReport`, using the same field meanings as `GateDriveResult`. Preserve the existing top-level blocked / gate-halted result and closed halt-cause vocabulary; unavailable may remain the coarse classification, but must no longer be the only explanation when a precise refusal exists.

Render the underlying reason, locator and applicable remedy in both JSON and human output. Apply the shared mapping to initial and continued local-gate execution. A genuinely detail-less failure retains the current generic fallback. Do not turn an admission refusal into suite failure, gate evidence, a continuation credential, a charged attempt, or permission to retry automatically. Do not overload `run_dir`, which describes the requested gate run, with an incumbent's path; the recovery guidance can carry the quoted incumbent path.

Update the maintained gate/finalize documentation where recovery is explained. Clarify that history cleanup assesses historical drives and that process recovery alone does not release a current raw admission slot. No expanded cleanup implementation is needed.

## Alternatives

1. **Selected: actionable diagnostics plus existing explicit recovery.** Fixes the demonstrated operator failure while preserving the established lifecycle and authority boundaries.
2. **Automatically release apparently dead incumbents during admission or recover.** Rejected: death alone is insufficient under ADR-0118, and no evidence establishes that the existing explicit teardown operation is inadequate. This would introduce recovery policy and concurrency obligations beyond the observed diagnostic defect.
3. **Expand history cleanup into current-slot recovery.** Rejected: duplicates existing stop/cancel responsibilities and changes the deliberately bounded historical-assessment surface.

## Acceptance and validation

1. In an isolated registered worktree, launch a raw command, await its real passed or failed terminal result, and attempt a finalize local gate. The attempt stays blocked, spawns no second process, and reports worktree-busy, the exact incumbent locator and executable observe/stop guidance in JSON and human text. The admission record remains unchanged by diagnosis.
2. Follow the reported existing stop remedy. The matching raw slot becomes released; a subsequent legitimate gate can admit. Preserve the completed run's original result and verify repeated stop does not affect a successor's slot.
3. A live raw incumbent receives conditional wait/explicit-stop guidance without being stopped or described as dead. A driven or epoch-owned incumbent never receives misleading raw-release or stale-epoch bypass guidance.
4. Missing/unreadable/corrupt identity and reserved/unresolved slots remain fail-closed. No guessed run path or owner credential appears. Test path quoting and credential omission.
5. Test the refusal-to-finalize mapping and human presenter directly, including an existing legacy-inventory refusal and a truly generic failure. Known detail survives; ordinary passed, failed and waiting outcomes remain unchanged.
6. Extend the existing topical admission/gate/finalize tests; do not introduce another test harness or broad source guard. At implementation, run the complete source-based suite through the configured build gate (currently `go run ./cmd/docket development test`), and review its budget findings. No suite execution is needed for this grooming-only specification.

## Scope and metadata

No new CLI command, configuration, persistent schema, lifecycle state, daemon, liveness implementation, automatic recovery policy, retry layer, or ADR is required. Preserve admission, stop, recovery, cancellation, epoch fences, suite budgets and historical classification semantics.

Keep priority medium and type fix. Related changes: 0368, 0375, 0428, 0435, 0437. Cite ADRs 0087, 0095, 0118 and 0120. No dependencies or stacking: the relevant foundational changes are already done. The proposal and linked specification share this diagnostic scope.
