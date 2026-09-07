<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0407 — Keyed gate-verdict misattributes its verdict to a concurrent loop's change id under parallel implement-next runs](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0407-keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent.md)**
<!-- docket:backlink:end -->

# Bind keyed dispatches to their successful claims

## Purpose

Change 0407 makes a fresh implement-next dispatch's ownership explicit at claim time. A keyed verdict must inspect the change that dispatch actually claimed, including when it completed before the first verdict. It must never authorize retry or continuation against a concurrent dispatch's change.

## Background and verified diagnosis

The incident recorded in change 0407 involved successful builds of changes 0403 and 0402 followed by keyed verdicts naming sibling work. Source inspection explains the failure class: `RunGateBefore` records a before-set and dispatch epoch but leaves fresh runs' `AttributedID` unset. `attributeGateClaim` considers only currently in-progress records absent from the before-set with a qualifying `claimed_at`. A completed change disappears from that set. A single remaining concurrent claim is then attributed to this gate; multiple remaining claims produce ambiguity.

The source does not choose the newest or highest-priority claim. The defect is inference from an incomplete, shared candidate set. Merely admitting implemented records would retain completed work but would not prove dispatch ownership. These findings come from source inspection; reproducing the reported failure class with a controlled regression is a build obligation.

`RunGateBefore` already creates a unique dispatch context and stores its hash. The context currently connects nested test drives to recovery machinery. `ChangeClaimRequest` and the successful claim receipt carry no dispatch identity. Verified explicit resume pre-binds an ID; later verdicts reuse an established attribution. The missing seam is the first successful claim of a fresh dispatch.

## Design

### Carry dispatch identity through the claim boundary

Extend the cataloged `change.claim` interface with optional dispatch-context input, using the existing context token emitted by `run.gate-before`. Update its request schema, capability signature, CLI plumbing, and implement-next's claim instructions together. The implementer passes its supplied context when claiming; it continues passing it to test-drive operations as today. An invocation without a gate context remains supported for ordinary ungated claims.

Validate supplied context against the current repository's durable gate/scope identity before changing metadata. Invalid, foreign, expired or incompatible context fails closed. Never treat a supplied but invalid context as an ungated claim. Preserve the context's role in test-drive recovery: claim binding must not consume the capability needed by those operations.

### Persist proof of the successful claim

Record a durable association between dispatch context, change ID, and the successful claim transaction's identity. Use the existing claim transaction/receipt machinery to retain the context hash with the committed claim proof; raw capabilities must not be written to tracked metadata, rendered artifacts, or public diagnostics. The receipt is authoritative proof of a successful claim. Local gate state may index or mirror that proof, but a caller-provided ID, timestamp, current status, or child completion message is insufficient.

The association is bind-once for an armed gate. Serialize competing binding attempts at the gate store boundary so one context cannot successfully claim two different changes. Preserve the existing exact-version eligibility check and transactional claim behavior. A failed or contended claim cannot establish successful ownership; an idempotent replay can confirm only its own original dispatch/claim association. In particular, two dispatches submitting the same ID and version must not both acquire the successful transaction through the existing idempotency path: dispatch identity participates in the claim request's identity and replay checks.

Metadata commits and local gate records cannot be assumed to update atomically together. Distinguish an incomplete reservation from confirmed ownership. If execution stops after the metadata transaction succeeds but before the local binding is finalized, recover only from the exact committed receipt for that dispatch. If proof cannot be resolved, stop without retry permission and preserve evidence for recovery. If an interrupted reservation cannot be proven safe to release, leave it refused rather than allow a different claim. Do not roll back another actor's metadata to repair local state.

Claim-time ownership is durable across claim timestamp refreshes, status transitions, process restart, and subsequent verdict reads. It identifies the original claim instance, not just the numeric change ID. If the change is later reclaimed and taken by another run, the old gate must detect that replacement and cannot retry or take over the new claim. Check claim-instance continuity before entering recovery or retry logic.

### Resolve verdicts by ownership

For fresh gates, replace before-set/cardinality attribution with the verified dispatch-to-claim binding. Snapshot and timestamp information may remain diagnostic data but cannot create retry authority. Resolve the bound change regardless of whether it remains in-progress; call the existing `RunVerify` predicate for its disposition rather than duplicate completion checks.

A successfully completed bound change must yield the existing `gate-done ... run-complete <id>` report when `RunVerify` certifies completion, even if unrelated changes were claimed, refreshed, or completed during the same interval. Persisting or reading an ownership binding is safety-critical; errors cannot be ignored before issuing a retry or continuation.

Missing, conflicting, corrupt, unsupported, or unprovable binding yields a terminal non-authorizing report, using `gate-stop ... gate-unavailable <reason>` with a precise typed reason. It must not name sibling candidates, consume a retry, or fall back to global claim inference. A key with no successful claim likewise stops without attribution; this change does not introduce a new protocol for proving a drained queue.

Keep `RunVerify` as the disposition authority, the existing single-retry CAS, and continuation-before-retry ordering. Retries and continuations retain the same ownership binding and key; they do not claim a different change. Existing verified explicit resume remains supported and must establish the same claim-instance identity before gaining authority.

### Compatibility and documentation

Version persisted gate state as required to distinguish explicit proof from historical inferred `AttributedID` values. Old records lacking verifiable ownership must fail closed with an actionable diagnostic; never bless an old guessed ID by migration. The supported recovery is a newly armed explicit resume for a still-valid in-progress change, through the existing verification path. For other states, direct read-only verification remains available. Missing local state must not trigger automatic gate recreation or renewed retry permission.

Keep unattributed mode observe-only. Change 0345 still owns creating attribution context for slash-command entry points; this work supplies a primitive that later work may reuse, without implementing those entry points. Change 0405's test-drive handshake investigation remains separate.

Update maintained contracts and generated dispatch instructions wherever claim-context propagation or attribution behavior changes. Find those sites by repository-wide search. Record a successor ADR during implementation, superseding ADR-0075's snapshot/cardinality attribution mechanism and accepted concurrency residual while explicitly preserving its conservative safety principle and unaffected halt semantics. Do not rewrite the Accepted ADR's historical decision.

## Alternatives considered

- Expand attribution to more lifecycle statuses: repairs the disappearing-completion symptom but leaves parallel ownership ambiguous.
- Pre-bind only a requested ID at dispatch: does not prove the child won that claim and does not cover automatic selection.
- Bind the dispatch to its successful claim transaction: selected because it ties authority to the actual ownership transition and works for both explicit and automatically selected changes.

## Acceptance and verification

Use isolated repositories and deterministic interleavings rather than sleep-based races. Add meaningful coverage at the application/store boundary plus CLI and skill-wiring coverage for context propagation.

1. Arm two gates before either claim. Claim distinct changes with their respective contexts. Complete A before either verdict while B remains in-progress. A verifies A as complete; B verifies only B. Reverse completion and verdict ordering and exercise both remaining in-progress and both completed.
2. A dispatch that claims nothing never acquires a sibling's claim. Unrelated timestamp refreshes, priority changes, and statuses do not affect ownership.
3. Competing dispatches claiming the same version produce at most one owner. Replay of the winning request is idempotent; replay under another context is refused. Competing different-ID claims using one context cannot both succeed.
4. Inject interruption before and after claim commit and before local binding persistence. Demonstrate exact-receipt recovery where available and a non-authorizing stop otherwise. Failed claims never become confirmed bindings.
5. Check foreign/invalid contexts, corrupt/missing binding records, old gate schemas, and conflicting receipts. Assert no sibling IDs, retry consumption, or takeover occurs on refusal.
6. Verify timestamp refresh and completion preserve ownership, while reclaim followed by a new claim prevents the old gate from recovering or retrying the replacement run.
7. Preserve verified resume, one retry under concurrent verdict calls, and continuation-before-retry behavior with explicit ownership proof. Test that a later verdict cannot overwrite a binding or erase concurrent retry/continuation state.
8. Verify end-to-end context propagation from dispatch instructions through the CLI to the committed claim receipt. Mutate the propagation or ownership check and demonstrate that the relevant regression fails for the intended reason, with uncached Go execution.

Run the complete suite using the resolved `build.test_command` at the build gate. Follow the repository's budget findings policy and `tests/README.md`; do not add another test runner or weaken existing safety coverage. Record controlled reproduction and mutation evidence in the implementation results.

## Scope and readiness

This is one change covering claim ownership, keyed verdict attribution, required persistence and interface updates, tests, and the successor decision record. It has no dependencies and is not stacked. Preserve related changes 0345 and 0405 and discovery provenance 0403. Scheduling/serialization of implement-next loops, slash-command interception, test-drive handshake repair, release determinism, and unrelated lifecycle redesign remain out of scope. Planning and implementation occur later through implement-next.
