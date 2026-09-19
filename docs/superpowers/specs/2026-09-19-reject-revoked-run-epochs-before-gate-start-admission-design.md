<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0437 — Reject revoked run epochs before gate-start admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-19-0437-reject-revoked-run-epochs-before-gate-start-admission.md)**
<!-- docket:backlink:end -->

# Reject revoked run epochs before gate-start admission and later launches

## Purpose and implementation order

Implement and merge **437 first, then 435**. Change 437 fences epoch-backed gate starts, delayed launch tickets, and automatic/recovered relaunches against cancellation, and makes their pending launch obligations visible to cancellation. Change 435 uses that accounting to retire stale epoch ownership and safely admit a replacement. Change 435 carries `depends_on: [437]`; 437 has no dependency on 435. Use sequential PRs against integration, not stacked or parallel changes.

Completing 437 alone deliberately leaves stale-slot refusal in place until 435 lands. Do not bypass it with a predecessor epoch or a durable-record hand edit. This change does not clear stale RunEpochID fields.

## Evidence and existing contract

The first review on main ab9216d2 found that `Driver.Admit` compares epoch identities without checking liveness. A scoped-start diagnostic reached WAITING with zero calls to an injected resolver that always returned revoked. Change 368's predecessor-epoch workaround exposed the same defect.

The second review on main 341174de8c1ad914b5de999c36f696282e5df9e5 found two further reachable paths. A successor admitted during the predecessor's terminal-before-release interval reuses its executing slot token; `StartAdmitted` can launch after cancellation releases that slot, leaving the slot's old raw-run identity unchanged. Separately, `Advance` can automatically relaunch a vanished process after slot release, without checking epoch revocation. Isolated diagnostics reproduced both. They demonstrate current-code defects, not verification of this proposed design.

`reconcileEpochTeardown` currently reads execution participants and the worktree slot, but not the scoped pending drive or relaunch reservation. Existing production gate starts do not register the execution participants that would make those gaps disappear. A released slot or successfully stopped predecessor is therefore insufficient proof of complete teardown.

ADR-0118 requires at most one reserved-or-running gate execution per canonical worktree, cancellation fencing before shutdown, complete accounting before success, and live epoch ownership between sequential drives. Preserve those requirements.

## Bounded design

### Validate epoch identity at admission and at every new launch

Resolve a nonempty requested epoch uniquely in the current repository's existing registry. It must be active and bound to the requested canonical worktree. Missing, corrupt, ambiguous, wrongly bound, cancelling, cancelled, superseded, and unknown-state identities fail closed. A scope's recorded epoch cannot be omitted or substituted. Preserve capability, gate-context, branch, worktree, predecessor, and takeover authority checks. The epoch is a locator, not a credential.

Initial admission must refuse an invalid epoch before returning a usable ticket or reserving a suite attempt. Admission is not perpetual launch permission: `StartAdmitted` must revalidate the epoch and the exact durable reservation before launching. Apply the same requirement to automatic relaunch and to recovery that would create a new process. Recovery may still identify and stop an already-created process after revocation; that is teardown, not permission to launch another.

Retain an admitted ticket's epoch identity in memory. For durable recovery, use the existing scope, admission, drive, and relaunch links. Losing an epoch-bearing slot or finding a different reservation cannot turn an old drive into an epoch-less standalone drive. Missing or inconsistent ownership linkage refuses new execution. A genuinely epoch-less standalone gate keeps its existing behavior and incumbent-epoch mismatch protection.

Derive the production launch sites from a repository-wide search and trace each epoch-backed route to this boundary. Checking only the public start command, only fresh reservations, or only takeover is insufficient. Wire build, finalize, task, and recovery constructors; a test-only resolver does not satisfy the contract.

### Serialize with existing locks and record pending launch obligations

The application owns epoch-registry interpretation; gatedrive owns reservations and launches. Reuse the existing epoch lock for the authoritative liveness read and durable admission/reservation decision, including same-scope paths. Factor a small private lock helper if needed. Do not wrap side-effecting admission in `epochCAS`: its trailing epoch rewrite can fail after admission has succeeded. A liveness read must not rotate or rewrite the epoch record.

Reuse the existing per-drive launch claimant lock currently used by `relaunchClaim` for the actual launch/attach interval of initial and replacement launches. While holding the epoch lock, acquire that claim nonblocking, re-read the exact reservation and owner, and establish its durable launch obligation in the existing records. Release the epoch lock before process launch; retain the per-drive claim until attachment or failure reconciliation is durable. A busy claim is pending work, never proof of a crashed or never-launched caller. Do not hold this claim while waiting for the suite or for user input between Admit and StartAdmitted.

Use a consistent order: epoch lock, nonblocking per-drive claim when needed, then worktree-admission, scope, and drive locks in their existing order. No path may acquire epoch while retaining an inner lock/claim; refactor recovery entry accordingly. Cancellation encountering a busy claim returns pending rather than waiting on it while holding epoch. Hold neither epoch nor admission locks across process launch, stop, network I/O, or suite execution. The existing claimant lock may span the bounded launch/attach or reservation-resolution operation, as it does today. Keep fingerprint computation outside epoch critical sections where possible.

This uses existing lock files, reserved drive/scope records, admission fields, and the relaunch token. Do not create another journal, registry, or ownership protocol. Publication failure retains unresolved evidence and blocks completion; it must not erase a reservation that may have launched.

### Give a successor its own execution reservation

A same-scope successor must not inherit the predecessor's executing token as release authority. After validating the predecessor receipt and teardown, transition through the existing reservation machinery to a fresh token/execution generation and bind the successor in the existing slot/drive/scope fields. Clear the predecessor's raw-run identity from the new reservation before launch; attach the successor's identity on confirmation. Preserve existing acknowledgement and crash-recovery behavior.

No stale predecessor release, abandon, or launch-failure cleanup may release the successor. Concurrent same-scope first-start peers may share the existing provisional reservation only while scope arbitration still permits exactly one winner; losers must not release a reservation adopted by that winner. A pending or ambiguous predecessor transition refuses a successor. Do not weaken ordinary release's preservation of RunEpochID.

### Make cancellation account for pending and replacement launches

This change owns the narrow addition to cancellation accounting required by these launch paths; 435 consumes it instead of implementing a second census. Reconcile the existing epoch-linked scope/drive reservations and relaunch journal as well as registered participants and the worktree slot. Use exact repository, canonical worktree, epoch, drive, and reservation associations. An obsolete slot RawRunDir is not an inventory of the drive's latest process, and an empty participant list is not proof that nothing launched.

After fencing, a free claimant lock plus exact reservation resolution can prove never-launched, identify a process to stop, or report unresolved. Perform resolution while excluding a competing launcher. A delayed ticket must be durably invalidated or fenced from consuming that reservation. A busy claim, missing/corrupt required record, ambiguous token resolution, failed publication, or unproven stop keeps cancellation pending. Re-read authoritative records before claiming all launch obligations settled. All new teardown actions must check their own epoch/reservation ownership and errors; 435 still owns the existing slot teardown helpers' broader foreign-slot/error audit.

The race has two valid outcomes:

- Cancellation fences first: no new launch claim or reservation can be authorized; delayed tickets and recovery refuse. A fresh admission refusal charges no suite attempt.
- A launch claim wins first: cancellation sees its durable obligation and remains pending while the claim is busy or resolution is uncertain. The process may be created after the fence, but cancellation cannot complete until it is identified and stopped or proven never launched. No process can first appear after completed cancellation from an old admitted ticket or relaunch intention.

Cancellation may release an execution while retaining its epoch only after those launch obligations and teardown are settled. A released slot alone never settles them. Preserve existing native-task and mutation accounting; this change is not a redesign of either subsystem.

### Refusals and budget behavior

Use existing revoked-epoch and unresolved-execution failure channels and bounded public locators, never capabilities. Preserve Admit/StartAdmitted budget ordering. An epoch refused by Admit consumes no suite attempt; an attempt legitimately reserved before a later cancellation remains charged even if its delayed launch is refused. Relaunch denial adds no attempt or relaunch allowance and resets no deadline, budget, or retry state. Do not retry launch automatically after a refusal or persistence failure.

## Acceptance criteria

1. Active, correctly bound scoped/scopeless starts and genuinely epoch-less standalone starts retain their supported behavior. Invalid/revoked explicit epochs refuse against both free and released slots, including scope epoch omission/substitution, without an admission-time launch or suite charge.
2. Deterministic barriers cover cancel before admission, between Admit and StartAdmitted, between final authorization and launch, and between launch and attachment. Cover initial starts, same-scope first-start contention, and successors. Assert no launch after completed cancellation and pending while an authorized launch remains unresolved; use no timing sleeps as the oracle.
3. Reproduce the terminal-predecessor/before-release successor window. The successor has its own reservation and current raw-run identity; a late predecessor release/abandon cannot free it. Verify crashes at each existing handoff journal boundary fail closed and recovery does not duplicate a process.
4. Cover fresh automatic relaunch and recovery of a reserved relaunch for never-launched, identified, busy-claim, and ambiguous resolutions. Cancellation sees the replacement even when the slot still names the predecessor. An identified run can be reconciled after revocation; a new one cannot be authorized.
5. A released or absent slot with a pending epoch-linked drive/claim is not complete accounting. Publication/read/stop failures preserve unresolved evidence. Cancellation replay converges once proof is available, without creating a second launch or relying on nonexistent participant registration.
6. Production service tests prove the shared fence and accounting are wired. Lock-order/concurrency tests prove no epoch/claim/admission deadlock and bounded pending behavior on claimant contention. The admission liveness wrapper performs no epoch write.
7. Retain between-drive ownership, takeover protection, no-budget-reset, and ordinary-release epoch-retention tests. Mutation-test the liveness, claim exclusion, successor ownership, and pending-launch accounting guards. Derive any launch-site guard from syntactic executable call sites, not a hand-maintained spelling list.
8. Run the entire source-resolved build suite and inspect the budget report. Record that stale ownership retirement and historical terminal repair remain assigned to 435.

## Complexity limit and exclusions

Reuse existing epoch, admission, scope, drive, and relaunch records, their states, lock files, and atomic writers. No new daemon, background loop, persistent store, schema, lifecycle state, configuration, CLI command, generic coordination framework, or additional retry layer. Small private helpers and reuse of the existing claimant lock are allowed; a second ownership protocol is not.

435 owns cancellation-specific epoch detachment, terminal-cancel repair, the existing slot teardown ownership/error audit, and the terminal-cleanup check at resume. This change covers every new-process route for epoch-backed gate drives, including automatic/recovered relaunch; it does not redesign relaunch policy or increase its allowance. Raw standalone launch/recover, mutation-owner lookup, run attribution, coordinator lifecycle, and normal successful-run epoch retirement remain outside scope. If the required launch/cancel serialization cannot fit this existing record-and-lock model, report the concrete design conflict rather than silently adding machinery.
