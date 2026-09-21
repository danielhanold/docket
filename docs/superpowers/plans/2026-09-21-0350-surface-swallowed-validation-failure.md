<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0350 — Surface the swallowed validation failure behind a bare internal-error in the transaction engine](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0350-surface-the-swallowed-validation-failure-behind-a-bare-inter.md)**
<!-- docket:backlink:end -->
# Surface the Swallowed Validation Failure (change 0350) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the app layer's shared outcome mappers diagnose the transaction engine's early call-shape validation errors (empty disposition + typed `*transaction.Failure`) instead of flattening them to a bare `internal-error` with no failure diagnosis.

**Architecture:** The transaction engine (`internal/repository/transaction/engine.go`, `Engine.Execute`) returns its base `Result` — disposition `""` — plus a typed `*Failure{Stage: StageValidateRequest, Kind: KindInvalidInput, ...}` for every call-shape validation error (invalid operation key, invalid expectations such as a malformed/short expected version object id, invalid idempotency key, non-branch target ref, nil loader). The app layer's shared mappers in `internal/app/planning.go` currently route the empty disposition to `internal-error` (`mapOutcome` default arm) and return no diagnosis (`failureStatus` returns nil on every non-failed disposition). The fix extends the existing change-0329 diagnostic machinery: `mapOutcome` routes an empty disposition through the existing `mapFailure(err)`, `failureStatus` converts an empty disposition carrying a non-nil error through its existing typed-Failure conversion, and `repairResultFromOutcome` (`internal/app/change_repair.go`) — the one envelope builder whose default arm does not attach `failureStatus` — attaches it there too. The engine and its validation rules are untouched.

**Consumer audit (already performed; scope is closed):** a whole-repo grep of `mapOutcome`/`mapFailure`/`failureStatus`/`ResultFromOutcome` over non-test Go sources shows every envelope builder (`claimResultFromOutcome`, `lifecycleResultFromOutcome`, `haltResultFromOutcome`, `reclaimResultFromOutcome`, `changeGroomResultFromOutcome`, `blockResultFromOutcome`, `clearBlockResultFromOutcome`, `attachResultFromOutcome`, `changeCreateResultFromOutcome`, `changeKillResultFromOutcome`, `changeReconcileResultFromOutcome`, the learning/ADR op builders, and the finalize closeout builders) already calls `r.Failure = failureStatus(res, execErr)` unconditionally after `mapOutcome`, so they inherit both helper changes with no edit. The two exceptions: (a) `repairResultFromOutcome` in `internal/app/change_repair.go` attaches `failureStatus` only in its explicit `DispositionFailed` arm — Task 3 fixes its default arm; (b) the two best-effort backlink legs (`cleanupBacklinkOp` caller in `internal/app/finalize_cleanup.go` and its twin in `internal/app/finalize_closeout.go`) deliberately fold the diagnosis into a finding message via `backlinkLegDetail` (`internal/app/finalize_backlink_loader.go`), which itself calls `failureStatus` — they inherit the fix through that helper and need no edit. Re-run the grep at build time to confirm no new consumer appeared:
`grep -rn "mapOutcome\|mapFailure\|failureStatus\|ResultFromOutcome" internal/ --include='*.go' | grep -v _test.go`

**Tech Stack:** Go; table-driven tests in package `app` (`internal/app/planning_test.go`, `internal/app/change_repair_test.go`, `internal/app/change_claim_test.go`); module `github.com/danielhanold/docket`.

**Spec:** none — trivial change; this plan is derived from the change file's `## What changes` and Verification paragraph: `docs/changes/active/0350-surface-the-swallowed-validation-failure-behind-a-bare-inter.md` (on the `docket` metadata branch, synchronized at `.docket/` in the primary tree).

## Global Constraints

- The transaction engine (`internal/repository/transaction/`) and its validation rules stay unchanged — every edit in this plan is in `internal/app/`.
- No new status/disposition/finding vocabulary, no new failure fields, no engine return-contract change (change file `## Out of scope`).
- Every test run in this plan passes `-count=1` (mutation checks against a cached runner are meaningless — the cache serves the pre-mutation tree).
- Every new mapping/propagation branch gets a mutation check: strip the branch, watch the named test redden, restore.
- Comments cross-reference symbols or verbatim clauses, never line numbers (repo rule, `TestCommentAnchorStyle`).
- Empty disposition with a **nil** error keeps today's behavior: `mapOutcome` → `ResultInternalError` (via `mapFailure`'s `AsFailure` miss), `failureStatus` → nil. Unknown **non-empty** dispositions keep mapping to `ResultInternalError` with no diagnosis.
- The full suite runs once at the docket-build gate (command resolved from `build.test_command`); per-task runs are the focused package tests below.

---

### Task 1: mapOutcome routes an empty disposition through mapFailure

**Files:**
- Modify: `internal/app/planning.go` (function `mapOutcome` and its doc comment)
- Test: `internal/app/planning_test.go` (table in `TestMapOutcome`)

**Interfaces:**
- Consumes: existing `mapFailure(err error) Result` (unchanged), `transaction.Result`, `transaction.Failure`.
- Produces: `mapOutcome(res transaction.Result, err error, refusalKind Result) (Result, bool)` now returns `mapFailure(err), false` when `res.Disposition == ""`; all other arms unchanged. Tasks 3 and 4 rely on this exact behavior.

- [ ] **Step 1: Write the failing test rows**

In `internal/app/planning_test.go`, extend the `cases` table in `TestMapOutcome` (append after the existing `"unknown-disposition"` row, keeping that row as the control that a *non-empty* unknown disposition still maps to internal-error). The existing `fail` helper in that test builds `&transaction.Failure{Stage: transaction.StageCommit, Kind: k}`; reuse it — `mapOutcome` keys on Kind only.

```go
		// Engine early call-shape validation: base result (empty disposition)
		// plus a typed *Failure routes through mapFailure instead of
		// flattening to internal-error (change 0350).
		{"empty-disposition-typed-failure", transaction.Result{}, fail(transaction.KindInvalidInput), ResultInvalidState, ResultInvalidInput, false},
		{"empty-disposition-typed-validation", transaction.Result{}, fail(transaction.KindValidation), ResultInvalidState, ResultInvalidState, false},
		{"empty-disposition-untyped-error", transaction.Result{}, errors.New("bare"), ResultInvalidState, ResultInternalError, false},
		{"empty-disposition-nil-error", transaction.Result{}, nil, ResultInvalidState, ResultInternalError, false},
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/ -run 'TestMapOutcome' -count=1 -v`
Expected: FAIL — `empty-disposition-typed-failure: mapOutcome = ("internal-error", false), want ("invalid-input", false)` and the `empty-disposition-typed-validation` row likewise; the untyped/nil rows already pass (they document preserved behavior).

- [ ] **Step 3: Implement the routing arm**

In `internal/app/planning.go`, replace the `mapOutcome` switch tail:

```go
	case transaction.DispositionFailed:
		return mapFailure(err), false
	case "":
		// The engine's call-shape validation fails before any attempt runs,
		// returning its base result — an empty disposition — with a typed
		// *Failure. Route it through the same failure mapping so the invalid
		// input is named instead of flattened to internal-error; a nil or
		// untyped error still lands on internal-error via mapFailure's
		// AsFailure miss.
		return mapFailure(err), false
	default:
		return ResultInternalError, false
	}
```

And extend the `mapOutcome` doc comment's last sentence family to describe both handled error shapes — e.g. append: `An empty disposition is the engine's early call-shape validation return and routes through mapFailure; an unknown non-empty disposition remains internal-error.`

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/app/ -run 'TestMapOutcome' -count=1 -v`
Expected: PASS (all rows, including the retained `failed-*`, `unknown-disposition`, and refusal controls).

- [ ] **Step 5: Mutation check**

Temporarily delete the whole `case "":` arm added in Step 3 (so empty falls back to `default`), run `go test ./internal/app/ -run 'TestMapOutcome' -count=1`, and confirm FAIL on the two typed empty-disposition rows. Restore the arm, re-run, confirm PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/planning.go internal/app/planning_test.go
git commit -m "fix(app): mapOutcome routes empty-disposition engine errors through mapFailure (change 0350)"
```

---

### Task 2: failureStatus diagnoses an empty disposition carrying a non-nil error

**Files:**
- Modify: `internal/app/planning.go` (function `failureStatus` and its doc comment)
- Test: `internal/app/planning_test.go` (table in `TestFailureStatus`)

**Interfaces:**
- Consumes: `transaction.AsFailure`, `transaction.DispositionFailed` (unchanged).
- Produces: `failureStatus(res transaction.Result, execErr error) *FailureStatus` now returns the existing typed-Failure conversion (preserving Stage, Kind, Detail, wrapped-cause folding) when `res.Disposition == "" && execErr != nil`; empty disposition with nil error still returns nil; every other non-failed disposition still returns nil. Task 3's default-arm attachment and `backlinkLegDetail` depend on this.

- [ ] **Step 1: Write the failing test rows**

In `internal/app/planning_test.go`, extend the `cases` table in `TestFailureStatus` (append after the `"nil-error-is-contract-violation"` row; keep the existing `"non-failed-disposition-is-nil"` refused-disposition control):

```go
		// Engine early call-shape validation (empty disposition + non-nil
		// error) yields the same typed conversion as a failed disposition
		// (change 0350).
		{"empty-disposition-typed-failure",
			transaction.Result{},
			&transaction.Failure{Stage: transaction.StageValidateRequest, Kind: transaction.KindInvalidInput, Detail: "invalid expectations", Err: errors.New("transaction: expected version object id must be 40 lowercase hex characters")},
			&FailureStatus{Stage: string(transaction.StageValidateRequest), Kind: string(transaction.KindInvalidInput), Detail: "invalid expectations: transaction: expected version object id must be 40 lowercase hex characters"}},
		{"empty-disposition-untyped-error",
			transaction.Result{}, errors.New("bare"),
			&FailureStatus{Kind: "internal-error", Detail: "bare"}},
		{"empty-disposition-nil-error-stays-undiagnosed",
			transaction.Result{}, nil, nil},
```

Note: the typed row's `Err` string is illustrative fixture text, not an engine-coupling — any wrapped error works because the assertion is on `failureStatus`'s own folding (`Detail + ": " + Err.Error()`).

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/ -run 'TestFailureStatus' -count=1 -v`
Expected: FAIL — `empty-disposition-typed-failure` and `empty-disposition-untyped-error` report `failureStatus = nil, want a diagnosis`; the nil-error row already passes.

- [ ] **Step 3: Implement the widened predicate**

In `internal/app/planning.go`, replace `failureStatus`'s opening guard:

```go
func failureStatus(res transaction.Result, execErr error) *FailureStatus {
	switch {
	case res.Disposition == transaction.DispositionFailed:
		// Mid-flight transaction failure — diagnose below even when the
		// error is missing or untyped.
	case res.Disposition == "" && execErr != nil:
		// The engine's early call-shape validation return: base result
		// (empty disposition) plus a typed *Failure. Same conversion.
	default:
		return nil
	}
	if execErr == nil {
```

(the remainder of the function — the nil-error contract-violation return, the `AsFailure` conversion, and the detail folding — is unchanged). Update the doc comment to describe both supported error shapes, e.g.:

```go
// failureStatus converts a transaction's typed Failure into the envelope's
// failure diagnosis. Two outcome shapes carry one: a failed disposition
// (mid-flight failure), and an empty disposition accompanied by a non-nil
// error — the engine's call-shape validation return, produced before any
// attempt runs. It returns nil on every other disposition, and on an empty
// disposition with a nil error (that contract violation keeps its
// internal-error/no-diagnosis shape at mapOutcome). A failed disposition
// whose error is missing or not a *transaction.Failure is the same contract
// violation mapFailure reports as internal-error; it still yields a
// diagnosis, so a failed result can never again reach the caller cause-free.
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/app/ -run 'TestFailureStatus' -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Mutation check**

Temporarily change `case res.Disposition == "" && execErr != nil:` back to an unconditional `default: return nil` shape (i.e. delete that middle case), run `go test ./internal/app/ -run 'TestFailureStatus' -count=1`, confirm FAIL on the two new diagnosis rows. Restore, re-run, confirm PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/planning.go internal/app/planning_test.go
git commit -m "fix(app): failureStatus diagnoses empty-disposition engine errors (change 0350)"
```

---

### Task 3: repairResultFromOutcome's default arm attaches the shared diagnosis

**Files:**
- Modify: `internal/app/change_repair.go` (function `repairResultFromOutcome` and its doc comment)
- Test: `internal/app/change_repair_test.go` (new focused test `TestRepairEarlyEngineErrorCarriesFailure`)

**Interfaces:**
- Consumes: Task 1's `mapOutcome` empty-disposition routing and Task 2's `failureStatus` empty-disposition conversion — this task must run after both.
- Produces: `repairResultFromOutcome(field, value string, res transaction.Result, execErr error, id int) RepairIdentityResult` whose default arm now sets `r.Failure = failureStatus(res, execErr)` before returning, so an early (empty-disposition) engine error carries the same envelope failure field the explicit `DispositionFailed` arm already attaches.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/change_repair_test.go` (the file already imports `errors` and package `transaction` types are reachable as in sibling tests — add the `transaction` import if this file does not already have it):

```go
// TestRepairEarlyEngineErrorCarriesFailure pins change 0350's propagation for
// the repair envelope: an engine call-shape validation error (empty
// disposition + typed *Failure) must reach the caller as the mapped result
// AND a populated failure diagnosis via the default mapping arm — not only
// via the explicit DispositionFailed arm.
func TestRepairEarlyEngineErrorCarriesFailure(t *testing.T) {
	execErr := &transaction.Failure{
		Stage:  transaction.StageValidateRequest,
		Kind:   transaction.KindInvalidInput,
		Detail: "invalid expectations",
		Err:    errors.New("short object id"),
	}
	r := repairResultFromOutcome("branch", "fix/x", transaction.Result{}, execErr, 350)
	if r.Result != ResultInvalidInput {
		t.Fatalf("Result = %q, want %q", r.Result, ResultInvalidInput)
	}
	if r.Failure == nil {
		t.Fatal("Failure = nil, want the typed diagnosis")
	}
	want := FailureStatus{
		Stage:  string(transaction.StageValidateRequest),
		Kind:   string(transaction.KindInvalidInput),
		Detail: "invalid expectations: short object id",
	}
	if *r.Failure != want {
		t.Errorf("Failure = %+v, want %+v", *r.Failure, want)
	}
	if r.ID != 350 {
		t.Errorf("ID = %d, want 350", r.ID)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/ -run 'TestRepairEarlyEngineErrorCarriesFailure' -count=1 -v`
Expected: FAIL with `Failure = nil, want the typed diagnosis` (the Result assertion already passes given Task 1).

- [ ] **Step 3: Implement the default-arm attachment**

In `internal/app/change_repair.go`, `repairResultFromOutcome`'s `default:` arm becomes:

```go
	default:
		result, _ := mapOutcome(res, execErr, ResultInvalidState)
		r := newRepairResult(result, RepairIdentityResult{
			ID: id, Reason: firstFindingCode(res.Findings), Findings: findingsToStatus(res.Findings),
		})
		r.Failure = failureStatus(res, execErr)
		return r
	}
```

Update the function's doc comment tail from "a failure carries its typed cause in the envelope" to name both shapes, e.g.: `a failure — mid-flight (failed disposition) or the engine's early call-shape validation return (empty disposition with an error) — carries its typed cause in the envelope's failure diagnosis.` (`failureStatus` returns nil for refused/no-op/interrupted outcomes, so the attachment is a no-op on every other default-arm disposition.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestRepair' -count=1 -v`
Expected: PASS — the new test plus every existing `TestRepair*` control (refusals and view-error paths must keep `Failure` nil; `failureStatus` guarantees that).

- [ ] **Step 5: Mutation check**

Temporarily delete the `r.Failure = failureStatus(res, execErr)` line added in Step 3, run `go test ./internal/app/ -run 'TestRepairEarlyEngineErrorCarriesFailure' -count=1`, confirm FAIL. Restore, re-run, confirm PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/change_repair.go internal/app/change_repair_test.go
git commit -m "fix(app): repair envelope carries early engine-error diagnosis (change 0350)"
```

---

### Task 4: real-engine malformed-version regression through claimResultFromOutcome

**Files:**
- Test: `internal/app/change_claim_test.go` (new test `TestClaimResultRealEngineMalformedVersion` plus two small test-only types)

**Interfaces:**
- Consumes: `transaction.NewEngine(client *gitcli.Client, clock transaction.Clock)`, `(*transaction.Engine).Execute(ctx, transaction.Request)`, `gitcli.NewClient()`, `claimResultFromOutcome(opKey string, res transaction.Result, execErr error) ChangeClaimResult` and the `OperationChangeClaim` constant — all existing; Tasks 1–2 supply the mapping behavior under test.
- Produces: nothing new for later tasks — this is the end-to-end regression the change's Verification paragraph requires: the real engine's early validation return, passed through the real result builder, surfaces `invalid-input` plus a populated failure diagnosis.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/change_claim_test.go` (add imports as needed: `context`, `errors`, `strings`, `time`, `github.com/danielhanold/docket/internal/gitcli`, `github.com/danielhanold/docket/internal/repository/transaction`):

```go
// claimEarlyErrOp is a minimal valid-keyed semantic operation for driving the
// real engine's call-shape validation; Execute fails on the malformed
// expectation before Plan can ever run.
type claimEarlyErrOp struct{}

func (claimEarlyErrOp) Key() transaction.OperationKey { return "change.claim" }

func (claimEarlyErrOp) Plan(context.Context, transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	return transaction.MutationPlan{}, transaction.OperationResult{}, errors.New("unreachable: call-shape validation fails first")
}

// claimEngineClock pins the engine's clock; the validation path never reads it.
type claimEngineClock struct{}

func (claimEngineClock) Now() time.Time { return time.Unix(1758400000, 0).UTC() }

// TestClaimResultRealEngineMalformedVersion is change 0350's end-to-end
// regression: a REAL transaction.Engine given a malformed (shortened)
// expected-version object id returns its base result — empty disposition —
// with a typed *Failure from StageValidateRequest, and claimResultFromOutcome
// must surface that as invalid-input with a populated failure diagnosis, not
// a bare internal-error.
func TestClaimResultRealEngineMalformedVersion(t *testing.T) {
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	eng, err := transaction.NewEngine(client, claimEngineClock{})
	if err != nil {
		t.Fatalf("transaction.NewEngine: %v", err)
	}
	res, execErr := eng.Execute(context.Background(), transaction.Request{
		TargetRef: "refs/heads/docket",
		Expected: []transaction.EntityExpectation{{
			Path: "docs/changes/active/0350-surface.md",
			// Shortened object id — the confirmed early-validation trigger
			// (a full-length well-formed wrong id follows the contended
			// path instead; see the change file's "## Why").
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: "abc123"},
		}},
		Operation: claimEarlyErrOp{},
		// Loader deliberately nil: expectations are validated before the
		// loader, so Execute must return before touching it or any git state.
	})
	if res.Disposition != "" {
		t.Fatalf("Disposition = %q, want empty (early validation return)", res.Disposition)
	}
	if execErr == nil {
		t.Fatal("Execute error = nil, want a typed *Failure")
	}
	out := claimResultFromOutcome(OperationChangeClaim, res, execErr)
	if out.Result != ResultInvalidInput {
		t.Fatalf("Result = %q, want %q", out.Result, ResultInvalidInput)
	}
	if out.Failure == nil {
		t.Fatal("Failure = nil, want the typed diagnosis")
	}
	if out.Failure.Stage != string(transaction.StageValidateRequest) {
		t.Errorf("Failure.Stage = %q, want %q", out.Failure.Stage, transaction.StageValidateRequest)
	}
	if out.Failure.Kind != string(transaction.KindInvalidInput) {
		t.Errorf("Failure.Kind = %q, want %q", out.Failure.Kind, transaction.KindInvalidInput)
	}
	if !strings.Contains(out.Failure.Detail, "invalid expectations") {
		t.Errorf("Failure.Detail = %q, want it to name the invalid expectations", out.Failure.Detail)
	}
}
```

(If a same-shape stub/clock already exists in package `app`'s test files, reuse it instead of adding a duplicate — check with `grep -rn "transaction.NewEngine\|SemanticOperation" internal/app/*_test.go` first and drop the redundant type.)

- [ ] **Step 2: Run the test — on the post-Task-2 tree it passes; prove it can fail**

Run: `go test ./internal/app/ -run 'TestClaimResultRealEngineMalformedVersion' -count=1 -v`
Expected: PASS (Tasks 1–2 already landed). A regression test that has never been seen red proves nothing, so the mutation check in Step 3 is this task's red proof — do not skip it.

- [ ] **Step 3: Mutation check (both propagation branches)**

1. In `internal/app/planning.go`, temporarily delete `mapOutcome`'s `case "":` arm. Run the Step 2 command; confirm FAIL on `Result = "internal-error", want "invalid-input"`. Restore.
2. In `internal/app/planning.go`, temporarily delete `failureStatus`'s `case res.Disposition == "" && execErr != nil:` arm. Run the Step 2 command; confirm FAIL on `Failure = nil`. Restore.
3. Re-run the Step 2 command; confirm PASS.

- [ ] **Step 4: Run the package and confirm the consumer audit still holds**

Run: `go test ./internal/app/ -count=1`
Expected: PASS. Then re-run the audit grep from the plan header and confirm the only `mapOutcome` call sites without an adjacent `failureStatus` attachment are the two backlink legs that route through `backlinkLegDetail`:
`grep -rn "mapOutcome\|mapFailure\|failureStatus\|ResultFromOutcome" internal/ --include='*.go' | grep -v _test.go`

- [ ] **Step 5: Commit**

```bash
git add internal/app/change_claim_test.go
git commit -m "test(app): real-engine malformed-version regression surfaces typed cause (change 0350)"
```

---

## Final gate (owned by docket-build)

The whole suite runs once at the build gate via the command `build.test_command` resolves to, entered from source — never a hand-picked subset. Read the budget report even on green (`BUDGET WATCH:` / `PARALLEL-SENSITIVE:` / `SERIAL CONFIRMED OVER BUDGET:` lines are findings nothing else surfaces).
