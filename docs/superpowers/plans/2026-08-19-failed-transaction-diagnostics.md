<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0329 — change refresh-claim reports invalid-state with empty findings — the transaction Failure is dropped on the DispositionFailed path** — `docs/changes/active/0329-change-refresh-claim-reports-invalid-state-with-empty-findin.md`
<!-- docket:backlink:end -->
# Failed-Transaction Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Propagate the typed `transaction.Failure` into the protocol-v1 result envelope on the `DispositionFailed` path, so a failed operation names its own cause instead of returning `result: invalid-state, disposition: invalid-state, findings: []`.

**Architecture:** One additive `failure` field (`{stage, kind, detail}`) on the shared `Envelope` struct, populated by one shared app-layer helper `failureStatus(res, execErr)` that every `*ResultFromOutcome` builder (grep-derived set) calls after constructing its result. Disposition mappers reached on `DispositionFailed` gain an explicit `failed` arm instead of echoing the result string or mislabeling the failure as a refusal/skip.

**Tech Stack:** Go (`internal/app`, `internal/repository/transaction`), Go unit tests in `internal/app/*_test.go`, git-backed app tests in the `claim_workflow_git_test.go` style, suite gate via `scripts/run-tests.sh` (the `finalize.test_command`).

**Spec:** `docs/superpowers/specs/2026-08-19-failed-transaction-diagnostics-design.md` (on the `docket` metadata branch)

## Global Constraints

- Protocol v1 is additive-only: the `failure` field is optional and absent on every non-failed disposition (`omitempty` pointer); no existing field changes name, type, or meaning.
- Findings remain the refusal channel — never synthesize a finding to carry a failure cause.
- The affected-builder set is **derived by grep at build time** (AGENTS.md: never hand-list the sites of an operation you are gating). The inventories in this plan are observations from planning time, not the authority.
- Every guard is mutation-tested: strip the propagation, watch the assert redden, restore. Go's test cache serves stale green — always `-count=1` on mutation probes (learnings: `cached-runner-serves-a-mutated-tree`).
- Mutation restore: revert the transient edit by hand, never `git checkout -- <file>` over uncommitted work (learnings: `mutation-restore-needs-a-backup-copy`).
- Comments anchor on symbol names or verbatim-quoted clauses, never line numbers (AGENTS.md).
- Full suite at the build gate: `scripts/run-tests.sh`. A trailing `OVER BUDGET:` line is a finding to act on.
- Commit message style: `fix(0329): <summary>` matching recent history.

---

### Task 1: Time-boxed reproduction of the 0316 refresh-claim failure

This is a **spike, not a ship gate** (spec: "the propagation fix ships regardless of its outcome"). Time box: at most three hypothesis probes or roughly one focused session, whichever ends first. Never probe against a live claimed change — hermetic fixtures only.

**Files:**
- Read: `internal/repository/transaction/engine.go` (stage sequence), `internal/repository/transaction/commitverify.go` (`verifyDelta` failures), `internal/app/change_claim.go` (`ChangeRefreshClaim`), `internal/app/claim_workflow_git_test.go` (fixture pattern to copy)
- Create (only if reproduced): a test in `internal/app/claim_workflow_git_test.go`

**Interfaces:**
- Consumes: existing git-backed test fixtures in `claim_workflow_git_test.go` (hermetic temp repo + real engine).
- Produces: either a committed reproducing test named `TestRefreshClaimFailsOnDirtiedCorpus` (or a truer name once the mechanism is known), or a recorded not-reproduced note in the build evidence. Task 4's regression tests do not depend on this task.

**Background for the implementer:** `domain.RefreshClaim` guards only `StatusInProgress`, and change 0316 *was* in-progress at a valid version when `refresh-claim` returned a deterministic `DispositionFailed` mapped to `invalid-state`. So the Failure originated below the domain guard. Ranked hypotheses:

1. **Verify-delta invalid-state** (prime suspect per spec): `verifyDelta` fails with `"an undeclared path changed in the worktree"` or `"a declared path is not an actual change"` (`internal/repository/transaction/commitverify.go`). Refresh re-renders derived views; if the corpus moved between the caller's read and the transaction (other records changed → the re-rendered board/views differ on paths the refresh plan did not declare, or a declared view renders byte-identical and is therefore "not an actual change"), verify-delta fails.
2. **Load-after tree-rule violation**: `"plan violates before/after tree rules"` (`internal/repository/transaction/engine.go`, `StageLoadAfter`, `KindInvalidState`).
3. **Idempotency-scan invalid-state**: `"request-id history is duplicate, malformed, or contradictory"` — note `ChangeRefreshClaim` sends **no** `Idempotency` key (unlike `ChangeClaim`), which weakens this hypothesis; confirm by reading the request it builds.

- [ ] **Step 1: Read the engine's failure sites**

Grep the failure constructors and note which `KindInvalidState` sites are reachable from a refresh-claim transaction:

```bash
grep -n "KindInvalidState" internal/repository/transaction/*.go | grep -v _test
```

- [ ] **Step 2: Build the hypothesis-1 fixture**

Copy the fixture setup from an existing test in `internal/app/claim_workflow_git_test.go` (temp origin repo, metadata branch, real engine deps). Sequence: create + claim a change; read the claimed record's blob version the way the workflow does; mutate a *different* record (or another corpus file a re-render consumes) directly on the metadata branch to simulate the shared `.docket` worktree moving; call `app.ChangeRefreshClaim` with the claimed change's still-valid version. Inspect the outcome:

```go
res := app.ChangeRefreshClaim(ctx, deps, repoDir, app.ChangeClaimRequest{ID: id, Version: version})
t.Logf("result=%s disposition=%s findings=%v", res.Result, res.Disposition, res.Findings)
```

To see the raw engine outcome while probing, temporarily log inside `claimResultFromOutcome` (`transaction.AsFailure(execErr)` gives `Stage`/`Kind`/`Detail`); remove the probe logging before any commit.

- [ ] **Step 3: Evaluate — reproduced or move to the next hypothesis**

If `DispositionFailed` reproduced: tighten the test to assert the mechanism with positive evidence (learnings: `assert-pins-outcome-not-mechanism`) — assert `transaction.AsFailure` yields the observed `Stage` and `Kind`, not merely "it failed". If not reproduced, spend at most the two remaining probes on hypotheses 2 and 3, then stop.

- [ ] **Step 4: Close the box**

Reproduced: commit the test.

```bash
git add internal/app/claim_workflow_git_test.go
git commit -m "test(0329): reproduce the refresh-claim DispositionFailed against a dirtied corpus"
```

Not reproduced: commit nothing; record in the build evidence exactly which hypotheses were probed, the fixture shape, and the observed outcomes ("not reproduced within the box" is the recorded verdict the spec asks for). If the probe uncovers an engine/worktree defect whose fix exceeds diagnostics, do not fix it here — it becomes its own change with `discovered_from: [329]`.

---

### Task 2: The `failure` envelope field and the shared `failureStatus` helper

**Files:**
- Modify: `internal/app/result.go` (`Envelope` struct; add `FailureStatus` type)
- Modify: `internal/app/planning.go` (add `failureStatus` beside `mapOutcome`/`mapFailure`)
- Test: `internal/app/planning_test.go`, `internal/app/result_test.go`

**Interfaces:**
- Consumes: `transaction.Result`, `transaction.Failure{Stage, Kind, Detail, Err}`, `transaction.AsFailure(err) (*Failure, bool)` (`internal/repository/transaction/result.go`).
- Produces: `type FailureStatus struct { Stage, Kind, Detail string }` (JSON `stage`/`kind`/`detail`), `Envelope.Failure *FailureStatus` (JSON `failure,omitempty`), and `func failureStatus(res transaction.Result, execErr error) *FailureStatus`. Tasks 3 and 4 call `failureStatus` and assign to the embedded envelope's `Failure`.

- [ ] **Step 1: Write the failing unit tests**

In `internal/app/planning_test.go`:

```go
func TestFailureStatus(t *testing.T) {
	failed := transaction.Result{Disposition: transaction.DispositionFailed}

	cases := []struct {
		name string
		res  transaction.Result
		err  error
		want *FailureStatus
	}{
		{"non-failed-disposition-is-nil",
			transaction.Result{Disposition: transaction.DispositionRefused},
			&transaction.Failure{Kind: transaction.KindInvalidState}, nil},
		{"typed-failure",
			failed,
			&transaction.Failure{Stage: transaction.StageVerifyDelta, Kind: transaction.KindInvalidState, Detail: "an undeclared path changed in the worktree"},
			&FailureStatus{Stage: "verify-delta", Kind: "invalid-state", Detail: "an undeclared path changed in the worktree"}},
		{"typed-failure-folds-wrapped-err",
			failed,
			&transaction.Failure{Stage: transaction.StageLoadAfter, Kind: transaction.KindInvalidState, Detail: "plan violates before/after tree rules", Err: errors.New("boom")},
			&FailureStatus{Stage: "load-after", Kind: "invalid-state", Detail: "plan violates before/after tree rules: boom"}},
		{"non-failure-error-is-internal-error",
			failed, errors.New("bare"),
			&FailureStatus{Kind: "internal-error", Detail: "bare"}},
		{"nil-error-is-contract-violation",
			failed, nil,
			&FailureStatus{Kind: "internal-error", Detail: "failed disposition carried no error (engine contract violation)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := failureStatus(tc.res, tc.err)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("failureStatus = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("failureStatus = nil, want a diagnosis")
			}
			if *got != *tc.want {
				t.Errorf("failureStatus = %+v, want %+v", *got, *tc.want)
			}
		})
	}
}
```

In `internal/app/result_test.go` (the marshalling contract — absent unless populated):

```go
func TestEnvelopeFailureMarshalsOnlyWhenPresent(t *testing.T) {
	env := NewEnvelope("change.claim", ResultApplied)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(`"failure"`)) {
		t.Errorf("failure must be absent on a non-failed envelope: %s", b)
	}

	env.Failure = &FailureStatus{Stage: "verify-delta", Kind: "invalid-state", Detail: "x"}
	b, err = json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"failure":{"stage":"verify-delta","kind":"invalid-state","detail":"x"}`)) {
		t.Errorf("failure field missing or misshapen: %s", b)
	}
}
```

Add the `bytes`, `encoding/json`, and `errors` imports to the test files as needed.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestFailureStatus|TestEnvelopeFailureMarshalsOnlyWhenPresent' -count=1`
Expected: FAIL — compile error, `FailureStatus`/`failureStatus` undefined.

- [ ] **Step 3: Implement the type, the envelope field, and the helper**

In `internal/app/result.go`, add beside `Envelope` (and extend the `Envelope` doc comment, which currently promises "the three fields every protocol-v1 result begins with", to name the optional failure diagnosis):

```go
// FailureStatus is the additive protocol-v1 diagnosis of a failed
// transaction: the engine stage that failed, the failure kind, and a bounded
// human-readable detail. It is populated only when the outcome's disposition
// was failed; on every other outcome the field is omitted entirely.
type FailureStatus struct {
	Stage  string `json:"stage,omitempty"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}
```

```go
type Envelope struct {
	ProtocolVersion int            `json:"protocol_version"`
	Operation       string         `json:"operation"`
	Result          Result         `json:"result"`
	Failure         *FailureStatus `json:"failure,omitempty"`
}
```

In `internal/app/planning.go`, beside `mapFailure`:

```go
// failureStatus converts a failed transaction's typed Failure into the
// envelope's failure diagnosis. It returns nil on every non-failed
// disposition. A failed disposition whose error is missing or not a
// *transaction.Failure is the same contract violation mapFailure reports as
// internal-error; it still yields a diagnosis, so a failed result can never
// again reach the caller cause-free.
func failureStatus(res transaction.Result, execErr error) *FailureStatus {
	if res.Disposition != transaction.DispositionFailed {
		return nil
	}
	if execErr == nil {
		return &FailureStatus{Kind: "internal-error", Detail: "failed disposition carried no error (engine contract violation)"}
	}
	f, ok := transaction.AsFailure(execErr)
	if !ok {
		return &FailureStatus{Kind: "internal-error", Detail: execErr.Error()}
	}
	detail := f.Detail
	if f.Err != nil {
		if detail == "" {
			detail = f.Err.Error()
		} else {
			detail = detail + ": " + f.Err.Error()
		}
	}
	return &FailureStatus{Stage: string(f.Stage), Kind: string(f.Kind), Detail: detail}
}
```

Note the `Kind: "internal-error"` literals are the protocol spelling from the spec's error-handling section, deliberately not `string(ResultInternalError)` — the failure kind vocabulary is the transaction taxonomy plus this one contract-violation spelling, not the result taxonomy.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestFailureStatus|TestEnvelopeFailureMarshalsOnlyWhenPresent' -count=1`
Expected: PASS. Then `go build ./...` and `go test ./internal/app/ -count=1` to confirm nothing else broke (the field is additive; existing goldens see no new bytes because `omitempty` hides a nil pointer).

- [ ] **Step 5: Commit**

```bash
git add internal/app/result.go internal/app/planning.go internal/app/result_test.go internal/app/planning_test.go
git commit -m "fix(0329): add the failure envelope field and the shared failureStatus helper"
```

---

### Task 3: Wire claim/refresh-claim and give `claimDisposition` a real failed arm

**Files:**
- Modify: `internal/app/change_claim.go` (`claimResultFromOutcome`, `claimDisposition`, the claim-disposition const block)
- Test: `internal/app/change_claim_test.go`

**Interfaces:**
- Consumes: `failureStatus(res, execErr) *FailureStatus` from Task 2; `Envelope.Failure`.
- Produces: `ClaimDispositionFailed = "failed"`; the wiring pattern every Task 4 builder repeats — construct via the `new*Result` helper, then assign `r.Failure = failureStatus(res, execErr)` (the constructors stamp a fresh `Envelope`, so the assignment must come after construction).

- [ ] **Step 1: Write the failing regression test**

In `internal/app/change_claim_test.go`:

```go
func TestClaimResultFromOutcomeFailedCarriesCause(t *testing.T) {
	execErr := &transaction.Failure{
		Stage:  transaction.StageVerifyDelta,
		Kind:   transaction.KindInvalidState,
		Detail: "an undeclared path changed in the worktree",
	}
	res := transaction.Result{Disposition: transaction.DispositionFailed}

	out := claimResultFromOutcome(OperationChangeRefreshClaim, res, execErr)

	if out.Result != ResultInvalidState {
		t.Fatalf("result = %q, want %q", out.Result, ResultInvalidState)
	}
	if out.Disposition != ClaimDispositionFailed {
		t.Errorf("disposition = %q, want %q", out.Disposition, ClaimDispositionFailed)
	}
	if out.Disposition == string(out.Result) {
		t.Errorf("disposition %q merely restates the result — the tautology is back", out.Disposition)
	}
	if out.Failure == nil {
		t.Fatal("failure diagnosis missing on a failed disposition — the Failure was dropped again")
	}
	if out.Failure.Detail == "" {
		t.Error("failure.detail is empty")
	}
	if out.Failure.Stage != string(transaction.StageVerifyDelta) || out.Failure.Kind != string(transaction.KindInvalidState) {
		t.Errorf("failure = %+v, want stage %q kind %q", out.Failure, transaction.StageVerifyDelta, transaction.KindInvalidState)
	}
	if len(out.Findings) != 0 {
		t.Errorf("findings = %v, want empty — findings are the refusal channel, not the failure channel", out.Findings)
	}
}
```

Also assert the untouched paths stay untouched — extend the test with a success-shape check:

```go
	ok := claimResultFromOutcome(OperationChangeClaim, transaction.Result{Disposition: transaction.DispositionApplied}, nil)
	if ok.Failure != nil {
		t.Errorf("failure must be nil on an applied outcome, got %+v", ok.Failure)
	}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/ -run TestClaimResultFromOutcomeFailedCarriesCause -count=1`
Expected: FAIL — `ClaimDispositionFailed` undefined; after adding only the const it fails on the nil `Failure` and the tautological disposition.

- [ ] **Step 3: Implement**

In `internal/app/change_claim.go`, extend the closed claim-disposition const block (and its doc comment, which enumerates the vocabulary) with:

```go
	// ClaimDispositionFailed is a transaction that failed mid-flight; the
	// cause is carried in the envelope's failure field.
	ClaimDispositionFailed = "failed"
```

Add the explicit arm to `claimDisposition`, above `default:`:

```go
	case transaction.DispositionFailed:
		return ClaimDispositionFailed
```

Wire the helper at the end of `claimResultFromOutcome` — replace `return newChangeClaimResult(opKey, result, out)` with:

```go
	r := newChangeClaimResult(opKey, result, out)
	r.Failure = failureStatus(res, execErr)
	return r
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/app/ -run TestClaimResultFromOutcomeFailedCarriesCause -count=1`
Expected: PASS. Then run the whole package: `go test ./internal/app/ -count=1` — the `default:` in `claimDisposition` still handles replayed/other cases, and existing claim tests must stay green.

- [ ] **Step 5: Mutation-test the guard**

Temporarily make `failureStatus` return `nil` unconditionally (edit its first line to `return nil` — do not delete the function).

Run: `go test ./internal/app/ -run TestClaimResultFromOutcomeFailedCarriesCause -count=1`
Expected: FAIL on "failure diagnosis missing". Then temporarily remove the `case transaction.DispositionFailed` arm from `claimDisposition` and re-run.
Expected: FAIL on the disposition asserts. Revert both transient edits by hand (never `git checkout -- <file>` — the real implementation is uncommitted) and re-run to green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/change_claim.go internal/app/change_claim_test.go
git commit -m "fix(0329): claim/refresh-claim surface the transaction Failure and a real failed disposition"
```

---

### Task 4: Grep-derive and wire every remaining builder

**Files:**
- Modify: every non-test file the Step-1 grep hits — observed at planning time: `internal/app/change_halt.go`, `change_kill.go`, `change_groom.go`, `change_reclaim.go`, `change_reconcile.go`, `change_lifecycle.go` (also serves `change_implemented.go`), `change_attach.go`, `change_create.go`, `finalize_block.go`, `finalize_cleanup.go`, `finalize_closeout.go`, `learning_ops.go`, `adr_ops.go`. **The grep is authoritative, not this list.**
- Test: `internal/app/change_lifecycle_test.go`, `internal/app/change_halt_test.go`, `internal/app/change_reclaim_test.go`

**Interfaces:**
- Consumes: `failureStatus(res, execErr)` (Task 2), the post-construction assignment pattern (Task 3).
- Produces: every transaction-backed result envelope carries `failure` on a failed disposition; `HaltDispFailed = "failed"`, `ReclaimDispFailed = "failed"`, and equivalents in each disposition mapper the sweep finds.

- [ ] **Step 1: Derive the affected set**

```bash
grep -rn "mapOutcome(" internal/app --include="*.go" | grep -v _test
```

Every hit is a builder that folds a transaction outcome into a result envelope. Sort each into:
- **redacting** — the built result's envelope never receives the Failure (all of them today except Task 3's claim path), and
- **mislabeling** — the builder also sets a `Disposition` and its failed path either echoes the result, falls into a refusal/skip token, or leaves the disposition empty. Observed at planning time: `haltResultFromOutcome` (failed → `HaltDispRefused`), `reclaimResultFromOutcome` (failed → `ReclaimDispSkipped` with an empty reason), `changeReconcileResultFromOutcome` (failed → empty disposition), `blockResultFromOutcome` and `clearBlockResultFromOutcome` (failed → `BlockDispRefused`), plus whatever the grep shows inside `finalize_cleanup.go` and `finalize_closeout.go` disposition handling.

Record the derived set in the build evidence.

- [ ] **Step 2: Write the failing representative tests**

One per operation shape the spec names (claim is covered by Task 3): the shared lifecycle family, plus the two mislabeling mappers with the richest existing vocabularies. In `internal/app/change_lifecycle_test.go`:

```go
func TestLifecycleResultFromOutcomeFailedCarriesCause(t *testing.T) {
	execErr := &transaction.Failure{
		Stage:  transaction.StageLoadAfter,
		Kind:   transaction.KindInvalidState,
		Detail: "plan violates before/after tree rules",
	}
	out := lifecycleResultFromOutcome(OperationChangeImplemented,
		transaction.Result{Disposition: transaction.DispositionFailed}, execErr)

	if out.Failure == nil {
		t.Fatal("failure diagnosis missing on a failed lifecycle transaction")
	}
	if out.Failure.Detail == "" {
		t.Error("failure.detail is empty")
	}
	if out.Failure.Stage != string(transaction.StageLoadAfter) {
		t.Errorf("failure.stage = %q, want %q", out.Failure.Stage, transaction.StageLoadAfter)
	}
}
```

(If the operation-key constant differs, use the one `change_implemented.go` passes to `lifecycleResultFromOutcome`.) In `internal/app/change_halt_test.go`:

```go
func TestHaltResultFromOutcomeFailedCarriesCause(t *testing.T) {
	execErr := &transaction.Failure{
		Stage:  transaction.StageVerifyDelta,
		Kind:   transaction.KindInvalidState,
		Detail: "an undeclared path changed in the worktree",
	}
	out := haltResultFromOutcome(OperationChangeHalt,
		transaction.Result{Disposition: transaction.DispositionFailed}, execErr,
		HaltDispHalted, ReasonHaltNotInProgress)

	if out.Disposition != HaltDispFailed {
		t.Errorf("disposition = %q, want %q — a failed transaction is not a refusal", out.Disposition, HaltDispFailed)
	}
	if out.Failure == nil || out.Failure.Detail == "" {
		t.Fatalf("failure diagnosis missing or empty: %+v", out.Failure)
	}
}
```

In `internal/app/change_reclaim_test.go`, the same shape against `reclaimResultFromOutcome(res, execErr)` asserting `out.Disposition == ReclaimDispFailed` and a non-nil, non-empty `out.Failure` (a failed reclaim is not a "skipped" with no reason).

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'FailedCarriesCause' -count=1`
Expected: FAIL — `HaltDispFailed`/`ReclaimDispFailed` undefined; lifecycle `Failure` nil.

- [ ] **Step 4: Wire every builder in the derived set**

The mechanical pattern for every site, exactly Task 3's: construct through the existing `new*Result` helper, then assign the failure before returning —

```go
	r := newXxxResult(/* existing args */)
	r.Failure = failureStatus(res, execErr)
	return r
```

For builders that return the constructor call directly (e.g. `changeKillResultFromOutcome`, `changeGroomResultFromOutcome`, `changeCreateResultFromOutcome`, `attachResultFromOutcome`, `lifecycleResultFromOutcome`, the `learning_ops.go`/`adr_ops.go`/`finalize_closeout.go`/`finalize_cleanup.go` sites), introduce the local `r` as above. For early-return branches that remap a refusal (halt/reclaim/reconcile contended short-circuits), no wiring is needed — those branches are only reached on `DispositionRefused`, and `failureStatus` would return nil anyway; wire only the main return.

Disposition arms — add a documented `= "failed"` const to each mislabeling mapper's closed vocabulary and an explicit arm. Concretely for the three observed mappers with result-keyed switches:

`change_halt.go` — const `HaltDispFailed = "failed"` (doc: "a transaction failure; the cause is in the envelope's failure field"), and rewrite the switch in `haltResultFromOutcome` to key the failed case on the disposition:

```go
	switch {
	case res.Disposition == transaction.DispositionFailed:
		out.Disposition = HaltDispFailed
	case result == ResultApplied:
		out.Disposition = appliedDisp
		out.Revision = string(res.AppliedCommit)
	case result == ResultNoOp:
		out.Disposition = appliedDisp
	case result == ResultContended:
		out.Disposition = HaltDispContended
	default:
		out.Disposition = HaltDispRefused
	}
```

`change_reclaim.go` — const `ReclaimDispFailed = "failed"`, same transformation of the switch in `reclaimResultFromOutcome` (the failed case sets no `Reason`; the old `default:` keeps setting `out.Reason = firstFindingCode(res.Findings)` for genuine refusals):

```go
	switch {
	case res.Disposition == transaction.DispositionFailed:
		out.Disposition = ReclaimDispFailed
	case result == ResultApplied:
		out.Disposition = ReclaimDispReclaimed
		out.Revision = string(res.AppliedCommit)
	case result == ResultNoOp:
		out.Disposition = ReclaimDispReclaimed
	case result == ResultContended:
		out.Disposition = ReclaimDispContended
	default:
		out.Disposition = ReclaimDispSkipped
		out.Reason = firstFindingCode(res.Findings)
	}
```

`change_reconcile.go` — const `ReconcileDispositionFailed = "failed"`, and in `changeReconcileResultFromOutcome` add to the existing result switch a disposition-keyed guard before it:

```go
	out := ChangeReconcileResult{Findings: findingsToStatus(res.Findings)}
	if res.Disposition == transaction.DispositionFailed {
		out.Disposition = ReconcileDispositionFailed
	}
```

(keep the existing `switch result` below it unchanged — its arms only fire on applied/contended).

`finalize_block.go` — const `BlockDispFailed = "failed"`, same disposition-keyed case ahead of the result switches in both `blockResultFromOutcome` and `clearBlockResultFromOutcome`. Apply the identical treatment to any further disposition mapper Step 1's sort marked mislabeling (check `finalize_cleanup.go` and `finalize_closeout.go` refusal/disposition handling on their `mapOutcome` paths). Watch for learnings `fix-reintroduces-its-own-defect-class`: after the sweep, re-grep for any `Disposition` assignment reachable on `DispositionFailed` that still emits a refusal-flavored or tautological token.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/app/ -count=1`
Expected: PASS across the package — the representative tests green, and no existing test broken (existing tests exercise refused/applied/contended paths whose behavior is unchanged).

- [ ] **Step 6: Mutation-test the sweep**

Repeat Task 3's probe at the sweep level: make `failureStatus` return `nil` unconditionally, run `go test ./internal/app/ -run 'FailedCarriesCause' -count=1`, and confirm **all** the new tests redden (claim, lifecycle, halt, reclaim). Delete the `HaltDispFailed` switch arm, confirm the halt test reddens. Revert the transient edits by hand and re-run to green.

- [ ] **Step 7: Update the spike's regression test (only if Task 1 reproduced)**

If Task 1 committed a reproducing test, extend it to assert the end-to-end payoff: the `ChangeClaimResult` returned by the real `ChangeRefreshClaim` now carries `Failure` with a non-empty `Detail` naming the reproduced stage, and `Disposition == ClaimDispositionFailed`.

- [ ] **Step 8: Commit**

```bash
git add internal/app/
git commit -m "fix(0329): every transaction-backed result surfaces the Failure; failed dispositions stop mislabeling"
```

---

### Task 5: Full-suite gate

**Files:**
- None created; run the suite the repo's own gate runs.

**Interfaces:**
- Consumes: everything above.
- Produces: the build-gate evidence.

- [ ] **Step 1: Run the whole suite**

Run: `scripts/run-tests.sh` (this is what `finalize.test_command` resolves to — never a hand-rolled subset; see `tests/README.md`).
Expected: PASS. The shell suite's protocol goldens are unaffected on success paths because `failure` is `omitempty`. If any test asserts an exact JSON key set for a failed outcome, update it to expect the new field — that is the change working, not collateral damage. Treat any trailing `OVER BUDGET:` line as a finding to act on.

- [ ] **Step 2: Record the gate evidence and finish**

Record the suite outcome (and the Task 1 spike verdict) in the build evidence. No commit unless Step 1 forced a test update; if it did:

```bash
git add tests/
git commit -m "test(0329): failed-outcome expectations learn the failure envelope field"
```

---

## Self-review notes

- **Spec coverage:** §1 field shape → Task 2; §2 shared helper + grep-derived scope → Tasks 2 and 4; §3 disposition arms → Tasks 3 and 4; task-shape item 1 (time-boxed repro) → Task 1; item 4 (regression + mutation tests, `-count=1`) → Tasks 3.5, 4.6; error handling (non-Failure error, nil error) → Task 2's table; out-of-scope items are excluded (no lease-semantics, TTL, reconcile-internal-refresh, or envelope-redesign edits anywhere).
- **Type consistency:** `FailureStatus{Stage, Kind, Detail string}` and `failureStatus(res transaction.Result, execErr error) *FailureStatus` are used with those exact shapes in Tasks 2–4; disposition consts are per-operation (`ClaimDispositionFailed`, `HaltDispFailed`, `ReclaimDispFailed`, `ReconcileDispositionFailed`, `BlockDispFailed`), all spelled `"failed"`.
- **Known non-goal:** `HumanText` renderers keep printing identity + disposition only; the JSON envelope is the diagnostic surface this change adds. Adding failure detail to human summaries is a separate UX decision.
