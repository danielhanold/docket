---
id: 422
slug: 'bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no'
title: 'Bind outer run-gate retry consumption to a dispatch epoch, not each observation'
status: 'in-progress'
priority: 'medium'
type: 'chore'
created: '2026-09-10'
updated: '2026-09-15'
depends_on: []
stacked_on:
related: [421, 425, 426, 427]
discovered_from: [421]
adrs: [111, 115, 118]
spec: 'docs/superpowers/specs/2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no-design.md'
plan: 'docs/superpowers/plans/2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-15T11:45:52Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no-design.md) |
| Plan | [2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no.md](https://github.com/danielhanold/docket/blob/chore/bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no/docs/superpowers/plans/2026-09-15-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no.md) |
| ADRs | [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md), [ADR-0115](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0115-outer-run-gate-retry-budget-is-a-counted-config-snapshotted.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Change 0421 made outer-run attempt limits configurable. At limits of three or more, repeatedly checking the same unfinished attempt can spend successive retry allowances even when no new dispatch occurred. The store already grants each numbered attempt only once; the verdict path causes the over-count by deriving the observed attempt number from the number of grants. The approved design fixes that input while retaining the existing ownership and dispatch-observation contracts.

## What changes

- Pass an explicit observed attempt number through the keyed verdict path, defaulting omitted input to attempt 1. Repeated checks of that attempt reuse its existing retry marker.
- Reuse the current per-attempt reservation mechanism, require evidence of the preceding grant for later attempts, and return the next attempt number only with a newly granted retry.
- Have the coordinator associate the returned number with the authorized dispatch and retain it across observations and continuations. Ownership proof, the configured attempt limit, report tokens and existing durable formats remain unchanged.
- Keep a reservation spent when response delivery or launch is uncertain; stop instead of guessing, refunding or relaunching.
- Preserve the existing trust boundary: the coordinator identifies which dispatch finished. Independent child-entry or launch certification is outside this fix. Add regressions for repeated and staggered concurrent observations and the required caller wiring.

## Out of scope

New attempt ledgers, random attempt identities, child-admission handshakes, new CLI operations, schema migrations, cancellation-reader or lock redesign, launch tracking/recovery, claim/workspace resume redesign, and exactly-once launch guarantees. Build and finalize budgets, configuration keys/defaults, report-token vocabulary, and change 0427's recovery-worktree fix remain outside this change.

## Reconcile log

### 2026-09-15

2026-09-15: Reconciled against current source. The defect is confirmed present: RunGateVerdict (internal/app/rungate_verdict.go) derives the observed attempt as `attempt := 1 + usedBefore` where usedBefore = GateRetryUsage, so repeated verdicts of one unfinished attempt at AttemptLimit >= 3 walk successive markers and spend future allowances. The existing per-attempt CAS (ConsumeGateRetry + gateRetryMarkerFor in internal/app/rungate_store.go, schema v4) is the mechanism to reuse. CLI verdict lives in internal/cli/run.go (attributed mode, one positional key); the additive result field goes on RunGateVerdictResult. No dependency or stacked base is added. Related parent-instruction changes 0425 (in-progress) and 0426 (proposed) are unmerged, so the feature branch cut from origin/main sees neither — no base conflict. Change 0421 is done; ADR-0115 gets the successor decision recorded during implementation, preserving its accepted body. Scope, out-of-scope, and acceptance criteria remain accurate; no body edits required.
