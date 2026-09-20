<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0435 — docket run cancel leaves a stale RunEpochID on a released gate-admission slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-20-0435-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga.md)**
<!-- docket:backlink:end -->
# docket run cancel leaves a stale RunEpochID on a released gate-admission slot — Results

## Outcome

Authorized cancellation completion now retires a cancelled epoch's `RunEpochID` from a **released** worktree admission slot, so a legitimate replacement build gate or an epoch-less finalize gate can admit that worktree again. Ordinary execution release is unchanged — `ReleaseWorktreeExecution` still retains `RunEpochID` for between-drive ownership — and the retirement is a separate, ownership-checked step reached only after complete launch/participant/mutation accounting.

Delivered on the feature branch, built against integration after dependency 437 (which owns launch fencing and pending-launch accounting) reached done:

- **New store operation** `RetireWorktreeExecutionEpoch(worktreeRoot, expectEpoch, expectToken)` (`internal/gatedrive/admission_retire.go`): one `admissionCAS` mutate closure under the existing admission flock/atomic writer that clears **only** `RunEpochID` on a released slot the expected epoch owns, preserving `DriveID`/`RawRunID`/`RawRunDir`/`ExecutionGen`/`ScopeID`/`Kind`/`ReservationToken` and legacy-inventory fields. Idempotent when already detached; refuses a foreign epoch (`ErrStaleRunEpoch`), a token mismatch (`ErrNotOwner`), a non-released state (`ErrWorktreeBusy`), an empty expected epoch (`ErrInvalidID`), and fails closed on an unreadable/absent slot.
- **Exported epoch-carrying reserve entry** `ReserveWorktreeExecutionForEpoch` (`internal/gatedrive/admission.go`) so the app-layer cancellation fixtures can construct a genuinely epoch-owned slot.
- **Ownership-checked teardown** (`internal/app/rungate_cancel.go`): `classifySlotOwnership` (owned / linked-legacy / foreign / unowned) gates every slot mutation in both the best-effort (`markWorktreeSlotStopping`) and authoritative (`reconcileWorktreeSlot`) paths; a foreign or unlinked-epoch-less slot is never marked, stopped, released, or cleared. Slot-release write errors are checked and fail closed (`slot-release-failed` → cancellation-pending).
- **Completion order**: after full accounting, `runCancel` retires the released slot's ownership under the admission lock and only then CASes the epoch `cancelling → cancelled`. A failed retirement leaves the fence held and ownership intact (`cancellation-pending`); a failed final epoch write returns `cancellation-pending` with `finalize-unpersisted` (never a rollback or a false completed cancellation). An interruption between retirement and the final epoch write converges on retry via the idempotent retire.
- **Bounded terminal repair**: the `EpochCancelled`/`EpochSuperseded` early return now runs `repairTerminalEpoch`, which re-proves quiescence with the same bounded accounting (`verifyTerminalEpochQuiescence`), retires a historical stale released slot, no-ops idempotently when there is nothing to repair, and refuses unsafe/unverifiable histories with a specific finding — never regressing terminal state and never returning `cancellation-pending` over a durable terminal record. The death guardian still reaps but never retires ownership.
- **Resume quiescence** (`internal/app/rungate_before.go`): `validateResumeQuiescence` reuses the same proof and the same ownership-checked retirement before the `EpochCancelled` branch reserves a replacement or the `EpochSuperseded` branch re-authorizes a previously reserved one; incomplete/unreadable proof refuses on the existing gate-unarmed channel, reserving no replacement. Exactly-one-replacement and same-key repeat semantics are preserved.

No new daemon, background loop, store, schema, lifecycle state, configuration, CLI command, or retry layer was added; 437's accounting is consumed through the existing launch-reconciler seam. ADR-0118 remains the governing contract. No departures from the spec.

## Verification performed

- TDD throughout: each of the 8 plan tasks was implemented test-first through the native gate driver, with a confirmed RED (intended failure only) before GREEN, and the ownership/completion-order/repair/resume guards were mutation-probed (each guard stripped one at a time, confirmed to redden the intended test, then restored byte-identical). Where the plan's literal mutation was masked by a defense-in-depth store guard, a faithful substitute mutation was used and recorded.
- Full source-resolved build suite (`go run ./cmd/docket development test`) driven build-owned through the gate driver to a terminal PASSED at the final branch head `84338c93` — `SUITE files=52 passed=52 failed=0 asserts=423`. Durable build evidence recorded and verified against that head (`result: green`).
- Budget report read on the green run: several `BUDGET WATCH:` screening lines (test_go_race ~296s, test_go_toolchain ~262s, and a few integration suites), each parallel `-j11` with a consecutive-overrun streak of 1/5. No `SERIAL CONFIRMED OVER BUDGET:` line — screening only, no action required.
- Whole-branch deep review (rung selected from the highest routed profile, premium): clean — 0 blocker/major/minor findings.
- One intermediate build gate went red on the first attempt from a cross-cutting repoguard guard (`TestRealProcessPackagesUseFixtureTempDir`) tripped by a new test's bare `t.TempDir()`; repaired (switched to `testsupport.TempDir(t)`) and the suite re-run green. This was a test-fixture guard, not a behavioral defect.

## Findings and limitations

### Coverage is automated; no manual scenario is uniquely needed

The behavior is internal gate-drive/admission bookkeeping with no user-facing surface, and it is covered end-to-end by package tests asserting durable fields, process/reservation state, and dispositions across AC1–AC9 (successor race, idempotent replay, release/retirement/final-write failure injection, empty-epoch, absent-slot, foreign/unlinked/linked epoch-less variants, resume one-replacement, and budget non-reset). No functional scenario is uncovered that a human check would meaningfully add, so no human-testing checklist is authored.

## Follow-ups

### Test-only exported reserve entry (nit from review, not a defect)

`internal/gatedrive/admission.go:ReserveWorktreeExecutionForEpoch` is an exported `Store` method whose only current caller is app-layer cancellation test fixtures (real epoch-owned reservations go through the unexported `reserveWorktreeExecution`). The deep review flagged this as a mild smell but recommended leaving it as-is: it is honestly documented, guards an empty epoch with `ErrInvalidID`, and is symmetric with the existing `ReserveRawWorktreeExecution`. Surfaced for a future reader's awareness; no change proposed. No change or issue minted.
