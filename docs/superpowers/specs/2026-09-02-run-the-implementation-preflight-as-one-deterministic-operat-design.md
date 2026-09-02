<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0397 — Run the implementation preflight as one deterministic operation instead of a docket-status dispatch, and drop status --json's corpus records by default](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0397-run-the-implementation-preflight-as-one-deterministic-operat.md)**
<!-- docket:backlink:end -->

# Implementation preflight as one deterministic operation — design

**Date:** 2026-09-02
**Status:** settled (brainstormed with Daniel)

## Problem

`docket-implement-next`'s Step 0 costs about two minutes of wall clock and close to 85k tokens per
run in this repository, while the maintenance sweep it exists to run finishes in under four seconds
and emits 159 bytes. Measured on `main` at `78d42319` (2026-09-02):

| Piece | Wall clock | Payload |
|---|---|---|
| `docket maintenance sweep --scope implementation --json` | 3.8 s | 159 B |
| `docket status --json` | ~1 s | 164 KB |
| `docket capabilities --json` | <1 s | 12 KB |
| `docket-status` + `docket-convention` skill preload | — | 78 KB |

The cost is the topology around the sweep, not the sweep. Step 0 dispatches the `docket-status`
subagent (a fresh model process with a 78 KB skill preload), which re-runs the capability bootstrap
and `repository.prepare`, runs the sweep, reads the 164 KB status payload, and writes a prose report
the parent then has to validate against the originals. The parent afterwards re-runs `repository.prepare`
and reads the same 164 KB `status --json` again for selection.

Of the status payload, 130 KB is the `records` array: kind, identity, location, path, and blob
version for every one of the 615 records in the corpus (all archived changes, every ADR, every
learning). It is the artifact-integrity inventory — useful to `repository.check`-style callers,
unused by preflight, by selection, and by the human `status` read.

Change 0389 (ADR-0101) made the sweep itself cheap by introducing `--scope implementation`, and
explicitly left "new agent topology" and "skipping Step 0" out of scope. This change takes that next
step. Change 0360 is the broader post-claim coordination-tax umbrella; it lists "skip unconditional
status sweep when the allowlist is one claim-eligible id" as one item, which this change makes moot
by making the sweep itself cost nothing worth skipping.

## Goals

1. An implementation run's preflight costs seconds and a few hundred tokens, in every harness.
2. The parent gets the same evidence Step 0 requires today — terminal sweep envelope at
   implementation scope, per-item dispositions, a post-sweep read, the post-sweep metadata revision —
   from one typed envelope, with no prose intermediary.
3. Every reader of `docket status --json` stops paying for the corpus inventory unless it asks.

## Non-goals

- Changing what the sweep does, its scope vocabulary, or ADR-0101's deferral of historical cleanup.
- Retiring the `docket-status` agent or skill. Humans still use it for see-only reads and full
  maintenance; only the implement-next Step-0 *caller* moves off it.
- Any of change 0360's post-claim items (context after claim, session-scoped sync, receipts).
- Model or effort pins in `agents/harness-defaults.yml`.

## Design

### 1. A new operation: `maintenance.preflight`

`docket maintenance preflight [--repo-dir <dir>] --json`, cataloged as `maintenance.preflight` with
effects `[metadata-write]` (it runs the sweep). One process, one envelope. It is a thin
composition in `internal/app` over two existing entry points — `MaintenanceSweep` at
`SweepScopeImplementation`, then `Status` over a fresh snapshot — and adds no new sweep or status
logic. The mutation stays where it is; this operation only sequences it with the read and returns
both results together.

Envelope (protocol v1):

```json
{
  "protocol_version": 1,
  "operation": "maintenance.preflight",
  "result": "applied | no-op | refused | error",
  "preflight": "clean | problem",
  "sweep": {
    "result": "applied | no-op | …",
    "scope": "implementation",
    "entries": [ …every MaintenanceEntry, unchanged shape… ],
    "problem_entries": [ …the blocked / failed / unknown / contended subset… ],
    "deferred_historical_cleanups": 241
  },
  "status": {
    "result": "applied",
    "summary": { … },
    "ready": [ … ],
    "findings": [ … ]
  },
  "metadata_revision": "<sha of the metadata branch HEAD after the sweep>"
}
```

- `preflight` is computed in Go with the rule implement-next states today: `problem` when any sweep
  entry is `blocked`, `failed`, or `unknown`, or `contended`; `clean` otherwise. An intentional
  policy skip (`reclaim-auto-disabled`) and a genuine `noop` are `clean`. The parent keys on this
  field and the `result`, never on prose and never on an exit code.
- The `status` half is the compact read of design item 2 (no `records`). The `changes` array is
  omitted here too: selection's authority is `context.implementation`, and the digest the parent
  needs at Step 0 is `ready` plus `summary`. A `refused`/`error` on the read after a successful sweep
  yields `result: error` with the sweep half intact, so a parent never mistakes a failed read for a
  failed sweep — and never advances on either.
- If the sweep refuses or errors, `status` is absent and `preflight` is `problem`.
- `metadata_revision` is read from the metadata worktree after the sweep; it is the same value the
  old child reported as its post-sweep revision.
- Human (non-`--json`) output reuses the two existing `HumanText` renderers, sweep then status.

### 2. `docket status --json` drops `records` by default

`records` becomes opt-in behind `--records`. Without the flag the field is absent (not an empty
array — absence is the signal that the caller did not ask). With it, the array is exactly what is
emitted today, in the same order. The human text renderer never printed records and is unchanged.

The `status` app-layer function grows an option (`IncludeRecords`) so the CLI and the preflight
composition share one code path; `corpusRecords` is not computed when it is not requested.

Every maintained consumer is audited before the default flips: the repo-wide grep for `records` in
`skills/`, `agents/`, `scripts/`, `tests/`, `internal/`, and `docs/superpowers` on the integration
branch. As of writing, no maintained skill, script, or test reads the field; the only references are
the 2026-08-15 status spec and the 2026-08-30 migration plan (frozen records, left as they are). The
audit is repeated at build time, and any consumer found is either switched to `--records` or the
field it actually needs is exposed narrowly.

### 3. `docket-implement-next` Step 0 runs the operation inline

Step 0 stops dispatching `docket-status`. It resolves `maintenance.preflight` from the capability
catalog and runs it **as its own Bash call**, then:

- validates the envelope (`protocol_version` 1, `operation` `maintenance.preflight`, `sweep.scope`
  `implementation`);
- on `preflight: problem` (or `result` `refused`/`error`), halts before claiming through the
  existing pre-claim run-reporting path, surfacing `problem_entries` — the same posture as today's
  failed-preflight handoff, minus the "incomplete child return" and "late notification" branches,
  which have no counterpart when there is no child;
- on `preflight: clean`, re-runs `repository.prepare` (unchanged: the parent still derives readiness
  and claim state from fresh origin after any mutation) and continues to Step 1.

The completion-barrier prose that only existed because a child could return early — the terminal
sweep evidence barrier, late-notification correlation, child retirement verification, the Tier-A
inline fallback — is removed from Step 0. The operation is already inline; a shell call that
returns is terminal by construction. The retained guards are envelope validation and the
`problem` halt, which are mutation-tested (strip the `preflight` check, watch the halt test redden).

### 4. Prose and reference updates

- `docket-status/SKILL.md`: remove the "implement-next step-0 implementation preflight" mode; the
  two remaining modes are see-only (`status --json`) and explicit refresh/cleanup
  (`maintenance.sweep --scope full`). The `--scope implementation` vocabulary stays documented as
  the preflight operation's scope, with a pointer to `maintenance.preflight`.
- `docket-convention/SKILL.md`: the *Composition* paragraph no longer lists a `docket-status`
  step-0 dispatch; `docket-implement-next` dispatches `docket-plan-writer` (step 4) and `docket-adr`
  (step 6). The Tier A row of *Dispatch-capability resolution* keeps `docket-adr` and the `docket-status`
  composition dispatch generally (humans can still dispatch it), but the step-0 sentence is dropped.
- `docket-status/SKILL.md` *When to use*: drop the "implement-next calls this at step 0" bullet.
- `scripts/docket-status.md` and any README paragraph naming the step-0 dispatch: updated to name
  the operation.
- A new ADR records the decision: *implementation preflight is a deterministic operation, not a
  composition dispatch* — relates to ADR-0012 (script-vs-model boundary: the sweep is mechanical,
  and the judgment follow-ups the status skill keeps are not preflight concerns), ADR-0024
  (fork/dispatch completion: a child that cannot signal completion is the failure class this
  removes), and ADR-0101 (the scope this operation runs at). It amends the change-0017 composition
  decision for step 0 only.
- The `docket-status` agent wrapper, its harness pins, and the `docket:dispatch` managed block are
  untouched.

### 5. Tests

- Go: `maintenance.preflight` composition — a merged-implemented fixture closes out and the envelope
  carries both halves and a `clean` verdict; a fixture with a blocked cleanup yields `problem` with
  the entry in `problem_entries`; a sweep refusal yields `problem`, no `status`, `result: refused`;
  a read failure after a good sweep yields `result: error` with the sweep half intact;
  `metadata_revision` equals the metadata HEAD after the sweep. Each verdict rule is mutation-tested.
- Go: the capability catalog carries `maintenance.preflight` with `metadata-write` effects; the
  production catalog test covers it.
- Go: `status --json` omits `records` by default and includes the identical array with `--records`;
  the human renderer is byte-identical either way.
- Prose guards: the suite already greps skill prose for retired invocation shapes; add the
  step-0 `docket-status` dispatch sentence to that retired set so it cannot come back, and assert
  implement-next names `maintenance.preflight`.
- Measurement, recorded in the results file: `docket status --json` byte size before and after on
  this repo; `maintenance preflight` wall clock; and one real `docket-implement-next` run's Step-0
  token and wall-clock cost before and after, from the harness transcript.

## Open questions resolved during brainstorm

- **Separate op vs a flag on `context.implementation`?** Separate. `context.implementation` is
  cataloged with `read` effects; folding a metadata-write into it would break the effect vocabulary
  the catalog exists to make honest.
- **Keep the subagent and slim it?** No. A fresh subagent bootstrap plus a prose report is the
  floor of the cost, and its completion-signalling failure class (0389's six-minute early return) is
  the thing being removed.
- **Skip Step 0 for targeted runs only?** Moot once the preflight costs seconds.
