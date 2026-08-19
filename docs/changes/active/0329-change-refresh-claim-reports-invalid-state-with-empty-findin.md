---
id: 329
slug: 'change-refresh-claim-reports-invalid-state-with-empty-findin'
title: 'change refresh-claim reports invalid-state with empty findings — the transaction Failure is dropped on the DispositionFailed path'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-08-19'
updated: '2026-08-19'
depends_on: []
stacked_on:
related: [315]
discovered_from: [316]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

An autonomous docket-implement-next run on change 0316 called `docket change refresh-claim` during its claim/reconcile phase and got back a deterministic, undiagnosable refusal: `result: invalid-state`, `disposition: invalid-state`, `findings: []`. Nothing in the result says why. The run had to abandon explicit refresh-claim entirely and lean on reconcile's own internal claim-refresh to keep the lease fresh, which happened to cover it — so the defect did not block that run, but it silently removed a lease-management operation the skills are supposed to be able to call.

The cause is a diagnostic-propagation gap, not a bad transition. On the `DispositionFailed` path the diagnosis lives in the typed `transaction.Failure` carried by the returned `error`, while `claimResultFromOutcome` builds its result exclusively from `transaction.Result`:

- `mapOutcome` (internal/app/planning.go) routes `DispositionFailed` through `mapFailure(err)`, which reads the Failure's `Kind` — so `KindInvalidState` correctly becomes `ResultInvalidState`. That much works.
- `claimResultFromOutcome` (internal/app/change_claim.go) then sets `Findings: findingsToStatus(res.Findings)`. On a *failed* transaction — as opposed to a *refused* one — `res.Findings` is empty, because findings are the refusal channel. The Failure's own message is never consulted.
- `claimDisposition` switches on `res.Disposition`. `DispositionFailed` matches no case, so it falls to `default:`, and with `replayed == false` it returns `string(result)` — restating "invalid-state" as the disposition rather than naming a cause.

So every field that could carry the reason is either empty or a tautology, and the operator sees a failure with no cause. The information exists in `execErr` and is thrown away at the boundary.

This is a general shape, not a refresh-claim quirk: `claimResultFromOutcome` serves `change claim` as well, and the `mapOutcome`/`mapFailure` pattern is shared across the app layer. Any operation that funnels a `DispositionFailed` into a result envelope built only from `transaction.Result` will redact its own failure the same way. Scope the fix by grepping the shared helpers, not by patching the one verb where it surfaced (AGENTS.md: never hand-list the sites of an operation you are gating — derive them from a whole-repo grep).

Separately, the trigger itself is still unexplained and worth pinning down as part of the fix: `domain.RefreshClaim` only guards `StatusInProgress`, and change 0316 *was* in-progress when the calls failed, so the Failure originated below the domain guard — plausibly the exact-version blob expectation or a loader/validation path. A repro is the first task; the run that found this could not investigate without mutating a live claim.

## What changes

Propagate the typed `transaction.Failure` into the result envelope on the `DispositionFailed` path so a failed operation names its own cause. Concretely: carry the Failure's kind and message into `Findings` (or an equivalent diagnostic field) rather than dropping it, and give `claimDisposition` a real `DispositionFailed` arm instead of letting it fall through to `default:` and echo the result string back as the disposition.

Derive the full set of affected call sites from a grep over the shared helpers (`mapOutcome`, `mapFailure`, `claimResultFromOutcome`, and the other `*ResultFromOutcome` builders), then sort them into ones that already surface failure detail and ones that redact it. Fix the shape once in the shared layer if the call sites permit it.

First task is a reproduction: establish what makes `refresh-claim` fail deterministically against an in-progress change at a valid version, since `domain.RefreshClaim`'s only guard is the status and that guard was satisfied. Build it as a hermetic fixture — do not reproduce against a live claimed change.

Add a regression test asserting that a failed transaction surfaces a non-empty, non-tautological cause: the assert must fail if the Failure detail is dropped again. Per AGENTS.md, mutation-test the guard — strip the propagation, watch it redden — or it is decoration.

## Out of scope

Changing the claim/refresh-claim lease semantics themselves, the reclaim TTL policy, or reconcile's internal claim-refresh (which is working and covered the gap). Not a redesign of the protocol-v1 envelope or the finding vocabulary. Not the unrelated Step-5 gate failures on change 0316's branch.
