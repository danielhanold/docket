---
id: 448
slug: 'named-implement-next-skips-unrelated-maintenance-preflight'
title: 'Named implement-next skips unrelated maintenance preflight'
status: 'in-progress'
priority: 'critical'
type: 'fix'
created: '2026-09-23'
updated: '2026-09-24'
depends_on: [446]
stacked_on:
related: [389, 397, 444, 446]
discovered_from: [446]
adrs: [101, 106]
spec: 'docs/superpowers/specs/2026-09-23-named-implement-next-skips-unrelated-maintenance-preflight-design.md'
plan: 'docs/superpowers/plans/2026-09-24-named-implement-next-skips-unrelated-maintenance-preflight.md'
results: 'docs/results/2026-09-24-named-implement-next-skips-unrelated-maintenance-preflight-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/named-implement-next-skips-unrelated-maintenance-preflight'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-24T06:57:47Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-23-named-implement-next-skips-unrelated-maintenance-preflight-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-23-named-implement-next-skips-unrelated-maintenance-preflight-design.md) |
| Plan | [2026-09-24-named-implement-next-skips-unrelated-maintenance-preflight.md](https://github.com/danielhanold/docket/blob/fix/named-implement-next-skips-unrelated-maintenance-preflight/docs/superpowers/plans/2026-09-24-named-implement-next-skips-unrelated-maintenance-preflight.md) |
| Results | [2026-09-24-named-implement-next-skips-unrelated-maintenance-preflight-results.md](https://github.com/danielhanold/docket/blob/fix/named-implement-next-skips-unrelated-maintenance-preflight/docs/results/2026-09-24-named-implement-next-skips-unrelated-maintenance-preflight-results.md) |
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

## Reconcile log

### 2026-09-24

2026-09-24 — Reconciled against main c67d07ad. Dependency 446 is done, and 444 landed; neither touched the grounding sites. `skills/docket-implement-next/SKILL.md`, `internal/app/maintenance_preflight.go` and the sweep worklist in `internal/app/maintenance.go` are unchanged since the spec's grounding commit 442770e, so the spec stands as written. Named-entry callers found by search on main: `skills/docket-implement-next/SKILL.md` (and its embedded copy under `internal/assets/embedded/tree/`), `skills/docket-convention/SKILL.md` Composition (and embedded copy), and prose-contract guards in `internal/repoguard/prose_contracts_test.go`. No scope change.
