<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0437 — Reject revoked run epochs before gate-start admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0437-reject-revoked-run-epochs-before-gate-start-admission.md)**
<!-- docket:backlink:end -->

# Reject revoked run epochs before gate-start admission

## Purpose and implementation order

Implement and merge **437 first, then 435**. Change 437 prevents a cancelled caller from acquiring a gate execution. Change 435 subsequently retires the cancelled epoch's stale ownership so a legitimate replacement or standalone finalize gate can use the worktree. Change 435 must carry `depends_on: [437]`; 437 has no dependency on 435. These are sequential PRs against integration, not stacked or parallel changes.

This order deliberately leaves the stale-slot refusal in place until 435 lands. Do not work around that temporary refusal by supplying a predecessor epoch or editing durable records. Completing 437 alone does not repair existing stale slots.

## Evidence and existing contract

The 435 review against main ab9216d2 found that `Driver.Admit` carries `StartRequest.RunEpochID` into the admission record, where `reserveWorktreeExecution` compares it with the incumbent epoch. Equality proves identity consistency, not liveness. The injected epoch-revocation resolver is consulted by takeover, not start. An isolated scoped-start diagnostic with a resolver that always returned revoked reached WAITING with zero resolver calls. The incident recorded for 368 likewise describes using the cancelled predecessor's epoch to pass the mismatch fence.

ADR-0118 requires cancellation to fence new work and ordinary execution release to preserve a live run's ownership between drives. Both requirements remain in force.

## Bounded design

### Resolve and validate the requested epoch

For fresh and successor `gate.drive.start` admissions that carry a nonempty epoch, the application layer resolves that exact epoch in the current repository's existing registry. It must be uniquely resolvable, active, and bound to the requested canonical worktree. A missing, corrupt, ambiguous, wrongly bound, cancelling, cancelled, superseded, or unknown-state explicit epoch refuses admission. A failure to read an explicitly requested identity is not standalone operation.

Use the existing scope identity when a scope carries an epoch: the request cannot substitute or omit that epoch. Preserve the existing capability, gate-context, worktree, branch, and predecessor checks. The epoch remains a locator, not a new credential.

A genuinely epoch-less standalone gate retains its existing behavior and the existing incumbent-epoch mismatch fence. Do not infer that every operation on a previously used worktree must belong to a historical epoch. Do not change raw launch, takeover, or ordinary release policy.

### Serialize admission with the existing cancellation fence

An unlocked liveness probe is insufficient. Reuse the existing per-epoch lock to validate the epoch and complete the durable admission decision, including same-scope reuse, before releasing the lock. The application owns registry interpretation; gatedrive owns worktree reservation. A narrow synchronous callback around the admission decision is sufficient; do not introduce a registry or generic admission framework in gatedrive.

When both locks are required, acquire the epoch lock before the worktree-admission lock. Do not acquire an epoch lock while holding the admission lock. Do not hold either lock across process launch, process stop, network I/O, or suite execution. Keep fingerprint computation outside the epoch critical section where possible.

The race has two valid outcomes:

- Cancellation fences first: admission refuses, creates no usable admission ticket, launches nothing, and charges no suite attempt.
- Admission wins first: its reservation is durable before cancellation can fence. Existing teardown accounting sees the admitted execution or unresolved reservation and cannot report cancellation complete while its launch/teardown is unresolved. This is admitted work to reconcile, not a second admission after cancellation.

Preserve the existing Admit/StartAdmitted split and reservation/confirmation/failure handling. Do not add another launch journal. The critical section must cover the reuse path as well as the fresh-reservation path; guarding only a successful reserve misses reuse after a busy result.

### Refusals and budget behavior

Use existing run-cancelled/stale-run-epoch refusal semantics for known revoked states and existing typed failure channels for unresolved identity. Diagnostics carry bounded public locators, never capabilities. A refused epoch must reach neither build attempt reservation nor process launch. Work admitted before the fence follows existing budget accounting; no refund or reset is introduced.

The production build, finalize, and task service constructors must wire the check. A unit-test-only resolver or a takeover-only check does not satisfy this contract.

## Acceptance criteria

1. Active, correctly bound explicit epochs admit scoped and scopeless starts under existing slot rules. Epoch-less standalone behavior is unchanged.
2. Cancelling, cancelled, and superseded epochs refuse against both a free slot and a released slot naming the same old epoch. Supplying the old value cannot bypass cancellation.
3. Missing, ambiguous, unreadable, wrong-worktree, and scope-mismatched explicit epochs refuse without a launch or suite charge. Scope epoch omission is also refused.
4. Deterministic barriers exercise both orders of admission versus cancellation, including same-scope reservation reuse and the interval before launch confirmation. Do not use timing sleeps as the concurrency oracle.
5. Existing between-drive mismatch protection, takeover protection, cancellation accounting, and no-budget-reset assertions remain green. Ordinary release still retains RunEpochID.
6. Production service tests prove the resolver/critical-section wiring. Mutation of the liveness check or its synchronization must redden the corresponding assertion.
7. Run the entire source-resolved build suite and inspect its budget report. Record that stale-slot recovery remains intentionally owned by 435.

## Complexity limit and exclusions

Reuse the epoch and admission records, their existing states, existing lock files, atomic writers, and public operations. No new daemon, background loop, persistent store, schema, lifecycle state, configuration, CLI command, generic coordination framework, or additional retry layer. A small private lock helper/callback is allowed; a second ownership protocol is not.

Cancellation-specific detachment, terminal-cancel replay repair, foreign-slot teardown protection, and cleanup write-error handling belong to 435. This change does not redesign mutation-owner lookup, run attribution, coordinator lifecycle, or automatic relaunch/recovery behavior, and does not claim complete cancellation certification for those other paths. If the required start/cancel serialization cannot be implemented within the existing lock-and-reservation model, report the concrete conflict for design review instead of expanding this scope silently.
