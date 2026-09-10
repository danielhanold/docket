<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0405 — Investigate the gate.drive.prepare-scope -> gate.drive.start handshake rejecting a build-task worker's focused gate](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-10-0405-investigate-the-gate-drive-prepare-scope-gate-drive-start-ha.md)**
<!-- docket:backlink:end -->
# Sequential Test Drives Within One Worker Recovery Scope — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let one build-task recovery scope carry a *sequence* of task-owned test drives (baseline, RED, GREEN, verification) — at most one current execution or launch reservation at a time, each successor explicitly acknowledging its predecessor's durable PASSED/FAILED result — with a cataloged terminal-acknowledgement operation that closes the scope, and typed ownership diagnostics replacing the generic `invalid-request`.

**Architecture:** The scope record (schema v2) becomes the serialization authority for the scope's single drive slot: a durable *reservation* is persisted under the scope CAS **before** any process launch, the predecessor's recovery authority is retired (its drive-record owner generation cleared) as the journaled second half of that transition, and only then does the launch happen. Takeover/enumeration consult the scope's current slot, never timestamps; acknowledged predecessors drop out of recovery-candidate enumeration because their owner generation is cleared (the existing terminal-and-consumed exclusion in `FindScopeDriveIDs`). A new `gate.drive.acknowledge` operation is the final drive's "successor": it consumes the last result and closes the scope. `mapDriveFailure` learns to surface `OwnershipErrorKind` tokens instead of collapsing them to `invalid-request`.

**Tech Stack:** Go (`internal/gatedrive`, `internal/app`, `internal/cli`), cobra CLI, the repo's own suite runner (`internal/suiterunner` via `build.test_command`), markdown caller contracts under `skills/` with embedded mirrors under `internal/assets/embedded/tree/`.

**Spec:** `docs/superpowers/specs/2026-09-10-investigate-the-gate-drive-prepare-scope-gate-drive-start-ha-design.md` (synchronized copy read from `.docket/`; the spec travels with this plan — executors read both). Change file: `docs/changes/active/0405-investigate-the-gate-drive-prepare-scope-gate-drive-start-ha.md` on the `docket` branch.

## Global Constraints

- Design baseline: main commit `2f83683c2e1b4967779d09c7d71b7c35f4757b47`. Change 0416's identity-bundle fix and its tests MUST be preserved unchanged in behavior (spec "Evidence and relationship to existing work").
- Invariants 1–7 of the spec bind every task: parent keeps the parent capability; at most one current launch reservation or execution per scope; only durably PASSED/FAILED predecessors permit successors (WAITING, HALTED, pending handoff, transferred ownership, closed scope, corrupt state, unresolved launch state do not); a new test is a new drive, never a relaunch; starting a successor acknowledges exactly the received predecessor result; all transitions recheck identity/capability/current-drive/ownership under synchronization; closure, claim, and takeover remain terminal for the child's start authority.
- Do NOT require the successor's worktree fingerprint to match the predecessor's (edits between RED and GREEN are expected); each drive fingerprints the current worktree independently. All EXISTING fingerprint checks for advancing/transferring a live drive stay mandatory.
- Fail closed everywhere: ambiguous launch/persistence failures get a typed cause and **no automatic second launch**. A reservation is never treated as PASSED, FAILED, or safely quiescent.
- Diagnostics never print credentials, raw argv, environment contents, or unsafe stored error text. Reason tokens are the bounded `OwnershipErrorKind`/`StoreErrorKind` vocabulary; `HALTED` cause tokens stay separate from command-rejection reasons; the WAITING/PASSED/FAILED/HALTED outcome vocabulary is NOT widened.
- Versioned persistence: the scope schema bumps to v2; a v1 (or unknown) scope record fails closed `ErrUnknownSchema` — never reinterpreted (drive-store precedent, `drive.go` `driveSchemaVersion` comment). Keep hash-only persisted capabilities, 0700/0600 permissions, unknown-schema behavior.
- Every guard and load-bearing synchronization/identity predicate gets mutation evidence: disable the guarded behavior, watch the specific assert redden for the intended reason, restore. Defeat Go's test cache on every mutation probe and re-verification: `-count=1`. Restore mutations from a saved copy, never `git checkout --` over uncommitted work.
- No arbitrary sleeps and no unbounded stress tests. Races are barrier-controlled (channels in a fake `ProcessSeam`); the deterministic test clock (`clock.go` pattern) replaces waiting.
- Assert on inspected state — actual `Launch` call counts and persisted identities — not only on exit codes or response shape.
- Whole-suite gate: the controller runs whatever `build.test_command` resolves to, from source. Read the budget report even on green (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines are findings). `internal/gatedrive` gains many tests in this change — if its row breaches, serial-confirm before acting.
- Comments/cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers (`TestCommentAnchorStyle`).
- Update maintained caller contracts and their embedded mirrors **together**, deriving affected sites by repository search, never a fixed file allowlist.
- Workers: stage by explicit path only; one commit per task on branch `fix/investigate-the-gate-drive-prepare-scope-gate-drive-start-ha`; never touch `.docket/`, the metadata branch, or the board.

## File Structure

| File | Responsibility after this change |
|---|---|
| `internal/gatedrive/ownership.go` | + `ErrStalePredecessor`, `ErrPredecessorNotReusable`, `ErrUnresolvedLaunchTransition`, `ErrScopeBusy` kinds |
| `internal/gatedrive/scope.go` | scope schema v2: single-slot lifecycle (`reserveScopeDrive`, `confirmScopeLaunch`, `clearPendingAck`), pending-ack journal, documented lock order |
| `internal/gatedrive/store.go` | + `NewReservedDrive` (pre-launch drive record) and `attachLaunch` (persist handle) |
| `internal/gatedrive/driver.go` | `Start` restructured: reserve → retire predecessor → launch → confirm; `StartRequest` gains predecessor receipt fields |
| `internal/gatedrive/acknowledge.go` (new) | `Driver.Acknowledge` — terminal acknowledgement, scope closure |
| `internal/gatedrive/takeover.go` | resolves the scope's current slot; refuses reserved/pending states and stale explicit drive ids |
| `internal/gatedrive/run_waiting.go` | `FindScopeDriveIDs` doc updated (acknowledged predecessors excluded by owner-cleared rule — behavior mostly free) |
| `internal/app/gate_drive.go` | request/seam/operation for acknowledge; predecessor fields; `mapDriveFailure` ownership-kind mapping; next-action human messages |
| `internal/app/schema_registry.go` | + `gate.drive.acknowledge` row; start request schema change ride-along |
| `internal/cli/gate.go` | + `--predecessor-drive-id`/`--predecessor-owner-gen` on start; + `acknowledge` subcommand |
| `skills/docket-build-task/SKILL.md`, `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-caller-loop.md` (+ grep-derived siblings) and embedded mirrors | sequential-start receipt, final acknowledgement, BLOCKED mapping |

Lock order (documented in `scope.go` package comment, Task 2): **scope lock before drive lock** when a single logical transition needs both; sequential (release-between) acquisitions elsewhere; revalidate the target after acquiring each authority. `Handoff`/`Claim`/`Takeover` keep their existing sequential shape.

---

### Task 1: Typed ownership diagnostics reach the caller

**Files:**
- Modify: `internal/gatedrive/ownership.go` (new kinds)
- Modify: `internal/app/gate_drive.go` (`mapDriveFailure`, next-action messages)
- Test: `internal/gatedrive/ownership_test.go`, `internal/app/gate_drive_test.go`

**Interfaces:**
- Consumes: `gatedrive.AsOwnershipError`, `OwnershipErrorKind` (existing).
- Produces: exported kinds `ErrStalePredecessor OwnershipErrorKind = "stale-predecessor"`, `ErrPredecessorNotReusable OwnershipErrorKind = "predecessor-not-reusable"`, `ErrUnresolvedLaunchTransition OwnershipErrorKind = "unresolved-launch-transition"`, `ErrScopeBusy OwnershipErrorKind = "scope-busy"`; `mapDriveFailure` returning `(ResultInvalidInput, string(kind))` for every `OwnershipError`; unexported `ownershipNextAction(kind OwnershipErrorKind) string` used by `GateDriveResult.Message`. Later tasks return these kinds and rely on this mapping.

- [ ] **Step 1: Write the failing tests.** In `internal/app/gate_drive_test.go`, table-drive `mapDriveFailure` over an injected `*gatedrive.OwnershipError` for each kind the spec names — `scope-identity-mismatch`, `scope-capability-mismatch`, `scope-closed`, `scope-second-live-drive`, `scope-busy`, `handoff-outstanding`, `stale-predecessor`, `predecessor-not-reusable`, `unresolved-launch-transition` — asserting `(ResultInvalidInput, "<kind token>")`, and assert the reason NEVER contains the wrapped error's free text (wrap the kind in `fmt.Errorf("…%w…", ownershipErr(...))` with a sentinel string like `SECRET-ARGV` and assert absence). Assert a plain unrecognized error still maps to `invalid-request`. In `internal/gatedrive/ownership_test.go`, assert the four new kind constants exist with those exact spellings.
- [ ] **Step 2: Run to verify failure.** `go test ./internal/gatedrive/ ./internal/app/ -run 'Ownership|MapDriveFailure' -count=1` — expect compile failures on the missing constants, then assertion failures on the `invalid-request` collapse.
- [ ] **Step 3: Implement.** Add the four constants beside the existing scope kinds in `ownership.go` with doc comments quoting their spec role. In `mapDriveFailure`, before the store-error branch add:

```go
if oe, ok := gatedrive.AsOwnershipError(err); ok {
    return ResultInvalidInput, string(oe.Kind)
}
```

Add `ownershipNextAction` mapping each kind to a one-line valid-next-action message (spec "Diagnostics and caller integration"): e.g. `ErrScopeBusy` → "another start or transition owns this scope's slot; do not retry blindly", `ErrHandoffOutstanding` → "claim the outstanding handoff instead of starting or taking over", `ErrScopeClosed` → "scope authority was transferred or finished; stop and return BLOCKED", `ErrStalePredecessor` → "the presented predecessor is not the scope's current drive", `ErrPredecessorNotReusable` → "the predecessor has no durable PASSED/FAILED result to acknowledge", `ErrUnresolvedLaunchTransition` → "a prior launch transition is unresolved; recover via the parent, not a retry", `ErrScopeCapabilityMismatch` → "use the complete identity bundle from your dispatch prompt", `ErrScopeIdentityMismatch` likewise, `ErrScopeSecondDrive` → "the scope already holds a drive; a successor start must present the predecessor receipt". Populate `GateDriveResult.Message` from it wherever `mapDriveFailure` classified an ownership error (extend `mapDriveResult`'s failure branch to look the kind up again, or have `mapDriveFailure` return the message as a third value — pick one and use it consistently).
- [ ] **Step 4: Run to verify pass.** Same command; expect PASS.
- [ ] **Step 5: Mutation evidence.** Copy `gate_drive.go` aside; delete the new ownership branch; `go test ./internal/app/ -run MapDriveFailure -count=1` must redden with the `invalid-request` collapse specifically; restore from the copy.
- [ ] **Step 6: Commit.** `git add internal/gatedrive/ownership.go internal/gatedrive/ownership_test.go internal/app/gate_drive.go internal/app/gate_drive_test.go && git commit -m "feat(0405): ownership error kinds surface as typed gate-drive reasons"`

---

### Task 2: Scope schema v2 — the single-slot lifecycle in the store

**Files:**
- Modify: `internal/gatedrive/scope.go`
- Modify: `internal/gatedrive/takeover.go` and `internal/gatedrive/driver.go` (mechanical rename only, plus a temporary compatibility bind)
- Test: `internal/gatedrive/scope_test.go`

**Interfaces:**
- Consumes: `scopeCAS`, `writeAtomicJSON`, `ownershipErr`, Task 1 kinds.
- Produces (later tasks call exactly these):

```go
// scopeRecord v2 replaces BoundDriveID with the slot lifecycle:
CurrentDriveID    string `json:"current_drive_id,omitempty"`
CurrentDriveState string `json:"current_drive_state,omitempty"` // "" | scopeStateReserved | scopeStateLaunched
PriorDriveID      string `json:"prior_drive_id,omitempty"`
DriveCount        int    `json:"drive_count"`
PendingAckDriveID string `json:"pending_ack_drive_id,omitempty"`
PendingAckOwnerGen string `json:"pending_ack_owner_gen,omitempty"`
FinalAcked        bool   `json:"final_acked,omitempty"`

const scopeStateReserved = "reserved"
const scopeStateLaunched = "launched"

type predecessorReceipt struct{ DriveID, OwnerGen string } // both set, or both empty

func (s *Store) reserveScopeDrive(scopeID, childCapability, newDriveID string, receipt predecessorReceipt) error
func (s *Store) confirmScopeLaunch(scopeID, driveID string) error
func (s *Store) clearPendingAck(scopeID, predecessorID string) error
```

- [ ] **Step 1: Write the failing tests** in `scope_test.go`:
  - Round-trip: `PrepareScope` then `LoadScope` returns v2 zero-slot state (`CurrentDriveID == ""`, `DriveCount == 0`).
  - Fail-closed versioning: hand-write a `record.json` whose `schema_version` is `1` (the old shape with `bound_drive_id`); `LoadScope` returns `ErrUnknownSchema` — "do not silently reinterpret in-flight one-drive records as reusable scopes".
  - `reserveScopeDrive` empty scope: succeeds only with an empty receipt; a non-empty receipt on an empty scope is `ErrStalePredecessor`; after success `CurrentDriveID == newID`, `CurrentDriveState == scopeStateReserved`, `DriveCount == 1`, no pending ack.
  - `reserveScopeDrive` occupied scope: with empty receipt → `ErrScopeSecondDrive`; with a receipt naming a non-current drive → `ErrStalePredecessor`; while `CurrentDriveState == scopeStateReserved` → `ErrScopeBusy`; while a pending ack is journaled → `ErrUnresolvedLaunchTransition`; with the correct receipt against a `scopeStateLaunched` current drive → succeeds, setting `CurrentDriveID = newID`, state reserved, `PriorDriveID = old id`, `PendingAckDriveID/PendingAckOwnerGen = receipt`, `DriveCount++`.
  - Receipt shape: one field set without the other is rejected (`ErrStalePredecessor`) with no write.
  - Capability/closed: wrong or empty capability → `ErrScopeCapabilityMismatch`; closed scope → `ErrScopeClosed`; every rejection leaves the persisted bytes unchanged (read the file before/after).
  - `confirmScopeLaunch`: flips reserved→launched only for the matching `driveID`; a mismatched id or an already-launched state is `ErrUnresolvedLaunchTransition`/no-op-idempotent respectively (idempotent same-id confirm returns nil).
  - `clearPendingAck`: clears only a matching journal entry; mismatch → `ErrUnresolvedLaunchTransition`.
  - Serialization: two goroutines calling `reserveScopeDrive` with the same valid receipt — exactly one nil error, the loser gets `ErrScopeBusy` or `ErrStalePredecessor`, and the persisted record names exactly one winner (`scopeCAS` generation CAS is the mechanism; the test proves the outcome).
- [ ] **Step 2: Run to verify failure.** `go test ./internal/gatedrive/ -run TestScope -count=1` — compile errors on the new fields/functions.
- [ ] **Step 3: Implement.** Bump `scopeSchemaVersion = 2`. Replace `BoundDriveID` with the field set above (delete the old field; v1 records are refused by version before shape matters). Write `reserveScopeDrive`/`confirmScopeLaunch`/`clearPendingAck` as `scopeCAS` transitions whose mutate closures perform every check listed in Step 1 in that order (capability, closed, shape, slot state) and mutate only on full agreement. Extend the package comment with the lock-order and recovery-phase documentation the spec requires verbatim intent: *scope lock before drive lock inside one logical transition; Start, final acknowledgement, Handoff/Claim, and Takeover revalidate their target after acquiring authority; the pending-ack journal is the recoverable transition record between reservation and predecessor retirement.* Mechanical ride-alongs so the tree stays green: in `takeover.go` `resolveTakeoverDrive`, read `scope.CurrentDriveID` where it read `scope.BoundDriveID`; in `driver.go` `Start`, replace the `scope.BoundDriveID != ""` pre-check with `scope.CurrentDriveID != ""` (same `ErrScopeSecondDrive`) and replace the post-launch `bindScopeDrive` call with `reserveScopeDrive(scopeID, cap, id, predecessorReceipt{})` followed by `confirmScopeLaunch(scopeID, id)` (behavior-equivalent single-drive bind for now; Task 3 moves it pre-launch). Delete `bindScopeDrive`.
- [ ] **Step 4: Run to verify pass.** `go test ./internal/gatedrive/ -count=1` — the whole package, so the 0416 scoped-start tests and takeover tests prove the rename broke nothing.
- [ ] **Step 5: Mutation evidence** (each: mutate, focused `-count=1` run reddens for the stated reason, restore from a saved copy): (a) drop the `Closed` check in `reserveScopeDrive` → closed-scope test reds; (b) drop the `CurrentDriveState == scopeStateReserved` refusal → busy test reds; (c) accept a one-field receipt → shape test reds; (d) drop the schema-version comparison in `readStoredScope` → v1 fail-closed test reds.
- [ ] **Step 6: Commit.** `git add internal/gatedrive/scope.go internal/gatedrive/scope_test.go internal/gatedrive/takeover.go internal/gatedrive/driver.go && git commit -m "feat(0405): scope schema v2 — durable single-slot lifecycle with reservation and pending-ack journal"`

---

### Task 3: Reservation before launch for the first scoped start

**Files:**
- Modify: `internal/gatedrive/store.go` (`NewReservedDrive`, `attachLaunch`), `internal/gatedrive/driver.go` (`Start`)
- Test: `internal/gatedrive/driver_test.go`, `internal/gatedrive/driver_concurrency_test.go`

**Interfaces:**
- Consumes: Task 2 transitions.
- Produces:

```go
// NewReservedDrive persists a drive record BEFORE its first launch: RawRunDir,
// RawOwnership and LastOutcome empty. Same id-minting and privacy as NewDrive.
func (s *Store) NewReservedDrive(rec driveRecord) (id string, gen string, err error)
// attachLaunch persists the raw launch identity onto a reserved record under
// ownerCAS; refuses (ErrUnresolvedLaunchTransition) if a handle is already set.
func (s *Store) attachLaunch(id, ownerGen, rawRunDir, rawOwnership string) error
```

Scoped `Start` order (the shape Tasks 4–7 build on): pre-checks → fingerprint → `NewReservedDrive` → `reserveScopeDrive` → *(Task 4: retire predecessor)* → `proc.Launch` → `attachLaunch` → `confirmScopeLaunch` → `driveAndPersist`. Scopeless starts keep the existing launch-then-`NewDrive` path untouched.

- [ ] **Step 1: Write the failing tests:**
  - Ordering: a fake `ProcessSeam` whose `Launch` asserts (by loading the scope through the store) that the scope already holds this drive id in `scopeStateReserved` — proving reservation precedes launch. After `Start` returns, state is `scopeStateLaunched` and the drive record carries the launch handle.
  - Launch failure fails closed: `Launch` returns an error → `Start` returns a `DriveDoc` with `Outcome: HALTED, Cause: "launch-failed"` and nil error OR (decide and pin: return the typed command error and a HALTED persisted record — choose the persisted-HALTED + returned-error shape so no fabricated verdict document flows, matching "A malformed request or a launch failure is a command failure"); the scope still names the drive (`CurrentDriveID` unchanged, state reserved), the drive record is persisted `HALTED/"launch-failed"` with `OwnerGeneration` retained (a terminal-unconsumed record outer recovery can see), and a subsequent `Start` on the scope is refused `ErrScopeBusy` — **no automatic second launch**.
  - Persist-failure orphan control: `attachLaunch` forced to fail (inject via a store wrapper or a read-only record dir) → the freshly launched run is stopped (`Stop` called on the fake with that run dir) and the scope slot is NOT treated as empty (subsequent start refused).
  - Barrier race (spec verification 5, empty-scope half): two goroutines `Start` the same empty scope through a fake seam that counts `Launch` calls and gates on a channel barrier at reserve time — assert **exactly one `Launch` call total**, one nil-error winner, loser typed `ErrScopeBusy`/`ErrScopeSecondDrive`.
  - Regression preservation: the existing 0416 scoped-start identity tests still pass unmodified.
- [ ] **Step 2: Run to verify failure.** `go test ./internal/gatedrive/ -run 'TestStart|TestScopedStart|TestDriverConcurrency' -count=1`.
- [ ] **Step 3: Implement.** Add `NewReservedDrive` (factor the id-mint/dir-create/write body out of `NewDrive`; `NewDrive` keeps its exact behavior for scopeless drives) and `attachLaunch` (an `ownerCAS` mutate verifying owner + empty `RawRunDir`). Restructure `Start`'s `req.ScopeID != ""` branch to the pinned order. On `Launch` error, persist the HALTED record via `ownerCAS` (`LastOutcome: HALTED, LastCause: "launch-failed"`) before returning the command error. Keep the fail-fast unlocked pre-check block (it now only short-circuits; `reserveScopeDrive` is the authority — say so in the comment, replacing the `bindScopeDrive` sentence).
- [ ] **Step 4: Run to verify pass.** `go test ./internal/gatedrive/ -count=1`.
- [ ] **Step 5: Mutation evidence.** (a) Move the `reserveScopeDrive` call to after `proc.Launch` → the ordering assert inside the fake's `Launch` reds; (b) on launch error, additionally call `reserveScopeDrive`-releasing code you do NOT write (i.e. prove the busy-refusal test reds if you clear `CurrentDriveID` on the launch-error path); (c) drop the `attachLaunch` failure `Stop` → orphan-control test reds. Restore each from a saved copy.
- [ ] **Step 6: Commit.** `git add internal/gatedrive/store.go internal/gatedrive/driver.go internal/gatedrive/driver_test.go internal/gatedrive/driver_concurrency_test.go && git commit -m "feat(0405): scoped starts reserve the scope slot durably before any launch"`

---

### Task 4: Successor starts — the sequential handshake

**Files:**
- Modify: `internal/gatedrive/driver.go` (`StartRequest`, `Start`), `internal/gatedrive/store.go` (`retirePredecessor`)
- Test: `internal/gatedrive/driver_test.go`, `internal/gatedrive/driver_concurrency_test.go`

**Interfaces:**
- Consumes: Tasks 2–3.
- Produces:

```go
// StartRequest gains the explicit predecessor receipt (spec "Subsequent tests"):
// both required together for a successor start, both forbidden for an empty scope.
PredecessorDriveID string
PredecessorOwnerGen string

// retirePredecessor clears the predecessor's recovery authority under ownerCAS:
// verifies OwnerGeneration == ownerGen (ErrStalePredecessor on mismatch), a durable
// PASSED/FAILED LastOutcome (ErrPredecessorNotReusable otherwise), and no
// outstanding handoff (ErrHandoffOutstanding); then clears OwnerGeneration.
// The record — command, fingerprint, verdict, run dirs — is retained as history.
func (s *Store) retirePredecessor(id, ownerGen string) error
```

`Start` successor flow between reserve and launch: `retirePredecessor(receipt)` then `clearPendingAck` — the journaled two-phase "one logical transition"; on `retirePredecessor` failure after a won reservation, fail closed `ErrUnresolvedLaunchTransition` (reservation retained, nothing launched — parent recovery is the exit, per "Ambiguous launch or persistence failures fail closed").

- [ ] **Step 1: Reproduce the current defect (spec verification 1) as the RED test.** `TestScopedSequentialStarts`: real store + fake seams; `PrepareScope`; first start with the complete identity bundle → PASSED (the positive control proving identity is not the failure); drive the fake to a durable PASSED; second `Start` with the same bundle **plus** `PredecessorDriveID`/`PredecessorOwnerGen` from the first response. Today this fails compile (no fields) — and the sibling assertion, a second start *without* receipt fields on the pre-change code path, documents the historical `scope-second-live-drive` rejection. Expected end-state asserts: second start succeeds as a **new** drive id, own owner generation, own fingerprint; first record still readable with verdict intact and `OwnerGeneration == ""`.
- [ ] **Step 2: Extend with the full sequence test (spec verification 2).** Baseline PASSED → RED FAILED → GREEN PASSED as three drives under one scope, with a **real worktree edit between RED and GREEN** (write a file through the fake git seam's fingerprint input — change the fake `GitSeam`'s returned fingerprint between drives to model the edit; the integration-level real-git version lands in Task 9). Assert per drive: distinct ids, distinct fingerprints persisted, the fake seam's `Launch` count is exactly 3 (each command executed once), `DriveCount == 3`, `PriorDriveID` chains correctly.
- [ ] **Step 3: Rejection matrix (spec verifications 3–4, start half).** Table-driven: missing/wrong repo-dir, branch, worktree, change, task, phase, gate context, capability, predecessor id, predecessor generation → typed rejection, **zero `Launch` calls for the rejected request**, and the legitimate predecessor NOT consumed (its `OwnerGeneration` still set, a correct successor start still succeeds afterward). Predecessor states: WAITING (live) → `ErrPredecessorNotReusable`; HALTED → `ErrPredecessorNotReusable`; outstanding handoff → `ErrHandoffOutstanding`; closed scope → `ErrScopeClosed`; transferred owner (takeover superseded the child's generation) → `ErrStalePredecessor`; receipt naming an already-acknowledged earlier drive → `ErrStalePredecessor`.
- [ ] **Step 4: Successor barrier race (spec verification 5).** Two goroutines present the SAME valid receipt; barrier inside `reserveScopeDrive`'s CAS window via the store (gate on the fake seam's `Launch`): exactly one `Launch`, one winner; the loser's typed error is `ErrScopeBusy` or `ErrStalePredecessor`; the predecessor is retired exactly once.
- [ ] **Step 5: Run to verify failure, then implement.** `go test ./internal/gatedrive/ -run 'TestScopedSequential|TestSuccessor' -count=1` (compile failures first). Implement: add the two `StartRequest` fields; in `Start`'s scope branch, derive `receipt := predecessorReceipt{req.PredecessorDriveID, req.PredecessorOwnerGen}`; fail-fast unlocked pre-validation of the predecessor record (load, check current/terminal/gen/handoff) for cheap typed rejections that consume nothing; then `NewReservedDrive` → `reserveScopeDrive(..., receipt)` → for a non-empty receipt `retirePredecessor` + `clearPendingAck` → `Launch` → `attachLaunch` → `confirmScopeLaunch` → `driveAndPersist`. Write `retirePredecessor` per the interface block. Delete the now-false half of the `ErrScopeSecondDrive` doc comment ("a second is refused rather than overwriting the first" still holds for receipt-less starts — reword to say a successor presents the receipt instead; `shared-resource-keeps-first-owner-assumptions`: sweep `scope.go`/`ownership.go`/`driver.go` comments for single-drive-per-scope prose and update each).
- [ ] **Step 6: Run to verify pass.** `go test ./internal/gatedrive/ -count=1`.
- [ ] **Step 7: Mutation evidence.** (a) In `retirePredecessor`, accept a HALTED predecessor → matrix test reds `predecessor-not-reusable` row; (b) skip the `OwnerGeneration == ownerGen` check → superseded-owner row reds; (c) make `reserveScopeDrive` accept a receipt naming `PriorDriveID` → acknowledged-predecessor row reds; (d) drop the launch-count assert's premise by relaunching the predecessor instead of launching a new drive (point the launch at the predecessor's record) → distinct-drive asserts red ("a new test is a new drive, never a relaunch"). Restore each from saved copies; run each probe with `-count=1`.
- [ ] **Step 8: Commit.** `git add internal/gatedrive/driver.go internal/gatedrive/store.go internal/gatedrive/driver_test.go internal/gatedrive/driver_concurrency_test.go internal/gatedrive/scope.go internal/gatedrive/ownership.go && git commit -m "feat(0405): successor starts acknowledge the predecessor receipt and reuse the scope slot"`

---

### Task 5: Terminal acknowledgement — `Driver.Acknowledge`

**Files:**
- Create: `internal/gatedrive/acknowledge.go`
- Test: `internal/gatedrive/acknowledge_test.go` (new)

**Interfaces:**
- Consumes: `retirePredecessor`, `scopeCAS`, Task 1 kinds.
- Produces (Task 7's app seam and Task 8's CLI call exactly this):

```go
// Acknowledge consumes the scope's final drive result and closes the scope
// (spec "Finishing the task"). It verifies the child capability, that driveID is
// the scope's current LAUNCHED drive, a durable PASSED/FAILED outcome, no
// outstanding handoff, and current ownership (ownerGen); then clears the drive's
// recovery authority and closes the scope with FinalAcked. A byte-identical
// repeat after success is an idempotent no-op returning the recorded document.
// Wrong credentials, a different drive, live/HALTED outcomes, or pending
// transitions are typed rejections that write nothing.
func (d *Driver) Acknowledge(scopeID, childCapability, driveID, ownerGen string) (DriveDoc, error)
```

- [ ] **Step 1: Write the failing tests** (spec verification 9 + acknowledgement half of 4):
  - Happy path: sequence of two drives, second durably PASSED; `Acknowledge` succeeds; scope `Closed && FinalAcked`, `CurrentDriveID` retained for history; drive record retains id, verdict, fingerprint, `RawRunDir` (evidence retention — acknowledgement must not delete execution evidence) with `OwnerGeneration` cleared; `FindScopeDriveIDs` for the change/context returns **zero** candidates (no stale recovery candidates after normal completion). Works for a FAILED final result too (a FAILED ack is valid; task success criteria are the caller's concern).
  - Idempotent repeat: same four arguments again → nil error, same recorded outcome document, no state change (compare persisted bytes).
  - Refusals (each writes nothing, asserted by before/after byte compare): WAITING (live) drive → `ErrPredecessorNotReusable`; HALTED → `ErrPredecessorNotReusable`; unrelated drive id → `ErrStalePredecessor`; superseded generation → `ErrStalePredecessor`; outstanding handoff → `ErrHandoffOutstanding`; wrong capability → `ErrScopeCapabilityMismatch`; scope closed by claim/takeover (not by final ack) → `ErrScopeClosed`; reserved-state slot → `ErrUnresolvedLaunchTransition`.
  - Post-ack successor refused: a `Start` presenting the acknowledged final drive as predecessor → `ErrScopeClosed`.
- [ ] **Step 2: Run to verify failure.** `go test ./internal/gatedrive/ -run TestAcknowledge -count=1`.
- [ ] **Step 3: Implement** `acknowledge.go`: load scope (unusable → typed store error passthrough); idempotency fast-path (`Closed && FinalAcked && CurrentDriveID == driveID` and the drive record's owner already cleared with matching terminal outcome → return `recordedDoc`-shaped document, nil); verify capability/closed/slot; `retirePredecessor(driveID, ownerGen)`; then `scopeCAS` setting `Closed = true, FinalAcked = true` re-verifying `CurrentDriveID == driveID` inside the mutate (revalidate after acquiring authority). Return the final document built from the authoritative record.
- [ ] **Step 4: Run to verify pass**, whole package: `go test ./internal/gatedrive/ -count=1`.
- [ ] **Step 5: Mutation evidence.** (a) Skip the terminal-outcome check → live/HALTED refusal tests red; (b) skip the `CurrentDriveID` re-check inside the closing `scopeCAS` mutate → wrong-drive refusal reds; (c) make repeat-ack re-run `retirePredecessor` unconditionally → idempotency byte-compare reds. Restore from copies.
- [ ] **Step 6: Commit.** `git add internal/gatedrive/acknowledge.go internal/gatedrive/acknowledge_test.go && git commit -m "feat(0405): terminal acknowledgement consumes the final result and closes the scope"`

---

### Task 6: Recovery interplay — takeover, handoff/claim, enumeration

**Files:**
- Modify: `internal/gatedrive/takeover.go`, `internal/gatedrive/run_waiting.go` (doc comment), `internal/gatedrive/driver.go` (`Claim` — no behavior change expected; verify)
- Test: `internal/gatedrive/takeover_test.go`, `internal/gatedrive/handoff_test.go`

**Interfaces:**
- Consumes: everything above.
- Produces: `resolveTakeoverDrive` resolving only the scope's current drive; a takeover refusal cause `"unresolved-launch-transition"` for a reserved/pending slot; explicit-drive-id validation against the scope's association.

- [ ] **Step 1: Write the failing tests** (spec verification 7 + remaining parts of 4/5):
  - Takeover after a completed predecessor: sequence PASSED → successor WAITING; child "returns without handoff"; `Takeover(scopeID, parentCap, "")` resolves the **successor** (current drive), never the acknowledged predecessor; the superseded child generation can no longer advance (`Advance` → HALTED `owner-superseded`); scope closed.
  - Takeover of an unacknowledged last terminal result: successor durably FAILED, not acknowledged, child gone → takeover resolves it (terminal-unconsumed).
  - Explicit stale drive id: `Takeover(scopeID, parentCap, <acknowledged predecessor id>)` → HALTED; an explicitly supplied old drive id cannot bypass the current-scope association (implement: when `scope.CurrentDriveID != ""` and an explicit `driveID` differs from it, halt `"stale-predecessor"` before loading the record).
  - Reserved slot / pending ack: takeover during `scopeStateReserved` or with a pending-ack journal → HALTED `"unresolved-launch-transition"` — a reservation is not a quiescent result.
  - Race successor-start vs takeover (spec verification 5): barrier so takeover's scope claim and a successor start's reservation contend — exactly one coherent winner: either the start wins (takeover halts `scope-closed`/`scope-busy` and no generation is superseded) or the takeover wins (start rejected `ErrScopeClosed`); assert no state where both proceeded (drive count and scope closed flags are consistent, at most the winner's `Launch` happened). Same shape racing final acknowledgement vs takeover.
  - WAITING → handoff → claim mid-sequence: after one completed predecessor, successor WAITING; `Handoff` then `Claim`; claim still closes the scope (existing `Claim` behavior — "Claim closes the child's scope as today; later worker dispatches get fresh scopes"); a further successor start under the closed scope → `ErrScopeClosed`; old child generation dead.
  - Outer discovery with history (spec verification 7 tail): `FindScopeDriveIDs` over a change with two acknowledged predecessors + one current WAITING drive returns exactly the current one (owner-cleared exclusion already does this — the test pins it against regression).
- [ ] **Step 2: Run to verify failure.** `go test ./internal/gatedrive/ -run 'TestTakeover|TestHandoff|TestFindScope' -count=1` — the stale-explicit-id and reserved-slot refusals fail (not yet implemented).
- [ ] **Step 3: Implement.** In `Takeover` before `resolveTakeoverDrive`: if `scope.CurrentDriveState == scopeStateReserved || scope.PendingAckDriveID != ""` → `haltDoc(..., string(ErrUnresolvedLaunchTransition))`. In `resolveTakeoverDrive`: when an explicit `driveID` is given and `scope.CurrentDriveID != "" && driveID != scope.CurrentDriveID` → return cause `string(ErrStalePredecessor)`. Update `FindScopeDriveIDs`'s doc comment to name the sequential-scope reading ("acknowledged predecessors are terminal-and-consumed — owner cleared by successor or final acknowledgement — and excluded"). Update `takeover.go`'s package/step comments that still say a scope "binds one live drive" to the slot vocabulary.
- [ ] **Step 4: Run to verify pass.** `go test ./internal/gatedrive/ -count=1`.
- [ ] **Step 5: Mutation evidence.** (a) Remove the reserved-slot halt → reserved-takeover test reds; (b) remove the explicit-id/current-drive comparison → stale-explicit-id test reds; (c) in `FindScopeDriveIDs`, include owner-cleared terminal records → historical-ambiguity test reds (two candidates). Restore from copies.
- [ ] **Step 6: Commit.** `git add internal/gatedrive/takeover.go internal/gatedrive/takeover_test.go internal/gatedrive/handoff_test.go internal/gatedrive/run_waiting.go internal/gatedrive/driver.go && git commit -m "feat(0405): takeover and enumeration resolve only the scope's current work"`

---

### Task 7: Fault injection and restart recovery

**Files:**
- Test: `internal/gatedrive/driver_faults_test.go` (new; fixtures may add small seam-wrapper types here)

**Interfaces:**
- Consumes: everything above; a `failingStore` pattern is NOT available — inject at the seams the driver already takes (`ProcessSeam`, `GitSeam`) and at the filesystem (read-only dirs, truncated `record.json`) for store faults.

- [ ] **Step 1: Write the tests** (spec verification 8). For each injection point — (a) reservation write fails (scope dir made read-only around the successor `reserveScopeDrive`), (b) `Launch` errors, (c) `attachLaunch` persistence fails, (d) predecessor retirement interrupted (kill the flow between `reserveScopeDrive` and `retirePredecessor` by asserting on the journaled intermediate state directly: hand-drive `reserveScopeDrive` then STOP, modeling a crash), (e) final acknowledgement interrupted between `retirePredecessor` and the closing `scopeCAS` — then **restart**: build a fresh `Driver` over a fresh `OpenStore` of the same root and assert: pre-reservation failures leave scope and predecessor byte-unchanged and a correct retry succeeds; post-reservation interruptions leave the pending-ack journal readable, every start/ack/takeover on the scope fails typed (`ErrScopeBusy`/`ErrUnresolvedLaunchTransition`) with **zero further `Launch` calls** (count via the fake seam) — never a duplicate launch, never a false "no work" (`FindScopeDriveIDs`/takeover do not report the scope empty); for (e), the drive's owner is already cleared, the scope is still open — a repeat `Acknowledge` with the same arguments completes the close (idempotent completion is the recovery), pin that.
- [ ] **Step 2: Run to verify** — these are new tests over existing behavior; where a case reveals the recovery hole (the likely one: repeat-`Acknowledge` after (e) hits `ErrStalePredecessor` because the owner is cleared but the scope is open), fix `Acknowledge`'s verification to accept an owner-already-cleared current drive with a matching terminal outcome as the resumable half of its own transition. `go test ./internal/gatedrive/ -run TestFault -count=1`.
- [ ] **Step 3: Mutation evidence.** In the restart path, treat a pending-ack scope as empty (clear the journal on load) → duplicate-launch/false-no-work asserts red. `probe-error-is-not-clean-absence` applies: anywhere a probe error and clean absence would share a branch, assert the injected-error case retains state.
- [ ] **Step 4: Commit.** `git add internal/gatedrive/driver_faults_test.go internal/gatedrive/acknowledge.go && git commit -m "test(0405): fault-injection and restart coverage for the sequential scope transitions"`

---

### Task 8: App seam, CLI, catalog and schema

**Files:**
- Modify: `internal/app/gate_drive.go`, `internal/app/schema_registry.go`, `internal/cli/gate.go`
- Test: `internal/app/gate_drive_test.go`, `cmd/docket/gate_cli_test.go` (follow the existing CLI-test pattern there), plus the existing catalog/schema parity guards

**Interfaces:**
- Consumes: `Driver.Acknowledge`, `StartRequest` predecessor fields.
- Produces:

```go
const OperationGateDriveAcknowledge = "gate.drive.acknowledge"
// GateDriveStartRequest gains:
PredecessorDriveID  string
PredecessorOwnerGen string
// driveEngine gains:
Acknowledge(scopeID, childCap, driveID, ownerGen string) (gatedrive.DriveDoc, error)
// GateDriveService gains:
func (s *GateDriveService) Acknowledge(scopeID, childCap, driveID, ownerGen string) GateDriveResult
```

CLI: `docket gate drive start` gains `--predecessor-drive-id` and `--predecessor-owner-gen` (recorded doc: "successor receipt — both together, from the previous drive's captured response; forbidden on a scope's first start"). New subcommand `docket gate drive acknowledge --scope-id <id> --child-cap <token> --drive-id <id> --owner-gen <gen> [--repo-dir <dir>] --json`, annotation `capability("gate.drive.acknowledge", EffectLocalWrite)`, composed over `NewCommandlessGateDriveService` (it needs no command or budget).

- [ ] **Step 1: Write the failing tests.** App seam (fake engine, per the existing pattern): `Start` forwards the two predecessor fields verbatim into `gatedrive.StartRequest`; `Acknowledge` forwards all four arguments and maps a produced document to `ResultApplied` + `Drive`, an `OwnershipError` to the Task-1 typed reason + next-action message. CLI: `gate drive acknowledge` requires all four flags (missing → usage error naming the flag); `gate drive start` accepts the pair only together (one without the other → error before any service construction — a rejected request must not consume the predecessor); `--json` output for acknowledge carries the shared document; human text omits generations (extend the existing human-output tests for the new op). Schema/catalog: add the registry row and rely on the existing parity guards — run them and satisfy whatever they demand (`go test ./internal/app/ -run 'Schema|Catalog' -count=1` to discover the exact guard names; the correspondence guard runs both directions — if it only iterates one way for operations, extend per its own pattern, not a new mechanism).
- [ ] **Step 2: Run to verify failure**, implement, run to verify pass. `go test ./internal/app/ ./internal/cli/ ./cmd/... -count=1`. Implementation notes: the flag-pair validation lives in the start `RunE` before owner routing; `Acknowledge`'s `RunE` composes `NewCommandlessGateDriveService` exactly as `advance` does and calls the seam. Registry row: `{ID: "gate.drive.acknowledge", Request: nil, Result: GateDriveResult{}}` with the comment naming `GateDriveService.Acknowledge` (`Request: nil` matches the sibling non-struct-request ops; the four scalars arrive as flags).
- [ ] **Step 3: Mutation evidence.** Drop the both-or-neither flag validation → CLI pair test reds; drop `Acknowledge` from the annotation/registry → parity guard reds (prove the guard actually fires by running it with the row removed, then restore).
- [ ] **Step 4: Commit.** `git add internal/app/gate_drive.go internal/app/gate_drive_test.go internal/app/schema_registry.go internal/cli/gate.go cmd/docket/gate_cli_test.go && git commit -m "feat(0405): gate.drive.acknowledge operation and successor-receipt start flags"`

---

### Task 9: Concurrent scopes in linked worktrees — real-git integration

**Files:**
- Test: `internal/gatedrive/integration_sequence_test.go` (new; follow `integration_test.go`'s real-git fixture helpers)

- [ ] **Step 1: Write the tests** (spec verifications 2-real-git and 6):
  - Real-git sequence: one repo, one linked worktree; scope; baseline PASSED (`/bin/sh -c 'exit 0'`-class fixture command via the real process service if `integration_test.go` already composes it — otherwise the fake seam with a real `GitSeam`), RED FAILED, real `git`-visible file edit between RED and GREEN, GREEN PASSED; per-drive fingerprints differ across the edit and persist independently.
  - Two scopes, two linked worktrees, ONE shared git common dir, run concurrently with distinguishable commands and opposite verdicts (one exits 0, one exits 1 — name the marker in each command's argv, not a shell comment: `exec-optimization-erases-the-process-marker`); each parent's takeover/enumeration resolves only its own scope's current drive. Repeat the resolution assert across: two tasks of one change, two different changes, and with acknowledged historical drives present in the store.
  - Credential/id theft: scope A's child capability presented on scope B's start → `ErrScopeCapabilityMismatch`; scope A's old drive id presented as scope B's takeover target → HALTED (identity mismatch / stale) — cross-scope credentials or explicit old drive ids cannot steal work.
- [ ] **Step 2: Run** `go test ./internal/gatedrive/ -run TestIntegrationSequence -count=1 -race` (the suite runs `-race` globally; run it here explicitly during development). Fix what it finds.
- [ ] **Step 3: Commit.** `git add internal/gatedrive/integration_sequence_test.go && git commit -m "test(0405): concurrent linked-worktree scope sequences over real git"`

---

### Task 10: Caller contracts, references, embedded mirrors

**Files:**
- Modify (derived, not assumed): run `grep -rln 'gate\.drive\.start\|prepare-scope\|scope id and child capability\|scope-second' skills/ internal/assets/embedded/tree/ docs/ --include='*.md'` and sort hits into maintained-source vs point-in-time records (archived changes/specs/results/plans and Accepted ADRs are historical — do NOT rewrite). Expected maintained set: `skills/docket-build-task/SKILL.md`, `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-caller-loop.md`, possibly `skills/docket-build/references/gate-execution.md` and `skills/docket-implement-next/SKILL.md`, plus each file's mirror under `internal/assets/embedded/tree/`.
- Test: existing embedded-mirror parity guard and any skill-body sentinel tests (`go test ./... -run 'Embedded|Skill' -count=1` to locate them)

- [ ] **Step 1: Update `skills/docket-build-task/SKILL.md`** ("The cycle" section): after the existing start invocation sentence, specify — *the task's FIRST test omits the predecessor flags; every LATER test passes `--predecessor-drive-id <previous drive id> --predecessor-owner-gen <previous generation>` from the previous captured `--json` response, acknowledging exactly the PASSED or FAILED result you received (a WAITING drive is never a predecessor — hand off instead); when no further test execution is needed and you are ready to return, perform the `gate.drive.acknowledge` operation with `--scope-id <id> --child-cap <token> --drive-id <final drive id> --owner-gen <gen> --json` before returning — a FAILED final result still gets acknowledged and does not authorize a success report; a failed acknowledgement returns `BLOCKED` with the typed cause, never `COMPLETE`* (the mapping named inside the clause — `prohibition-needs-a-return-value`). Keep the drive id + verdict in `VERIFICATION`/`NOTES` evidence — acknowledgement retires recovery authority, not your report's evidence.
- [ ] **Step 2: Update `skills/docket-build/references/gate-caller-loop.md`**: `start` row gains the successor-receipt sentence; new `acknowledge` op table row ("consume the scope's final PASSED/FAILED result and close the task scope; idempotent on exact repeat; effects: local-write"); JSON-capture table row for `acknowledge` (nothing new to capture — the confirmation document; the receipt fields for the NEXT start come from `start`'s existing captured row — extend that row's wording: "the drive identifier **and** the ownership generation — also the successor receipt for this task's next start"). Update `skills/docket-build/SKILL.md` wherever grep finds start/scope wording (controller-side: workers now run several drives per scope; the controller's claim of a WAITING handoff is unchanged).
- [ ] **Step 3: Regenerate/copy the embedded mirrors** under `internal/assets/embedded/tree/` for every maintained file touched (check `internal/assets/embedded.go` and any sync script for the sanctioned mechanism before hand-copying), then run the parity/sentinel guards: `go test ./internal/assets/... ./internal/repoguard/... -count=1`. If a sentinel greps old wording (`restatement-accumulates-its-own-guards`), update the guard together with the copy in this same task.
- [ ] **Step 4: Mutation evidence for the doc guards you touched:** re-break one mirror byte → parity guard reds; restore.
- [ ] **Step 5: Commit.** `git add skills/ internal/assets/ <any guard files> && git commit -m "docs(0405): sequential-drive receipt and terminal acknowledgement in the worker and caller contracts"`

---

### Task 11: ADR — sequential scope lifecycle and the acknowledgement boundary

> **Note for the executor/orchestrator:** per repo convention the ADR is recorded in the review phase via the `docket-adr` agent (it assigns the number, updates the index, commits on the `docket` branch, and returns the number for the change's `adrs:` relation). This task marks the requirement; do NOT hand-author the ADR file or number here, and do not restate 0416's already-shipped identity fix as a new ADR.

- [ ] **Step 1:** During review, dispatch `docket-adr` with Context/Decision/Consequences: a recovery scope is one parent/child dispatch, not one test execution; it serializes a sequence of drives through a durable pre-launch reservation and an explicit predecessor-acknowledgement receipt; final results are consumed by a cataloged terminal acknowledgement that closes the scope; relates to ADR-0107 and preserves its one-live-drive rule and event-authorized direct-parent takeover (the "one live drive" now reads "at most one current execution or launch reservation per scope").

---

## Verification-coverage map (spec "Verification and acceptance" 1–10)

1. Second-start reproduction + positive first-start control → Task 4 Step 1.
2. Baseline/RED/GREEN distinct drives, real edit, one execution each → Task 4 Step 2 (fake seams) + Task 9 Step 1 (real git).
3. Missing/wrong identity/context/capability/receipt launches nothing, predecessor unconsumed → Task 4 Step 3, Task 8 Step 1 (CLI pair).
4. Live/HALTED/handoff/closed/transferred rejections; ack refuses live/HALTED/unrelated/superseded → Task 4 Step 3, Task 5 Step 1, Task 6 Step 1.
5. Barrier races prove exactly one launch and one coherent ownership transition → Task 3 (empty-scope), Task 4 Step 4 (successor), Task 6 Step 1 (vs handoff/takeover/ack).
6. Two scopes, linked worktrees, shared common dir, theft attempts → Task 9.
7. WAITING→handoff→claim and takeover after a completed predecessor; discovery unambiguous → Task 6.
8. Interruption/persistence injection + restart; no duplicate launch or false no-work → Task 7 (+ Task 3 launch/persist failures).
9. Final ack, idempotent repeat, evidence retention, closed-scope rejection, zero stale candidates → Task 5.
10. Typed JSON + human diagnostics, redaction, unsupported persisted versions, catalog/schema parity, maintained/embedded agreement → Task 1 (mapping/redaction), Task 2 (schema fail-closed), Task 8 (parity), Task 10 (mirrors).

Mutation evidence is embedded per task; every probe uses `-count=1` and restores from a saved copy. The controller's whole-suite gate (`build.test_command`, from source) closes the change; read its budget report.

## Self-Review

- **Spec coverage:** every spec section maps to tasks (Decision/scope → 2–5; Invariants → global constraints + per-task asserts; First/Subsequent/Finishing/Waiting → 3/4/5/6; Durable state and concurrency → 2, 3, 7 (lock order documented in Task 2, revalidation pinned in Tasks 5–6); Diagnostics and caller integration → 1, 8, 10; Verification 1–10 → map above; ADR → 11). Deliberately not built, per "Alternatives and non-goals": per-test scopes, worker-minted authority, unfenced BoundDriveID replacement, gate-history GC, 412's yield behavior, run-gate attribution.
- **Placeholder scan:** the only deferrals are decisions explicitly delegated to a named existing pattern (fixture helpers in `integration_test.go`, mirror-sync mechanism in `internal/assets`) plus the ADR number (assigned by `docket-adr` at review) — flagged, not silent.
- **Type consistency:** `predecessorReceipt`, `reserveScopeDrive`/`confirmScopeLaunch`/`clearPendingAck`/`retirePredecessor`/`NewReservedDrive`/`attachLaunch`, `Acknowledge(scopeID, childCapability, driveID, ownerGen)`, `PredecessorDriveID`/`PredecessorOwnerGen`, and the four `OwnershipErrorKind` spellings are used identically across Tasks 1–8.
