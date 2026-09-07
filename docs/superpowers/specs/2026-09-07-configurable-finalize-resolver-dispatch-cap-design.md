<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0349 — Make the finalize rebase-resolver dispatch cap configurable](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0349-configurable-finalize-resolver-dispatch-cap.md)**
<!-- docket:backlink:end -->

# Configurable, durable finalize resolver budget

## Decision and purpose

Add `finalize.resolver_max_attempts` with a built-in default of **3**. The value is a finite positive integer. Resolve it through normal configuration layering and enforce it in Go using the owned rebase receipt.

The human approved durable Go enforcement and explicitly selected default 3 on 2026-09-07. This intentionally changes the previous default of two resolver dispatches. A repository can set 2 to retain the previous ceiling.

The limit covers resolver dispatch opportunities across one owned rebase attempt, including conflicts surfaced by successive commits. It is not a per-file, per-commit retry allowance, a limit on automatically replayed commits, or the integration-repair agent's separate two-attempt limit.

## Current behavior and related work

The source `skills/docket-finalize-change/SKILL.md` contains the clause “Resolver loop (skill-enforced ≤2 attempts)”. Its failure reference repeats the limit. `FinalizeRebaseContinue` verifies a resolver report and calls `StageAndContinueRebase`, but records no resolver budget. A session restart therefore has no authoritative counter to recover.

Change 334 exposed the practical problem: separate commits conflicted on derived artifacts, and the third conflict exceeded the skill's two-call ceiling. That change is done. Change 396 is also done; it persists the local suite's WAITING continuation in `RebaseReceipt`. Reuse that existing ownership boundary and preserve its gate continuation. Change 291 concerns failure-reference loading order and remains separate. Changes 392 and 403 provide install-time config tolerance and actionable configuration diagnostics.

The source resolver agent already edits only conflicted regions and returns a report; Go owns staging, continue, and abort. The old project-local wrapper description in the stub is historical drift, not a reason to transfer Git mechanics to the resolver.

No unmerged dependency or stacked base is needed. Related changes: 291, 334, 392, 396, 399, 403. Relevant accepted decisions: ADR-0010 (resolver/repair separation), ADR-0019 (configuration scope), ADR-0105 (receipt-owned continuation), and ADR-0109 (schema fidelity).

## Configuration contract

- Add the active integer leaf to the Go schema, effective `Finalize` type, defaults, and resolution wiring. Use the existing positive-decimal validator with minimum 1. Reject zero, negative values, strings, booleans, fractions, collections, non-decimal spellings, and overflow with the existing source/key/line diagnostics. There is no unlimited sentinel or arbitrary additional upper ceiling.
- Allow global and machine-local overrides. Precedence remains repository-local > repository-committed > global > built-in. This is a local workflow-budget policy, classified like `finalize.gate` under ADR-0019; it does not select a shared storage location or change merge authorization.
- Export the resolved integer in `repository.prepare`'s typed `context.finalize` object and the effective configuration diagnostics. Never have the skill parse YAML or use the deleted Bash export path.
- Snapshot the effective limit when a new owned rebase receipt is created, before Git starts the rebase. The stored limit governs that entire attempt. Valid configuration changes during an attempt apply to the next fresh attempt, not to the current receipt. Invalid configuration retains the existing operational refusal behavior.
- `finalize.gate: off` continues to skip the rebase; it creates no reservation and spends no resolver budget.

## Durable admission before native dispatch

A check performed only in `finalize.rebase-continue` is too late to bound dispatches: the child has already run. Add one typed operation, semantic id `finalize.resolver-reserve`, with a CLI shape of `docket finalize resolver-reserve --id <id> --attempt <attempt> [--repo-dir <dir>]`. Register its `local-write` effect, capability signature, schema, human rendering, and install routing together. This operation is new work for implementation; grooming does not invoke it.

The operation verifies the implemented change, owned workspace and rebase attempt, fixed base/remote identities, and the live stopped Git commit before authorizing work. It does not launch an agent, stage paths, continue Git, run tests, or mutate metadata.

Under a per-workspace cross-process lock, it reloads the receipt and:

1. Refuses foreign, malformed, legacy-unbudgeted, moved, or non-conflicted state.
2. If a reservation is already outstanding, returns `disposition: pending`. This authorizes **no new dispatch**, leaves the count unchanged, and reports the existing reservation identity for matching an already-known child/report.
3. If attempts used equals the snapshotted limit, returns `disposition: exhausted`, reason `resolver-budget-exhausted`, with the count and limit. It changes no Git or receipt state.
4. Otherwise, durably increments attempts used and records an opaque reservation token bound to this attempt and stopped commit. Only after the write and verification succeed does it return `disposition: reserved`. This authorizes exactly one native `docket-rebase-resolver` dispatch.

The closed reservation dispositions are `reserved`, `pending`, `exhausted`, `blocked`, and `contended`, mapped onto the existing protocol-v1 result vocabulary. Results carry the change id, full attempt token, bounded reason, limit, used and remaining counts, and reservation/stopped-commit identity when applicable. Counts come only from the validated receipt; a caller cannot supply or reset them.

The measurable unit is a **durably reserved dispatch opportunity**. A failed launch or lost success response may consume an opportunity without a child running. There is no refund. This conservatism keeps the configured upper bound across interruptions. Native dispatch remains the harness's responsibility; Go does not claim to count arbitrary agent launches performed outside this protocol.

## Receipt and continuation contract

Extend `RebaseReceipt` with a versioned resolver-budget group: version, limit, used count, and the current reservation's token, stopped-commit identity, and continuation-started marker. Keep the receipt comparable with scalar fields and use the existing crash-safe write discipline. Validate all invariants on both read and write: a supported version, limit >= 1, 0 <= used <= limit, complete reservation fields, a valid stopped commit, and no continuation marker without a reservation.

Serialize the budget read/check/write and each associated Git mutation across processes using the workspace locking facilities. Release locks before native dispatch and before a long suite wait. Existing gate-continuation writes must reload and preserve the budget fields rather than overwrite them with a stale receipt; receipt deletion and rebase begin/abort must participate in the same ownership/serialization discipline. Do not introduce an independent ledger or infer liveness from a PID or timestamp.

Add `resolver_reservation` to the resolver report and dispatch payload. `FinalizeRebaseContinue` must verify the report's change id, full rebase attempt, current reservation token, stopped-commit identity, and live unmerged paths. The report remains an authored hint. A missing, foreign, previously consumed, or stale reservation refuses before staging or continuing. In particular, identical conflicted paths on two different commits cannot make an old report reusable.

After validating the report and before staging/continuing, durably mark that this reservation's continuation has started. Consume/clear the outstanding reservation only when the corresponding Git outcome is reconciled, preserving the used count. A failed report validation neither spends another attempt nor refunds the current one, and does not authorize another child.

A continuation on the last allowed reservation is valid: used == limit prohibits the next reservation, not completion of work already reserved. If that continuation finishes the rebase, run the normal gate. If it surfaces another conflict, return the existing rebase disposition `blocked` with reason `resolver-budget-exhausted`, counts, and the owned attempt; let the existing abort-and-block flow handle it. Only the new reservation result uses the `exhausted` disposition; the rebase disposition vocabulary does not grow.

All conflicted-result paths, including initial begin, later continue, and receipt recovery, surface authoritative budget state and direct the caller to reserve; none directly authorizes dispatch. No-op rebases and automatic commit replay spend zero opportunities. Suite WAITING polling, gate continuation, evidence creation, and the integration-repair loop spend none.

## Interruption, replay, and compatibility

- A fresh invocation recovers the same receipt, limit, count, and pending reservation. It cannot silently start another owned rebase attempt or reset a budget.
- Repeating a reservation request after a lost success response returns pending, never a second dispatch permit. If the controller can identify its existing child, wait for that child or process its matching report. If dispatch ownership cannot be established, halt with the reservation retained; do not guess whether the child ran.
- A repeated or response-lost continuation must not replay Git blindly. If durable state says continuation started, reconcile against the recorded stopped commit and live Git. A provably advanced or completed rebase can be recovered without another charge. An ambiguous or unchanged stopped commit is retained and blocked, with no second continue or new reservation. Conservative human recovery is acceptable; automatic retry based only on matching filenames is not.
- Reservation write/verification failure emits no dispatch permission. Counter arithmetic checks capacity before incrementing. Concurrent reservation calls admit at most one outstanding child and cannot exceed the limit; concurrent continue calls cannot stage or continue twice.
- Existing receipts with the entire resolver-budget group absent remain recognizable as legacy receipts, not corrupt or freshly budgeted receipts. Never initialize their unknown used count to zero. A legacy conflicted attempt refuses new resolver reservation/continuation with `resolver-budget-unavailable`, while owned abort remains available. A completed legacy rebase may preserve its existing gate-resume/publish/cleanup path because it authorizes no new resolver.
- Partial or invalid new budget groups are malformed and retain the existing fail-closed posture. Downgraded binaries are not claimed to enforce a budget they do not understand; supported operation requires the new binary and matching generated skills.
- An explicit human retry after verified abort may create a new receipt with the then-current limit. The skill must never use abort-and-restart automatically to replenish an exhausted budget.

## Skill behavior and failure routing

Replace the literal limit and in-memory counter in the finalize skill and its failure reference with reserve → dispatch once → verified continue. The controller branches on typed dispositions from catalog-resolved operations, never on prose or exit codes alone. Update the resolver's report-field contract while retaining its editing-only charter.

A stuck resolver, unavailable dispatch, exhausted budget, or unrecoverable reservation follows the existing halted finalize path. Before restoring a workspace that might still have a child editing it, establish that child's completion/quiescence through the native harness; inability to establish this retains the workspace and records the block rather than aborting beneath a possible writer. Once quiescent, abort the owned rebase and verify restoration, then record `## Finalize blocked` through the existing operation. Failed restoration is itself a halt.

Exhaustion messages state attempts used and limit, name `finalize.resolver_max_attempts`, and explain that changing it takes effect on the next explicit finalize attempt. The PR stays open and the change stays implemented. No new change lifecycle status, merge permission, repair sign-off exception, or inline resolver fallback is added.

## Delivery surfaces and validation

Implement through the existing config, workspace receipt, Git inspection, app finalize, CLI, and schema layers. A focused additional stopped-commit probe in `internal/gitcli` is in scope. Update the sample configuration, canonical config/CLI references, landing guide where relevant, maintained convention/skill prose, resolver agent contract, and embedded generated assets. Generated wrappers are regenerated from maintained source, never hand-edited. Frozen specs, ADRs, plans, results, and historical fixture trees remain historical; if a live-input drift guard requires a new fixture, follow its versioned replacement protocol.

Record an additive ADR during implementation explaining pre-dispatch reservation, conservative consumption, and the legacy receipt policy, relating it to ADR-0010, ADR-0019, and ADR-0105. Do not rewrite accepted decisions.

Required meaningful tests:

1. Configuration defaults to 3; explicit 1, 2, and larger values resolve correctly through all four layers; invalid values produce located diagnostics; prepare and diagnostic JSON agree.
2. A real Git fixture with three successive conflicting commits completes under the default. A fourth conflict exhausts at three. Raising the limit to 4 completes it; setting 2 restores the previous ceiling. Multiple files in one resolver report use one reservation.
3. A process restart after reservation and after each continuation preserves the limit/count. Mid-attempt config changes do not replenish or shrink that receipt. Suite WAITING recovery advances the same drive without spending budget.
4. Exhaustion refuses admission before any extra dispatch, and continue without a current reservation cannot mutate Git. Pending reservation and concurrent-call tests prove no double admission; repeated/stale reports and repeated continue cannot advance a later commit, even when paths repeat.
5. Inject failures around receipt publication, continuation-start marking, Git continue, and outcome recording. Prove no permission is emitted before durability; ambiguous recovery blocks without resetting or double-continuing.
6. Cover last-permitted successful continuation, stuck/unavailable resolver, legacy conflicted abort, completed legacy gate recovery, malformed budget fields, preserved receipt equality/publish behavior, and no-op/gate-off paths.
7. CLI capability/schema tests cover the new operation, report field, result fields, and closed vocabulary. Verify installed/generated skill contracts use reserve-before-dispatch and preserve the repair agent's independent two-attempt rule.
8. Mutation-test enforcement guards: bypass capacity rejection, remove the durable write, or ignore reservation validation and observe the corresponding behavior test fail. Derive maintained literal/dispatch sites with a repository-wide shape search, classifying generated and historical copies explicitly.

At build time, read and run the whole suite from resolved `build.test_command` using the Go-native runner; at finalize, use resolved `finalize.test_command`. Handle authoritative serial budget breaches under the repository's tests policy. Grooming itself changes metadata only and does not run the code suite.

## Non-goals

Unlimited/until-stuck operation; changing integration-repair attempts or sign-off; resolver-driven Git mechanics; general workflow scheduling or agent liveness infrastructure; unrelated wrapper cleanup; changing repository merge policy; implementing sibling changes; or implementing this spec during grooming.
