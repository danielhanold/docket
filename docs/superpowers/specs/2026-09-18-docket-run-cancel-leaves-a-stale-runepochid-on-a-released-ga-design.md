<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0435 — docket run cancel leaves a stale RunEpochID on a released gate-admission slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-20-0435-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga.md)**
<!-- docket:backlink:end -->

# Safely retire cancelled run ownership from a released gate-admission slot

## Purpose and implementation order

Implement and merge **437 first, then 435**. Change 437 fences epoch-backed starts and relaunches and accounts for their pending launch obligations. This change uses that proof to retire stale worktree ownership and protect resume. The record's `depends_on: [437]` enforces the sequence. Build 435 against integration after 437 reaches done, retaining all its launch/cancellation tests. Do not stack or implement the pair concurrently.

## Problem and evidence

Ordinary execution release intentionally retains RunEpochID so a live epoch owns its worktree between drives. `reserveWorktreeExecution` and `rawStaleEpochRefusal` correctly enforce mismatch even for a released slot. Cancellation has no separate ownership-retirement step, and `runCancel` immediately returns for cancelled/superseded epochs. A completed cancellation can therefore leave a stale released slot that neither replacement nor standalone finalize can use, and retrying cancel does not repair it.

The recorded incidents were finalize of change 434 and build/resume of change 368. The latter's predecessor-epoch workaround exposed 437's admission defect; it is not supported recovery. The first review on main ab9216d2 also reproduced foreign-slot teardown and found epoch-less fixtures that missed the real owning-epoch case.

The second review on main 341174de8c1ad914b5de999c36f696282e5df9e5 showed that delayed successors and automatic relaunches can outlive slot release. It also found that resume trusts durable cancelled/superseded state: returning cancellation-pending for such a terminal record does not prevent replacement. This revision requires 437's launch accounting before detachment and makes terminal repair and resume use consistent proof.

## Decision

Separate ordinary execution release from cancellation-specific epoch retirement. Ordinary `ReleaseWorktreeExecution` retains RunEpochID. Add a small store operation under the existing admission lock/atomic writer that clears only the expected epoch on a released slot while preserving history. Only authorized cancellation invokes it after complete accounting.

Reuse 437's existing-record launch accounting alongside native-task, execution-participant, and mutation accounting. Do not implement a competing inventory or treat released state as sufficient proof. Keep existing schemas and lifecycle states; ADR-0118 remains the governing contract.

## Ownership and teardown rules

Pass expected epoch identity through the existing slot-marking and reconciliation helpers. Establish ownership before marking, stopping, releasing, or retiring; the current token alone is not proof of epoch ownership.

A different nonempty RunEpochID is a foreign owner: never mark it, stop its RawRunDir, release it, or clear it. Continue accounting only for execution and launch obligations independently linked to the old epoch. An empty epoch is not ownership proof either. Stop an epoch-less legacy slot only when its exact execution is independently linked to that epoch's registered participants; otherwise leave it untouched and report unresolved ownership if it obstructs completion. Update the existing raw-slot fixture instead of preserving its any-slot-at-this-worktree assumption.

Every slot mutation atomically checks expected epoch and reservation token; retirement also requires released state. A stale snapshot, changed reservation, unreadable slot, unresolved execution, or unknown state never authorizes a clear. Re-read a raced slot and distinguish a successor to leave alone from unresolved old work. Never retry blindly with the successor's token. Preserve 437's separate successor reservation and protection against a late predecessor release.

Check all slot-write errors. A successful process stop does not prove release or retirement was durably recorded. Unaccounted persistence failures cannot produce completed cancellation.

## Complete accounting and interruption

Fence before teardown. Use 437's accounting of pending initial launches, delayed tickets, scoped successors, current replacement processes, and reserved/recovered relaunches; also account for registered tasks/processes, late participants, and admitted mutations. A stopped predecessor, empty participant list, or released slot is insufficient. Busy launch claims and unresolved reservation/teardown evidence keep cleanup incomplete. The death guardian may perform teardown but never retires epoch ownership: it leaves cancelling and requires authorized run.cancel completion.

For authorized completion, reuse the epoch lock and re-read/revalidate the epoch and accounting evidence. In the same protected completion decision, retire the matching released slot under the admission lock and only then persist cancelled. Retain 437's epoch/claim/admission/scope/drive lock order; do not acquire epoch while retaining an inner lock, or hold epoch/admission over process stops or network work. A changed or unaccounted launch, participant, or mutation prevents detachment. The 437 fence prevents any new old-epoch launch authorization; its claimant/reservation accounting proves that pre-fence authorizations can no longer create a process.

These are two existing records, not a new cross-store transaction:

- Failure before retirement leaves ownership intact and cancellation incomplete.
- Retirement succeeds but persisting cancelled fails, or the process dies between writes: the epoch stays fenced. Detachment was safe because all launch, execution, and mutation obligations were settled first. A retry revalidates that proof, accepts an absent/already-detached slot, leaves a foreign successor untouched, and finishes the epoch transition. Never restore the old owner or add a cleanup journal.
- After cancelled is persisted, retries are no-ops unless a historical stale released slot needs the bounded repair below.

A cancellation-pending result can therefore follow safe detachment if the final epoch write failed. The invariant is that unresolved launch, execution, or mutation accounting never permits detachment, not that every pending result retains a slot.

## Terminal repair and resume use the same proof

Retain key/repository/epoch/claim authority checks. Before the terminal shortcut for cancelled/superseded, revalidate the old epoch's existing launch, participant, and mutation evidence using the same bounded accounting rules. Terminal state by itself is not proof of quiescence. If accounted and a released slot still carries this epoch, perform the same ownership-checked retirement. Never revive the epoch, replay mutations, reset budgets, or stop a replacement's process.

A missing slot, empty epoch field, or foreign successor slot is an idempotent no-op only after old-epoch obligations are independently accounted. A terminal epoch with a busy claim, unresolved launch, nonreleased owned slot, contradictory mutation evidence, or unreadable required record is not safely repairable by this bounded path. Leave it fenced and untouched; return a bounded finding. Do not turn historical repair into a general recovery engine.

Use existing cancellation dispositions consistently:

- Successful historical retirement: cancelled/applied.
- Proven complete with nothing to repair: already-cancelled/no-op.
- Incomplete cleanup of a durably cancelling epoch: cancellation-pending.
- Unsafe or unverifiable repair of a terminal epoch: refused with the specific incomplete-cleanup finding. Do not return cancellation-pending while leaving durable cancelled/superseded state, and do not regress terminal state to cancelling. Authority failures remain refused.

At `run.gate-before --resume`, reuse the same old-epoch quiescence validation before reserving a replacement from cancelled or returning dispatch authorization for a previously reserved replacement from superseded. Integrate the check into the existing serialized resume decision, not an unlocked preflight. Incomplete/unreadable proof uses the existing refusal channel with a bounded explanation; it creates no replacement and yields no new dispatch authorization. Do not cancel or alter an already-reserved successor. A safe repeat still observes the same reserved key and never mints a second one. This narrow resume check is required because the cancellation command's last reported disposition is not durable authority.

For historical terminal records, do not require new fields or fabricated proof. Use the records already present; missing evidence needed to distinguish live/pending work from completed teardown fails closed. Normal records with complete evidence remain resumable.

## Acceptance criteria

1. Start with a released slot bearing a nonempty epoch that rejects both a different epoch and an epoch-less reservation. Authorized cancellation detaches it; separately prove a valid replacement build gate and an epoch-less finalize gate can then admit.
2. Cover already-released and executing-then-released slots. Retirement preserves DriveID, RawRunID, RawRunDir, execution generation, and other historical fields; only RunEpochID and normal update metadata change. Ordinary release retains epoch and between-drive mismatch protection.
3. Carry 437's delayed-ticket, successor, automatic-relaunch, and relaunch-recovery barriers forward. Pending launch claims, ambiguous reservations, live replacement processes, pending mutations, late participants, and guardian-only cleanup cannot retire ownership. Assert fields and process launches, not only dispositions.
4. Cancellation never stops or mutates a foreign slot. Exercise replacement between load and mutation, concurrent cancellation replay, and stale predecessor cleanup after successor reservation. Epoch-less slots require independent linkage before teardown.
5. Inject release, retirement, and final-epoch write failures. Interrupt between retirement and cancelled persistence; retry must converge without touching a successor or enabling an old launch. No false completed cancellation may be reported.
6. Repair historical cancelled and superseded released slots; repeated repair is a no-op. Unsafe terminal histories return refused without slot/epoch mutation. They cannot authorize resume, even after a refused cancel or with a previously reserved replacement. Include both cancelled and superseded, busy claims, unresolved relaunches, contradictory mutations, and unreadable evidence.
7. Proven-complete cancellation remains resumable, including after an interrupted final write is repaired. Concurrent/repeated resume reserves exactly one replacement and returns the same key. A foreign slot alone neither proves nor disproves old-epoch quiescence.
8. Clearing ownership must not allow the old epoch's fresh start, delayed ticket, or recovered relaunch to execute. Cancellation/repair/denied resume consume no suite attempt and reset no budget, deadline, or retry state.
9. Mutation-test ownership, complete-accounting, terminal-resume validation, and error-propagation guards. Run the entire source-resolved build suite and inspect its budget report.

## Complexity limit and exclusions

No new daemon, background loop, store, schema, lifecycle state, configuration, CLI command, generic coordination framework, or retry layer. Reuse 437's launch accounting, existing locks/atomic writers, cancellation/replay, and the existing resume boundary. No extension of raw gate recover, weakening of state-independent epoch mismatch, change to takeover authority, or redesign of mutation-owner lookup or normal successful-run epoch retirement.

437 alone owns launch fencing and pending-launch accounting, including automatic/recovered relaunch. This change owns ownership retirement, existing slot teardown protection, historical terminal repair, and its matching resume check. If the ordering cannot fit those existing mechanisms, surface the concrete conflict instead of adding machinery during implementation.
