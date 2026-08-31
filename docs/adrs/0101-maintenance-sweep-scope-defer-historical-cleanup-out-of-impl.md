---
id: 101
slug: 'maintenance-sweep-scope-defer-historical-cleanup-out-of-impl'
title: 'Maintenance sweep scope: defer historical cleanup out of implementation startup'
status: 'Accepted'
date: '2026-08-31'
supersedes: []
reverses: []
relates_to: [12, 24]
change: 389
---

## Context

The `docket-implement-next` Step-0 `docket-status` merge sweep called `docket maintenance sweep` with the FULL worklist on every implementation startup. `sweepWorklist` enqueues an independent ownership-safe cleanup item for EVERY `done`/`stacked-merged` record present at the pinned inventory; each such item drives a fresh authority reload plus a `FinalizeCleanup` probe, so startup cost grows with the whole historical archive rather than with the work actually in flight. On a 388-change backlog this was observed as roughly 234 historical cleanup attempts and a multi-minute sweep on every single build start. That amplification — together with a `docket-status` child that returned before its sweep had finished, so the parent selected against a half-swept inventory — motivated change 389.

## Decision

Introduce a closed `--scope` vocabulary on `docket maintenance sweep` with exactly two members: `full` (the default; today's whole worklist, behaviour unchanged) and `implementation` (the implementation-startup preflight). The scope is resolved ONCE in the CLI (`internal/cli/maintenance.go`) into a typed `app.SweepScope` and threaded into the app layer; the app never re-derives scope from caller names, age cutoffs, or config-file presence.

In `implementation` scope, `sweepWorklist` still schedules every current merged-implemented closeout, the cleanup SUFFIX carried by such a closeout within the same invocation, and `reclaim.auto`-gated reclaims. What it does NOT enqueue is an independent cleanup item for a record that was ALREADY terminal at the pinned inventory. Those skipped candidates are counted and reported as `deferred_historical_cleanups`, an additive protocol-v1 field: it is an unprobed COUNT — never a per-item outcome, and never an inference that any counted record is dirty, blocked, or in need of attention.

Deferral is keyed SOLELY on terminal-at-pinned-inventory status. `FinalizeCleanup`'s ownership, merge, backlink, and exact-ref proofs are untouched: deferral removes candidates from ONE invocation's worklist; it never turns "unknown" into "absent" or "clean".

## Consequences

Implementation startup no longer pays per-historical-record cleanup work — its sweep cost is flat in the archive population rather than linear in it, which is the amplification the change set out to fix.

The product tradeoff, user-approved: historical cleanup recovery now requires an explicit `docket maintenance sweep --scope full` (or the targeted finalize cleanup), instead of riding along on every implementation startup. A failed or interrupted cleanup suffix remains recoverable there, so nothing becomes unreachable — it becomes deliberately scheduled rather than incidental.

Deliberately NOT introduced: a durable retry queue, a recency heuristic for choosing which historical records to probe, a new change status for deferred cleanup, and any automatic scheduled maintenance. Each would add state or policy the closed two-member vocabulary is meant to avoid.

This relates to ADR-0012 (the script-vs-model boundary — scope is deterministic plumbing resolved at one edge, not a judgement the model re-derives) and to ADR-0024 (fork/dispatch completion — a dispatched child has no channel to be awaited asynchronously). Change 389 also adds two completion barriers on that second axis — a `docket-status` command barrier and a `docket-implement-next` agent barrier — which together ensure the scoped sweep is observed through to its terminal protocol-v1 result before selection reads the inventory.

## Alternatives considered

A recency or age cutoff (probe only records terminal within the last N days) was rejected: it makes the deferral boundary a tunable policy rather than a fact about the pinned inventory, and any cutoff silently strands older records with no explicit recovery command. A durable retry queue for failed cleanups was rejected as new persistent state solving a problem the explicit `--scope full` run already solves. Deriving the scope inside the app layer from the caller's identity or from config presence was rejected outright — it reintroduces implicit, untestable behaviour at exactly the boundary this ADR is making explicit. Doing nothing and simply accepting the multi-minute startup was rejected because the cost grows without bound as the archive grows.
