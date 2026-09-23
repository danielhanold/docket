---
id: 448
slug: 'named-implement-next-skips-unrelated-maintenance-preflight'
title: 'Named implement-next skips unrelated maintenance preflight'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-23'
updated: '2026-09-23'
depends_on: [446]
stacked_on:
related: [389, 397, 444, 446]
discovered_from: [446]
adrs: [101, 106]
spec: 'docs/superpowers/specs/2026-09-23-named-implement-next-skips-unrelated-maintenance-preflight-design.md'
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
| Spec | [2026-09-23-named-implement-next-skips-unrelated-maintenance-preflight-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-23-named-implement-next-skips-unrelated-maintenance-preflight-design.md) |
| ADRs | [ADR-0101](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0101-maintenance-sweep-scope-defer-historical-cleanup-out-of-impl.md), [ADR-0106](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0106-implementation-preflight-is-a-deterministic-operation-not-a.md) |
<!-- docket:artifacts:end -->

## Why

An explicitly named implement-next run executes the maintenance preflight before selecting its change. The sweep still runs merged-implemented closeouts, their cleanup suffixes, and expired-claim reclaims for every change. Any blocked, failed, unknown, or contended entry makes the verdict `problem`, and Step 0 halts before claiming. So another change's failing closeout can stop an unrelated named change from ever starting. Split out of change 446 (its former design section 7).

## What changes

- For a named change, go from repository preparation directly to `context.implementation --id` and the existing claim/resume path, with no maintenance preflight or status sweep first. This applies to initial named calls, attributed retries, and named resumes.
- Keep every local check for the named change: readiness, entity version, dependencies, effective base, claim eligibility, gate context, and resume/cancellation. Never fall back to selecting another change.
- Handle the named change's own merged-but-unclosed dependencies: bounded closeout of only those, or a local refusal with a working remedy.
- Keep the no-ID preflight, explicit `maintenance.preflight`, and `maintenance.sweep` unchanged.
- Record the narrowing of ADR-0101/ADR-0106.

Built after change 446 and before the metadata-validation change, in succession.

## Out of scope

New commands, flags, preflight scopes, verdict policies, background cleanup, or deferred queues. No-ID selection behavior, automatic finalize ordering, and driver stop/continue policy. Gate admission and run bookkeeping (change 446). Scoped metadata validation and board rendering (the follow-on change).
