---
id: 111
slug: 'run-gate-attribution-binds-a-dispatch-to-its-successful-clai'
title: 'Run-gate attribution binds a dispatch to its successful claim transaction'
status: 'Accepted'
date: '2026-09-07'
supersedes: [75]
reverses: []
relates_to: []
change: 407
---

## Context

ADR-0075's before-set + dispatch-epoch inference attributes "exactly one surviving new in-progress claim" and explicitly accepted a concurrency residual. Under parallel implement-next loops the intended change completes, drops out of the in-progress set, and a lone concurrent claim survives the filters — keyed verdicts then named sibling changes (observed 2026-09-04 on changes 0403/0402; change 0407's reconcile log confirms the mechanism in `RunGateBefore`/`attributeGateClaim`).

## Decision

`change.claim` optionally carries the dispatch context minted by `run.gate-before`; a gated claim reserves a bind-once binding at the store boundary before its metadata transaction, folds the context hash into its idempotency digest, commits the hash inside the claim receipt (the durable, authoritative ownership proof on the metadata branch), and confirms the local binding after. Keyed verdicts resolve the bound change — complete or not — through `RunVerify`, recover an interrupted binding only from the exact committed receipt, check claim-instance continuity (a replacement claim stops the old gate), and fail closed (`gate-unavailable` with a typed reason, or `no-attributable-claim`) on anything unprovable. Gate records are schema-versioned so a pre-proof record can never bless an inferred id.

Two commitments are preserved unchanged from ADR-0075:

- The conservative safety principle — never guess, never choose among candidates, a wrong grant is the one unrecoverable move. Attribution now discharges that principle with proof rather than with inference.
- The halt-reporting and exit-code semantics ADR-0075 also decided are untouched.

## Consequences

- Attribution rests on a committed claim receipt rather than an inference over a mutable set, so a concurrent loop's claim can no longer be mistaken for this dispatch's.
- Unattributed mode stays observe-only.
- Ungated claims stay supported — the dispatch context is optional on `change.claim`.
- Anything unprovable now fails closed with a typed reason (`gate-unavailable`, `no-attributable-claim`) instead of resolving to a best guess; callers see more refusals and no wrong grants.
- Gate records carry a schema version, so a record written before the proof mechanism existed can never bless an inferred id.
- Changes 0345 (slash-command context) and 0405 (test-drive handshake) may reuse the binding primitive.

## Alternatives considered

- **Keep ADR-0075's snapshot/cardinality inference and tighten its filters.** Rejected: the residual is structural, not a filter gap — when the intended change completes and leaves the in-progress set, no amount of filtering distinguishes a lone surviving concurrent claim from the dispatched one. The observed 0403/0402 misattribution is exactly that case.
- **Refuse attribution whenever more than one claim is in flight.** Rejected: it makes the gate unusable under the parallel implement-next loops that motivated it, and still misattributes the single-surviving-claim case it cannot detect.
- **Attribute from run-local state (timestamps, process ids, launch shape).** Rejected: none of it is durable across an interrupted run or verifiable on the metadata branch, so it cannot serve as ownership proof; the committed claim receipt can.
