---
id: 126
slug: 'named-implement-next-skips-unrelated-maintenance-preflight'
title: 'Named implement-next skips unrelated maintenance preflight'
status: 'Accepted'
date: '2026-09-24'
supersedes: []
reverses: []
relates_to: [101, 106]
change: 448
---

## Context

An explicitly named implement-next run executed the implementation-scope maintenance preflight (ADR-0101, ADR-0106) before selecting its change. That sweep runs merged-implemented closeouts, cleanup suffixes, and expired-claim reclaims for every change, and any blocked, failed, unknown, or contended entry makes the preflight verdict `problem`, halting before claim. An unrelated change's failing closeout could therefore stop a named change from ever starting.

## Decision

1. The mandatory implementation-startup preflight of ADR-0101/ADR-0106 now binds only selection-path invocations: no argument, or an id set of two or more. An invocation naming exactly one explicit change id (initial call, attributed retry, or named resume) goes directly to `context.implementation --id` and the claim/resume path. All local checks for that change remain: readiness, version, dependencies, effective base, claim eligibility, gate context, and resume/cancellation. A named request never falls back to selecting another change. Deliberate `maintenance.preflight` / `maintenance.sweep` runs keep every honest outcome.

2. Dependency handling uses existing operations only. The named path runs one bounded `finalize.closeout --id` per own `depends_on` entry at status `implemented` (plus `implemented` stack ancestors on a stack-base-unresolved refusal) and nothing else. `finalize.closeout` re-proves the merged PR, so `pr-not-merged` surfaces as the ordinary waiting-dependency refusal; any other non-applied disposition halts locally naming that dependency. No new operation, flag, scope, or queue is introduced.

3. Boundary: a multi-id allowlist remains selection behavior, preflight included.

This narrows ADR-0101 and ADR-0106; it does not supersede or reverse them.

## Consequences

Unrelated maintenance failures no longer block named builds. Those failures stay visible only to deliberate maintenance runs, so they may linger unnoticed longer. The behavior lives in skill prose guarded by prose-contract rows, not in Go code, so enforcement depends on those contract rows staying current.

## Alternatives considered

Keep the mandatory preflight for every invocation (rejected: unrelated failures block named work). Add a new scoped preflight operation, flag, or queue for named runs (rejected: existing `finalize.closeout --id` already covers the only dependency maintenance a named change needs). Treat multi-id allowlists like named runs (rejected: they still select, so they keep selection-path preflight).
