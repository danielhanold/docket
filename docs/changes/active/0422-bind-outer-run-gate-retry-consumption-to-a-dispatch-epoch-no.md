---
id: 422
slug: 'bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no'
title: 'Bind outer run-gate retry consumption to a dispatch epoch, not each observation'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-09-10'
updated: '2026-09-10'
depends_on: []
stacked_on:
related: [421]
discovered_from: [421]
adrs: [115]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0115](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0115-outer-run-gate-retry-budget-is-a-counted-config-snapshotted.md) |
<!-- docket:artifacts:end -->

## Why

Surfaced by change 0421 (configurable build and outer run-gate attempt limits) and confirmed by its docket-review-deep pass. With the new counted per-attempt outer-gate retry budget (GateRecord schema v4, ADR-0115), the retry counter is advanced by the act of *observing* an incomplete run rather than by an actual re-dispatch. So when run.max_attempts >= 3, repeated diagnostic `run gate-verdict` observations of the SAME unchanged, quiescent, run-incomplete record each grant another retry until the budget is spent (empirically confirmed in the 0421 run: 3 grants at run.max_attempts = 4). The hard bound still holds — total dispatches can never exceed the configured limit — and the shipping default (run.max_attempts = 2) is immune, so nothing unsafe ships today. But the counter over-counts against benign re-observation, which is a real correctness wart in the attribution model and a latent foot-gun for anyone who raises run.max_attempts. The proper fix binds a retry grant to a distinct dispatch epoch so that re-observing an unchanged attempt returns gate-stop / no-attributable-claim rather than advancing the counter. That change was deliberately deferred out of 0421 because it touches the deferred ship-once attribution/concurrency model, which is an open design question rather than a mechanical tweak.

## What changes

Change the outer run-gate retry accounting so a retry is consumed per dispatch epoch, not per gate-verdict observation. A diagnostic re-observation of an unchanged, quiescent, run-incomplete record must NOT advance the retry counter — it should return the existing gate-stop / no-attributable-claim disposition instead. Only a genuine new dispatch attempt against the change spends budget. Relevant surfaces from the 0421 build: the counted retry-consumed-<n> CAS budget snapshotted into GateRecord (schema v4) in internal/rungate (rungate_store.go / rungate_verdict.go), and the outer-gate verdict path that grants retries from the snapshotted run.max_attempts budget. This is a design-open change: it must reconcile with the deferred ship-once attribution/concurrency model before implementation, so it needs a brainstorm before it is build-ready (stub only).

## Out of scope

The durable per-phase build suite-attempt budget in internal/gatedrive (ADR-0116) — that is a separate budget with separate semantics and is not implicated by this over-count. The config leaves build.max_attempts / run.max_attempts themselves and their defaults (unchanged). Any change to the report token vocabulary. Not a fix to 0421, whose behavior is safe as shipped; this is follow-up hardening only.
