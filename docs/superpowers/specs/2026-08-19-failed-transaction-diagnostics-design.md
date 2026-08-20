<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0329 — change refresh-claim reports invalid-state with empty findings — the transaction Failure is dropped on the DispositionFailed path](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-20-0329-change-refresh-claim-reports-invalid-state-with-empty-findin.md)**
<!-- docket:backlink:end -->

# Failed-transaction diagnostics — propagate the typed Failure into the result envelope

**Change:** 0329 · **Date:** 2026-08-19 · **Type:** fix · **Priority:** high

## Problem

On the `DispositionFailed` path, the app layer's result builders construct the protocol-v1 result
envelope exclusively from `transaction.Result`, discarding the typed `transaction.Failure` carried
by the returned `error`. The Failure holds the diagnosis — `Stage`, `Kind`, a human-readable
`Detail`, and an optional wrapped `Err` — and none of it reaches the caller:

- `mapOutcome` (internal/app/planning.go) routes `DispositionFailed` through `mapFailure(err)`,
  which reads only the Failure's `Kind` to pick a `Result` code. Correct, but lossy: `Stage`,
  `Detail`, and `Err` are dropped there.
- `claimResultFromOutcome` (internal/app/change_claim.go) sets
  `Findings: findingsToStatus(res.Findings)`. On a *failed* transaction — as opposed to a
  *refused* one — `res.Findings` is empty, because findings are the refusal channel.
- `claimDisposition` has no `DispositionFailed` arm; the `default:` with `replayed == false`
  returns `string(result)`, restating the result code as the disposition.

Observed live (change 0316's autonomous run): `docket change refresh-claim` returned
`result: invalid-state`, `disposition: invalid-state`, `findings: []` — a deterministic,
undiagnosable refusal. The run abandoned explicit refresh-claim and leaned on reconcile's internal
claim-refresh.

This is a general shape, not a refresh-claim quirk: every `*ResultFromOutcome` builder that folds
a `DispositionFailed` into an envelope built only from `transaction.Result` redacts its own
failure the same way.

## Decision summary (settled in the groom)

1. **Output shape:** a new, additive `failure` field on the result envelope — never a synthetic
   finding (findings remain the refusal channel) and never detail smuggled into the disposition
   string.
2. **Scope:** fix every affected builder in this change via one shared helper — not claim-only
   with a follow-up.
3. **Repro:** the 0316 trigger investigation is a **time-boxed first task**, not a ship gate; the
   propagation fix ships regardless of its outcome.

## Design

### 1. The `failure` envelope field

Add a `failure` object to the protocol-v1 result envelope, populated **only** on
`DispositionFailed`:

```
failure:
  stage:  <transaction.Failure.Stage>   # e.g. verify-delta, materialize, load-after
  kind:   <transaction.Failure.Kind>    # e.g. invalid-state, external, cancelled
  detail: <transaction.Failure.Detail>  # human-readable; wrapped Err appended when present
```

- Additive and optional: absent on every non-failed disposition, so existing readers are
  unaffected. No existing field changes meaning.
- `detail` folds the wrapped `Err` (when non-nil) into the message, e.g.
  `"plan violates before/after tree rules: <err>"` — one string, no nested error object.
- A `DispositionFailed` whose error is not a `transaction.Failure` (the contract violation
  `mapFailure` already maps to `ResultInternalError`) still emits a `failure` field with
  `kind: internal-error` and the bare error text as `detail` — the caller must never again see a
  failed result with no cause.

### 2. Shared conversion helper

One app-layer helper (e.g. `failureStatus(execErr) *FailureStatus`) converts the typed Failure
into the envelope field. Every result builder on the `DispositionFailed` path calls it; none
hand-roll the conversion.

**Derive the affected set by grep, never by list** (AGENTS.md): grep the shared helpers
(`mapOutcome`, `mapFailure`, `claimResultFromOutcome`, and the `*ResultFromOutcome` family) and
sort the hits into builders that already surface failure detail and builders that redact it. At
grooming time the family is ~10 builders across `change_claim.go`, `change_kill.go`,
`change_groom.go`, `change_halt.go`, `change_reclaim.go`, `change_reconcile.go`,
`change_lifecycle.go`, `change_implemented.go`, `change_attach.go`, `change_create.go`, and
`finalize_block.go` — but the build-time grep is authoritative, not this enumeration.

### 3. Real `DispositionFailed` disposition arms

`claimDisposition` — and every sibling disposition mapper with the same `default:` fall-through —
gains an explicit `DispositionFailed` arm returning a real `failed` disposition token, instead of
echoing `string(result)` back as a tautology. The cause lives in `failure`; the disposition names
what happened to the attempt.

## Task shape

1. **Time-boxed repro (first task).** Establish what makes `refresh-claim` fail deterministically
   against an in-progress change at a valid version. `domain.RefreshClaim` guards only
   `StatusInProgress` and that guard was satisfied on 0316, so the Failure originated below the
   domain layer. **Prime suspect:** the engine's verify-delta invalid-state failures
   (`"an undeclared path changed in the worktree"` / `"a declared path is not an actual change"`,
   internal/repository/transaction/commitverify.go) — the `.docket` metadata worktree is shared
   across agents and is routinely dirtied mid-operation. Build the repro as a **hermetic
   fixture** — never against a live claimed change. If reproduced: record the cause in the
   results file and cover it. If not reproduced within the box: record that, and move on — once
   the propagation ships, the next natural occurrence explains itself.
2. **Envelope field + helper.** Add the `failure` field and the shared conversion helper.
3. **Wire the builders.** Grep-derive the sites; wire each redacting builder to the helper; add
   the explicit `DispositionFailed` disposition arms.
4. **Regression tests.** For each wired operation shape (at minimum claim/refresh-claim plus one
   representative of the shared `lifecycleResultFromOutcome` family): a failed transaction must
   surface a non-empty, non-tautological cause — assert `failure.detail` is non-empty AND that
   the disposition is not merely the result string restated. **Mutation-test the guard**
   (AGENTS.md): strip the propagation (return nil from the helper), watch the asserts redden,
   defeating the Go test cache (`-count=1`).

## Error handling

- Non-`Failure` error on `DispositionFailed`: `kind: internal-error`, bare error text as detail
  (see §1) — the no-cause envelope becomes unrepresentable.
- Nil error on `DispositionFailed` (double contract violation): same internal-error shape with a
  fixed detail naming the violation.

## Out of scope

- Claim/refresh-claim lease semantics, the reclaim TTL policy, and reconcile's internal
  claim-refresh (working; covered the 0316 gap).
- Any non-additive redesign of the protocol-v1 envelope or the finding vocabulary — findings stay
  the refusal channel.
- The unrelated Step-5 gate failures on change 0316's branch.
- Fixing whatever the repro uncovers, if the fix is more than the diagnostics: a discovered
  engine/worktree defect becomes its own change, `discovered_from: [329]`.
