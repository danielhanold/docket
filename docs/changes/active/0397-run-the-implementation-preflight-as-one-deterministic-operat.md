---
id: 397
slug: 'run-the-implementation-preflight-as-one-deterministic-operat'
title: 'Run the implementation preflight as one deterministic operation instead of a docket-status dispatch, and drop status --json''s corpus records by default'
status: 'implemented'
priority: 'high'
type: 'perf'
created: '2026-09-02'
updated: '2026-09-02'
depends_on: []
stacked_on:
related: [389, 360, 17, 94]
discovered_from: []
adrs: [12, 24, 101, 47, 106]
spec: 'docs/superpowers/specs/2026-09-02-run-the-implementation-preflight-as-one-deterministic-operat-design.md'
plan: 'docs/superpowers/plans/2026-09-02-run-the-implementation-preflight-as-one-deterministic-operat.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'perf/run-the-implementation-preflight-as-one-deterministic-operat'
pr: 'https://github.com/danielhanold/docket/pull/269'
blocked_by:
reconciled: true
claimed_at: '2026-09-02T12:08:09Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-02-run-the-implementation-preflight-as-one-deterministic-operat-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-02-run-the-implementation-preflight-as-one-deterministic-operat-design.md) |
| Plan | [2026-09-02-run-the-implementation-preflight-as-one-deterministic-operat.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-02-run-the-implementation-preflight-as-one-deterministic-operat.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0101](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0101-maintenance-sweep-scope-defer-historical-cleanup-out-of-impl.md), [ADR-0047](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0047-digest-only-read-tier-skips-preflight.md), [ADR-0106](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0106-implementation-preflight-is-a-deterministic-operation-not-a.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next`'s Step 0 costs about two minutes and close to 85k tokens per run in this repository, while the maintenance sweep it exists to run finishes in under four seconds and emits 159 bytes (measured 2026-09-02 on `main` at `78d42319`: sweep 3.8 s / 159 B; `docket status --json` 164 KB; `docket capabilities --json` 12 KB; the `docket-status` + `docket-convention` skill preload 78 KB).

The cost is the topology around the sweep, not the sweep. Step 0 dispatches the `docket-status` subagent — a fresh model process carrying a 78 KB skill preload — which re-runs the capability bootstrap and `repository.prepare`, runs the sweep, reads the 164 KB status payload, and writes a prose report the parent must then validate against the originals. The parent afterwards re-runs `repository.prepare` and reads the same 164 KB status payload again for selection. 130 KB of that payload is the `records` array: kind, identity, location, path, and blob version for all 615 corpus records (every archived change, ADR, and learning) — the artifact-integrity inventory, which neither preflight, selection, nor the human `status` read uses.

Change 0389 (ADR-0101) made the sweep itself cheap with `--scope implementation` and explicitly left "new agent topology" and "skipping Step 0" out of scope; this change takes that next step. It also removes the failure class 0389 diagnosed — a child returning before its sweep finished — because an inline shell call has no early return.

## What changes

- A new cataloged operation, `maintenance.preflight` (`docket maintenance preflight`, effects `metadata-write`): one process that runs the implementation-scope sweep, then a compact post-sweep status read (`summary`, `ready`, `findings`; no `records`, no `changes`), and returns one protocol-v1 envelope carrying both halves, a Go-computed `preflight: clean | problem` verdict over the sweep entries, the `problem_entries` subset, and the post-sweep metadata revision. A thin composition over the existing `MaintenanceSweep` and `Status` entry points — no new sweep or status logic.
- `docket status --json` omits `records` by default; `--records` restores the identical array. The human renderer is unchanged. Maintained consumers are audited by whole-repo grep before the default flips.
- `docket-implement-next` Step 0 runs `maintenance.preflight` inline as its own call, validates the envelope, halts pre-claim on `problem`, and re-runs `repository.prepare` on `clean`. The child-only prose — completion barrier, late-notification correlation, child retirement, the Tier-A inline fallback — is removed with the dispatch.
- Prose follow-through: `docket-status` loses its step-0 mode (see-only and explicit full maintenance remain); the convention's *Composition* paragraph and Tier-A row drop the step-0 dispatch; `scripts/docket-status.md` and README pointers are updated; a new ADR records that the implementation preflight is a deterministic operation, not a composition dispatch (relates to ADR-0012, ADR-0024, ADR-0101; amends the change-0017 composition for step 0 only).
- Tests: Go coverage for the composition and each verdict rule (mutation-tested), the catalog entry, and the `records` opt-in; prose guards retiring the step-0 dispatch sentence; before/after measurements (status payload bytes, preflight wall clock, one real implement-next Step-0 token and wall-clock cost) recorded in the results file.

## Out of scope

- Changing what the sweep does, its scope vocabulary, or ADR-0101's deferral of historical cleanup retries.
- Retiring the `docket-status` agent, skill, wrapper, or harness pins — humans still use it for see-only reads and full maintenance; only the implement-next Step-0 caller moves off it.
- Any of change 0360's post-claim items (context after claim, session-scoped sync, mutation receipts, reconcile no-op).
- Slimming the `docket-convention` or `docket-implement-next` skill preload itself (change 0294 territory).
- Model or effort changes in `agents/harness-defaults.yml`.

## Reconcile log

### 2026-09-02

2026-09-02 — Reconciled against `main` at 78d42319 and `docket` at 209aeb31. Verified every spec assumption still holds in current code: no `maintenance.preflight` operation exists in `internal/` or `cmd/`; `docket status --json` computes `corpusRecords` unconditionally (`internal/app/status.go`) with no `IncludeRecords` option or `--records` flag; `docket-implement-next` Step 0 still dispatches the `docket-status` subagent; `maintenance.sweep --scope implementation` exists (used by this run's preflight) and ADR-0101 is Accepted. Related changes 389/360/17/94 and cited ADRs 12/24/101/47 remain accurate — no scope drift. No work done elsewhere to drop; scope unchanged. Building as specified: the new `maintenance.preflight` composition op, the `--records` opt-in for `status --json`, the Step-0 inline rewrite, prose follow-through, and a new ADR.
