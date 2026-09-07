# ADR draft — Run-gate attribution binds a dispatch to its successful claim transaction

> **Draft, not the recorded ADR.** This file is the decision text prepared on the change-0407
> feature branch so `docket-implement-next` Step 6 can hand it to the `docket-adr` agent verbatim.
> The immutable ADR is recorded by `docket-adr` into `docs/adrs/` — never by this task, and never by
> hand-editing that ledger. Fill/adjust only against the reality built on this branch
> (verify-the-claim, never from memory).

**Title:** Run-gate attribution binds a dispatch to its successful claim transaction

**Supersedes:** ADR-0075 (conservative snapshot/cardinality claim attribution). **Change:** 407.

## Context

ADR-0075's before-set + dispatch-epoch inference attributes "exactly one surviving new in-progress
claim" and explicitly accepted a concurrency residual. Under parallel implement-next loops the
intended change completes, drops out of the in-progress set, and a lone concurrent claim survives the
filters — keyed verdicts then named sibling changes (observed 2026-09-04 on changes 0403/0402;
change 0407's reconcile log confirms the mechanism in `RunGateBefore`/`attributeGateClaim`).

## Decision

`change.claim` optionally carries the dispatch context minted by `run.gate-before`; a gated claim
reserves a bind-once binding at the store boundary before its metadata transaction, folds the context
hash into its idempotency digest, commits the hash inside the claim receipt (the durable,
authoritative ownership proof on the metadata branch), and confirms the local binding after. Keyed
verdicts resolve the bound change — complete or not — through `RunVerify`, recover an interrupted
binding only from the exact committed receipt, check claim-instance continuity (a replacement claim
stops the old gate), and fail closed (`gate-unavailable` with a typed reason, or
`no-attributable-claim`) on anything unprovable. Gate records are schema-versioned so a pre-proof
record can never bless an inferred id.

## Preserved from ADR-0075

- The conservative safety principle — never guess, never choose among candidates, a wrong grant is
  the one unrecoverable move.
- The halt-reporting/exit-code semantics ADR-0075 also decided are untouched.

## Consequences

- Unattributed mode stays observe-only.
- Ungated claims stay supported.
- Changes 0345 (slash-command context) and 0405 (test-drive handshake) may reuse the primitive.

Recorded by the docket-adr transaction at implement-next Step 6; this draft is the handed-over
decision text. ADR-0075's frozen body is not rewritten — its status line alone flips to "Superseded
by ADR-NN" inside that same transaction.
