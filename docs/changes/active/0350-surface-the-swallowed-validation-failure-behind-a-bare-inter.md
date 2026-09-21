---
id: 350
slug: surface-the-swallowed-validation-failure-behind-a-bare-inter
title: 'Surface the swallowed validation failure behind a bare internal-error in the transaction engine'
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-26
updated: '2026-09-21'
depends_on: []
stacked_on:
related: [309, 329]
discovered_from: [348]
adrs: [50, 55]
spec:
plan: 'docs/superpowers/plans/2026-09-21-0350-surface-swallowed-validation-failure.md'
results:
trivial: true
auto_groomable:
branch: 'fix/surface-the-swallowed-validation-failure-behind-a-bare-inter'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-21T07:35:29Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-09-21-0350-surface-swallowed-validation-failure.md](https://github.com/danielhanold/docket/blob/fix/surface-the-swallowed-validation-failure-behind-a-bare-inter/docs/superpowers/plans/2026-09-21-0350-surface-swallowed-validation-failure.md) |
| ADRs | [ADR-0050](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0050-backstop-checks-must-compute-not-reenumerate.md), [ADR-0055](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0055-exhaustive-vocabulary-mappings-require-array-pinned-set-equality.md) |
<!-- docket:artifacts:end -->

## Why

Early request validation returns a typed transaction.Failure through the engine's Go error channel with an empty disposition. The shared app mapper currently reports internal-error and the diagnosis helper returns nil, hiding the actual cause. A temporary real-engine reproduction against main 48e76b7c confirmed this for malformed expectations, invalid operation and idempotency keys, invalid target refs, and a missing loader.

The historical discovery during change 0348 remains valid context, but a full-length, well-formed incorrect object ID follows the existing contended path. A shortened or malformed object ID is the confirmed early-validation trigger; do not describe every wrong version as this bug.

## What changes

Trivial rationale: extend the existing diagnostic machinery from change 0329 to the engine's already-established early-error return shape. No new mechanism or policy is needed.

- In internal/app/planning.go, make mapOutcome handle an empty disposition through existing mapFailure(err). Keep unknown non-empty dispositions mapped to internal-error.
- Let failureStatus process an empty disposition with a non-nil error through its existing conversion, preserving stage, kind, detail, and wrapped cause. Empty disposition with nil error retains its existing internal-error/no-diagnosis behavior. Other dispositions retain their current behavior.
- The caller audit found one additional envelope builder: repairResultFromOutcome in internal/app/change_repair.go currently attaches failure only in its explicit failed arm. Attach the shared diagnosis in its default mapping path too, so early errors receive the same existing failure field. Derive consumers with a whole-repo search of mapOutcome/mapFailure/failureStatus and the result-builder family.
- Update the helper comments to describe both supported error shapes. Keep the transaction engine and its validation rules unchanged.

Verification: extend existing app helper tables for empty disposition plus typed/wrapped, untyped, and nil errors; keep existing failed/unknown/non-failed controls. Add a real-engine malformed-version regression passed through claimResultFromOutcome and a focused repair-result regression asserting the populated failure fields. Remove each new mapping/propagation branch in turn and confirm the relevant tests fail, with caching disabled. Run the configured whole source suite at implementation's build gate and inspect its budget report.

Evidence reviewed: change 0309 and its Outcomes and failure posture contract (call-shape errors use Go errors and the app maps them); change 0329 and commit c465f717 (shared optional failure field, findings reserved for refusals); change 0348 (discovery context). ADR-0050 supports behavior-based regression evidence; ADR-0055 distinguishes deliberate defaults from exhaustive vocabulary mappings. No vocabulary is being added, and no new ADR is warranted. Relevant learnings: groomed-root-cause-is-a-hypothesis and shared-resource-keeps-first-owner-assumptions.

Relations: related [309, 329]; discovered_from [348]; adrs [50, 55]; no dependencies or stack parent.

## Out of scope

Engine return-contract changes, validation-rule changes, new status/disposition/finding vocabulary, new failure fields, per-command error frameworks, retries, telemetry, frequency measurement, and unrelated interrupted-error handling. This grooming authorizes no implementation.

## Reconcile log

### 2026-09-21

2026-09-21: Reconciled against current main (48e76b7c). The groomed design holds unchanged. Confirmed in code: transaction.Engine.Execute returns base (Result{Operation:op}, empty Disposition) plus a typed *Failure for every call-shape validation error (invalid operation key/expectations/idempotency key, non-branch target ref, nil loader) in internal/repository/transaction/engine.go. internal/app/planning.go mapOutcome routes empty disposition to its default arm -> ResultInternalError, and failureStatus returns nil on every non-failed disposition, so the invalid-input cause is swallowed. repairResultFromOutcome in internal/app/change_repair.go attaches failureStatus only in its explicit DispositionFailed arm, not its default arm. Scope, relations (related [309,329], discovered_from [348], adrs [50,55]), and the trivial verdict are all still accurate; no ADR or scope adjustment needed.
