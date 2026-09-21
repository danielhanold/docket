<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0350 — Surface the swallowed validation failure behind a bare internal-error in the transaction engine](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0350-surface-the-swallowed-validation-failure-behind-a-bare-inter.md)**
<!-- docket:backlink:end -->
# Surface the swallowed validation failure behind a bare internal-error in the transaction engine — Results

## Outcome

The transaction engine's early call-shape validation returns its base `Result` (an empty
disposition `""`) together with a typed `*transaction.Failure{Stage: StageValidateRequest,
Kind: KindInvalidInput, ...}` for malformed operation keys, malformed expectations (a shortened or
malformed expected-version object id), invalid idempotency keys, non-branch target refs, and a nil
loader. The app layer's shared outcome mappers previously flattened that shape to a bare
`internal-error` with no failure diagnosis, hiding the actual cause. This change extends the
existing change-0329 diagnostic machinery in `internal/app` — no new mechanism, vocabulary, field,
or engine change:

- `mapOutcome` (`internal/app/planning.go`) now routes an empty disposition through the existing
  `mapFailure(err)`; unknown non-empty dispositions still map to `internal-error`.
- `failureStatus` (`internal/app/planning.go`) now converts an empty disposition carrying a
  non-nil error through its existing typed-`Failure` conversion (preserving stage, kind, detail,
  and wrapped cause). An empty disposition with a nil error keeps its prior
  internal-error/no-diagnosis (`nil`) behavior; every other non-failed disposition is unchanged.
- `repairResultFromOutcome` (`internal/app/change_repair.go`) — the one envelope builder whose
  default arm did not attach the diagnosis — now attaches `failureStatus` there too, so an early
  (empty-disposition) engine error carries the same failure field its explicit `DispositionFailed`
  arm already attached.

Every other `*ResultFromOutcome` builder already calls `failureStatus` unconditionally after
`mapOutcome`, so all of them inherit the fix with no edit; the two best-effort backlink legs fold
the diagnosis through `backlinkLegDetail`, which itself calls `failureStatus`, and likewise need no
edit. The transaction engine and its validation rules are untouched.

## Verification performed

- Four TDD tasks, each driven RED→GREEN through the native gate driver with per-task mutation
  checks (all runs `-count=1`, caching disabled so mutations are meaningful):
  - `mapOutcome` empty-disposition routing — new `TestMapOutcome` rows; deleting the `case "":`
    arm reddens the two typed rows.
  - `failureStatus` empty-disposition diagnosis — new `TestFailureStatus` rows; deleting the
    `Disposition=="" && execErr != nil` case reddens the two diagnosis rows.
  - `repairResultFromOutcome` default-arm attachment — new
    `TestRepairEarlyEngineErrorCarriesFailure`; deleting the `r.Failure = failureStatus(...)` line
    reddens it, and every existing `TestRepair*` refusal/view-error control keeps `Failure` nil.
  - Real-engine regression `TestClaimResultRealEngineMalformedVersion` — drives a real
    `transaction.Engine` with a malformed (shortened) expected-version object id through
    `claimResultFromOutcome`, asserting `invalid-input` plus a populated diagnosis. It is the
    red-proof for both propagation branches (each mutation reddens it), satisfying the named
    learning *groomed-root-cause-is-a-hypothesis* by validating the root cause against the real
    engine rather than a synthetic `Result{}`.
- A whole-repo audit grep (`mapOutcome|mapFailure|failureStatus|ResultFromOutcome` over non-test Go
  sources) confirmed the only `mapOutcome` sites without an adjacent `failureStatus` attachment are
  the two backlink legs that route through `backlinkLegDetail` — no new consumer appeared.
- Full suite green at the build gate via the gate driver
  (`go run ./cmd/docket development test`): `SUITE files=52 passed=52 failed=0 asserts=423`.
  Build evidence recorded and verified against the branch head.
- Independent whole-branch review (docket-review-lean rung, selected from the economy build
  profile with a 557-line diff below the 1500-line bump threshold): clean, 0 findings.

## Findings and limitations

### Shared-mapper fix broadens diagnosis beyond the repair path (intended)

Because the fix lives in the shared mappers, every envelope builder that already calls
`failureStatus` unconditionally now surfaces a named result plus diagnosis for empty-disposition
outcomes, not only the repair envelope. An empty disposition arises only on the engine's call-shape
validation error path, so this strictly improves previously-swallowed cases. One behavior detail
outside the CLAUDE.md invariant (which pins only the nil-error case): an empty disposition carrying
an *untyped* error now yields a `Detail`-bearing `internal-error` `FailureStatus` where it was
previously `nil` — aligned with the change's "surface the swallowed failure" goal.

### Budget report (screening only, no breach)

The green full-suite run emitted `BUDGET WATCH:` screening lines (streak 1/5) for
`test_go_race.sh`, `test_go_toolchain.sh`, and several finalize/integration app suites under `-j11`.
No `SERIAL CONFIRMED OVER BUDGET:` line was present, so there is no authoritative serial breach to
act on; the parallel wall-clock numbers are machine-dependent screening findings only.
