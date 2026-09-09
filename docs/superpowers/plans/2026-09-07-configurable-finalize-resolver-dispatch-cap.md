<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0349 — Make the finalize rebase-resolver dispatch cap configurable](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-09-0349-configurable-finalize-resolver-dispatch-cap.md)**
<!-- docket:backlink:end -->
# Configurable Finalize Resolver Dispatch Cap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the finalize rebase-resolver dispatch cap a configurable `finalize.resolver_max_attempts` (default 3) enforced durably in Go via the owned rebase receipt, with a new `finalize.resolver-reserve` typed operation that must authorize every native resolver dispatch.

**Architecture:** A new positive-int config leaf resolves through the normal layers and is snapshotted into an extended `RebaseReceipt` when a fresh owned rebase begins. A new `finalize.resolver-reserve` operation performs durable reserve-before-dispatch admission under the per-workspace operation lock; `FinalizeRebaseContinue` verifies the reservation before staging/continuing and consumes it only when the Git outcome is reconciled. The skill's in-memory "≤2 attempts" counter is replaced by reserve → dispatch once → verified continue.

**Tech Stack:** Go (internal/config, internal/workspace, internal/gitcli, internal/app, internal/cli), cobra CLI, docket capability catalog/schema surfaces, maintained skill markdown + generated wrappers, Go-native suite runner.

**Spec:** `docs/superpowers/specs/2026-09-07-configurable-finalize-resolver-dispatch-cap-design.md` (on the `docket` metadata branch; synchronized copy under `.docket/` in the primary tree). The plan argues from the spec; executors read both.

## Global Constraints

- Default is **3**; the value is a finite positive integer, minimum 1. No unlimited sentinel, no extra upper ceiling.
- Precedence: repository-local > repository-committed > global > built-in (ADR-0019 classification, like `finalize.gate`; `scopeAny`).
- The stored per-receipt limit governs an entire owned attempt; config changes apply only to the next fresh attempt.
- Measured unit: a **durably reserved dispatch opportunity**. No refunds — a failed launch or lost success response may consume an opportunity.
- Closed reservation dispositions: `reserved`, `pending`, `exhausted`, `blocked`, `contended` (protocol-v1 vocabulary). Only the reservation result uses `exhausted`; the rebase disposition vocabulary does not grow — post-continuation exhaustion is rebase `blocked` with reason `resolver-budget-exhausted`.
- Legacy receipts (whole budget group absent) are legacy, never corrupt and never freshly budgeted: refuse new reservation/continuation with reason `resolver-budget-unavailable`; owned abort stays available; a completed legacy rebase keeps its gate-resume/publish/cleanup path.
- Integration-repair's separate two-attempt limit and the resolver's editing-only charter are unchanged.
- `finalize.gate: off` skips the rebase: no reservation, no budget spent.
- Receipt writes keep the existing crash-safe discipline (temp + fsync + rename + dir fsync + round-trip guard); every field stays a scalar string so the struct stays comparable.
- All cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).
- Generated wrappers are regenerated from maintained source, never hand-edited. Frozen specs/ADRs/plans/results/fixtures stay historical.
- Suite gate: run resolved `build.test_command` (`go run ./cmd/docket development test`) at the end of the build — the docket-build final gate owns it; per-task steps run focused `go test` packages with `-count=1`.
- An additive ADR is recorded during implementation (Task 10), related to ADR-0010, ADR-0019, ADR-0105; accepted decisions are not rewritten.
- Learnings applied throughout: grep the suite for prose you delete before deleting it (restatement-accumulates-its-own-guards); every prohibition names its mapped return value (prohibition-needs-a-return-value); ship the config knob end-to-end with user-register docs (config-knob-ship-end-to-end); every transition out of "reservation outstanding" clears or reconciles the reservation (presence-encoded-state); mutation-test every new guard with `-count=1` (cached-runner-serves-a-mutated-tree).

---

### Task 1: Config leaf `finalize.resolver_max_attempts`

**Files:**
- Modify: `internal/config/schema.go` (leaf table, near the existing `finalize.*` entries)
- Modify: `internal/config/config.go` (the `Finalize` struct)
- Modify: whichever file maps leaves into the effective struct (follow how `finalize.require_pr_approval` flows from leaf to `Finalize.RequirePRApproval`; grep `require_pr_approval` across `internal/config/`)
- Modify: `.docket.example.yml` (commented sample entry) and the canonical config reference (grep `require_pr_approval` across `README.md` and `docs/` maintained pages to find every surface that lists finalize keys)
- Test: `internal/config/schema_test.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces: `Finalize.ResolverMaxAttempts Value[int]` with JSON tag `resolver_max_attempts`; leaf path `finalize.resolver_max_attempts`, kind `kindInt`, `def: 3`, `validate: intLeaf(1)`, `merge: mergeScalar`, `scope: scopeAny`, `disp: dispSupported`.

- [ ] **Step 1: Write the failing tests**

In `internal/config/schema_test.go`, extend the existing table-driven leaf tests (mirror the `{"int positive", "learnings.cap", ...}` rows):

```go
{"resolver max attempts default", "finalize.resolver_max_attempts", "", 3, ""},
{"resolver max attempts explicit", "finalize.resolver_max_attempts", "5", 5, ""},
{"resolver max attempts floor ok", "finalize.resolver_max_attempts", "1", 1, ""},
{"resolver max attempts zero", "finalize.resolver_max_attempts", "0", nil, "must be >= 1"},
{"resolver max attempts negative", "finalize.resolver_max_attempts", "-2", nil, "must be >= 1"},
{"resolver max attempts string", "finalize.resolver_max_attempts", `"three"`, nil, ""},
{"resolver max attempts bool", "finalize.resolver_max_attempts", "true", nil, ""},
{"resolver max attempts fraction", "finalize.resolver_max_attempts", "2.5", nil, ""},
{"resolver max attempts list", "finalize.resolver_max_attempts", "[3]", nil, ""},
```

Adapt row shape/assertion helper to the actual table in that file (read it first; keep house style, including the exact diagnostic phrasing `intLeaf` produces — copy from the `must be >= %d, got %d` format). Add a `Finalize.ResolverMaxAttempts` resolution assertion beside the existing `RequirePRApproval` coverage in `internal/config/config_test.go`, including four-layer precedence (repo-local > repo-committed > global > built-in) using that file's existing layered-fixture helper.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run 'TestSchema|TestResolve' -count=1`
Expected: FAIL (unknown leaf `finalize.resolver_max_attempts` / no field `ResolverMaxAttempts`)

- [ ] **Step 3: Implement leaf + struct field**

In `internal/config/schema.go`, beside the other finalize leaves:

```go
{path: "finalize.resolver_max_attempts", kind: kindInt, def: 3,
	merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: intLeaf(1)},
```

In `internal/config/config.go`:

```go
type Finalize struct {
	Gate                Value[string] `json:"gate"`         // local|off (ci/both classify deferred-active)
	TestCommand         Value[string] `json:"test_command"` // "" == unconfigured (legacy `auto` resolves away)
	RequirePRApproval   Value[bool]   `json:"require_pr_approval"`
	// ResolverMaxAttempts caps durable resolver dispatch reservations per owned
	// rebase attempt (change 0349). Positive; snapshotted into the rebase
	// receipt when a fresh attempt begins.
	ResolverMaxAttempts Value[int] `json:"resolver_max_attempts"`
}
```

Wire the leaf into the effective-struct mapping exactly where `finalize.require_pr_approval` is mapped. Check for correspondence guards: `internal/config/example_correspondence_test.go` and any schema/JSON reflection tests will demand the `.docket.example.yml` entry and reflected schema stay in step — satisfy them in this task, not later.

- [ ] **Step 4: Add the sample-config and reference-doc entries**

In `.docket.example.yml`, under the `finalize:` block, add a commented entry written for a user deciding whether to set it (learnings: config-knob-ship-end-to-end — lead with the payoff, no change numbers, no internal mechanism):

```yaml
  # How many times finalize may send in the conflict-resolver during one
  # rebase before stopping for you. Each successive conflicting commit costs
  # one attempt. Default 3; set 2 to restore the old, stricter ceiling.
  # resolver_max_attempts: 3
```

Update every maintained page found by `grep -rn "require_pr_approval" README.md docs/ --include='*.md'` that enumerates finalize keys, in the same register.

- [ ] **Step 5: Run the config package tests**

Run: `go test ./internal/config/ -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/ .docket.example.yml README.md docs/
git commit -m "feat(0349): add finalize.resolver_max_attempts config leaf (default 3, min 1)"
```

---

### Task 2: Export the resolved limit through `repository.prepare` context and config diagnostics

**Files:**
- Modify: the prepare-context finalize export (grep `require_pr_approval` in `internal/app/` — follow the file that builds the typed `context.finalize` object handed to skills, and the effective-config diagnostics in `internal/app/config_diagnostics.go`)
- Test: the corresponding `_test.go` files beside each

**Interfaces:**
- Consumes: `Finalize.ResolverMaxAttempts Value[int]` (Task 1).
- Produces: `resolver_max_attempts` integer in the prepare document's `context.finalize` JSON and in the effective-configuration diagnostics; both agree with each other (spec test 1).

- [ ] **Step 1: Write the failing tests**

Find the existing test asserting `context.finalize` carries `require_pr_approval` (grep in `internal/app/*_test.go`); add a sibling assertion that a config setting `finalize: {resolver_max_attempts: 4}` surfaces `"resolver_max_attempts": 4`, and that the default surfaces `3`. Assert the resolved non-default value, not just presence (learnings: defaulted-param-hides-caller-wiring). Do the same in `internal/app/config_diagnostics_test.go`, plus one test that reads both surfaces from the same fixture and asserts equality.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/app/ -run 'Prepare|ContextFinalize|ConfigDiagnostics' -count=1`
Expected: FAIL (field absent)

- [ ] **Step 3: Implement the exports**

Add the field to the finalize context payload struct and the diagnostics rendering, copying the exact pattern used by `require_pr_approval` (same provenance/source-line reporting the diagnostics give other leaves — the existing plumbing should carry it once the leaf exists; verify rather than assume, learnings: check-plumbing-auto-discovery; if diagnostics auto-reflect the schema, convert Step 1's diagnostics test into a pin of that behavior instead of adding code).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run 'Prepare|ContextFinalize|ConfigDiagnostics' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/
git commit -m "feat(0349): export resolver_max_attempts via context.finalize and config diagnostics"
```

---

### Task 3: `RebaseReceipt` resolver-budget group

**Files:**
- Modify: `internal/workspace/rebasereceipt.go`
- Test: `internal/workspace/rebasereceipt_test.go` (extend beside existing receipt tests; grep for the current receipt test file if named differently)

**Interfaces:**
- Produces (all scalar strings, keeping the struct comparable and the receipt byte-comparable):

```go
// Resolver-budget group (change 0349). All-empty == legacy receipt (pre-budget
// binary wrote it): recognizable, never corrupt, never freshly budgeted. When
// present: BudgetVersion is "1"; ResolverLimit/ResolverUsed are decimal ints
// with limit >= 1 and 0 <= used <= limit. ReservationToken/ReservationStopped
// record the outstanding reservation (both-empty or both-set); Reservation
// ContinuationStarted ("" or "1") may be set only while a reservation is
// outstanding.
ResolverBudgetVersion          string `json:"resolver_budget_version,omitempty"`
ResolverLimit                  string `json:"resolver_limit,omitempty"`
ResolverUsed                   string `json:"resolver_used,omitempty"`
ResolverReservationToken       string `json:"resolver_reservation_token,omitempty"`
ResolverReservationStopped     string `json:"resolver_reservation_stopped,omitempty"` // full object id of the stopped commit
ResolverContinuationStarted    string `json:"resolver_continuation_started,omitempty"` // "" | "1"
```

- Produces helpers on the package:

```go
// HasResolverBudget reports whether the receipt carries the budget group.
func (r RebaseReceipt) HasResolverBudget() bool
// ResolverBudget decodes limit/used; only call when HasResolverBudget.
func (r RebaseReceipt) ResolverBudget() (limit, used int, err error)
```

- [ ] **Step 1: Write the failing tests**

In the receipt test file:

```go
func TestRebaseReceiptResolverBudgetValidation(t *testing.T) {
	base := validReceipt() // reuse/extract the existing valid-receipt fixture helper
	cases := []struct {
		name   string
		mutate func(*workspace.RebaseReceipt)
		ok     bool
	}{
		{"legacy all-empty group", func(r *workspace.RebaseReceipt) {}, true},
		{"complete budget no reservation", withBudget("1", "3", "1", "", "", ""), true},
		{"outstanding reservation", withBudget("1", "3", "1", "tok-1", fullOID, ""), true},
		{"continuation started", withBudget("1", "3", "1", "tok-1", fullOID, "1"), true},
		{"used equals limit", withBudget("1", "3", "3", "", "", ""), true},
		{"unsupported version", withBudget("2", "3", "1", "", "", ""), false},
		{"limit zero", withBudget("1", "0", "0", "", "", ""), false},
		{"used exceeds limit", withBudget("1", "2", "3", "", "", ""), false},
		{"used negative", withBudget("1", "3", "-1", "", "", ""), false},
		{"non-decimal limit", withBudget("1", "three", "0", "", "", ""), false},
		{"partial group limit only", func(r *workspace.RebaseReceipt) { r.ResolverLimit = "3" }, false},
		{"token without stopped commit", withBudget("1", "3", "1", "tok-1", "", ""), false},
		{"stopped commit without token", withBudget("1", "3", "1", "", fullOID, ""), false},
		{"short stopped commit", withBudget("1", "3", "1", "tok-1", "abc123", ""), false},
		{"continuation without reservation", withBudget("1", "3", "1", "", "", "1"), false},
		{"continuation marker not 0/1 shape", withBudget("1", "3", "1", "tok-1", fullOID, "yes"), false},
	}
	// for each: mutate a copy, WriteRebaseReceipt must refuse invalid and
	// round-trip valid; ReadRebaseReceipt of a hand-written invalid file must
	// return an error, never clean absence.
}
```

Write `withBudget` as a small local helper setting the six fields. Also test `HasResolverBudget` (false on legacy, true on budgeted) and `ResolverBudget()` decoding. Exercise both the write gate and the read gate — the validator is shared, but prove it from both directions per the file's own contract.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/workspace/ -run TestRebaseReceiptResolverBudget -count=1`
Expected: FAIL (fields don't exist)

- [ ] **Step 3: Implement fields, validation, helpers**

Add the six fields to `RebaseReceipt` and extend `validateRebaseReceipt` with the whole-group rule: all six empty (legacy) OR (version == "1", limit/used decimal with `limit >= 1 && 0 <= used <= limit`, token/stopped both-empty or both-set with stopped a valid full object id via `validObjectID`, continuation `""` or `"1"` and `"1"` only when token set). Implement `HasResolverBudget` (version non-empty) and `ResolverBudget` with `strconv.Atoi`. Keep the doc comment on the struct explaining the group and the legacy rule.

- [ ] **Step 4: Run the workspace package tests**

Run: `go test ./internal/workspace/ -count=1`
Expected: PASS (existing receipt tests keep passing — legacy receipts stay valid)

- [ ] **Step 5: Mutation-check the validator**

Temporarily invert one clause (e.g. allow `used > limit`), run Step 4's command with `-count=1`, confirm the matching case reddens, restore. Do NOT restore via `git checkout --` (learnings: mutation-restore-needs-a-backup-copy) — re-edit by hand or copy the file aside first.

- [ ] **Step 6: Commit**

```bash
git add internal/workspace/
git commit -m "feat(0349): add versioned resolver-budget group to RebaseReceipt"
```

---

### Task 4: `internal/gitcli` stopped-commit probe

**Files:**
- Modify: `internal/gitcli/rebase.go`
- Test: `internal/gitcli/rebase_integration_test.go`

**Interfaces:**
- Produces: `func (c *Client) StoppedRebaseCommit(ctx context.Context, worktreeDir string) (ObjectID, error)` — the full object id of the commit the in-progress rebase is stopped on (`git rev-parse REBASE_HEAD` in `worktreeDir`, via the package's existing exec seam), an explicit error when no rebase is in progress or `REBASE_HEAD` cannot be resolved. An error is never a clean "not stopped" (learnings: probe-error-is-not-clean-absence) — callers that need "no rebase" ask `RebaseState`.

- [ ] **Step 1: Write the failing integration test**

In `internal/gitcli/rebase_integration_test.go` (note its `//go:build integration` tag — match the file's existing fixture helpers that build a repo with a conflicting rebase):

```go
func TestIntegrationStoppedRebaseCommit(t *testing.T) {
	// Arrange: reuse the existing conflicted-rebase fixture from this file's
	// BeginRebase conflict test. After BeginRebase reports conflicted:
	oid, err := client.StoppedRebaseCommit(ctx, worktree)
	// Assert: err == nil, oid is 40/64 lowercase hex, and equals the fixture's
	// known conflicting commit (rev-parse it in the fixture for comparison).
	// Then abort the rebase and assert StoppedRebaseCommit returns an error.
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test -tags integration ./internal/gitcli/ -run TestIntegrationStoppedRebaseCommit -count=1`
Expected: FAIL (method undefined)

- [ ] **Step 3: Implement the probe**

Follow the exec/classification style of `RebaseState` in the same file: run `rev-parse REBASE_HEAD`, trim, validate the object-id shape (reuse the package's id validation used by `classifyRebaseResult`), return a typed `Failure` consistent with the package's error style otherwise.

- [ ] **Step 4: Run tests**

Run: `go test -tags integration ./internal/gitcli/ -run 'Rebase' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gitcli/
git commit -m "feat(0349): add StoppedRebaseCommit probe to gitcli"
```

---

### Task 5: Snapshot the budget at rebase begin; surface budget state on conflicted results

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`FinalizeRebase`, `newRebaseAttempt`'s call site, `mapBegunRebase`, `mapContinuedRebase`, `recoverFromReceipt`, `clearGateContinuation`, `FinalizeRebaseResult`)
- Test: `internal/app/finalize_rebase_test.go`, `internal/app/finalize_rebase_integration_test.go`

**Interfaces:**
- Consumes: `Finalize.ResolverMaxAttempts` (Task 1), receipt budget group + helpers (Task 3), `StoppedRebaseCommit` (Task 4).
- Produces: `FinalizeRebaseResult` gains three JSON fields, populated on every result derived from a budgeted receipt with a live conflict or exhaustion:

```go
ResolverLimit     int    `json:"resolver_limit,omitempty"`
ResolverUsed      int    `json:"resolver_used,omitempty"`
ResolverRemaining int    `json:"resolver_remaining,omitempty"` // limit - used
```

- Produces the receipt-write rule every later task relies on: **every** receipt rewrite in `finalize_rebase.go` (gate-continuation set/clear included) starts from a freshly reloaded receipt and copies the six resolver fields forward unchanged unless the operation's contract is precisely to change them.

- [ ] **Step 1: Write the failing tests**

Unit (`finalize_rebase_test.go`, using the file's existing fake deps):
- A fresh `FinalizeRebase` with config `resolver_max_attempts: 2` writes a receipt with `ResolverBudgetVersion == "1"`, `ResolverLimit == "2"`, `ResolverUsed == "0"`, empty reservation fields. Assert the resolved non-default (2, not 3).
- A `conflicted` result carries `resolver_limit: 2, resolver_used: 0, resolver_remaining: 2` in the JSON document and mentions the counts in `HumanText()`.
- `recoverFromReceipt` on an existing budgeted receipt does NOT re-snapshot: with config now 5 and receipt limit 2, the result reports limit 2 (mid-attempt config changes don't apply — spec test 3).
- The gate-continuation write path (`clearGateContinuation`, and wherever `GateDriveID`/`GateOwnerGeneration` are set) preserves the budget fields: seed a receipt with used=1 and an outstanding reservation, drive a gate-continuation set+clear, assert the six fields survive byte-identically.
- `finalize.gate: off` path: no receipt (hence no budget) is created where the current code already skips the rebase — pin the existing behavior with an assert that no receipt file exists.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/app/ -run TestFinalizeRebase -count=1`
Expected: FAIL

- [ ] **Step 3: Implement**

In `FinalizeRebase` where the fresh receipt is built (near `newRebaseAttempt`), read `deps`' resolved config (`Finalize.ResolverMaxAttempts.Value` — follow how `RequirePRApproval` is read in this package) and stamp:

```go
rec.ResolverBudgetVersion = "1"
rec.ResolverLimit = strconv.Itoa(limit)
rec.ResolverUsed = "0"
```

Populate the three result fields in `mapBegunRebase`/`mapContinuedRebase`/`recoverFromReceipt` whenever the receipt has a budget and the disposition is `conflicted` (and on the exhaustion paths added in Task 7). Add the counts to `HumanText()`. Audit every `WriteRebaseReceipt` call site in the file and make each start from the current on-disk receipt (`requireOwnedAttempt`'s reloaded value) so the budget group is never overwritten by a stale in-memory copy.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestFinalizeRebase -count=1` then `go test ./internal/app/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/
git commit -m "feat(0349): snapshot resolver budget into fresh rebase receipts and surface counts"
```

---

### Task 6: `finalize.resolver-reserve` app operation

**Files:**
- Create: `internal/app/finalize_reserve.go`
- Create: `internal/app/finalize_reserve_test.go`
- Modify: `internal/app/finalize_rebase.go` only if a helper (e.g. `loadRebaseContext`, `requireOwnedAttempt`) needs exporting within the package — reuse, don't duplicate

**Interfaces:**
- Consumes: `loadRebaseContext`, `requireOwnedAttempt`-style receipt loading, `StoppedRebaseCommit` (Task 4), receipt helpers (Task 3), workspace operation lock (`acquireOperationLock` via the workspace seam — expose it through `FinalizeWorkspace` the way other receipt operations reach the workspace; follow how `FinalizeRebaseContinue` serializes today and extend the seam if it doesn't).
- Produces:

```go
// OperationFinalizeResolverReserve is the operation key.
const OperationFinalizeResolverReserve = "finalize.resolver-reserve"

// The closed reservation dispositions and reasons.
const (
	ReserveReserved  = "reserved"
	ReservePending   = "pending"
	ReserveExhausted = "exhausted" // reason: resolver-budget-exhausted
	// blocked reasons include: resolver-budget-unavailable (legacy receipt),
	// plus the existing rebase refusal reasons for foreign/malformed/moved/
	// non-conflicted state.
)

type FinalizeReserveResult struct {
	// mirrors FinalizeRebaseResult's document conventions: Operation,
	// Disposition, Reason, Message, ChangeID, Attempt, plus:
	Reservation   string `json:"reservation,omitempty"`    // opaque token
	StoppedCommit string `json:"stopped_commit,omitempty"` // full object id
	ResolverLimit int    `json:"resolver_limit,omitempty"`
	ResolverUsed  int    `json:"resolver_used,omitempty"`
	ResolverRemaining int `json:"resolver_remaining,omitempty"`
}

func FinalizeResolverReserve(ctx context.Context, deps FinalizeDeps, repoDir string, id int, attempt string) FinalizeReserveResult
```

(Read `internal/app/finalize_rebase.go`'s result plumbing — `newRebaseResult`, `OperationResult` — and match it exactly, including `HumanText()`.)

- [ ] **Step 1: Write the failing tests**

In `finalize_reserve_test.go`, with the package's fake deps and a temp workspace receipt:

```go
// reserved: budgeted receipt used=0 limit=2, live conflicted rebase whose
// StoppedRebaseCommit returns oid X → disposition reserved; reloaded receipt
// has used="1", token non-empty, stopped=X; result carries token, X, 2/1/1.
// pending: receipt already carries an outstanding token → disposition pending,
// same token echoed, used unchanged (no double admission).
// exhausted: used == limit, no outstanding token → disposition exhausted,
// reason resolver-budget-exhausted, counts present, receipt byte-unchanged,
// no Git state touched.
// legacy: receipt without budget group → blocked, reason
// resolver-budget-unavailable, receipt unchanged.
// foreign attempt token → blocked (reuse requireOwnedAttempt's refusal).
// non-conflicted rebase state (RebaseState clean) → blocked.
// stopped-commit probe error → blocked, no increment.
// write-failure injection: make WriteRebaseReceipt fail (unwritable dir or a
// seam fake) → no `reserved` disposition is returned and the on-disk used
// count is unchanged (no permission before durability — spec test 5).
// concurrency: two goroutines calling FinalizeResolverReserve on the same
// workspace; assert exactly one gets reserved and the other pending or
// blocked-contended, and final used == 1 (spec test 4).
```

Mint the reservation token like `newRebaseAttempt` does (clock + stopped-commit prefix) so tests can use the fake clock.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/app/ -run TestFinalizeResolverReserve -count=1`
Expected: FAIL (function undefined)

- [ ] **Step 3: Implement**

Under the per-workspace operation lock: reload receipt → refuse foreign/malformed/legacy/moved/non-conflicted (each with its named reason; each prohibition maps to a concrete disposition — learnings: prohibition-needs-a-return-value) → if token outstanding return `pending` → if `used == limit` return `exhausted` → else check capacity BEFORE incrementing, write `used+1` + token + stopped commit via `WriteRebaseReceipt` (round-trip guard is the verification), and only then return `reserved`. Never launch anything, stage anything, or touch Git/metadata. Release the lock before returning.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -run TestFinalizeResolverReserve -count=1` then `go test ./internal/app/ -count=1`
Expected: PASS

- [ ] **Step 5: Mutation-test the admission guards**

One at a time (restore by hand between each, `-count=1` every run): (a) skip the `used == limit` check → exhausted test reddens; (b) return `reserved` before `WriteRebaseReceipt` → write-failure test reddens; (c) skip the outstanding-token branch → pending/no-double-admission test reddens. All three must redden or the guard is decoration.

- [ ] **Step 6: Commit**

```bash
git add internal/app/
git commit -m "feat(0349): add finalize.resolver-reserve durable admission operation"
```

---

### Task 7: Reservation-verified continue, consumption, and exhaustion routing

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`ResolverReport`, `FinalizeRebaseContinue`, `mapContinuedRebase`, `FinalizeRebaseAbort`)
- Test: `internal/app/finalize_rebase_test.go`, `internal/app/finalize_rebase_integration_test.go`

**Interfaces:**
- Consumes: receipt budget group (Task 3), `StoppedRebaseCommit` (Task 4), result fields (Task 5), reservation semantics (Task 6).
- Produces: `ResolverReport` gains `ResolverReservation string \`json:"resolver_reservation"\`` (the token from the `reserved` result; the dispatch payload instructs the resolver to echo it). `FinalizeRebaseContinue` verifies, on a budgeted receipt: report change id, attempt, `ResolverReservation == rec.ResolverReservationToken` (non-empty), live `StoppedRebaseCommit` equals `rec.ResolverReservationStopped`, and reported paths against live unmerged paths (existing check). Any mismatch refuses BEFORE staging (`blocked`, reasons `reservation-missing` / `reservation-stale`), spends nothing, refunds nothing.

- [ ] **Step 1: Write the failing tests**

Unit tests on `FinalizeRebaseContinue` with a budgeted receipt carrying token `tok-1` bound to stopped commit X:
- Report without `resolver_reservation` → refusal `reservation-missing`, no staging call recorded by the fake, receipt unchanged.
- Report with a foreign token → `reservation-stale`.
- Correct token but live stopped commit != X (simulate the next conflicting commit having identical conflicted paths — spec's stale-report case) → `reservation-stale`; identical paths must NOT rescue it.
- Correct token + commit: before `StageAndContinueRebase` is invoked, receipt has `ResolverContinuationStarted == "1"` (assert via a seam fake that reads the receipt when staging is called).
- Continue completes the rebase → reservation fields cleared, `used` preserved, gate composed as today.
- Continue surfaces the NEXT conflict with `used < limit` → `conflicted` result carrying counts (Task 5 fields), reservation cleared (the outcome is reconciled; the next dispatch requires a fresh reserve).
- Continue surfaces the next conflict with `used == limit` → disposition `blocked`, reason `resolver-budget-exhausted`, counts present (last-permitted continuation is valid; only the NEXT reservation is prohibited — spec test 6).
- Legacy receipt (no budget group) + any report → refusal `resolver-budget-unavailable`; but `FinalizeRebaseAbort` on the same legacy receipt still succeeds.
- Repeated continue with the same already-consumed reservation → refusal, no second `StageAndContinueRebase` (spec test 4: repeated continue cannot advance a later commit).
- Continuation-started-but-ambiguous recovery: receipt has `ResolverContinuationStarted == "1"` and live stopped commit still equals X → refusal that retains state and directs to human/abort, no second continue; live rebase provably completed (RebaseState clean, head advanced) → recoverable without another charge.
- Exhaustion messages name `finalize.resolver_max_attempts`, the used count, and the limit, and state that a change takes effect on the next explicit finalize attempt (assert on `HumanText()` with a bounded phrase match).

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/app/ -run 'TestFinalizeRebaseContinue|TestFinalizeRebaseAbort' -count=1`
Expected: FAIL

- [ ] **Step 3: Implement**

In `FinalizeRebaseContinue`, after `requireOwnedAttempt`: branch on `rec.HasResolverBudget()`. Budgeted path: run the four identity verifications, durably set `ResolverContinuationStarted = "1"` (reload-modify-write under the operation lock), then stage/continue; in `mapContinuedRebase`, on a reconciled outcome clear the three reservation fields (keep `used`), and route the exhausted-next-conflict case to `blocked`/`resolver-budget-exhausted`. Legacy path: refuse with `resolver-budget-unavailable`. `FinalizeRebaseAbort` remains reservation-agnostic (abort is always available) but must preserve/clear the receipt exactly as it does today. Hold the lock only around receipt read/verify/write and the Git mutation, per the spec's serialization clause.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/ -count=1`
Expected: PASS

- [ ] **Step 5: Mutation-test the verification guards**

One at a time with hand-restore and `-count=1`: (a) drop the reservation-token comparison → foreign-token test reddens; (b) drop the stopped-commit comparison → identical-paths stale test reddens; (c) skip the continuation-started write → the staging-seam test reddens. (Spec test 8's "ignore reservation validation" mutation.)

- [ ] **Step 6: Commit**

```bash
git add internal/app/
git commit -m "feat(0349): verify resolver reservations in rebase-continue and route exhaustion"
```

---

### Task 8: CLI subcommand, capability, schema, install routing, and end-to-end Git fixture

**Files:**
- Modify: `internal/cli/finalize.go` (new `newFinalizeResolverReserveSubcommand`, registered beside `rebase-continue`)
- Modify: `internal/cli/install.go` (routing map entry `"finalize resolver-reserve": true`)
- Modify: whatever schema/capability reflection surfaces enumerate operations (run the existing capability/schema tests after wiring; they name the files that must change — e.g. `internal/app/capabilities.go`, schema goldens; follow the diffs the failing tests demand, per ADR-0109 the schema payload is its own reflected surface)
- Test: `internal/cli/finalize_test.go`, `internal/app/finalize_rebase_integration_test.go`

**Interfaces:**
- Consumes: `FinalizeResolverReserve` (Task 6).
- Produces: `docket finalize resolver-reserve --id <id> --attempt <attempt> [--repo-dir <dir>]`, annotation `capability("finalize.resolver-reserve", EffectLocalWrite)` (it writes only the local receipt; no process-control — it launches nothing).

- [ ] **Step 1: Write the failing CLI tests**

Extend `TestFinalizeRebaseResolverSubcommandsRegistered`'s loop to include `"resolver-reserve"`, the capability-annotation table to include `"finalize resolver-reserve"`, and add a JSON-document test mirroring the existing `"operation":"finalize.rebase-continue"` assertion:

```go
// invoke: "finalize", "resolver-reserve", "--id", "7", "--attempt", "tok",
// "--repo-dir", dir  → document contains `"operation":"finalize.resolver-reserve"`.
```

Also assert `--id` and `--attempt` are required flags (copy the MarkFlagRequired assertions pattern used for `rebase`).

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/cli/ -run TestFinalize -count=1`
Expected: FAIL

- [ ] **Step 3: Implement the subcommand + routing**

Copy `newFinalizeRebaseContinueSubcommand`'s shape minus the `--input` report (identity rides on flags; there is no authored input):

```go
func newFinalizeResolverReserveSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolver-reserve",
		Short: "Durably reserve one conflict-resolver dispatch for an owned rebase",
		Args:  cobra.NoArgs,
		// local-write: increments the receipt's used count and records the
		// reservation; launches nothing, stages nothing, runs no suite.
		Annotations: capability("finalize.resolver-reserve", EffectLocalWrite),
		RunE: func(c *cobra.Command, _ []string) error { /* resolveRepoDir, GetInt id, GetString attempt, newFinalizeDeps, setResult(app.FinalizeResolverReserve(...)) */ },
	}
	// int flag --id, string flag --attempt, both required
	return cmd
}
```

Register it in the finalize command tree and add the `internal/cli/install.go` routing entry. Then run the whole capability/schema test surface and satisfy every guard it raises (signature registry, schema golden, human rendering) in this same task — those guards exist precisely so the four surfaces ship together.

- [ ] **Step 4: Write the end-to-end integration test (real Git fixture)**

In `internal/app/finalize_rebase_integration_test.go` (build tag `integration`), reusing that file's conflicted-rebase fixture builders, cover spec test 2 end-to-end at the app layer:

```go
func TestIntegrationResolverBudgetSuccessiveConflicts(t *testing.T) {
	// Fixture: base branch + feature branch with three successive commits each
	// conflicting on the same file (distinct content per commit).
	// Default limit 3: reserve → resolve (write merged content, report with
	// reservation) → continue, three times; rebase completes; gate composes.
	// Variant limit 2 (config finalize.resolver_max_attempts: 2): second
	// continue's next conflict returns blocked/resolver-budget-exhausted;
	// fresh reserve returns exhausted; abort restores OrigHead.
	// Variant limit 4 with a fourth conflicting commit: completes.
	// One resolver report listing multiple conflicted files consumes exactly
	// one reservation (fixture commit touching two files).
	// Process-restart simulation: rebuild deps (fresh Service) between reserve
	// and continue; limit/used/reservation survive via the receipt.
}
```

- [ ] **Step 5: Run the packages**

Run: `go test ./internal/cli/ -count=1 && go test -tags integration ./internal/app/ -run TestIntegrationResolverBudget -count=1 && go test ./internal/app/ -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/cli/ internal/app/
git commit -m "feat(0349): register finalize resolver-reserve CLI/capability/schema and e2e budget fixture"
```

---

### Task 9: Skill prose — reserve-before-dispatch replaces the skill-enforced counter

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (the *Resolver loop* block)
- Modify: `skills/docket-finalize-change/references/gate-failure.md` (mirror clauses)
- Modify: the resolver agent contract (grep `-rn "ResolverReport\|conflicted_paths" skills/ agents/ --include='*.md'` for the maintained file that specifies the report fields; add `resolver_reservation`)
- Modify: `skills/docket-convention/SKILL.md` only if it restates the two-attempt cap (grep first)
- Test: existing prose-sentinel tests (see Step 1)

**Interfaces:**
- Consumes: the operation shapes from Tasks 6–8 (`finalize.resolver-reserve` with `--id --attempt`; `resolver_reservation` report field; dispositions `reserved`/`pending`/`exhausted`; rebase reason `resolver-budget-exhausted`).

- [ ] **Step 1: Grep the suite for the prose being deleted — BEFORE editing**

Run: `grep -rn "at most two\|≤2 attempts\|skill-enforced" tests/ internal/ --include='*.sh' --include='*.go'` and `grep -rn "resolver" tests/ -l`. Every hit on the clauses being removed is a dependent to repoint at the new canonical text, never to placate by re-adding the old text (learnings: restatement-accumulates-its-own-guards). List the hits in the task's commit message body.

- [ ] **Step 2: Rewrite the resolver loop in `SKILL.md`**

Replace the block headed `**Resolver loop (skill-enforced ≤2 attempts).**` with a reserve-gated loop. Required content (adapt wording to the file's voice; keep every retained mechanic — report validation, `report-not-resolved`, abort flow — verbatim where unchanged):

```markdown
**Resolver loop (Go-enforced budget).** On `conflicted`, first run the
`finalize.resolver-reserve` operation with `--id <id> --attempt <attempt>`.
Route on its disposition:
- `reserved` — authorizes exactly ONE `docket-rebase-resolver` dispatch.
  Include the returned `reservation` token in the payload; the resolver echoes
  it as the report's `resolver_reservation` field.
- `pending` — a reservation is already outstanding: dispatch NOTHING new. If
  the child it belongs to is yours and identifiable, wait for it or feed its
  matching report to `finalize.rebase-continue`; if you cannot establish
  dispatch ownership, this is `halted` with the reservation retained.
- `exhausted` (`resolver-budget-exhausted`) — the configured
  `finalize.resolver_max_attempts` budget (default 3) is spent: `halted` via
  the abort flow below. Raising the limit takes effect on the next explicit
  finalize attempt, never this receipt.
- `blocked` / `contended` — as elsewhere: `blocked` is `halted`
  (`resolver-budget-unavailable` marks a pre-budget receipt: abort remains
  available, resolver dispatch does not); `contended` re-reads context.
Feed the report back with `finalize.rebase-continue` (unchanged flags). A
continue returning `blocked` with `resolver-budget-exhausted` means the last
permitted continuation surfaced another conflict: enter the abort flow. Never
count dispatches yourself, and never abort-and-restart to replenish a budget.
```

Keep the existing stuck/unavailable/abort sentences, adding the quiescence clause: before `finalize.rebase-abort` on a workspace that might still have a live resolver child, establish the child's completion through the harness; inability to establish it is `halted` with the workspace retained (the prohibition names its outcome — learnings: prohibition-needs-a-return-value).

- [ ] **Step 3: Mirror in `references/gate-failure.md` and the resolver contract**

Update gate-failure.md's `The resolver gets **at most two dispatches, enforced by the skill**.` sentence and the numbered flow to the reserve protocol (same vocabulary as Step 2, condensed). In the resolver agent contract, add `resolver_reservation` (string, echoed verbatim from the dispatch payload) to the report field list, leaving the editing-only charter and every other field untouched. Do not touch the integration-repair two-attempt clauses anywhere.

- [ ] **Step 4: Repoint the dependents found in Step 1**

Fix each sentinel/test hit: repoint asserts at the new canonical clauses (e.g. assert `resolver-reserve` and `resolver-budget-exhausted` appear in the resolver-loop block; assert the OLD clause is ABSENT — the absence assert cannot go stale). Where a test greps repair's "two attempts", leave it alone (different budget).

- [ ] **Step 5: Regenerate wrappers and run the prose/sentinel tests**

Regenerate generated skill/agent wrappers via the repo's tracked generation path (grep `scripts/` and `internal/install` for the generator the suite's drift guards check against; run it — never hand-edit generated copies). Then run the focused shell/prose tests found in Step 1 through the Go runner, e.g.: `go run ./cmd/docket development test -- tests/test_finalize_skill.sh` (adapt to the runner's actual file-selection syntax in `tests/README.md`).
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add skills/ agents/ tests/
git commit -m "docs(0349): replace skill-counted resolver cap with reserve-before-dispatch protocol"
```

---

### Task 10: Additive ADR

**Files:**
- Create: `docs/adrs/<next-number>-<slug>.md` via the docket-adr flow (dispatch the registered `docket-adr` agent per repo rules; it owns numbering and the index)

**Interfaces:**
- Consumes: the shipped design (Tasks 3–9).

- [ ] **Step 1: Record the ADR**

Dispatch `docket-adr` to record an additive decision titled approximately "Resolver dispatches are admitted by durable pre-dispatch reservation" covering: (a) reserve-before-dispatch in Go because a continue-time check cannot bound dispatches; (b) conservative consumption — a reserved opportunity is never refunded, so lost responses cannot overrun the configured bound; (c) legacy receipts refuse budgeted operations (`resolver-budget-unavailable`) rather than being granted a fresh budget; (d) the limit is snapshotted per receipt. Relate it to ADR-0010, ADR-0019, ADR-0105; do not rewrite them. List the new ADR id in change 0349's `adrs:` frontmatter through the tracked metadata flow the build role owns (learnings: adr-update-delivery — the ADR ships with this change, never standalone).

- [ ] **Step 2: Verify and commit**

Confirm the ADR file and regenerated index are committed on the branch the docket-adr flow targets; confirm the feature branch holds no hand-edited generated ADR artifacts.

---

### Task 11: Repo-wide literal sweep, guard mutation matrix, and final gate

**Files:**
- Modify: whatever the sweep surfaces (expected: none beyond Tasks 1–10)
- Test: full suite

**Interfaces:** none new — this task proves the change is whole.

- [ ] **Step 1: Derive the site list by shape, not memory**

Run and classify every hit into prose vs executable vs historical (frozen specs/plans/archived changes/fixtures stay untouched):

```bash
grep -rn "at most two\|two attempts\|≤2\|resolver_max_attempts\|resolver-reserve\|resolver-budget" \
  --include='*.go' --include='*.md' --include='*.sh' --include='*.yml' \
  . --exclude-dir=.git --exclude-dir=.docket --exclude-dir=.worktrees
```

Executable or maintained-source hits still carrying the old cap (outside repair's own budget) are defects — fix them. Record the classification in the results file at build close.

- [ ] **Step 2: Run the spec's remaining mutation cells**

Confirm the three Task 6 and three Task 7 mutations were actually run (their step boxes ticked). Add the one not yet covered: bypass capacity rejection at the CLI layer is not a separate surface (the CLI delegates), so document in the results file that the app-layer mutation covers it.

- [ ] **Step 3: Run the whole suite**

Run the resolved `build.test_command` — in this repo `go run ./cmd/docket development test` — from the feature worktree. The docket-build final gate owns this run; treat `SERIAL CONFIRMED OVER BUDGET:` lines as authoritative breaches to act on.
Expected: SUITE green.

- [ ] **Step 4: Commit any sweep fixes**

```bash
git add -u
git commit -m "chore(0349): repo-wide resolver-cap literal sweep and guard verification"
```

(Skip the commit if the sweep changed nothing.)

---

## Self-Review

- **Spec coverage:** config contract → Tasks 1–2; durable admission → Task 6; receipt/continuation contract → Tasks 3, 5, 7; interruption/replay/compatibility → Tasks 6–8 (restart simulation, pending semantics, legacy refusal, ambiguous-continuation retention); skill behavior/failure routing → Task 9; delivery surfaces (sample config, references, gitcli probe, CLI/schema/capability/install, generated assets) → Tasks 1, 4, 8, 9; ADR → Task 10; required tests 1–8 → mapped in Tasks 1, 2, 5, 6, 7, 8, 9, 11; whole-suite gate → Task 11.
- **Known deliberate simplifications:** result-field names (`resolver_limit` etc.) and refusal reason spellings (`reservation-missing`/`reservation-stale`) are this plan's canonical choices — implementers keep them consistent across Tasks 5–9 rather than inventing variants. The reservation token rides in the existing receipt (spec's "no separate ledger").
- **Type consistency check:** `ResolverMaxAttempts Value[int]` (Tasks 1–2, 5); receipt fields as scalar strings with `HasResolverBudget()/ResolverBudget()` (Tasks 3, 5–7); `StoppedRebaseCommit(ctx, worktreeDir) (ObjectID, error)` (Tasks 4, 6, 7); `FinalizeResolverReserve(ctx, deps, repoDir, id, attempt)` (Tasks 6, 8); `ResolverReport.ResolverReservation` (Tasks 7–9). Consistent.
