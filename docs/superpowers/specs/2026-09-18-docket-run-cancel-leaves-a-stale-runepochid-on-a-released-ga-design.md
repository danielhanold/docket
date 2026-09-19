<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0435 — docket run cancel leaves a stale RunEpochID on a released gate-admission slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0435-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga.md)**
<!-- docket:backlink:end -->

# Safely retire cancelled run ownership from a released gate-admission slot

## Purpose and implementation order

Implement and merge **437 first, then 435**. Change 437 rejects revoked epochs at gate-start admission; this change repairs the stale worktree ownership that blocks legitimate replacement and standalone finalize gates. The record's `depends_on: [437]` enforces that sequence. Build 435 against integration after 437 reaches done, retaining its admission tests. Do not stack or implement the pair concurrently.

## Problem and evidence

An admission slot intentionally retains RunEpochID after ordinary execution release so a live epoch continues to own its worktree between drives. Both `reserveWorktreeExecution` and `rawStaleEpochRefusal` enforce the epoch mismatch regardless of released state. Those checks are correct and stay unchanged.

Cancellation currently has no separate ownership-retirement step. `reconcileWorktreeSlot` treats a released slot as accounted, while `ReleaseWorktreeExecution` preserves the epoch. `runCancel` also returns immediately for cancelled/superseded epochs. Thus a completed cancellation can leave a released slot that rejects a replacement forever, and retrying the supported cancel operation does not repair it.

The recorded incidents were finalize of change 434 and build/resume of change 368. The latter's successful use of the cancelled predecessor epoch additionally revealed a separate start-admission defect, now assigned to 437; it is not an acceptable recovery technique. The 435 review on main ab9216d2 reproduced stale-slot refusal after cancelled, terminal replay leaving the stale field, and cancellation touching a foreign epoch's slot. Existing cancellation tests used epoch-less raw slots and did not prove this ownership contract.

## Decision

Separate ordinary execution release from cancellation-specific retirement of epoch ownership. Ordinary `ReleaseWorktreeExecution` must retain RunEpochID. Introduce a small dedicated store operation using the existing admission lock and atomic writer; it clears only the expected epoch on a released slot and preserves execution history. It is invoked only by authorized cancellation after complete accounting, never as an automatic consequence of releasing a process.

Keep the existing epoch and admission schemas and state vocabulary. ADR-0118 remains the governing contract.

## Ownership and teardown rules

Pass the expected epoch identity through both slot-marking and slot-reconciliation helpers. Establish ownership before marking, stopping, releasing, or retiring a slot. The current slot's token alone is not proof that it belongs to the cancelling epoch.

A different nonempty RunEpochID is a foreign owner: do not mark it, stop its RawRunDir, release it, or clear it. Continue accounting only for participants registered to the cancelled epoch. An empty epoch is not ownership proof either; an epoch-less legacy slot may be stopped only when its exact recorded execution is independently linked to that epoch's registered execution participants. Otherwise leave it untouched and report unresolved ownership if it obstructs completion. Update the existing raw-slot fixture rather than preserving its assumption that any slot at the worktree belongs to the epoch.

Each slot mutation must atomically check the expected reservation token and epoch. Retirement additionally requires released state. A stale snapshot, changed reservation, unreadable slot, unresolved execution, or unknown state cannot authorize a clear. Re-read a raced slot and distinguish a successor that must be left alone from unresolved teardown; do not blindly retry with the successor's token.

Check slot-write errors. Process-stop success is not proof that release or retirement was durably recorded. Preserve bounded diagnostics and do not report completed cancellation on an unaccounted write failure.

## Completion ordering and interruption

Keep the existing fence-before-teardown flow. Teardown may release an execution while retaining its epoch. Late participants and admitted mutations must then be accounted before epoch ownership is retired. The death guardian shares teardown but does not retire ownership: it leaves cancelling and still requires authorized run.cancel completion.

For authorized completion, reuse the existing epoch lock, re-read/revalidate the cancelling epoch and accounting snapshot, then retire its matching released slot under the admission lock, and only then persist cancelled. Acquire epoch before admission whenever both are held; do not hold them across process stops or network work. Any newly discovered unaccounted participant/mutation keeps the epoch cancelling and preserves ownership. The 437 admission fence prevents a new old-epoch admission after cancellation fencing.

There are two existing records, not a new cross-store transaction. Their interruption contract is explicit:

- Failure before slot retirement leaves ownership intact and cleanup incomplete.
- Retirement succeeds but persisting cancelled fails, or the process dies between writes: the epoch remains fenced. Complete accounting had already been established, so releasing ownership was safe. Repeated cancellation rechecks accounting, accepts an absent/already-detached slot, leaves a foreign successor untouched, and finishes the epoch transition. Do not restore the old ownership field or introduce a cleanup journal.
- After cancelled is persisted, normal retries are no-ops unless a historical stale released slot needs the bounded repair below.

Do not claim that every cancellation-pending result necessarily retains the slot: after a failed final epoch write it can already be safely detached. The invariant is that unresolved execution or mutation accounting never permits detachment.

## Repair of existing terminal records

Retain the current key/repository/epoch/claim authority checks. Before the terminal-epoch shortcut, inspect whether a cancelled or superseded epoch still owns a released slot. Validate that its persisted participant and mutation evidence does not contradict completed teardown, then perform the same ownership-checked retirement. Never revive the epoch, replay mutations, reset budgets, or stop a replacement's execution.

A missing slot, already-empty epoch field, or a slot owned by a different epoch is an idempotent no-op after the old epoch is otherwise accounted. A terminal record still naming a nonreleased/ambiguous execution is not automatically trustworthy: leave it untouched and return bounded incomplete-cleanup findings. The historical released-slot repair does not become a general recovery engine.

A terminal replay that repairs stale ownership reports cancelled/applied; a replay with nothing to repair reports already-cancelled/no-op. Incomplete cleanup reports cancellation-pending, with a finding explaining the remaining slot/accounting condition, without changing a terminal epoch back to cancelling. Authority failures remain refused. No new disposition is introduced.

## Acceptance criteria

1. A fixture first proves a released slot carries a nonempty owning epoch and rejects both a different epoch and an epoch-less reservation. Authorized cancellation detaches ownership; a valid replacement build gate and an epoch-less finalize gate can then admit independently.
2. Cover already-released and executing-then-released slots. Preserve DriveID, RawRunID, RawRunDir, execution generation, and other historical fields; only RunEpochID and the normal update metadata change at retirement.
3. Ordinary release retains epoch ownership and the existing between-drive mismatch tests stay green.
4. Pending mutations, late participants, unproven teardown, ambiguous slots, and guardian-only cleanup cannot retire ownership. Assert the field, not just the disposition.
5. Cancelling an old epoch never stops or mutates a foreign slot. Deterministic barriers cover replacement between load and mutation and concurrent cancellation replay. Epoch-less slots require independent linkage before teardown.
6. Failed release/retirement/final-epoch writes cannot produce false completion. Inject interruption between retirement and epoch persistence and prove supported retry converges without touching a successor.
7. Repair already-cancelled and superseded historical released slots; verify repeated repair is a no-op and unsafe historical states remain diagnostic rather than silently freed.
8. Carry 437's revoked-start tests forward: clearing old ownership must not let the old epoch reclaim a free slot. Cancellation and repair consume no suite attempt and reset no budget/retry state.
9. Mutation-test the ownership, full-accounting, and error-propagation guards. Run the entire source-resolved build suite and inspect the budget report.

## Complexity limit and exclusions

No new daemon, background loop, store, schema, lifecycle state, configuration, CLI command, generic coordination framework, or retry layer. Reuse existing locks, atomic writes, cancellation, and replay. No extension of raw gate recover, no weakening of the state-independent epoch mismatch checks, no change to takeover, and no redesign of mutation-owner lookup or normal successful-run retirement. Change 437 alone owns revoked-epoch start admission. If the ordering cannot be implemented within these constraints, surface the precise conflict rather than adding machinery during implementation.
