<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0405 — Investigate the gate.drive.prepare-scope -> gate.drive.start handshake rejecting a build-task worker's focused gate](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-10-0405-investigate-the-gate-drive-prepare-scope-gate-drive-start-ha.md)**
<!-- docket:backlink:end -->

# Sequential test drives within one worker recovery scope

## Decision and scope

A build-task recovery scope represents one parent/child dispatch, not one test execution. It permits a sequence of task-owned drives, with at most one current execution or launch reservation. Each drive remains a separate immutable execution identity with its own command, fingerprint, result, deadline, and ownership generation. A worker can perform baseline, RED, GREEN, and later verification under the same parent-issued scope without obtaining another parent capability.

The user approved this design on 2026-09-10 after a plain-language explanation. This specification is the grooming artifact; planning and implementation follow in the implementer workflow.

## Evidence and relationship to existing work

Design baseline: main commit 2f83683c2e1b4967779d09c7d71b7c35f4757b47.

Change 416 is done. It fixed the maintained controller and worker contracts to pass the complete identity that prepare-scope pins. Its archived record and results report that the old incomplete call fails and a complete call succeeds; it explicitly leaves repeated-drive and concurrent-scope behavior to 405. Preserve that fix and its tests.

Source inspection shows two remaining mechanisms:
- Driver.Start rejects any nonempty BoundDriveID before checking whether its drive has finished. Store.bindScopeDrive also permanently rejects a different drive id.
- mapDriveFailure maps ownership errors to generic invalid-request, hiding distinctions already represented by OwnershipErrorKind.
- FindScopeDriveIDs includes terminal records with a remaining owner generation. Sequential execution therefore needs durable result acknowledgement, not just replacement of BoundDriveID, or historical results can become multiple recovery candidates.

The original change-402 reports established symptoms, not proof of consumed capabilities or cross-build collisions. Do not claim every historical incident has been reproduced. Reconcile these observations against the build checkout and reproduce the complete-identity, second-start failure before changing behavior.

Dependency: 416, already done. Related work: 359's recovery ownership model, 402's discovery, and 412's separate foreground/yield problem. This change preserves ADR-0107's one-live-drive rule and event-authorized, direct-parent takeover.

## Invariants

1. The parent retains the parent capability; the worker receives only its child capability and complete pinned identity bundle.
2. A scope authorizes at most one current launch reservation or execution. Two racing calls must not both launch test processes, even if one test finishes very quickly.
3. Only a durably PASSED or FAILED predecessor permits another test. WAITING, HALTED, pending handoff, transferred ownership, closed scope, corrupt state, and unresolved launch/cleanup state do not.
4. A new test is a new drive, never a relaunch of the predecessor and never reuse of its passing evidence.
5. Starting a successor acknowledges exactly the predecessor result the worker received. Prior results remain readable as history, but lose authority to advance, hand off, or compete as current recovery candidates.
6. All transitions recheck the scope identity, capability, expected current drive, and relevant ownership state under synchronization. Gate-context identity is included when the scope pinned it; omission or alteration must not detach a drive from outer recovery.
7. Scope closure, cooperative claim, and parent takeover remain terminal for the dispatched child's start authority. Child capability alone cannot reopen a transferred scope.

## Public workflow and transition contract

### First test

The controller prepares one scope per dispatch and passes the complete bundle introduced by 416. A first start carries no predecessor receipt and succeeds only if the scope has no current drive or reservation. Validate identity and capability before launch.

Return the existing structured drive document, including drive id, owner generation, and outcome. Capture credentials from JSON. A missing or uncertain response is not permission to call start again.

### Subsequent tests

After receiving PASSED or FAILED, the worker may edit the task's code and start another test. The successor start additionally supplies the previous drive id and its current owner generation as an explicit predecessor receipt. Both fields are required together for a successor and forbidden for an empty scope. Expose them on gate.drive.start through the catalog, schema, CLI, and application request.

The driver must verify that this exact predecessor is still the scope's current drive, has a durable PASSED or FAILED outcome, has no outstanding handoff, and is still owned by the supplied generation. A start cannot acknowledge an unrelated or superseded drive. Reserve the successor and retire the predecessor's recovery authority as one logical transition before launching anything. Two calls presenting the same predecessor receipt have at most one winner; the loser gets a typed stale-predecessor or busy diagnostic, with no process launch.

The scope's repository, worktree, change, task, phase, branch, and gate-context identity remain fixed. The next drive fingerprints the current worktree independently. Do not require the edited worktree to match the predecessor's fingerprint: edits between RED and GREEN are expected. The predecessor's recorded fingerprint and verdict remain historical facts; existing fingerprint checks for advancing or transferring a live drive remain mandatory.

### Finishing the task

A final test has no successor to acknowledge it. Provide a cataloged native terminal-acknowledgement operation that takes the scope id, child capability, final drive id, and owner generation. It requires the same complete scope/drive association, a PASSED or FAILED terminal result, no outstanding handoff, and current ownership. Its logical effect is to mark the last result consumed, clear its recovery authority, and close the task scope. A valid repeated acknowledgement is a no-op; incorrect credentials or a different final drive are refused.

The worker invokes this operation only when no further test execution is needed and it is ready to return its final task outcome. A FAILED result does not authorize a success report: existing task completion criteria still apply. Retain the final drive identity and verdict in the worker report/evidence; acknowledgement must not delete execution evidence. A failed acknowledgement returns BLOCKED with the typed cause, not COMPLETE.

### Waiting or interrupted work

On WAITING, the worker follows the existing immediate handoff and return contract. It does not start another test, acknowledge a live drive, background a loop, or infer completion from elapsed time. The controller claims the handoff and observes that same drive. Claim closes the child's scope as today; later worker dispatches get fresh scopes.

If the worker returns without handing off, the direct parent uses the existing event-authorized takeover. It resolves the current execution or an unacknowledged last terminal result, never an acknowledged predecessor. An explicitly supplied old drive id cannot bypass the current-scope association. Owner-generation supersession and the restriction on skipping a live parent remain unchanged.

## Durable state and concurrency

Implement sequential replacement as a serialized scope transition, not a read-then-launch check followed by a best-effort bind. The current Start launches before its authoritative bind; simply relaxing the nonempty BoundDriveID check would let racing starts execute duplicate tests before stopping the loser.

Persist a successor reservation and its identity before calling the process supervisor. Coordinate predecessor acknowledgement, current-slot publication, and drive ownership using an explicit lock order and durable transition journal or equivalent recoverable state. Document the chosen lock order and recovery phases in source so Start, final acknowledgement, Handoff/Claim, and Takeover cannot deadlock or act on different current drives.

Retain old drive records and enough durable predecessor/successor linkage to distinguish history from current work. The current scope state is authoritative; never choose a drive by newest timestamp. Outer recovery candidate enumeration must understand acknowledged predecessors and reservations so it neither sees two valid candidates from one sequence nor silently treats pending work as no work.

Failures before a reservation leave the scope and predecessor unchanged. Once a reservation exists, failure or interruption must leave a recoverable transition record. A proven pre-launch failure may safely release the reservation through the owning transition; a launched or uncertain process must never cause the slot to be treated as empty. Preserve any known supervisor handle and use existing ownership-proven process operations. Ambiguous launch or persistence failures fail closed with a typed cause and no automatic second launch. A reservation is not itself a PASSED, FAILED, or safely quiescent result.

Serialize competing starts, completion, handoff/claim, and takeover against the same slot. Revalidate the target after acquiring the transition's authority: a takeover that read the predecessor before a successor reservation must not later close the scope around the wrong drive. The winner owns the current transition; stale owners cannot acknowledge or replace it.

Version changed persistent formats. Unsupported or malformed versions fail closed; do not silently reinterpret in-flight one-drive records as reusable scopes. Keep hash-only persisted capabilities, private file permissions, and unknown-schema behavior.

## Diagnostics and caller integration

Map known ownership errors to their bounded, stable reason tokens instead of generic invalid-request. Distinguish at least identity mismatch, capability mismatch, scope closed, live/busy scope, outstanding handoff, stale predecessor, non-reusable HALTED predecessor, and unresolved launch transition. Keep command rejection separate from a produced WAITING/PASSED/FAILED/HALTED document; do not widen that outcome vocabulary or invent a successful drive on a failed start.

Human messages explain the valid next action for the actual state: retain/hand off the current drive, claim an existing handoff, stop and return BLOCKED for lost authority, or use the complete identity bundle. Do not print credentials, raw argv, environment contents, or unsafe stored error text. A diagnostic never authorizes blind start retries or a keyless fallback.

Update docket-build, docket-build-task, the shared gate-caller-loop and gate-execution references, CLI/schema documentation, and embedded mirrors together. Derive affected sites through repository search rather than a fixed file allowlist. Cover first start, successor receipt, final acknowledgement, and rejected transitions at the points workers execute them. Preserve 416's identity guard and JSON capture requirements.

## Verification and acceptance

At implementation time, include the following behavioral coverage through the existing Go suite runner:

1. Reproduce the current second-start rejection using a freshly prepared scope and the complete identity bundle. A positive first-start control proves this is the one-drive defect, not missing identity.
2. One scope completes baseline PASSED, RED FAILED, and GREEN PASSED as distinct drives, including a real worktree edit between RED and GREEN. Each command executes once and retains its own fingerprint and result.
3. Missing/wrong identity, gate context, capability, predecessor id, or predecessor generation launches nothing. A rejected request does not consume the legitimate predecessor.
4. A live or HALTED predecessor, pending handoff, closed scope, and transferred owner reject successors. Acknowledgement refuses live, HALTED, unrelated, and superseded drives.
5. Barrier-controlled races for empty-scope starts and successor starts prove exactly one launch, not merely one successful response or eventual loser cleanup. Race successor start against handoff, takeover, and final acknowledgement; prove only one coherent ownership transition wins.
6. Run two scopes in distinct linked worktrees sharing one Git common directory concurrently, with distinguishable output and opposite verdicts. Each parent resolves only its own current drive. Repeat across tasks in one change, across different changes, and with historical acknowledged drives present. Cross-scope credentials or explicit old drive ids cannot steal work.
7. Exercise WAITING -> handoff -> claim and direct-parent takeover after at least one completed predecessor. Verify old generations cannot act and outer discovery finds the current candidate without historical ambiguity.
8. Inject interruption/persistence failures around reservation, launch, handle persistence, predecessor retirement, and final acknowledgement. Restart the store/driver and prove it preserves known ownership or refuses ambiguity without a duplicate launch or false no-work result.
9. Check final acknowledgement, valid idempotent repetition, historical evidence retention, closed-scope rejection, and zero stale recovery candidates after normal task completion.
10. Exercise typed JSON and human diagnostics, credential redaction, unsupported persistent versions, catalog/schema parity, and maintained/embedded caller agreement.

Mutation-test new guards and load-bearing synchronization/identity predicates with bounded fixtures; disabling the guarded behavior must fail for the intended reason. Inspect actual process-launch counts and persisted identities, not only exit codes. Use the configured build.test_command for the complete build gate and inspect budget findings, including serially confirmed breaches. Do not add arbitrary sleeps or unbounded stress tests.

Record a new ADR for the sequential scope lifecycle and acknowledgement boundary, relating to ADR-0107 and preserving its ownership principles. Accepted ADR text remains immutable. No new ADR is required merely to restate the already-shipped identity fix.

## Alternatives and non-goals

A fresh parent-issued scope for every test would interrupt the worker's local test cycle and multiply dispatch coordination; rejected. Letting workers mint their own parent authority would destroy the separation that makes takeover meaningful; rejected. Replacing BoundDriveID after any terminal outcome without acknowledgement, reservation, or ownership fencing would lose recovery information and permit races; rejected.

Out of scope: redoing 416, weakening identity/fingerprint checks, parallel tests within one worker scope, changing top-level build/finalize command selection, background/yield behavior tracked by 412, run-gate attribution or retry redesign, cross-machine recovery, and a general gate-history garbage collector.
