<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0407 — Keyed gate-verdict misattributes its verdict to a concurrent loop's change id under parallel implement-next runs](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-07-0407-keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent.md)**
<!-- docket:backlink:end -->
# Bind Keyed Gate Dispatches to Their Successful Claims — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a fresh implement-next dispatch's ownership explicit at claim time — `change.claim` carries the dispatch context and records durable proof, and keyed `run.gate-verdict` resolves that proof instead of inferring ownership from a before-set/epoch snapshot — so a keyed verdict can never attribute a concurrent loop's change (change 0407).

**Architecture:** Three seams change. (1) The rungate store (`internal/app/rungate_store.go`) gains a serialized, bind-once claim-binding record per gate key (schema v3). (2) `change.claim` (`internal/app/change_claim.go`) accepts an optional `--gate-context` token: it validates the token against the durable gate record, reserves the binding before the metadata transaction, folds the context hash into the idempotency digest and the committed claim receipt (the authoritative proof, durable as a `Docket-Result` commit trailer on the metadata branch), and confirms the binding after the transaction. (3) `run.gate-verdict` (`internal/app/rungate_verdict.go`) replaces before-set/cardinality attribution with binding resolution plus exact-receipt recovery through a new `ClaimProofScanner` seam, and checks claim-instance continuity before any retry/continuation. Missing/conflicting/unprovable proof fails closed to non-authorizing reports.

**Tech Stack:** Go (stdlib + existing `internal/gitcli`, `internal/repository/transaction`); the repo's Go-native suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-09-07-keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent-design.md` (synchronized copy read from `.docket/`; the spec travels on the metadata branch).

## Global Constraints

- **Fail closed everywhere.** A supplied-but-invalid gate context is NEVER treated as an ungated claim; missing/conflicting/corrupt/old-schema binding state yields `gate-stop <key> gate-unavailable <typed-reason>` (or `gate-done <key> no-attributable-claim` for a provably absent claim) — never sibling ids, never a retry grant, never fallback to global claim inference.
- **Raw capabilities never surface.** The raw dispatch-context token is hashed at the boundary (`gateHashToken`, sha256 lowercase hex — matches gatedrive's `capHash`); only the hash reaches tracked metadata (the claim receipt), rendered artifacts, or diagnostics. The 0600-private gate record is the only place a raw parent capability lives (unchanged).
- **`RunVerify` stays the sole disposition authority**; the verdict mapper never re-derives a run-* verdict. The single-retry O_EXCL CAS and continuation-before-retry ordering are preserved exactly.
- **BeforeIDs / DispatchEpoch become diagnostics only** — they remain in the record but can no longer create retry authority.
- **No sleeps in tests.** Interleavings are exercised by explicit sequential calls over isolated temp repos and injected fakes.
- **Mutation evidence uses uncached Go**: every mutation probe runs `go test -count=1` (learning `cached-runner-serves-a-mutated-tree`).
- **The build gate runs the whole suite** via `go run ./cmd/docket development test` from the worktree root, never only the tests named here. New Go tests in existing packages are picked up by the existing `test_go_toolchain.sh` / `test_go_race.sh` wrappers — add **no** new `tests/*.sh` file and no budget row.
- Comments cross-reference **symbol names or verbatim-quoted clauses**, never line numbers (ADR-0054).
- Out of scope: slash-command attribution context (change 0345), the test-drive prepare-scope/start handshake (0405), loop scheduling/serialization, release determinism (0406). `refresh-claim`, reclaim, and resume-halted semantics are unchanged.

## File Structure

| File | Responsibility |
|---|---|
| `internal/app/rungate_store.go` (modify) | Schema v3; `GateRecord.BoundRequestID`/`BoundRevision`; claim-binding file primitives (`ReserveGateClaim`, `ConfirmGateClaim`, `LoadGateClaimBinding`), `FindGateRecordByContextHash`; new error kinds |
| `internal/app/rungate_store_test.go` (modify) | Store-boundary tests for the above |
| `internal/app/claim_proof.go` (create) | `ClaimProof`, `ClaimProofScanner`, production scanner over commit trailers |
| `internal/app/claim_proof_git_test.go` (create) | Real-git integration test: committed claim receipt → scanner output |
| `internal/app/workspace_ops.go` (modify) | `WorkspaceDeps.ClaimProofs` field |
| `internal/app/change_claim.go` (modify) | `GateContext` request field; validate/reserve/digest/receipt/confirm |
| `internal/app/change_claim_test.go` (modify) | Claim-boundary tests (fake engine) |
| `internal/app/rungate_verdict.go` (modify) | `resolveGateOwnership` replaces `attributeGateClaim`; continuity check; new reasons |
| `internal/app/rungate_verdict_test.go` (modify) | Rewritten attribution tests + fail-closed matrix |
| `internal/app/rungate_ownership_test.go` (create) | Deterministic two-gate interleaving acceptance matrix |
| `internal/cli/change.go` (modify) | `--gate-context` flag on `change claim` |
| `internal/cli/change_test.go` (modify) | Flag registration + request-wiring test |
| `internal/cli/run.go` (modify) | Wire `wdeps.ClaimProofs` for gate-verdict |
| `skills/docket-implement-next/SKILL.md` (modify) | Step-2 claim carries `--gate-context`; run-verify note |
| `internal/assets/embedded/tree/...` (regenerate) | Embedded copy of the skill via `go generate ./internal/assets` |
| `internal/repoguard/prose_contracts_test.go` (modify) | Skill-wiring sentinel rows |
| `docs/superpowers/plans/2026-09-07-adr-supersede-0075-draft.md` (create) | Successor-ADR draft handed to docket-adr at Step 6 |

---

### Task 1: Rungate store — claim-binding primitives and schema v3

**Files:**
- Modify: `internal/app/rungate_store.go`
- Test: `internal/app/rungate_store_test.go`

**Interfaces:**
- Consumes: existing `GateRecord`, `MintGateRecord`, `LoadGateRecord`, `SaveGateRecord`, `gateGitCommonDir`, `writeGateRecordAtomic`, `GateStoreError`, test helper `newGateRepo(t)`.
- Produces (later tasks rely on these exact signatures):
  - `type GateClaimBinding struct { Schema int; ChangeID int; RequestID string; Confirmed bool; Revision string }` (json: `schema`, `change_id`, `request_id`, `confirmed`, `revision,omitempty`)
  - `func ReserveGateClaim(repoDir, key string, changeID int, requestID string) error`
  - `func ConfirmGateClaim(repoDir, key string, changeID int, requestID, revision string) error`
  - `func LoadGateClaimBinding(repoDir, key string) (GateClaimBinding, bool, error)` (`false` = no binding file)
  - `func FindGateRecordByContextHash(repoDir, contextHash string) (string, GateRecord, error)`
  - Error kinds `ErrGateBindingConflict GateStoreErrorKind = "binding-conflict"` and `ErrGateContextAmbiguous GateStoreErrorKind = "context-ambiguous"`
  - `GateRecord` fields `BoundRequestID string` (json `bound_request_id,omitempty`) and `BoundRevision string` (json `bound_revision,omitempty`)
  - `gateSchemaVersion = 3`

- [ ] **Step 1: Write the failing store tests** (append to `internal/app/rungate_store_test.go`, following its `newGateRepo` fixture style):

```go
// TestGateSchemaV2RecordFailsClosed: a hand-written schema-2 record (the pre-0407
// shape whose AttributedID may be an inferred guess) must fail closed on load as
// corrupt-record — never a silent migration that blesses an old guessed id.
func TestGateSchemaV2RecordFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, GateRecord{Target: "docket-implement-next", Retry: RetryUnused})
	if err != nil { t.Fatalf("mint: %v", err) }
	rec, err := LoadGateRecord(repo, key)
	if err != nil { t.Fatalf("load: %v", err) }
	rec.Schema = 2 // bypass SaveGateRecord's authoritative stamp: write the file directly
	common, _ := gateGitCommonDir(repo)
	buf, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(common, "docket", "rungate", key, gateRecordFileName), buf, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, lerr := LoadGateRecord(repo, key)
	gse, ok := AsGateStoreError(lerr)
	if !ok || gse.Kind != ErrGateCorruptRecord {
		t.Fatalf("want corrupt-record for schema 2, got %v", lerr)
	}
}

// TestReserveGateClaimIsBindOnce: the first reservation wins; a different
// (change, request) under the same key is refused binding-conflict; an
// identical replay is a no-op.
func TestReserveGateClaimIsBindOnce(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	if err := ReserveGateClaim(repo, key, 3, "claim-3-aaa"); err != nil { t.Fatalf("reserve: %v", err) }
	if err := ReserveGateClaim(repo, key, 3, "claim-3-aaa"); err != nil { t.Fatalf("replay reserve: %v", err) }
	err := ReserveGateClaim(repo, key, 4, "claim-4-bbb")
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateBindingConflict {
		t.Fatalf("want binding-conflict, got %v", err)
	}
}

// TestConfirmGateClaimMirrorsRecord: confirm finalizes the binding and mirrors
// AttributedID/BoundRequestID/BoundRevision onto the record; a mismatched
// confirm is binding-conflict; a re-confirm is idempotent.
func TestConfirmGateClaimMirrorsRecord(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	if err := ReserveGateClaim(repo, key, 3, "claim-3-aaa"); err != nil { t.Fatalf("reserve: %v", err) }
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-aaa", "deadbeef"); err != nil { t.Fatalf("confirm: %v", err) }
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-aaa", "deadbeef"); err != nil { t.Fatalf("re-confirm: %v", err) }
	if err := ConfirmGateClaim(repo, key, 4, "claim-4-bbb", "cafe"); err == nil {
		t.Fatalf("mismatched confirm must fail")
	}
	b, ok, err := LoadGateClaimBinding(repo, key)
	if err != nil || !ok || !b.Confirmed || b.ChangeID != 3 || b.Revision != "deadbeef" {
		t.Fatalf("binding = %+v ok=%v err=%v", b, ok, err)
	}
	rec, err := LoadGateRecord(repo, key)
	if err != nil { t.Fatalf("load: %v", err) }
	if rec.AttributedID != 3 || rec.BoundRequestID != "claim-3-aaa" || rec.BoundRevision != "deadbeef" {
		t.Fatalf("record mirror = %+v", rec)
	}
}

// TestConfirmWithoutReservationFails: a failed or absent reservation can never
// become a confirmed binding (spec: "Failed claims never become confirmed
// bindings").
func TestConfirmWithoutReservationFails(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-aaa", "deadbeef"); err == nil {
		t.Fatalf("confirm without reservation must fail")
	}
}

// TestLoadGateClaimBindingCorruptFailsClosed: unparseable binding bytes are a
// typed corrupt-record error, never (ok=false, nil).
func TestLoadGateClaimBindingCorruptFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	common, _ := gateGitCommonDir(repo)
	if err := os.WriteFile(filepath.Join(common, "docket", "rungate", key, gateClaimBindingName), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := LoadGateClaimBinding(repo, key)
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateCorruptRecord {
		t.Fatalf("want corrupt-record, got %v", err)
	}
}

// TestFindGateRecordByContextHash: exactly-one non-terminal match resolves;
// zero is not-found; two armed gates sharing a hash is context-ambiguous;
// a terminal record does not match.
func TestFindGateRecordByContextHash(t *testing.T) {
	repo := newGateRepo(t)
	keyA := mintGateWithHash(t, repo, "ha", false)
	_ = mintGateWithHash(t, repo, "hb", false)
	_ = mintGateWithHash(t, repo, "ht", true) // terminal
	k, rec, err := FindGateRecordByContextHash(repo, "ha")
	if err != nil || k != keyA || rec.ChildContextHash != "ha" {
		t.Fatalf("k=%q rec=%+v err=%v", k, rec, err)
	}
	if _, _, err := FindGateRecordByContextHash(repo, "ht"); err == nil {
		t.Fatalf("terminal record must not match")
	}
	if _, _, err := FindGateRecordByContextHash(repo, "nope"); err == nil {
		t.Fatalf("zero matches must error")
	}
	_ = mintGateWithHash(t, repo, "hb", false)
	_, _, err = FindGateRecordByContextHash(repo, "hb")
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateContextAmbiguous {
		t.Fatalf("want context-ambiguous, got %v", err)
	}
}
```

Add the two tiny helpers beside them:

```go
// mintPlainGate mints a minimal armed record for store-primitive tests.
func mintPlainGate(t *testing.T, repoDir string) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{Target: "docket-implement-next", Retry: RetryUnused, Disposition: "gate-armed"})
	if err != nil { t.Fatalf("MintGateRecord: %v", err) }
	return key
}

// mintGateWithHash mints a record carrying ChildContextHash, optionally terminal.
func mintGateWithHash(t *testing.T, repoDir, hash string, terminal bool) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{Target: "docket-implement-next", Retry: RetryUnused, ChildContextHash: hash, Terminal: terminal})
	if err != nil { t.Fatalf("MintGateRecord: %v", err) }
	return key
}
```

(Import `encoding/json`, `os`, `path/filepath` in the test file if not present.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run 'TestGateSchemaV2|TestReserveGateClaim|TestConfirmGateClaim|TestConfirmWithout|TestLoadGateClaimBinding|TestFindGateRecordByContextHash' -count=1`
Expected: FAIL — undefined: `ReserveGateClaim`, `gateClaimBindingName`, etc.

- [ ] **Step 3: Implement in `rungate_store.go`**

1. `const gateSchemaVersion = 3` (update its doc comment: v3 adds the claim-binding proof fields and file; a v2 record — whose `AttributedID` may be a snapshot-inferred guess, the exact defect of change 0407 — fails closed as corrupt-record with the existing schema-mismatch diagnostic; the supported recovery is a newly armed `gate-before --resume` for a still-valid in-progress change).
2. `const gateClaimBindingName = "claim-binding.json"` beside `gateRecordFileName`.
3. `GateRecord` gains, in the schema-v3 comment block: `BoundRequestID string \`json:"bound_request_id,omitempty"\`` and `BoundRevision string \`json:"bound_revision,omitempty"\`` — documented as the local MIRROR of the confirmed claim binding (the committed claim receipt is authority); enforce the pair rule (both empty or both set) in a `gateBoundPairOK(rec GateRecord) bool` checked exactly where `gateContinuationTripleOK` is checked (load + atomic write), failing closed as corrupt-record.
4. New error kinds with doc comments: `ErrGateBindingConflict GateStoreErrorKind = "binding-conflict"` (a competing binding attempt for a different claim under one context), `ErrGateContextAmbiguous GateStoreErrorKind = "context-ambiguous"` (more than one live gate record claims one context hash).
5. `GateClaimBinding` struct as in Interfaces, with `bindingSchemaVersion = 1` stamped into `Schema`; load fails closed (corrupt-record) on any other value.
6. `ReserveGateClaim`: validate key; resolve dir (exists check like `ConsumeGateRetry`); if `gateClaimBindingName` exists → load it; matching (ChangeID, RequestID) → `nil` (idempotent replay), else `ErrGateBindingConflict`. If absent: write `GateClaimBinding{Schema:1, ChangeID, RequestID, Confirmed:false}` to a same-directory `os.CreateTemp` file, then **`os.Link(tmp, final)`** — the hard-link create is the CAS (fails `fs.ErrExist` if a concurrent reservation won; on `fs.ErrExist`, re-load and apply the match-or-conflict rule above), then remove the temp name. This serializes competing binding attempts at the store boundary with whole-file atomicity (no partially-written CAS winner).
7. `ConfirmGateClaim`: load binding; absent → `ErrGateNotFound`; mismatched (ChangeID, RequestID) → `ErrGateBindingConflict`; already confirmed with same fields → idempotent `nil` (a differing Revision on a confirmed binding is `ErrGateBindingConflict` — a later verdict or replay can never overwrite a binding). Otherwise atomically rewrite (same-dir temp + `os.Rename`) with `Confirmed: true, Revision: revision`, then mirror onto the record: load record, set `AttributedID = changeID`, `BoundRequestID = requestID`, `BoundRevision = revision`, `SaveGateRecord`. The mirror save's error is returned (callers may treat it as best-effort; the binding file already made the confirm durable).
8. `LoadGateClaimBinding`: validate key; read file; `fs.ErrNotExist` → `(GateClaimBinding{}, false, nil)`; unparseable/bad-schema → corrupt-record error; else `(b, true, nil)`.
9. `FindGateRecordByContextHash`: `gateRoot` + `os.ReadDir`; for each entry directory whose name passes `validateGateKey`, `LoadGateRecord` and **skip on error** (a foreign-repo or corrupt sibling never blocks an unrelated claim); collect keys where `!rec.Terminal && rec.ChildContextHash == contextHash && contextHash != ""`. Zero → `ErrGateNotFound`; more than one → `ErrGateContextAmbiguous`; one → return it.
10. In `PruneGateRecords` nothing changes (the binding file lives inside the key dir and is removed with it).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/app/ -run 'TestGateSchemaV2|TestReserveGateClaim|TestConfirmGateClaim|TestConfirmWithout|TestLoadGateClaimBinding|TestFindGateRecordByContextHash' -count=1`
Expected: PASS. Also run `go test ./internal/app/ -run 'Gate' -count=1` — pre-existing store/verdict tests must still pass (schema stamp is authoritative in Mint/Save, so they mint v3 transparently).

- [ ] **Step 5: Mutation-test the CAS** — temporarily change `os.Link` to an unconditional `os.Rename` (which overwrites nothing here but skips the exists check path) or drop the `fs.ErrExist` re-load branch; run Step 4's command with `-count=1`; `TestReserveGateClaimIsBindOnce` must redden. Restore by re-applying your edit (keep a copy of the correct block before mutating — never `git checkout` over uncommitted work, learning `mutation-restore-needs-a-backup-copy`).

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_store.go internal/app/rungate_store_test.go
git commit -m "feat(0407): rungate store claim-binding primitives, schema v3"
```

---

### Task 2: Claim-proof scanner over committed receipts

**Files:**
- Create: `internal/app/claim_proof.go`
- Create: `internal/app/claim_proof_git_test.go`
- Modify: `internal/app/workspace_ops.go` (one field)
- Modify: `internal/cli/run.go` (wire the seam for gate-verdict)

**Interfaces:**
- Consumes: `PlanningDeps` (`Reader.PinContext` → `StatusPin.MetadataRevision`; `Client.Discover`; `Client.ScanCommitTrailers`), trailer keys as committed by the engine (`Docket-Operation`, `Docket-Request-ID`, `Docket-Result` — see `engineTrailers` in `internal/repository/transaction/idempotency.go`), `changeClaimReceipt`, `OperationChangeClaim`.
- Produces:
  - `type ClaimProof struct { RequestID string; ChangeID int; GateContextHash string; Revision string }`
  - `type ClaimProofScanner interface { ScanClaimProofs(ctx context.Context, repoDir string) ([]ClaimProof, error) }` — newest-first (git log order)
  - `func NewClaimProofScanner(deps PlanningDeps) ClaimProofScanner`
  - `WorkspaceDeps.ClaimProofs ClaimProofScanner` field

- [ ] **Step 1: Write the failing real-git test** in `internal/app/claim_proof_git_test.go`, modeled on the fixture style of `claim_workflow_git_test.go` (real bare-remote temp repos + real `transaction.Engine`): drive a real `ChangeClaim` for a build-ready change **with** `GateContext` set (this depends on Task 3's field — so write the test now asserting the *scanner* half against a hand-committed trailer instead of a full claim; keep it engine-independent):

```go
// TestScanClaimProofsReadsCommittedReceipt: a metadata-branch commit carrying
// the engine's trailer block for a change.claim applied receipt is returned as
// one ClaimProof, newest-first, decoding gate_context_hash from the receipt.
func TestScanClaimProofsReadsCommittedReceipt(t *testing.T) { ... }
```

Fixture recipe (all plumbing exists): create a temp repo (reuse this package's real-git repo helper from `claim_workflow_git_test.go`'s fixtures), commit twice with hand-built trailer blocks in the commit message —

```
first (older):  Docket-Transaction-ID: t1
                Docket-Operation: change.claim
                Docket-Request-ID: claim-3-v1
                Docket-Request-Digest: sha256:aaaa
                Docket-Result: <base64url of {"branch":"fix/x","claimed_at":"2026-09-07T00:00:00Z","gate_context_hash":"","id":3,"lease":"live","op":"change.claim","status":"in-progress"}>
second (newer): same shape with Docket-Request-ID: claim-4-v2, id 4, gate_context_hash "abc123"
```

plus one non-claim commit (`Docket-Operation: change.groom`) that must be ignored. Build a `PlanningDeps` whose `Reader` is a fake pinning `MetadataRevision` to the tip hash and whose `Client` is a real `gitcli` client (`Discover` on the temp repo). Assert: two proofs, order `[{claim-4-v2,4,"abc123",<tip>}, {claim-3-v1,3,"",<older>}]`, and the groom commit absent.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/app/ -run TestScanClaimProofsReadsCommittedReceipt -count=1`
Expected: FAIL — undefined: `NewClaimProofScanner`.

- [ ] **Step 3: Implement `internal/app/claim_proof.go`**

```go
package app

// ClaimProof is one committed change.claim applied receipt read back from the
// metadata branch's engine trailer blocks — the durable, authoritative proof of
// a successful claim (spec: "The receipt is authoritative proof of a successful
// claim"). Proofs are returned newest-first (git log order), so the FIRST proof
// naming a change id is that id's newest claim.
type ClaimProof struct {
	RequestID       string // Docket-Request-ID trailer
	ChangeID        int    // receipt id
	GateContextHash string // receipt gate_context_hash ("" = ungated claim)
	Revision        string // commit hash carrying the receipt
}

// ClaimProofScanner is the verdict path's read-only seam onto committed claim
// proofs; tests inject fakes, production scans real commit trailers.
type ClaimProofScanner interface {
	ScanClaimProofs(ctx context.Context, repoDir string) ([]ClaimProof, error)
}
```

Production impl `type gitClaimProofScanner struct{ deps PlanningDeps }` with `NewClaimProofScanner(deps PlanningDeps) ClaimProofScanner`: `PinContext` (fresh-origin re-sync — the same plumbing every gate read uses), `Discover`, then `ScanCommitTrailers(ctx, repo, gitcli.ObjectID(pin.MetadataRevision), []string{"Docket-Request-ID"})`; for each returned commit whose trailers include `Docket-Operation: change.claim` **and** a `Docket-Result`, base64url-decode (`base64.RawURLEncoding`) the result, `json.Unmarshal` into `changeClaimReceipt`, and skip undecodable blocks (they are other operations' receipts or corrupt history — never a reason to fail the whole scan); require the `RequestID` trailer non-empty. Preserve scan order. Keep the trailer-key literals as private consts in this file with a comment cross-referencing `engineTrailers` by name (the engine's spellings are authoritative; a drift reddens the git test).

Add to `WorkspaceDeps` (in `workspace_ops.go`, beside `Continuation`):

```go
	// ClaimProofs reads committed change.claim receipts for the verdict path's
	// ownership resolution (change 0407). A nil scanner fails closed there —
	// unlike Continuation, ownership can never proceed without proof access.
	ClaimProofs ClaimProofScanner
```

Wire production in `internal/cli/run.go` where the gate-verdict command builds `wdeps` (beside the `newContinuationSeam` assignment): `wdeps.ClaimProofs = app.NewClaimProofScanner(deps)`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/app/ -run TestScanClaimProofsReadsCommittedReceipt -count=1 && go build ./...`
Expected: PASS, clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/app/claim_proof.go internal/app/claim_proof_git_test.go internal/app/workspace_ops.go internal/cli/run.go
git commit -m "feat(0407): claim-proof scanner over committed change.claim receipts"
```

---

### Task 3: `change.claim` carries the dispatch context and records proof

**Files:**
- Modify: `internal/app/change_claim.go`
- Test: `internal/app/change_claim_test.go`

**Interfaces:**
- Consumes: Task 1's store primitives; `gateHashToken` (rungate_before.go); existing claim plumbing (`claimPreflight`, `resolveClaimTarget`, `claimRequestID`, `canonicalDigest`, fake-engine test harness in `change_claim_test.go`).
- Produces:
  - `ChangeClaimRequest.GateContext string \`json:"gate_context,omitempty"\`` (optional; no `docket:"required"`)
  - `claimDigestPayload.GateContextHash string \`json:"gate_context_hash"\``
  - `changeClaimReceipt.GateContextHash string \`json:"gate_context_hash"\`` — inserted between `ClaimedAt` and `ID` (fields stay alphabetical so `json.Marshal` emits the canonical sorted-key form the receipt validator requires)
  - Refusal tokens `ClaimDispositionGateContextInvalid = "gate-context-invalid"` and `ClaimDispositionGateContextConflict = "gate-context-conflict"` (also used as finding codes)
  - `changeClaimOp.gateContextHash string` field feeding the receipt

- [ ] **Step 1: Write the failing tests** (extend `change_claim_test.go`, using its existing fake-engine + `newGateRepo` fixtures — the claim tests must now run in a repo that HAS a git common dir, so use `newGateRepo(t)` as `repoDir` where the existing tests pass a plain dir; follow whatever repoDir the existing applied-path test uses and arm a real gate record in it):

```go
// TestClaimGateContextInvalidRefusesBeforeTransaction: a supplied context that
// matches no armed gate record is a typed refusal that writes nothing and never
// degrades to an ungated claim (spec: "Never treat a supplied but invalid
// context as an ungated claim").
```
Arrange: repo with NO gate record; `ChangeClaim` with `GateContext: "tok"`. Assert: `Result == ResultInvalidState`, `Disposition == "gate-context-invalid"`, and the fake engine recorded **zero** Execute calls.

```go
// TestClaimGateContextReservesAndConfirms: a valid context reserves before the
// transaction and confirms after the applied outcome; the digest payload and
// receipt carry the context hash, never the raw token.
```
Arrange: arm a gate via `MintGateRecord` with `ChildContextHash: gateHashToken("tok")`, fake engine returning applied + a canonical receipt. Assert: reservation existed at Execute time (fake engine callback loads the binding: `Confirmed == false`, `ChangeID == req.ID`); after return, `LoadGateClaimBinding` shows `Confirmed == true` with `Revision == res.AppliedCommit`; the captured `transaction.Request.Idempotency.Digest` differs from the same request's digest without context (compute both via `canonicalDigest`); the receipt bytes handed to the engine contain `"gate_context_hash":"<hash>"` and do NOT contain the raw token `"tok"`.

```go
// TestClaimGateContextConflictRefused: a second claim for a DIFFERENT change id
// under the same context is refused gate-context-conflict before its
// transaction (criterion 3: one context cannot claim two changes).
```
Arrange: same armed gate; first `ChangeClaim` (id 3) applied; second `ChangeClaim` (id 4, same `GateContext`). Assert second: `Disposition == "gate-context-conflict"`, zero new Execute calls.

```go
// TestClaimSameIDDifferentContextDigestDiffers: two dispatches submitting the
// SAME (id, version) under different contexts must not share the idempotency
// path — their digests differ while their request ids match, so the engine's
// replay scan refuses the second as id-reuse rather than replaying the first's
// receipt (criterion 3).
```
Pure digest assertion: `canonicalDigest(OperationChangeClaim, claimDigestPayload{ID, Version, GateContextHash: h1})` != same with `h2`, and `claimRequestID` equal for both.

```go
// TestClaimUngatedUnchanged: no GateContext → no gate lookup, no reservation,
// digest equals the empty-hash payload, receipt carries gate_context_hash "".
```

```go
// TestClaimTerminalGateRefused: a context whose only record is Terminal is
// gate-context-invalid (the dispatch it named is already decided).
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run 'TestClaimGateContext|TestClaimSameID|TestClaimUngated|TestClaimTerminalGate' -count=1`
Expected: FAIL — `GateContext` undefined.

- [ ] **Step 3: Implement in `change_claim.go`**

1. Add `GateContext` to `ChangeClaimRequest` with a doc comment: the run-gate dispatch context token from `run.gate-before`; optional — an ungated claim omits it; hashed at this boundary, the raw token never enters the transaction, receipt, or findings. Add the two disposition consts beside the existing ones.
2. In `ChangeClaim`, after `resolveClaimTarget` and before `canonicalDigest`:

```go
	var gateKey, gateHash string
	if req.GateContext != "" {
		gateHash = gateHashToken(req.GateContext)
		key, _, ferr := FindGateRecordByContextHash(repoDir, gateHash)
		if ferr != nil {
			return newChangeClaimResult(OperationChangeClaim, ResultInvalidState, ChangeClaimResult{
				Disposition: ClaimDispositionGateContextInvalid,
				Findings: []StatusFinding{lifecycleFinding(FindingCode(ClaimDispositionGateContextInvalid),
					"supplied gate context matches no live armed gate in this repository; refusing — an invalid context is never an ungated claim: "+ferr.Error())},
			})
		}
		gateKey = key
		if rerr := ReserveGateClaim(repoDir, gateKey, req.ID, claimRequestID(req)); rerr != nil {
			return newChangeClaimResult(OperationChangeClaim, ResultInvalidState, ChangeClaimResult{
				Disposition: ClaimDispositionGateContextConflict,
				Findings: []StatusFinding{lifecycleFinding(FindingCode(ClaimDispositionGateContextConflict),
					"this dispatch context is already bound to a different claim; one context cannot claim two changes: "+rerr.Error())},
			})
		}
	}
```

(The raw token appears in neither finding. `FindGateRecordByContextHash` already excludes Terminal records and wrong-repo records — that IS the "validate supplied context against the current repository's durable gate/scope identity" check; it reads the store only, so the gatedrive scope capability the test-drive operations consume is untouched.)
3. Digest: `claimDigestPayload{ID: req.ID, Version: req.Version, GateContextHash: gateHash}` (add the struct field; ungated stays `""`, so existing ungated digests are changed too — that is fine: the digest guards replay identity going forward, and a pre-0407 in-flight replay across the upgrade simply re-proves eligibility as a fresh attempt or fails exact-version as contended, both safe).
4. `changeClaimOp` gains `gateContextHash string`; set it from `gateHash`; in `Plan`, add `GateContextHash: o.gateContextHash` to the `changeClaimReceipt` literal.
5. After the engine call, in `ChangeClaim` (not the generic `claimResultFromOutcome`, which refresh-claim shares):

```go
	out := claimResultFromOutcome(OperationChangeClaim, res, execErr)
	if gateKey != "" && out.Result == ResultApplied {
		if cerr := ConfirmGateClaim(repoDir, gateKey, req.ID, claimRequestID(req), out.Revision); cerr != nil {
			// The metadata claim is committed and authoritative; the local binding
			// confirm is the mirror. Surface, never fail the applied claim — the
			// verdict path recovers from the exact committed receipt (spec:
			// "recover only from the exact committed receipt for that dispatch").
			out.Findings = append(out.Findings, lifecycleFinding(FindingCode(ClaimDispositionGateContextConflict),
				"claim committed but the local gate binding could not be confirmed; the keyed verdict will recover from the committed receipt: "+cerr.Error()))
		}
	}
	return out
```

Note an `already-claimed` replay also returns `ResultApplied` with the replayed receipt — confirming with the same (id, request) is the idempotent no-op Task 1 built, satisfying "an idempotent replay can confirm only its own original dispatch/claim association". A replay whose `out.Revision` is empty (older engine replays) must skip Confirm rather than write an empty revision: guard with `out.Revision != ""`.
6. `ChangeRefreshClaim` is untouched (it never reads `GateContext`; its digest-free exact-version transaction is unchanged).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/app/ -run 'TestClaim|TestChangeClaim' -count=1`
Expected: PASS (new tests and every pre-existing claim test — the ungated digest change breaks no test unless one pins digest bytes; if one does, update its expected digest to the new canonical payload, which is the guard's own remedy).

- [ ] **Step 5: Mutation-test the propagation (criterion 8, claim half)** — with `-count=1` each, restore between probes: (a) hard-code `gateHash = ""` after hashing → `TestClaimGateContextReservesAndConfirms`'s digest/receipt asserts must redden; (b) delete the `ReserveGateClaim` call → `TestClaimGateContextConflictRefused` must redden. Record both readings for the results file.

- [ ] **Step 6: Commit**

```bash
git add internal/app/change_claim.go internal/app/change_claim_test.go
git commit -m "feat(0407): change.claim carries gate context — validate, reserve, prove, confirm"
```

---

### Task 4: CLI `--gate-context` flag on `change claim`

**Files:**
- Modify: `internal/cli/change.go`
- Test: `internal/cli/change_test.go`

**Interfaces:**
- Consumes: `changeIDVersionSubcommand` builder; Task 3's `ChangeClaimRequest.GateContext`.
- Produces: `docket change claim --id <id> --version <v> [--gate-context <token>]`; `change refresh-claim` gains no flag.

- [ ] **Step 1: Write the failing test** (beside `TestChangeClaimCommandsRegistered`):

```go
// TestChangeClaimGateContextFlag: claim registers the optional --gate-context
// flag and refresh-claim does NOT (refresh re-proves nothing about ownership).
func TestChangeClaimGateContextFlag(t *testing.T) {
	root := captureTree(t)
	claimCmd, _, err := root.Find([]string{"change", "claim"})
	if err != nil { t.Fatalf("find claim: %v", err) }
	if claimCmd.Flags().Lookup("gate-context") == nil {
		t.Fatalf("change claim must register --gate-context")
	}
	refreshCmd, _, err := root.Find([]string{"change", "refresh-claim"})
	if err != nil { t.Fatalf("find refresh-claim: %v", err) }
	if refreshCmd.Flags().Lookup("gate-context") != nil {
		t.Fatalf("refresh-claim must not register --gate-context")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run TestChangeClaimGateContextFlag -count=1`
Expected: FAIL.

- [ ] **Step 3: Implement** in `change.go` — the claim closure reads the flag itself (the shared builder stays claim/refresh-agnostic):

```go
	claim := changeIDVersionSubcommand("claim",
		"Claim a build-ready change at an exact version, moving it to in-progress",
		func(c *cobra.Command, deps app.PlanningDeps, repoDir string, req app.ChangeClaimRequest) {
			req.GateContext, _ = c.Flags().GetString("gate-context")
			setResult(app.ChangeClaim(c.Context(), deps, repoDir, req))
		}, EffectMetadataWrite)
	claim.Flags().String("gate-context", "", "run-gate dispatch context `token` from gate-before, binding this claim to its armed gate (optional; omitted for an ungated claim)")
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/cli/ -count=1`
Expected: PASS, including every pre-existing CLI surface test. If a capability-signature or schema-surface guard reddens on the new flag/field (e.g. a pinned usage string or reflected-schema fixture), obey the failing guard's own remedy message to regenerate/update its pinned copy in this same task — never relax the guard (learning `config-edit-trips-its-own-frozen-drift-guard`).

- [ ] **Step 5: Commit**

```bash
git add internal/cli/change.go internal/cli/change_test.go
git commit -m "feat(0407): change claim --gate-context flag"
```

---

### Task 5: `gate-verdict` resolves ownership from proof, never inference

**Files:**
- Modify: `internal/app/rungate_verdict.go`
- Modify: `internal/app/rungate_verdict_test.go`
- Modify: `internal/app/rungate_before.go` (comment-only: the header's before-set/epoch prose becomes "diagnostic")

**Interfaces:**
- Consumes: Tasks 1–2 (`LoadGateClaimBinding`, `ReserveGateClaim`, `ConfirmGateClaim`, `GateRecord.BoundRequestID`/`BoundRevision`, `WorkspaceDeps.ClaimProofs`).
- Produces:
  - `func resolveGateOwnership(ctx context.Context, wdeps WorkspaceDeps, repoDir, key string, rec *GateRecord) *RunGateVerdictResult` — nil = ownership established (rec mutated in place); non-nil = the terminal report to return
  - Reason consts: `ReasonGateBindingUnreadable = "binding-unreadable"`, `ReasonGateBindingConflict = "binding-conflict"`, `ReasonGateClaimReplaced = "claim-replaced"`, `ReasonGateProofUnavailable = "proof-unavailable"`
  - `attributeGateClaim` **deleted**

- [ ] **Step 1: Rewrite the attribution tests and add the fail-closed matrix.** In `rungate_verdict_test.go`: delete the tests that exercise the three before-set/epoch filters (the block under `// --- attribution filters (never reach RunVerify)`), and the corpus-driven helpers they alone use. Extend `gateMintArmed` with a `hash string` parameter (set `ChildContextHash`), fixing existing callers. Add:

```go
// fakeProofScanner returns canned proofs newest-first, or errs.
type fakeProofScanner struct {
	proofs []ClaimProof
	err    error
}
func (f *fakeProofScanner) ScanClaimProofs(context.Context, string) ([]ClaimProof, error) {
	return f.proofs, f.err
}
```

New tests (each arranges a repo via `newGateRepo`, a gate via `gateMintArmed(t, repo, nil, 1, "ha")`, and `wdeps.ClaimProofs`):

1. `TestVerdictConfirmedBindingResolvesBoundChange` — reserve+confirm binding (id from the run-verify fixture, request `"claim-N-v"`, revision `"r1"`); scanner proofs `[{RequestID:"claim-N-v", ChangeID:N, GateContextHash:"ha", Revision:"r1"}]`; full rv fixture with the change complete → `gate-done <key> run-complete N` (criterion: completion before first verdict resolves the bound change even though it left in-progress).
2. `TestVerdictNoBindingNoProofIsNoAttributableClaim` — no binding, scanner `proofs: nil` → `gate-done <key> no-attributable-claim`, and the retry marker file must not exist afterward (criterion 2: a dispatch that claims nothing never acquires a sibling's claim — arrange a sibling proof `{RequestID:"x", ChangeID:9, GateContextHash:"OTHER"}` in the scanner to prove the hash filter, and assert AttributedID stays 0 and no report field names 9).
3. `TestVerdictUnconfirmedReservationRecoversFromExactReceipt` — reservation only (no confirm); scanner carries the matching proof (same RequestID, hash `"ha"`) → binding confirmed, verdict delegates to the bound id (criterion 4 recovery). Variant `TestVerdictUnconfirmedReservationWithoutReceiptStops`: scanner has no matching proof → `gate-done <key> no-attributable-claim`, and `LoadGateClaimBinding` still shows the unconfirmed reservation (left refused, never released).
4. `TestVerdictAbsentBindingAdoptsSoleProof` — no binding file at all, scanner has exactly one proof with hash `"ha"` → adopted (record mirror updated best-effort), delegation proceeds; two proofs with hash `"ha"` → `gate-stop <key> gate-unavailable binding-conflict`.
5. `TestVerdictClaimReplacedStops` — confirmed binding (id N, request `"claim-N-v1"`); scanner proofs newest-first `[{RequestID:"claim-N-v2", ChangeID:N, GateContextHash:""}, {RequestID:"claim-N-v1", ChangeID:N, GateContextHash:"ha"}]` → `gate-stop <key> gate-unavailable claim-replaced`, retry marker absent, binding unchanged (criteria 6, 7).
6. `TestVerdictNilProofScannerFailsClosed` — fresh claim-bound gate, `wdeps.ClaimProofs == nil` → `gate-stop <key> gate-unavailable proof-unavailable`.
7. `TestVerdictProofScanErrorFailsClosed` — scanner `err` set → `proof-unavailable`, no retry consumed.
8. `TestVerdictCorruptBindingFailsClosed` — write `{not json` into the key dir's `claim-binding.json` → `gate-stop <key> gate-unavailable binding-unreadable` (criterion 5).
9. `TestVerdictResumeBindingSkipsContinuity` — record minted with `AttributedID` pre-set and `BoundRequestID` empty (the `gate-before --resume` shape, `gateMintAttributed`), `ClaimProofs` a scanner that would report a replacement — verdict must NOT stop on continuity and must delegate to `RunVerify` (preserved verified-resume behavior, criterion 7).
10. `TestVerdictOwnershipIgnoresBeforeSetAndEpoch` — the regression pinning the 0407 defect: gate armed with `BeforeIDs: nil`, epoch `1`, hash `"ha"`; corpus contains a sibling in-progress change (id 9, `claimed_at` after the epoch — exactly what the old filters would have attributed); scanner has no `"ha"` proof → `gate-done <key> no-attributable-claim`, never 9. **Mutation partner:** re-introducing epoch/before-set inference (Step 5) must redden exactly this test.

Keep untouched (they must stay green): `TestRunGateVerdictConcurrentRetryGrantsOnce`, `TestVerdictIncompleteWithTrackedDriveContinuesWithoutRetry`, `TestVerdictWaitingIsNonterminalContinue`, the observe-mode tests, and every mapping-table test that enters via `gateMintAttributed` (they exercise the resume-shaped binding and the unchanged RunVerify mapping).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run 'TestVerdict|TestRunGateVerdict' -count=1`
Expected: new tests FAIL (undefined `fakeProofScanner` semantics / old inference still active); pre-existing kept tests still compile after the `gateMintArmed` signature fix.

- [ ] **Step 3: Implement in `rungate_verdict.go`**

1. Add the four reason consts with doc comments (each names its fail-closed cause; `claim-replaced` documents: "the bound change was reclaimed and re-claimed by another run — the old gate must neither retry nor take over the replacement (spec: 'the old gate must detect that replacement and cannot retry or take over the new claim')").
2. Replace the `if rec.AttributedID == 0 { ... attributeGateClaim ... }` block in `RunGateVerdict` with:

```go
	if stop := resolveGateOwnership(ctx, wdeps, repoDir, key, &rec); stop != nil {
		return *stop
	}
```

3. `resolveGateOwnership` logic (mutating `rec`, returning the terminal report or nil):
   - **Resume-verified shape:** `rec.AttributedID != 0 && rec.BoundRequestID == ""` → return nil immediately (pre-bound by `gate-before --resume` through `WorkspaceInspect`; continuity for it is RunVerify's job, exactly as today).
   - Load the binding: error → `gateStopUnavailable(..., ReasonGateBindingUnreadable)` (map any `*GateStoreError` through its Kind only into the message; the reason token stays `binding-unreadable`).
   - Everything below needs proofs: `wdeps.ClaimProofs == nil` → `gateStopUnavailable(..., ReasonGateProofUnavailable)`; scan error → same. (Scan once, up front, for all branches that need it — which is every remaining branch.)
   - **Confirmed binding present:** continuity — find the FIRST (newest) proof whose `ChangeID == binding.ChangeID`; if found and its `RequestID != binding.RequestID` → `gateStopUnavailable(..., ReasonGateClaimReplaced)`. (Absent from the scan entirely — e.g. truncated history — the binding's own confirmed receipt could not be seen either: `ReasonGateProofUnavailable`, terminal, no retry.) Continuity held → mirror onto `rec` if the record lags (`rec.AttributedID == 0`): set `AttributedID`/`BoundRequestID`/`BoundRevision`, best-effort `SaveGateRecord`, and best-effort `BindScopeChange` (move the existing defense-in-depth block — with its `[MUTATION: ...]` comment and `TestVerdictFreshRunBindsScopeChange` anchor — here). Return nil.
   - **Unconfirmed reservation:** find a proof with `RequestID == binding.RequestID && GateContextHash == rec.ChildContextHash`; found → `ConfirmGateClaim` (best-effort; a confirm error still proceeds on the proof — the receipt is authority), adopt as above, return nil. Not found → the reserved claim never committed: persist and return `gate-done <key> no-attributable-claim` (the reservation is deliberately left in place — "leave it refused rather than allow a different claim").
   - **No binding file:** filter proofs to `GateContextHash == rec.ChildContextHash` (skip when `rec.ChildContextHash == ""` — a hashless record cannot prove ownership: `ReasonGateProofUnavailable`). Exactly one → adopt: best-effort `ReserveGateClaim` + `ConfirmGateClaim` with the proof's fields, mirror onto rec, return nil. Zero → `gate-done <key> no-attributable-claim`. More than one → `gateStopUnavailable(..., ReasonGateBindingConflict)`.
4. Delete `attributeGateClaim` (whole function) and drop the now-unused `domain`/`repository` imports if nothing else in the file uses them. Rewrite the file-header ATTRIBUTION paragraph: fresh gates resolve the verified dispatch-to-claim binding (change 0407); the before-set and dispatch epoch are retained in the record as diagnostics only and can never create retry authority; quote the spec clause "replace before-set/cardinality attribution with the verified dispatch-to-claim binding".
5. In `rungate_before.go`, update the comments on `DispatchEpoch`/`BeforeIDs` (steps 2–3) to say the captured values are diagnostics for humans reading the record — keyed attribution now binds at claim time (change 0407) — without changing any behavior (gate-before still records them, still pre-binds `--resume`).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/app/ -run 'TestVerdict|TestRunGateVerdict|TestGate' -count=1`
Expected: PASS, including all kept continuation/retry/observe tests.

- [ ] **Step 5: Mutation-test the ownership check (criterion 8, verdict half)** — with `-count=1`, restore between probes: (a) re-introduce inference: in the no-binding zero-proof branch, return the sibling in-progress id from a fresh corpus read instead of no-attributable-claim → `TestVerdictOwnershipIgnoresBeforeSetAndEpoch` reddens; (b) drop the `GateContextHash == rec.ChildContextHash` filter → the sibling-proof assert in `TestVerdictNoBindingNoProofIsNoAttributableClaim` and `TestVerdictAbsentBindingAdoptsSoleProof`'s conflict half redden; (c) drop the continuity comparison → `TestVerdictClaimReplacedStops` reddens. Record all three readings.

- [ ] **Step 6: Commit**

```bash
git add internal/app/rungate_verdict.go internal/app/rungate_verdict_test.go internal/app/rungate_before.go
git commit -m "feat(0407): keyed verdict resolves ownership from claim proof; inference deleted"
```

---

### Task 6: Deterministic two-gate interleaving matrix

**Files:**
- Create: `internal/app/rungate_ownership_test.go`

**Interfaces:**
- Consumes: everything above; the run-verify fixtures (`rvFixture` et al.) and lifecycle corpus builders already used by `rungate_verdict_test.go`.
- Produces: the acceptance-criteria interleaving evidence (spec items 1, 2, 6, 7) as one readable matrix file.

- [ ] **Step 1: Write the matrix tests.** One temp repo per test; two gates `A` (`hash "ha"`) and `B` (`hash "hb"`) armed **before either claim**; claims materialized as store bindings + scanner proofs (the app/store boundary — Task 3 already proved ChangeClaim produces exactly these); dispositions driven by the run-verify fixtures. No sleeps; every interleaving is a literal call sequence.

```go
// TestTwoGatesEachVerifyOnlyTheirOwn — spec acceptance item 1, all four
// orderings: (complete A, verdict A, verdict B), (verdict B, complete A,
// verdict A), (both in-progress), (both complete). Gate A must always report
// on A's id and gate B on B's id; neither line may carry the sibling id.
```

Table-drive it: subtests `A-completes-first`, `B-verdict-first`, `both-in-progress`, `both-complete`. For each: bind A→idA (confirmed, proof `{reqA, idA, "ha", "rA"}`), B→idB likewise; scanner for each gate's verdict call carries BOTH proofs (the shared history); fixture corpus per subtest sets each id's disposition (complete = the rv complete fixture shape; in-progress = `gateIncompleteRecord`-style). Assert each verdict's `AttributedID` and outcome, and assert the OTHER id appears nowhere in the report line (`strings.Contains` on `HumanText()`).

```go
// TestUnrelatedChurnDoesNotMoveOwnership — spec acceptance item 2/6 first
// half: after binding A→idA, mutate the corpus (sibling claims, refreshed
// claimed_at on idA, a priority edit) and re-run the verdict: same id, same
// outcome family; ownership survives timestamp refresh and completion.
```

```go
// TestReplacementClaimBlocksOldGate — spec item 6 second half: A bound to
// idA at reqA; add a NEWER proof {reqA2, idA, "" or "hb"}; A's verdict is
// gate-stop gate-unavailable claim-replaced; then assert ConsumeGateRetry's
// marker file does not exist and a subsequent verdict still refuses (no
// takeover of the replacement run).
```

```go
// TestLaterVerdictCannotOverwriteBinding — spec item 7: after A's verdict
// bound and reported, hand-call ConfirmGateClaim with a different revision →
// binding-conflict; re-run the verdict → same bound id as before.
```

- [ ] **Step 2: Run to verify** (these should pass immediately if Tasks 1–5 are correct; a failure here is a real integration bug — debug it, never weaken the assert)

Run: `go test ./internal/app/ -run 'TestTwoGates|TestUnrelatedChurn|TestReplacementClaim|TestLaterVerdict' -count=1`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/app/rungate_ownership_test.go
git commit -m "test(0407): deterministic two-gate ownership interleaving matrix"
```

---

### Task 7: Skill wiring — implement-next claims with its dispatch context

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md`
- Regenerate: `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md` (via `go generate ./internal/assets`)
- Modify: `internal/repoguard/prose_contracts_test.go`

**Interfaces:**
- Consumes: Task 4's flag spelling `--gate-context`.
- Produces: the generated dispatch surface instructs the child to bind its claim.

- [ ] **Step 1: Add the sentinel rows first** (they fail until the prose lands). In `prose_contracts_test.go`'s contract table, add for `skills/docket-implement-next/SKILL.md` the present-phrases:
  - `"pass it to the claim as --gate-context"`
  - `"an invalid or conflicting gate context is a typed refusal that writes nothing — never retried as an ungated claim"`

Run: `go test ./internal/repoguard/ -run Prose -count=1` (use the file's actual test name if different — read it before running). Expected: FAIL (phrases absent).

- [ ] **Step 2: Edit `skills/docket-implement-next/SKILL.md`.** Two edits, wording exact so the sentinels match:
  1. In **Step 2 (Claim)** — the paragraph beginning "Claim by transaction: the `change.claim` operation with `--id <id> --version <entity-version>`" — append after its first sentence: `When the dispatch prompt carried a **dispatch context** token, pass it to the claim as --gate-context <token>: the claim validates it against the armed gate, records durable proof binding this dispatch to its successful claim, and the parent's keyed verdict resolves ownership from that proof; an invalid or conflicting gate context is a typed refusal that writes nothing — never retried as an ungated claim. A claim without the token stays supported for ungated invocations.`
  2. In **Verify the run** (the paragraph ending "pass it into every `gate.drive.prepare-scope` / `gate.drive.start --gate-context` this run performs") — extend that final sentence: `..., and into the Step-2 claim's --gate-context.`
- [ ] **Step 3: Regenerate the embedded bundle**

Run: `go generate ./internal/assets`
Then `git status --short` — expect the embedded skill copy (and manifest, if fingerprinted) modified; `go test ./internal/assets/ -count=1` must pass (its drift guard is the byte-equality authority — obey its remedy if it names one).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/repoguard/ ./internal/assets/ -count=1`
Expected: PASS.

- [ ] **Step 5: Repo-wide sweep for other instruction sites** (Global-Constraints rule: derive sites from a whole-repo grep, never a hand list):

Run: `grep -rn "change.claim\|dispatch context" --exclude-dir=.git --exclude-dir=docs skills/ internal/assets/embedded/tree/ cursor-rules/ 2>/dev/null` (also grep `agents/` if present). For each MAINTAINED hit (skill bodies, cursor rules, reference docs — not archived changes/specs/plans, which are point-in-time history and stay untouched): if it documents how a gated child claims, align it with the Step-2 wording; `cursor-rules/run-gate.md` and the AGENTS.md run-gate block are parent-side (arm/verdict) and need no edit unless the sweep shows they narrate child claim mechanics. Re-run `go generate ./internal/assets` if any embedded-rooted source changed.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-implement-next/SKILL.md internal/assets/embedded internal/repoguard/prose_contracts_test.go
git commit -m "docs(0407): implement-next claims with its dispatch context; sentinels + embedded regen"
```

---

### Task 8: Successor-ADR draft (recorded later by docket-adr, not here)

**Files:**
- Create: `docs/superpowers/plans/2026-09-07-adr-supersede-0075-draft.md`

The ADR ledger lives on `metadata_branch` and is written by the `docket-adr` agent during implement-next Step 6 through one atomic ADR transaction (number allocation, `supersedes:` edge, the old ADR's status flip to `"Superseded by ADR-NN"`, index re-render). This task — following the precedent of `docs/superpowers/plans/2026-09-02-adr-supersede-0098-draft.md` — only prepares the decision text on the feature branch so Step 6 hands it over verbatim.

- [ ] **Step 1: Write the draft** with exactly these sections:
  - **Title:** `Run-gate attribution binds a dispatch to its successful claim transaction`
  - **Supersedes:** ADR-0075 (conservative snapshot/cardinality claim attribution). **Change:** 407.
  - **Context:** ADR-0075's before-set + dispatch-epoch inference attributes "exactly one surviving new in-progress claim" and explicitly accepted a concurrency residual. Under parallel implement-next loops the intended change completes, drops out of the in-progress set, and a lone concurrent claim survives the filters — keyed verdicts then named sibling changes (observed 2026-09-04 on changes 0403/0402; change 0407's reconcile log confirms the mechanism in `RunGateBefore`/`attributeGateClaim`).
  - **Decision:** `change.claim` optionally carries the dispatch context minted by `run.gate-before`; a gated claim reserves a bind-once binding at the store boundary before its metadata transaction, folds the context hash into its idempotency digest, commits the hash inside the claim receipt (the durable, authoritative ownership proof on the metadata branch), and confirms the local binding after. Keyed verdicts resolve the bound change — complete or not — through `RunVerify`, recover an interrupted binding only from the exact committed receipt, check claim-instance continuity (a replacement claim stops the old gate), and fail closed (`gate-unavailable` with a typed reason, or `no-attributable-claim`) on anything unprovable. Gate records are schema-versioned so a pre-proof record can never bless an inferred id.
  - **Preserved from ADR-0075:** the conservative safety principle — never guess, never choose among candidates, a wrong grant is the one unrecoverable move; the halt-reporting/exit-code semantics ADR-0075 also decided are untouched.
  - **Consequences:** unattributed mode stays observe-only; ungated claims stay supported; changes 0345 (slash-command context) and 0405 (test-drive handshake) may reuse the primitive.
  - Close with: `Recorded by the docket-adr transaction at implement-next Step 6; this draft is the handed-over decision text. ADR-0075's frozen body is not rewritten — its status line alone flips to "Superseded by ADR-NN" inside that same transaction.`

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/plans/2026-09-07-adr-supersede-0075-draft.md
git commit -m "docs(0407): ADR draft — claim-transaction-bound gate attribution superseding ADR-0075"
```

---

### Task 9: Full-suite gate

- [ ] **Step 1: Run the whole suite from the worktree root**

Run: `go run ./cmd/docket development test`
Expected: `SUITE` summary green. Treat `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` lines as screening findings; a `SERIAL CONFIRMED OVER BUDGET:` line is a breach to act on (serial-confirm per `tests/README.md` before believing a parallel number).

- [ ] **Step 2: Fix forward anything red** (root-cause per `superpowers:systematic-debugging`; never weaken an existing safety assert — the retry-CAS, continuation-ordering, and observe-isolation tests are load-bearing).

- [ ] **Step 3: Collect evidence for the results file** (the build role records it): the three Task-5 mutation readings, the two Task-3 readings, the Task-1 CAS reading — each with the exact reddened test name and `-count=1` on every probe.

- [ ] **Step 4: Commit any fixes**

```bash
git add -u
git commit -m "test(0407): full-suite gate fixes"
```

---

## Self-Review (performed at authoring)

- **Spec coverage:** carry context through claim boundary (Tasks 3–4); schema/capability/CLI/skill updated together (Tasks 3, 4, 7); validate-before-mutate, fail-closed invalid context (Task 3); persist proof via receipt machinery, no raw capabilities in metadata (Task 3); bind-once serialized at store boundary (Task 1); idempotency: dispatch identity in digest, replay confirms only its own association (Task 3); interrupted reservation ↔ exact-receipt recovery ↔ left-refused (Tasks 1, 5 test 3); durable ownership across refresh/status/restart + replacement detection (Tasks 5, 6); verdict by ownership, `RunVerify` authority, `run-complete` for completed bound change (Task 5 test 1, Task 6); missing/conflicting/corrupt/old-schema → typed non-authorizing stop, no sibling names, no inference fallback (Task 5 matrix, Task 1 schema test); retry CAS + continuation-before-retry + verified resume preserved (Task 5 kept-tests list, test 9); unattributed observe-only untouched (no observe-path edit anywhere); schema versioning fails old records closed (Task 1); successor ADR (Task 8); acceptance items 1–8 mapped: 1→T6, 2→T5#2+T6, 3→T3, 4→T3+T5#3, 5→T1+T5#6–8, 6→T5#5+T6, 7→T5#9+kept tests+T6, 8→T3/T5 mutation steps+T2 git test+T7 sentinels.
- **Placeholder scan:** no TBDs; every code step carries concrete code or an exact edit recipe with named anchors.
- **Type consistency:** `ReserveGateClaim(repoDir, key string, changeID int, requestID string)` / `ConfirmGateClaim(..., revision string)` / `LoadGateClaimBinding` / `FindGateRecordByContextHash` / `ClaimProof{RequestID, ChangeID, GateContextHash, Revision}` / `WorkspaceDeps.ClaimProofs` / `ChangeClaimRequest.GateContext` are spelled identically in Tasks 1–6.
