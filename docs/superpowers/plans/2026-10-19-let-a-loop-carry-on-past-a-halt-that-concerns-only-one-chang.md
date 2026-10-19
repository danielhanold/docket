<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0570 — Let a loop carry on past a halt that concerns only one change](../../changes/active/0570-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang.md)**
<!-- docket:backlink:end -->

# Let a loop carry on past a halt that concerns only one change — Implementation Plan

> **For agentic workers:** executed by `docket-build` task-by-task (one tier worker per task, one
> full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A tracked implement-next loop carries on past a halt on a change it picked from the
queue, while a per-repository count of halts in a row stays under `run.max_consecutive_halts`
(default 2); nothing outside a loop-picked halt changes.

**Architecture:** The claim records *how* the change was chosen (`--selection queue|named`) in its
committed receipt and mirrors it on the run's claim binding. A new halt-count store under the
per-repo state folder (`<state>/run-tracker-halts/`, beside — never inside — `run-tracker/`) keeps
the count and a once-per-key marker. `run.verdict` consults it only on the `run-halted` branch and
after a final `run-done … run-complete`, and any fault there falls back to today's line. A new
`run.reset-halts` operation lets a person zero the count.

**Tech Stack:** Go (internal/app, internal/config, internal/cli), cobra, Markdown skills/docs,
`go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang-design.md`
(on the `docket` metadata branch; the build copy rides the feature branch).

## Global Constraints

- No bump of `runTrackerSchemaVersion` (stays `4`) or `bindingSchemaVersion` (stays `1`). New
  fields are additive, `omitempty`, and carry **no** save-time guard.
- `RunTrackerRecord` gains **no** fields at all (the selection lives on the binding and receipt).
- Count faults never change a report line and never produce `run-tracker-unavailable`; a named
  halt never reads the count; any lock on the count is bounded.
- The claim's selection input never refuses a claim and is not part of `claimDigestPayload` or
  `claimRequestID`.
- A named run, resume, retry, continuation, untracked run, `--unattributed` observe, and finalize
  print exactly today's lines.
- Config key `run.max_consecutive_halts`: int, default **2**, minimum **1** (`intLeaf(1)`),
  `scopeAny`, layered like every key; invalid values are ordinary config errors (never clamped).
- New decision token `run-next`; line shape `run-next <key> run-halted <id>`; it keeps
  `terminal: true`, goes through `persistRunVerdict`, never `runTrackerCompleteRun`; the run stays
  `active`. No retry is ever spent on a halt.
- An agent never resets the count on its own; the at-limit note says so.
- `run.verdict` always exits 0.
- Agent-executed markdown (skills/, cursor-rules/, AGENTS.md) names operation ids
  (`run.reset-halts`, `run.start`), never a hard-coded `docket <verb>` (TestCapabilitySurface…).
- Docs describe current behavior only — no change or PR citations. Comments in maintained source
  anchor on symbol names, never line numbers.
- Gate: the whole suite, `go run ./cmd/docket development test`. Mutation probes use
  `go test -count=1`. Integration tests are `//go:build integration` and named with an existing
  shard prefix (`TestIntegrationRunVerdict…`, `TestIntegrationRunRecord…`,
  `TestIntegrationRecordOps…`); hand runs: `go test -tags integration -count=1 -run '^<Prefix>' ./internal/app/`.

## Spec reconciliation notes (read before Task 3)

1. **The named path passes `--selection named`.** Spec §6 says "the named path passes nothing",
   but §1 maps an absent input to `unknown`, and `unknown` *adds to the count* — which contradicts
   the change's own scope ("a halt on a named change … never touches it") and the isolation table
   ("Named run … the count is never read"). Passing `named` explicitly is the only reading that
   satisfies both. The binary behavior for an absent input is exactly as specified (`unknown`, never
   a refusal), so an older skill copy still claims as today. Call this out in the results file.
2. **A visibility switch does not reset the count.** `privateStateFolderPhase` (and its shared
   twin) renames the whole state folder, so `run-tracker-halts/` moves with it. Task 7 documents
   what is true (the count moves with the folder), not the spec's "effectively resets".
3. **No marker pruning.** `PruneRunTrackerRecords` has no production caller today; the counted
   markers are tiny and are not pruned (YAGNI).
4. **The limit is read at verdict time** (`deps.Reader.PinContext` → `Run.MaxConsecutiveHalts`),
   not snapshotted — no record field, so no schema question.
5. **ADR.** Recording the decision (amends ADR-0153's "every `halted` stops the driver" context,
   extends ADR-0115's run accounting) is the parent's Step 6 job, not a build task.

## Review Focus

1. `ConfirmRunTrackerClaim` rewrites the binding from scratch — if it drops `Selection`, every loop
   halt silently prints today's line and the feature is inert. Pinned in Task 3.
2. Re-running `run.verdict` for an old key after the count moved (reset, other halts) must replay
   the saved decision, never flip `run-next` ↔ `run-stop`, and an old `run-complete` replay must
   never re-zero. Pinned in Tasks 2 and 4.
3. Two halts settling at the same time (two sessions, two keys) must not lose an increment. Pinned
   in Task 2 (concurrent distinct keys).
4. A lock holder that never lets go (a stuck process) must not hang the verdict. Pinned in Tasks 2
   and 4 with a shortened `haltLockWait`.
5. A person lowers the limit below the current count (count 3, limit 2): the next loop-picked halt
   is `run-stop` + note, never `run-next`; a corrupt `count.json` is a fault (today's line), never
   silently read as 0, and `run.reset-halts` repairs it. Pinned in Tasks 2, 4 and 5.

---

## File structure

| File | Responsibility |
|---|---|
| `internal/config/{config,schema,defaults,resolve}.go` | the new key |
| `internal/app/runtracker_halts.go` (new) | halt-count store, `run.reset-halts` app op, note text |
| `internal/app/runtracker_halts_test.go` (new, unit, no git) | store semantics |
| `internal/app/runtracker_halts_integration_test.go` (new, integration) | verdict + reset-halts behavior |
| `internal/app/change_claim.go` | `Selection` request field, normalization, receipt field |
| `internal/app/runtracker_store.go` | binding `Selection`, `ReserveRunTrackerClaimSelection`, confirm keeps it |
| `internal/app/claim_proof.go` | `ClaimProof.Selection` |
| `internal/app/runtracker_verdict.go` | `run-next`, halt mode, halted branch, reset on run-complete, `reportLine` |
| `internal/cli/change.go`, `internal/cli/run.go`, `internal/cli/install.go`, `internal/app/schema_registry.go` | flag + new leaf wiring |
| `skills/docket-implement-next/SKILL.md`, `cursor-rules/run-tracker.md`, `AGENTS.md` | driver contract, claim flag, dispatch sentence |
| docs (Task 7) | current behavior |

---

### Task 1: The `run.max_consecutive_halts` config key

**Build tier:** standard

**Files:**
- Modify: `internal/config/config.go` (type `Run`), `internal/config/schema.go` (row after `run.max_attempts`), `internal/config/defaults.go` (`Run{…}`), `internal/config/resolve.go` (the `assign` list), `internal/app/config.go` (the `leafLine` list, right after `run.max_attempts`)
- Modify: `.docket.example.yml` (the `run:` block), then `go generate ./internal/assets/`
- Modify: `docs/reference/config-keys.md` (row after `run.max_attempts`)
- Test: `internal/config/schema_test.go`, `defaults_test.go`, `decode_test.go`, `setting_paths_test.go`, `resolve_test.go`, `internal/app/config_test.go`

**Interfaces:**
- Produces: `config.Run.MaxConsecutiveHalts Value[int]` (`json:"max_consecutive_halts"`), resolved path `run.max_consecutive_halts`, built-in 2.

- [ ] **Step 1: Write the failing tests** — mirror every `run.max_attempts` site (grep `run.max_attempts` under `internal/config/*_test.go` and `internal/app/config_test.go`):
  - `schema_test.go`: add `"run.max_consecutive_halts"` to the path list right after `"run.max_attempts"`; default map `"run.max_consecutive_halts": 2`; validation cases `{"run halts explicit","run.max_consecutive_halts","5",5,""}`, `{"run halts one",…,"1",1,""}`, `{"run halts zero",…,"0",nil,CodeInvalidValue}`, `{"run halts negative",…,"-1",nil,CodeInvalidValue}`, `{"run halts string",…,`"two"`,nil,CodeInvalidType}`.
  - `defaults_test.go`: `"run.max_consecutive_halts": eff.Run.MaxConsecutiveHalts.Value`.
  - `decode_test.go`: row `run.max_consecutive_halts`, block `"run:\n  max_consecutive_halts: 3\n"`, flow `"run: {max_consecutive_halts: 3}\n"`, value 3.
  - `setting_paths_test.go`: add the path after `"run.max_attempts"`.
  - `resolve_test.go`: the `case "run.max_consecutive_halts":` accessor, plus `TestPrecedenceRunMaxConsecutiveHalts` copied from `TestPrecedenceRunMaxAttempts` (built-in 2; global 3; repository 4 beats global; repository-local 5 beats all).
  - `internal/app/config_test.go`: alongside the `run.max_attempts = 2  [built-in]` assertions, assert HumanText contains `run.max_consecutive_halts = 2  [built-in]` and the effective JSON carries 2.

- [ ] **Step 2: Run to verify failure**
  Run: `go test -count=1 ./internal/config/ && go test -count=1 -run 'Config' ./internal/app/`
  Expected: FAIL (unknown key / missing field).

- [ ] **Step 3: Implement** — schema row `{path: "run.max_consecutive_halts", kind: kindInt, def: 2, merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: intLeaf(1)}`; struct field with a comment saying it caps halts in a row on changes picked from the queue before a tracked loop stops; `builtinValue(2)`; `set(assign(&eff.Run.MaxConsecutiveHalts, r.declared, "run.max_consecutive_halts"))`; the diagnostics `leafLine`. Example config entry under `run:` (reader register, no change numbers):

```yaml
  # max_consecutive_halts — how many changes picked from the queue may halt in a row before a
  # tracked implement-next loop stops. A halted change always waits for a person; under this limit
  # the loop moves on to the next build-ready change instead of ending the drain. 2 by default;
  # 1 stops the loop at the first halt. A clean build resets the count, and so does
  # `docket run reset-halts`. A whole number of 1 or more.
  # scope: any layer
  max_consecutive_halts: 2
```

  Then `go generate ./internal/assets/`. Docs row: `| \`run.max_consecutive_halts\` | \`2\` | any layer | halts in a row, on changes a loop picked from the queue, before the loop stops |`.

- [ ] **Step 4: Run to verify pass**
  Run: `go test -count=1 ./internal/config/ ./internal/assets/ && go test -count=1 -run 'Config' ./internal/app/`
  Expected: PASS (the example-correspondence guard sees the new documented key).

- [ ] **Step 5: Commit**
```bash
git add internal/config internal/app/config.go internal/app/config_test.go .docket.example.yml internal/assets/embedded docs/reference/config-keys.md
git commit -m "feat(config): run.max_consecutive_halts caps halts in a row for a loop (default 2)"
```

---

### Task 2: The halt-count store

**Build tier:** premium (concurrency/locking; once-per-key semantics named in the spec)

**Files:**
- Create: `internal/app/runtracker_halts.go`
- Modify: `internal/app/runtracker_verdict.go` (add `RunDecisionNext = "run-next"` to the decision-token const block)
- Test: `internal/app/runtracker_halts_test.go` (no build tag; pure filesystem under `t.TempDir()` — must not start git)

**Interfaces:**
- Produces:
  - `const RunDecisionNext = "run-next"`
  - `const runTrackerHaltsDirName = "run-tracker-halts"`
  - `type haltCountStore struct{ dir string }`
  - `func haltCountStoreAt(stateDir string) haltCountStore` — `dir = <stateDir>/run-tracker-halts`
  - `func openHaltCountStore(repoDir string) (haltCountStore, error)` — via `runTrackerStateDir`
  - `type haltCounted struct { Schema int "schema"; Decision string "decision,omitempty"; HaltsInARow int "halts_in_a_row"; HaltLimit int "halt_limit,omitempty"; Noted bool "noted,omitempty" }`
  - `func (s haltCountStore) recordHalt(key string, loopPicked bool, limit int) (haltCounted, error)`
  - `func (s haltCountStore) recordReset(key string) error`
  - `func (s haltCountStore) resetAll() (previous int, unreadable bool, err error)`
  - `func (s haltCountStore) count() (int, error)` (unlocked read; tests/diagnostics)
  - `var haltLockWait = 2 * time.Second` (tests shorten it, restoring it in `t.Cleanup`; a test that sets it never calls `t.Parallel`), `var errHaltCountBusy = errors.New("halt count busy")`

On disk: `count.json` = `{"schema":1,"halts_in_a_row":N}`; markers `counted/<key>.halt.json` (a `haltCounted`) and `counted/<key>.reset.json` (`{"schema":1}`); `lock` (flock file). Every key passes `validateRunKey` before a path is built. Writes use a same-directory `os.CreateTemp` + `os.Rename`.

- [ ] **Step 1: Write the failing tests** (`runtracker_halts_test.go`)

```go
func TestHaltCountRecordHaltUnderLimitIsRunNext(t *testing.T) {
	s := haltCountStoreAt(t.TempDir())
	out, err := s.recordHalt("k1", true, 2)
	// err == nil; out.Decision == RunDecisionNext; out.HaltsInARow == 1; out.HaltLimit == 2; !out.Noted
}
func TestHaltCountRecordHaltAtLimitIsRunStopNoted(t *testing.T) {
	// k1 then k2 at limit 2 → second: Decision == RunDecisionStop, Noted, HaltsInARow == 2
}
func TestHaltCountRecordHaltOncePerKey(t *testing.T) {
	// recordHalt("k1") twice → count() == 1 and both results equal;
	// resetAll(); recordHalt("k1") again → returns the SAVED run-next/1 and count() stays 0
}
func TestHaltCountUnknownModeCountsWithoutDecision(t *testing.T) {
	// recordHalt("k1", false, 0) → Decision == "", HaltsInARow == 1, count() == 1
}
func TestHaltCountLimitBelowCountStops(t *testing.T) {
	// three unknown halts (k1..k3), then recordHalt("k4", true, 2) → run-stop, Noted, HaltsInARow == 4
}
func TestHaltCountRecordResetOncePerKey(t *testing.T) {
	// halts k1,k2 → 2; recordReset("c1") → 0; recordHalt("k3",…) → 1; recordReset("c1") → count() still 1
}
func TestHaltCountCorruptCountIsAFault(t *testing.T) {
	// write "{" to <dir>/count.json → recordHalt returns err, file bytes unchanged, no marker written;
	// resetAll() → unreadable == true, err == nil, count() == 0
}
func TestHaltCountLockIsBounded(t *testing.T) {
	// haltLockWait = 100*time.Millisecond (restore via t.Cleanup); open <dir>/lock and
	// syscall.Flock(LOCK_EX) it in the test; recordHalt returns errors.Is(err, errHaltCountBusy)
	// within 1s; count() still 0
}
func TestHaltCountConcurrentDistinctKeysAllCount(t *testing.T) {
	// 16 goroutines, keys k0..k15, recordHalt(key, true, 100) → count() == 16
}
func TestHaltCountRejectsMalformedKey(t *testing.T) {
	// recordHalt("../x", true, 2) and recordReset("../x") error; <dir> holds no count.json
}
```

- [ ] **Step 2: Run to verify failure**
  Run: `go test -count=1 -run 'TestHaltCount' ./internal/app/`
  Expected: FAIL (undefined: haltCountStoreAt).

- [ ] **Step 3: Implement `runtracker_halts.go`.** File header comment: what the count is, why it lives outside `run-tracker/` (the run-key scanners never see it — same reason as `acquireResumeLock`'s `run-tracker-resume/`), and the two crash orders below. Lock: `MkdirAll(<dir>/counted, 0o700)`, open `<dir>/lock` `0o600`, loop `syscall.Flock(fd, LOCK_EX|LOCK_NB)`; on `EWOULDBLOCK` sleep 20ms until `haltLockWait` elapses, then `errHaltCountBusy`. `readCount`: missing → 0; unparseable, wrong schema, or negative → error (never 0). The algorithm the tests pin:

```go
// recordHalt, under the lock:
//   saved marker exists → return it unchanged (once per key; replay never flips)
//   n := readCount() + 1                       (any read error → return it, write nothing)
//   out := haltCounted{Schema: 1, HaltsInARow: n}
//   if loopPicked { out.HaltLimit = limit
//       if n < limit { out.Decision = RunDecisionNext } else { out.Decision = RunDecisionStop; out.Noted = true } }
//   write count FIRST, then the marker — a crash between re-counts this key on replay:
//   an over-count, which stops a loop sooner, never later.
// recordReset, under the lock:
//   reset marker exists → nil (an old run-complete never re-zeroes)
//   write the marker FIRST, then count 0 — a crash between leaves the count high (stops sooner).
// resetAll, under the lock: read (error → unreadable=true, previous=0), write count 0. Markers stay.
```

- [ ] **Step 4: Run to verify pass**
  Run: `go test -count=1 -race -run 'TestHaltCount' ./internal/app/`
  Expected: PASS. Mutation probe: swap the replay check below the increment → `OncePerKey` reddens; restore.

- [ ] **Step 5: Commit**
```bash
git add internal/app/runtracker_halts.go internal/app/runtracker_halts_test.go internal/app/runtracker_verdict.go
git commit -m "feat(run): a per-repository halt count with once-per-key markers and a bounded lock"
```

---

### Task 3: The claim records how the change was chosen

**Build tier:** premium (claim ownership binding; isolation guarantee 3)

**Files:**
- Modify: `internal/app/change_claim.go` (`ChangeClaimRequest`, `changeClaimReceipt`, `changeClaimOp`, `ChangeClaim`)
- Modify: `internal/app/runtracker_store.go` (`RunTrackerClaimBinding`, `ReserveRunTrackerClaim`, `ConfirmRunTrackerClaim`)
- Modify: `internal/app/claim_proof.go` (`ClaimProof`, `ScanClaimProofs`)
- Modify: `internal/app/runtracker_verdict.go` (the sole-proof adopt branch of `resolveRunTrackerOwnership`)
- Modify: `internal/cli/change.go` (claim flag), `internal/cli/capability_production_test.go` (signature)
- Test: `internal/app/change_claim_integration_test.go`, `internal/app/runtracker_store_integration_test.go`, `internal/app/claim_proof_git_integration_test.go`, the existing sole-proof adoption test in `internal/app/runtracker_verdict_integration_test.go`

**Interfaces:**
- Produces:
  - `const ClaimSelectionQueue = "queue"`, `const ClaimSelectionNamed = "named"`
  - `ChangeClaimRequest.Selection string` (`json:"selection,omitempty"`)
  - `func claimSelection(raw string) string` — `"queue"`/`"named"` pass through (after `strings.TrimSpace`), anything else → `""` (unknown)
  - `changeClaimReceipt.Selection string` `json:"selection,omitempty"` placed between `Op` and `Status` (alphabetical json-key order is load-bearing for the receipt validator)
  - `RunTrackerClaimBinding.Selection string` `json:"selection,omitempty"`
  - `func ReserveRunTrackerClaimSelection(repoDir, key string, changeID int, requestID, selection string) error`; `ReserveRunTrackerClaim(repoDir, key, changeID, requestID)` becomes a wrapper passing `""` (no test call sites change)
  - `ClaimProof.Selection string`
  - CLI: `change claim --selection <mode>`; signature `--id <id> --revision <revision> [--repo-dir <dir>] [--run-context <token>] [--selection <mode>]`

- [ ] **Step 1: Write the failing tests**
  - `TestIntegrationRecordOpsClaimSelectionRecordedInBindingAndReceipt` — table over input `"queue"→"queue"`, `"named"→"named"`, `"loop"→""`, `""→""`; per row a fresh `newRunTrackerRepo`, `mintRunTrackerWithHash(t, repo, runTrackerHashToken("tok"), false)`, `ChangeClaim(… ChangeClaimRequest{ID: 3, Revision: gateClaimRevision, RunContext: "tok", Selection: in})`. Assert: `res.Result == ResultApplied` for every row (never refuses); post-claim binding `Selection == want` and `Confirmed`; `engine.calls[0].Idempotency.Digest` equals `canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 3, Revision: gateClaimRevision, RunContextHash: hash})` and `RequestID == claimRequestID(ChangeClaimRequest{ID: 3, Revision: gateClaimRevision, RunContext: "tok"})` (identity unchanged); `op.selection == want`; the planned receipt (via `claimPlanFor`) contains `"selection":"queue"` when `want != ""` and contains no `"selection"` substring when `want == ""`.
  - `TestIntegrationRunRecordConfirmKeepsSelection` — `ReserveRunTrackerClaimSelection(repo, key, 3, "claim-3-v", ClaimSelectionQueue)`, `ConfirmRunTrackerClaim(repo, key, 3, "claim-3-v", "r1", "")` → binding `Confirmed && Selection == "queue"`; a replayed `ReserveRunTrackerClaimSelection(…, ClaimSelectionNamed)` returns nil and the binding still reads `"queue"` (bind-once).
  - `TestIntegrationRecordOpsScanClaimProofsReadsSelection` — copy the committed-receipt pattern of `TestIntegrationRecordOpsScanClaimProofsReadsCommittedReceipt` (add `Selection string \`json:"selection,omitempty"\`` to its `wireClaimReceipt`, between `Op` and `Status`): a receipt with `Selection: "queue"` scans to `proof.Selection == "queue"`, one without to `""`.
  - Extend the existing absent-binding sole-proof adoption test in `runtracker_verdict_integration_test.go` (the one that adopts the single proof matching the record's context hash): give its proof `Selection: ClaimSelectionQueue` and assert the binding afterwards reads `Selection == "queue"`.
  - `capability_production_test.go`: update the `"change.claim"` signature to the one above.

- [ ] **Step 2: Run to verify failure**
  Run: `go test -tags integration -count=1 -run '^(TestIntegrationRecordOpsClaim|TestIntegrationRecordOpsScanClaimProofs|TestIntegrationRunRecordConfirm)' ./internal/app/ && go test -count=1 -run 'Signature' ./internal/cli/`
  Expected: FAIL (unknown field `Selection`).

- [ ] **Step 3: Implement.** In `ChangeClaim`, compute `sel := claimSelection(req.Selection)` once; reserve via `ReserveRunTrackerClaimSelection(repoDir, runKey, req.ID, claimRequestID(req), sel)`; set `changeClaimOp.selection = sel` and fold it into the receipt. `claimDigestPayload` and `claimRequestID` are untouched. In `ConfirmRunTrackerClaim`, the rewritten binding carries `Selection: b.Selection` (this is Review Focus 1). `reserveMatchOrConflict` still compares only `(changeID, requestID)`. In `ScanClaimProofs`, copy `rec.Selection` into the proof. In the adopt branch of `resolveRunTrackerOwnership`, call `ReserveRunTrackerClaimSelection(repoDir, key, p.ChangeID, p.RequestID, p.Selection)`. CLI: `claim.Flags().String("selection", "", "how this change was chosen: \`mode\` queue (picked from the build-ready queue) or named (the run named it); any other value is recorded as unknown")` and `req.Selection, _ = c.Flags().GetString("selection")`.

- [ ] **Step 4: Run to verify pass**
  Run: same as Step 2, plus `go test -tags integration -count=1 -run '^TestIntegrationRunVerdict' ./internal/app/`
  Expected: PASS. Mutation probe: drop `Selection: b.Selection` in `ConfirmRunTrackerClaim` → `ConfirmKeepsSelection` reddens; restore.

- [ ] **Step 5: Commit**
```bash
git add internal/app/change_claim.go internal/app/runtracker_store.go internal/app/claim_proof.go internal/app/runtracker_verdict.go internal/cli/change.go internal/cli/capability_production_test.go internal/app/*_integration_test.go
git commit -m "feat(claim): record whether a claimed change was picked from the queue or named"
```

---

### Task 4: The verdict counts halts and prints `run-next`

**Build tier:** premium (the isolation guarantees are the named risk)

**Files:**
- Modify: `internal/app/runtracker_verdict.go`
- Create: `internal/app/runtracker_halts_integration_test.go` (`//go:build integration`)
- Test (unit): append to `internal/app/runtracker_halts_test.go`

**Interfaces:**
- Consumes: Task 2's store API and `RunDecisionNext`; Task 3's `RunTrackerClaimBinding.Selection`, `ClaimSelection*`, `ReserveRunTrackerClaimSelection`; Task 1's `Run.MaxConsecutiveHalts`.
- Produces:
  - `RunVerdictResult` fields `HaltsInARow int \`json:"halts_in_a_row,omitempty"\``, `HaltLimit int \`json:"halt_limit,omitempty"\``, `Note string \`json:"note,omitempty"\``; the `Decision` comment lists `run-next`.
  - `func (r RunVerdictResult) reportLine() string` — today's `HumanText` body, unchanged; `HumanText()` = `reportLine()` plus `"\n"+Note` when `Note != ""`. `persistRunVerdict` and `runTrackerCompleteRun` store `reportLine()` in `rec.Disposition` (byte-identical for every existing result, since `Note` is empty).
  - `type haltMode int` with `haltModeUnknown`, `haltModeLoopPicked`, `haltModeNamed`; `func runTrackerHaltMode(repoDir, key string, rec RunTrackerRecord) haltMode` — `rec.resumeAttributed()` → named; else `LoadRunTrackerClaimBinding`: error/absent → unknown; `Selection` `queue` → loop-picked, `named` → named, anything else → unknown.
  - `func runTrackerHaltLimit(ctx context.Context, deps PlanningDeps, repoDir string) (int, bool)` — `deps.Reader.PinContext` → `pin.Config.Effective.Run.MaxConsecutiveHalts.Value`; nil reader, error, or value < 1 → `(0, false)`.
  - `func runTrackerApplyHaltCount(ctx context.Context, deps PlanningDeps, repoDir, key string, rec RunTrackerRecord, res RunVerdictResult) RunVerdictResult`
  - `func runTrackerResetHaltCount(repoDir, key string)` — best-effort `recordReset`.
  - `func haltLimitNote(n, limit int) string`

- [ ] **Step 1: Write the failing tests.** Helper in the new integration file:

```go
// haltLoopRun mints a fresh started run for change id whose confirmed binding carries selection,
// and returns its key plus a WorkspaceDeps whose proof scanner names that claim.
func haltLoopRun(t *testing.T, repo string, id int, selection string) (string, WorkspaceDeps)
// uses runTrackerMintStarted(t, repo, nil, 1, hash), ReserveRunTrackerClaimSelection,
// ConfirmRunTrackerClaim(…, "r<id>", ""), and fakeProofScanner{{RequestID, ChangeID: id, RunContextHash: hash, Revision}}.
// Deps: runTrackerLightDeps(t, []StatusBlob{runTrackerHaltedInProgressBlob(3, rvSlug), runTrackerHaltedInProgressBlob(4, "other-change")}).
```

  Tests (all `TestIntegrationRunVerdictHalt…` so they land in the runverdict shard):
  - `…LoopPickedUnderLimitPrintsRunNext`: `HumanText() == "run-next "+key+" run-halted 3"`, `Terminal`, `HaltsInARow == 1`, `HaltLimit == 2`, `Note == ""`; persisted `Disposition` equals that line; `RunTrackerRetryUsage == 0`; with a `MintRunRecord(repo, key, "3")` beside it the run stays `RunActive` (as `TestIntegrationRunVerdictHaltedLeavesRunActive`).
  - `…LoopPickedAtLimitStopsWithNote`: change 3 then change 4 (two keys) → second `reportLine() == "run-stop "+k2+" run-halted 4"`, `Note` contains `run.reset-halts` and `never`, `HumanText() == reportLine()+"\n"+Note`, persisted `Disposition == reportLine()`.
  - `…ReplayNeverFlips`: verdict k1 → `run-next`; `store.resetAll()`; verdict k1 again → still `run-next … 3` and `count() == 0`; same for the at-limit key (still `run-stop` + note).
  - `…NamedNeverTouchesCount`: seed count 5 (five unknown `recordHalt`s on keys `s0..s4`); a `ClaimSelectionNamed` run → `HumanText() == "run-stop "+key+" run-halted 3"`, `HaltsInARow == 0`, `count() == 5`. A resume-shaped record (`runTrackerMintAttributedLimit(t, repo, 3, 2)`) in a fresh repo → today's line, and `<state>/run-tracker-halts` does not exist.
  - `…UnknownCountsButPrintsTodaysLine`: binding via legacy `ReserveRunTrackerClaim` (no selection), and one with raw selection `"loop"` → today's line, `count()` increments each time.
  - `…CountFaultKeepsTodaysLine`: corrupt `count.json` → loop-picked: `Decision == RunDecisionStop`, `Outcome == VerdictRunHalted`, `HumanText() == "run-stop "+key+" run-halted 3"`; named under the same fault: identical bytes.
  - `…LockBusyKeepsTodaysLine`: `haltLockWait = 100ms`, test holds the flock → loop-picked returns today's line in under 2s.
  - `…LimitOneStopsAtFirstHalt`: fake reader pin with `Run.MaxConsecutiveHalts.Value = 1` → `run-stop` + note at count 1.
  - `…CountAboveLoweredLimitStops`: seed 3, default limit 2 → `run-stop` + note, `HaltsInARow == 4`.
  - `…UnreadableLimitCountsAsUnknown`: pin limit value 0 → today's line, no note, `count()` incremented.
  - `TestIntegrationRunVerdictRunCompleteResetsHaltCount`: `newVerdictCompletionFixture`, seed 2 → `run-done … run-complete`, `count() == 0`; then one unknown halt (→1) and re-run the same keyed verdict → still `run-done`, `count() == 1`.
  - `TestIntegrationRunVerdictBlockedCloseoutKeepsHaltCount`: completion fixture with the closeout blocked (`fx.observer.defaultProven = false`, as the completion-unaccounted tests do) → `run-stop … completion-unaccounted`, count unchanged.
  - `TestIntegrationRunVerdictUnclaimedAndNoClaimKeepHaltCount`: seed 2; `runTrackerMintAttributedLimit` + `runTrackerProposedBlob(3, rvSlug)` → `run-done … run-unclaimed 3`; `newNoClaimRunFixture` → `no-attributable-claim`; count stays 2 both times.
  - `TestIntegrationRunVerdictObserveNeverTouchesHaltCount`: `RunVerdictObserve` over a halted blob in a fresh repo → `run-observe run-halted 3`, halts dir absent.
  - Unit (`runtracker_halts_test.go`): `TestRunVerdictHumanTextAppendsNote` — `RunVerdictResult{Decision: RunDecisionNext, Key: "k", Outcome: VerdictRunHalted, AttributedID: 3}` renders `"run-next k run-halted 3"`; with `Note: "note: x"` `HumanText()` is two lines and `reportLine()` is the first.

- [ ] **Step 2: Run to verify failure**
  Run: `go test -tags integration -count=1 -run '^TestIntegrationRunVerdict(Halt|RunComplete|Blocked|UnclaimedAndNoClaim|Observe)' ./internal/app/ && go test -count=1 -run 'TestRunVerdictHumanText' ./internal/app/`
  Expected: FAIL.

- [ ] **Step 3: Implement.** In `RunVerdict`:

```go
case VerdictRunComplete, VerdictRunUnclaimed:
	res := runTrackerCompleteRun(repoDir, key, rec, v.Verdict, id, verdictSeams(repoDir, wdeps))
	if v.Verdict == VerdictRunComplete && res.Decision == RunDecisionDone {
		runTrackerResetHaltCount(repoDir, key) // only after the run-done line is final
	}
	return res
case VerdictRunHalted:
	res := runVerdictLine(key, RunDecisionStop, VerdictRunHalted, id, true, nil)
	return persistRunVerdict(repoDir, key, rec, runTrackerApplyHaltCount(ctx, deps, repoDir, key, rec, res))
```

  `runTrackerApplyHaltCount` order (each fault returns `res` unchanged):
  1. `mode := runTrackerHaltMode(…)`; `haltModeNamed` → return `res` (the count is never opened).
  2. loop-picked: `runTrackerHaltLimit`; on `!ok` demote to unknown.
  3. `openHaltCountStore` → `recordHalt(key, mode == haltModeLoopPicked, limit)`.
  4. `out.Decision == ""` → return `res` (unknown, or a replayed unknown marker). Otherwise trust the **saved** decision (never re-derive from the current mode): set `HaltsInARow`/`HaltLimit`; `run-next` → `res.Decision = RunDecisionNext`; `run-stop` with `Noted` → `res.Note = haltLimitNote(…)`.

  Note copy (one line, no loop assumption):

```go
fmt.Sprintf("note: %d changes picked from the queue have halted in a row, reaching the limit of %d "+
	"(run.max_consecutive_halts). Ask the person whether to reset the halt count to 0; only on their "+
	"yes, run the run.reset-halts operation (docket run reset-halts). Never reset it on your own.", n, limit)
```

  Update the file-header comment with a short HALT COUNT paragraph (named never reads it; unknown counts but keeps today's line; faults keep today's line; once per key).

- [ ] **Step 4: Run to verify pass**
  Run: Step 2's commands, then the whole verdict and completion shards: `bash tests/test_go_integration_app_runverdict.sh && bash tests/test_go_integration_app_runcompletion.sh`
  Expected: PASS, and every pre-existing halted test (`TestIntegrationRunVerdictHaltedLeavesRunActive`, `…HaltPrecedenceOverBudget`) still prints `run-stop … run-halted 3`. Mutation probes (`-count=1`): delete the named early return → `NamedNeverTouchesCount` reddens; replace "trust the saved decision" with a mode check → `ReplayNeverFlips` reddens; restore each.

- [ ] **Step 5: Commit**
```bash
git add internal/app/runtracker_verdict.go internal/app/runtracker_halts_integration_test.go internal/app/runtracker_halts_test.go
git commit -m "feat(run): a halt on a queue-picked change under the limit prints run-next"
```

---

### Task 5: The `run.reset-halts` operation

**Build tier:** standard

**Files:**
- Modify: `internal/app/runtracker_halts.go` (app op), `internal/app/schema_registry.go`, `internal/cli/run.go`, `internal/cli/install.go` (`assetIndependent`), `internal/cli/capability_production_test.go`
- Test: `internal/app/runtracker_halts_integration_test.go`

**Interfaces:**
- Consumes: Task 2's `openHaltCountStore`, `resetAll`, `errHaltCountBusy`.
- Produces:
  - `const OperationRunResetHalts = "run.reset-halts"`
  - `type RunResetHaltsResult struct { Envelope; Previous int \`json:"previous"\`; PreviousUnreadable bool \`json:"previous_unreadable,omitempty"\`; Reason string \`json:"reason,omitempty"\`; Message string \`json:"message,omitempty"\` }`
  - `func RunResetHalts(repoDir string) RunResetHaltsResult`
  - HumanText: applied → `halt count reset to 0 (was N)` or `halt count reset to 0 (the previous count was unreadable)`; otherwise `run.reset-halts: <result> (<reason>)`.
  - Results: success `ResultApplied`; lock timeout `ResultContended` reason `halt-count-busy`; state folder unresolvable `ResultInvalidState` reason `run-tracker-unavailable`; write failure `ResultInternalError` reason `halt-count-unwritable`.
  - CLI leaf `run reset-halts`, `cobra.NoArgs`, `--repo-dir`, `capability("run.reset-halts", EffectLocalWrite)`; signature `[--repo-dir <dir>]`; registry `{ID: "run.reset-halts", Request: nil, Result: RunResetHaltsResult{}}` in sorted position.

- [ ] **Step 1: Write the failing tests**
  - `TestIntegrationRunVerdictResetHaltsZeroesCount`: seed 2 → `Result == ResultApplied`, `Previous == 2`, `HumanText() == "halt count reset to 0 (was 2)"`, `count() == 0`.
  - `TestIntegrationRunVerdictResetHaltsRepairsCorruptCount`: corrupt `count.json` → applied, `PreviousUnreadable`, `count() == 0`.
  - `TestIntegrationRunVerdictResetHaltsLockBusy`: `haltLockWait = 100ms`, flock held → `ResultContended`, `Reason == "halt-count-busy"`.
  - `capability_production_test.go`: `"run.reset-halts": "[--repo-dir <dir>]"` and effects `[local-write]`.

- [ ] **Step 2: Run to verify failure**
  Run: `go test -tags integration -count=1 -run '^TestIntegrationRunVerdictResetHalts' ./internal/app/ && go test -count=1 ./internal/cli/`
  Expected: FAIL.

- [ ] **Step 3: Implement** the app op, the cobra leaf (added to `runCmd.AddCommand`), `"run reset-halts": true` in `assetIndependent`, and the registry entry.

- [ ] **Step 4: Run to verify pass**
  Run: Step 2's commands plus `go test -count=1 ./internal/app/ -run 'Schema|Registry'`
  Expected: PASS (`TestAssetIndependentSetExact` and the registry↔catalog guard included).

- [ ] **Step 5: Commit**
```bash
git add internal/app/runtracker_halts.go internal/app/schema_registry.go internal/app/runtracker_halts_integration_test.go internal/cli/run.go internal/cli/install.go internal/cli/capability_production_test.go
git commit -m "feat(run): run reset-halts sets the halt count back to 0"
```

---

### Task 6: Skill, dispatch block, and their guards

**Build tier:** standard

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Step 2 claim paragraph; *Terminal disposition (driver contract)*)
- Modify: `cursor-rules/run-tracker.md` (step 3) and the identical text inside `AGENTS.md`'s `docket:dispatch` block (`CLAUDE.md` is a symlink to it)
- Regenerate: `go generate ./internal/assets/`
- Modify: `internal/repoguard/budgets_test.go` (`dispatchBudget`, the `docket-implement-next/SKILL.md` row), `internal/repoguard/prose_contracts_test.go` (`alignmentContracts`)

**Interfaces:**
- Consumes: the `--selection` flag (Task 3), `run-next` (Task 4), `run.reset-halts` (Task 5).

- [ ] **Step 1: Write the failing guards** — append to `alignmentContracts`:

```go
{sentinel: "change_0570_run_next_dispatch", file: "cursor-rules/run-tracker.md",
	present: []string{
		"`run-next <key> run-halted <id>` also forbids re-dispatching that change",
		"only a loop driver (`/loop`) may go on to its next run, with a fresh `run.start`",
	}},
{sentinel: "change_0570_run_next_dispatch", file: "AGENTS.md", present: /* same two clauses */},
{sentinel: "change_0570_driver_contract", file: "skills/docket-implement-next/SKILL.md",
	present: []string{
		"pass `--selection queue` on the selection path and `--selection named` on the named path",
		"a tracked loop whose verdict is `run-next`, which continues",
		"an untracked driver has no verdict and stops on every `halted`",
	},
	absent: []string{"The driver's decision is binary"}},
```

- [ ] **Step 2: Run to verify failure**
  Run: `go test -count=1 -run 'TestAlignmentContracts' ./internal/repoguard/`
  Expected: FAIL naming the missing clauses.

- [ ] **Step 3: Edit the prose.**
  - `cursor-rules/run-tracker.md` step 3, after "`run-halted` means a human is needed.", add: "`run-next <key> run-halted <id>` also forbids re-dispatching that change; only a loop driver (`/loop`) may go on to its next run, with a fresh `run.start`; a single dispatch stops, as on `run-stop`." Apply the byte-identical edit to `AGENTS.md`'s block, then verify the run-tracker portion matches: `diff <(sed -n '/^## Run tracker/,$p' cursor-rules/run-tracker.md) <(awk '/docket:dispatch:start/{f=1;next}/docket:dispatch:end/{f=0}f' AGENTS.md | sed -n '/^## Run tracker/,$p')` prints nothing.
  - SKILL.md Step 2, after "A claim without the token stays supported for ungated invocations.": "Also pass `--selection queue` on the selection path and `--selection named` on the named path: the run tracker counts a halt toward a loop's halt limit only for a change picked from the queue."
  - SKILL.md driver table, `halted` row action: `**stop + surface** — except a tracked loop whose verdict is \`run-next\`, which continues`.
  - Replace "The driver's decision is binary: **continue on `advanced`/`contended`, stop on `drained`/`halted`.**" with: "The driver's rule: **continue on `advanced`/`contended`, stop on `drained`/`halted`** — except that a tracked loop continues past `halted` when the run tracker's verdict is `run-next` (a halt on a change picked from the queue, still under `run.max_consecutive_halts`); an untracked driver has no verdict and stops on every `halted`." The child still declares `halted`; the four dispositions stay four.
  - `go generate ./internal/assets/`.

- [ ] **Step 4: Re-baseline the two budgets at the exact measured counts** — run `go test -count=1 -run 'TestSkillSizeBudgets|TestDispatch' ./internal/repoguard/`, read the measured numbers from the failure, set `dispatchBudget` and the SKILL.md row to them, and prepend the rationale in each comment's house style (`0570: the run-next sentence (was 932)`; `0570: the --selection claim sentence and the run-next driver rule (218/8555 -> L/W)`). `dispatchBudget` must stay `< dispatchOld` (1156).

- [ ] **Step 5: Run to verify pass**
  Run: `go test -count=1 ./internal/repoguard/ ./internal/assets/ ./internal/harness/`
  Expected: PASS (capability-surface guard included — no hard-coded `docket <verb>` added). Mutation probe: delete the dispatch sentence from `AGENTS.md` → `TestAlignmentContracts` reddens; restore.

- [ ] **Step 6: Commit**
```bash
git add skills/docket-implement-next/SKILL.md cursor-rules/run-tracker.md AGENTS.md internal/assets/embedded internal/repoguard/budgets_test.go internal/repoguard/prose_contracts_test.go
git commit -m "docs(skills): a tracked loop continues on run-next; the claim names how the change was chosen"
```

(AGENTS.md is read at session start: the results file's human-verify list should say a restarted session is needed before the loop behavior is trusted live.)

---

### Task 7: Docs

**Build tier:** standard

**Files:**
- Modify: `docs/guide/building-without-supervision.md` (*Draining the queue hands-free*), `docs/guide/landing-changes.md` (*Closing out hands-free with `/loop`*), `docs/concepts/run-tracker.md` (*The moving parts* diagram and bullets, *The invariants*), `docs/reference/glossary.md` (*Run verdict* table, *Attempt budgets*, *Finalize drain / run outcome*), `docs/reference/cli.md` (the `docket run` bullet), `docs/reference/outcomes.md` (wherever run-verdict lines are listed — grep `run-retry-once`)

- [ ] **Step 1: Edit**, current behavior only:
  - building-without-supervision: the driver rule gains the `run-next` exception; a new short subsection *When a change halts mid-drain*: a tracked loop carries past a halted change it picked from the queue, up to `run.max_consecutive_halts` (default 2) halts in a row; which runs count (picked from the queue, including a one-off "implement the next change"; never a named run or a resume); a clean build resets the count; at the limit the loop stops and the session asks whether to reset with `docket run reset-halts`, which it never runs on its own; the count is shared by every session and worktree of the repository and moves with the state folder on a visibility switch; a stopped-at-limit count stays until a clean build or a reset, so the next loop stops at its first halt; a visibility switch refuses while halted runs are still active, and a loop can leave up to the limit's worth of them. State docket supports **one implement loop and one finalize loop at a time** (side by side is fine) and why a second implement loop breaks the halt limit (shared count; one loop's clean builds reset the other's halts). Replace "Budget and iteration caps belong to the driver, not to docket, which does not reimplement them." with wording that keeps iteration caps with the driver and names the halt limit as docket's own.
  - landing-changes: implement and finalize no longer share one stop rule — finalize stops on every `halted`; implement's tracked loop may continue on `run-next`. Add the one-loop-per-stage sentence.
  - run-tracker concept: diagram row `├─ run-next ────────► this change needs a human; never re-dispatch it. A loop may start its next run (fresh run start); a single dispatch stops` placed before `run-stop`; a *halt count* bullet; invariants line: a halt never re-dispatches its change; `run-next` only lets a loop start a new run; only the person resets the count.
  - glossary: `run-next …` row in the run-verdict table; halt count under *Attempt budgets* (`run.max_consecutive_halts`); finalize drain entry notes finalize still stops on every `halted`.
  - cli.md: the `docket run` bullet mentions resetting the halt count.
  - Grep `docs/guide docs/concepts docs/reference README.md` for `/loop` and "loops" and correct any wording that implies two same-stage loops at once are supported.

- [ ] **Step 2: Verify**
  Run: `go test -count=1 ./internal/repoguard/`
  Expected: PASS (no retired vocabulary, no change citations flagged).

- [ ] **Step 3: Commit**
```bash
git add docs README.md
git commit -m "docs: a loop carries past a halted change up to the halt limit; one loop per stage"
```

---

### Final gate

- [ ] Run the whole suite: `go run ./cmd/docket development test` — exit 0; read the budget report (runverdict shard row is 20s; act only on `SERIAL CONFIRMED OVER BUDGET:` / `OVER BUDGET:` lines).
