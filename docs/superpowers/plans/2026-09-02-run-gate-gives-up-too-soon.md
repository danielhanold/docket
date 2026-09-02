<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0359 — Run gate gives up too soon](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0359-run-gate-gives-up-too-soon.md)**
<!-- docket:backlink:end -->
# Run Gate Gives Up Too Soon — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. On docket's own path this plan is executed by the `docket-build` role, one `### Task N` heading per worker.

**Goal:** Make the implement-next run gate continue a healthy tracked run instead of prematurely stopping it: every implement-next test runs through the native gate driver, ownership transfers deterministically upward, parents recover a drive after their direct child returns, and the outer gate emits a nonterminal `gate-continue` that keeps the same key and spends no retry.

**Architecture:** Extend `internal/gatedrive` (change 0342's driver) with durable recovery *scopes* — one per parent/child dispatch boundary, carrying two separately-minted opaque capabilities (child vs parent) — and an event-authorized `Takeover` transition that atomically invalidates the child's owner generation. Extend the run-gate facade (`internal/app/rungate_*.go`) so `gate-before` prepares the outer scope and supports explicit resume attribution, and `gate-verdict` performs the outer takeover, synthesizes a normal single-use handoff, lets the *unchanged* `RunVerify` run-waiting predicate validate it, and reports `gate-continue <key> run-waiting <change-id> <continuation-id> <phase>`. Rewrite the build skills so every test-intent command routes through the driver by workflow role (no duration prediction, no command-spelling list) and the first task-level `WAITING` always hands off to the controller. Regenerate the authored-to-installed dispatch surfaces from `cursor-rules/run-gate.md`.

**Tech Stack:** Go (stdlib only, matching `internal/gatedrive`'s existing patterns: flock + generation CAS, atomic JSON writes, sha256 capability hashes), Cobra CLI adapters, markdown skill contracts, `internal/repoguard` Go-native structural guards.

**Spec:** `docs/superpowers/specs/2026-08-28-run-gate-gives-up-too-soon-design.md` (synchronized on the `docket` metadata branch). The change file's 2026-09-02 reconcile entry carries a binding scope decision: the four-harness real-world acceptance probes are a **human pre-merge verification, not built here** — see Task 14. Never fabricate harness-probe evidence.

## Global Constraints

- The production observation slice stays exactly **30 seconds** (`productionSlice` in `internal/gatedrive/driver.go`); the overall deadline stays `gate_observation_budget` with its **30-minute default**. Neither is changed by any task.
- **No timers, heartbeats, log-activity checks, claim-age or process-name liveness guesses anywhere.** Parent takeover is authorized by the direct child's dispatch-return event (a workflow fact the caller asserts by *when* it calls) plus capability + identity proof — never by elapsed time.
- `FAILED` means the test itself completed red — nothing else is ever converted to `FAILED`. Every uncertainty is `HALTED`, fail-closed.
- **No command-spelling allowlists and no duration prediction** anywhere: test intent comes from the workflow role; a guard keys on syntactic shape or docket's own closed identifiers, never on `go test`/`pytest`-style spellings (repo rule + spec §1).
- Capabilities and owner generations never appear in human text, logs, drive prose, or verdict output. They travel only in the protocol JSON documents and the private 0600 records.
- Migration is **atomic**: all tasks land on this one branch/PR; no task may leave the tree describing both direct test execution and driver execution as supported workflow paths, or mapping a tracked live drive to both continuation and retry. Task order below follows the spec's 7-step migration.
- `RunVerify` keeps its durable-postcondition model — **no generic liveness branch is added to it**. The facade synthesizes a normal single-use handoff so the *existing* run-waiting predicate validates.
- Schema changes bump the persisted schema version and fail closed on unknown versions (drive records: v1→v2; gate records: v1→v2). An in-flight pre-upgrade record halting after merge is correct fail-closed behavior.
- Tests use injected time and process seams; no test sleeps for production durations (the existing `Driver.slice/pollInterval/sleep` test overrides and `Clock` seam).
- Run the configured whole suite at the build gate (`go run ./cmd/docket development test` via the driver, per docket-build's own gate policy) and act on every `SERIAL CONFIRMED OVER BUDGET:` line.

## File Map

| Area | Files |
|---|---|
| Recovery scopes + takeover | Create `internal/gatedrive/scope.go`, `internal/gatedrive/scope_test.go`, `internal/gatedrive/takeover.go`, `internal/gatedrive/takeover_test.go`; modify `internal/gatedrive/drive.go` (schema v2), `internal/gatedrive/driver.go` (Start binding), `internal/gatedrive/run_waiting.go` (continuation handle) |
| App seam + CLI | Modify `internal/app/gate_drive.go`, `internal/cli/gate.go`, `internal/cli/install.go`, `internal/cli/capability_test.go`, `internal/cli/capability_production_test.go` |
| Run gate | Modify `internal/app/rungate_before.go`, `internal/app/rungate_store.go`, `internal/app/rungate_verdict.go`, `internal/cli/run.go`; create `internal/app/rungate_continuation.go` (+ tests) |
| Skills | Modify `skills/docket-build-task/SKILL.md`, `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-caller-loop.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-build/references/gate-execution.md` |
| Dispatch surfaces | Modify `cursor-rules/run-gate.md`; regenerate `internal/assets/embedded/**` (via `go generate ./internal/assets`) and the repo's installed surfaces (`CLAUDE.md`, harness rule files) |
| Guards | Create `internal/repoguard/testexec_boundary_test.go` |
| ADR prep | Create `docs/superpowers/plans/2026-09-02-adr-supersede-0098-draft.md` |

---

### Task 1: Recovery-scope persistence with separated parent/child capabilities

**Build profile:** premium

**Files:**
- Create: `internal/gatedrive/scope.go`
- Create: `internal/gatedrive/scope_test.go`
- Modify: `internal/gatedrive/drive.go` (driveRecord schema v2: `ScopeID`, `GateContextHash`; bump `driveSchemaVersion` to 2)

**Interfaces:**
- Consumes: `Store` internals from `internal/gatedrive/store.go` — `writeAtomicJSON`, `ensurePrivateDir`, `acquireExclusiveLock`, `validateID`, `randomToken`, `StoreError` kinds (all package-level; reuse, do not reimplement).
- Produces (later tasks rely on these exact names):

```go
// ScopeRequest identifies one parent/child dispatch boundary. GateContext is the
// RAW outer child-context token linking nested drives to the outer gate (may be
// empty for the outer scope itself); it is stored only as a sha256 hash.
type ScopeRequest struct {
	RepoIdentity string
	ChangeID     string // may be "" for a fresh outer scope; binds once later
	TaskID       string
	Phase        string
	Branch       string
	Worktree     string
	GateContext  string
}

// ScopeGrant returns the scope locator and the two SEPARATE opaque capabilities.
// ChildCapability goes to the dispatched child; ParentCapability is retained by
// the preparing parent and never exposed to the child.
type ScopeGrant struct {
	ScopeID          string
	ChildCapability  string
	ParentCapability string
}

func (s *Store) PrepareScope(req ScopeRequest) (ScopeGrant, error)
func (s *Store) LoadScope(id string) (scopeRecord, error)           // package-private consumer surface
func (s *Store) bindScopeDrive(scopeID, childCapability, driveID string) error
func (s *Store) closeScope(scopeID string) error
func (s *Store) bindScopeChange(scopeID, changeID string) error      // outer scope bind-once
func capHash(capability string) string                               // sha256 lowercase hex
```

- Scope records live at `<git-common-dir>/docket/gate-scopes/v1/<opaque-scope-id>/record.json`, same 0700/0600 + flock + physical-generation CAS discipline as drives (mirror the `storedRecord` envelope with its own `scopeSchemaVersion = 1`).

- [ ] **Step 1: Write failing tests in `scope_test.go`** (table-driven; use `t.TempDir()` as the common dir, as `store_test.go` does):

```go
// TestPrepareScopeMintsSeparatedCapabilities: PrepareScope returns a ScopeID,
// ChildCapability and ParentCapability that are all non-empty, pairwise distinct,
// and 32 lowercase hex chars; the persisted record stores ONLY sha256 hashes of
// the two capabilities (read the record.json bytes and assert neither raw token
// appears anywhere in them) plus a hash — never the raw value — of GateContext.
// TestScopeBindOnce: bindScopeDrive succeeds once with the exact child
// capability; a second bind of a different drive id fails with a typed error
// (second-live-drive); a bind with a wrong/empty capability fails
// (missing-capability); a bind on a closed scope fails.
// TestScopeIdentityFailClosed: LoadScope on an unknown schema version or corrupt
// record returns ErrUnknownSchema/ErrCorruptRecord (reuse StoreError kinds);
// a traversal-shaped scope id is rejected before any path is built.
// TestBindScopeChangeOnce: bindScopeChange sets an empty ChangeID exactly once;
// rebinding to a different change fails closed; rebinding same id is a no-op.
// TestScopeCASConcurrent (race): 8 goroutines racing bindScopeDrive on one scope
// grant exactly one winner; losers get typed rejections; run under -race.
```

Define the record in `scope.go`:

```go
const scopeSchemaVersion = 1

type scopeRecord struct {
	SchemaVersion   int    `json:"schema_version"`
	RepoIdentity    string `json:"repo_identity"`
	ChangeID        string `json:"change_id"`
	TaskID          string `json:"task_id"`
	Phase           string `json:"phase"`
	Branch          string `json:"branch"`
	Worktree        string `json:"worktree"`
	GateContextHash string `json:"gate_context_hash,omitempty"`
	ChildCapHash    string `json:"child_cap_hash"`
	ParentCapHash   string `json:"parent_cap_hash"`
	BoundDriveID    string `json:"bound_drive_id,omitempty"`
	Closed          bool   `json:"closed"`
}
```

New typed ownership-error kinds (extend `OwnershipErrorKind` in `ownership.go`): `ErrScopeCapabilityMismatch OwnershipErrorKind = "scope-capability-mismatch"`, `ErrScopeSecondDrive = "scope-second-live-drive"`, `ErrScopeClosed = "scope-closed"`, `ErrScopeIdentityMismatch = "scope-identity-mismatch"`.

- [ ] **Step 2: Run `go test ./internal/gatedrive/ -run TestScope -count=1` — expect FAIL (undefined symbols).**
- [ ] **Step 3: Implement `scope.go`.** Root at `filepath.Join(gitCommonDir-derived store root, "..", "gate-scopes", "v1")` — cleaner: give `Store` a sibling `scopeRoot` computed in `OpenStore` (`filepath.Join(gitCommonDir, "docket", "gate-scopes", "v1")`; add the field to `Store` in `store.go`). CAS transitions serialize on a per-scope flock exactly as `Store.CAS` does; a mutate error aborts with no write. `bindScopeDrive` re-verifies inside the lock: not closed, `ChildCapHash == capHash(cap)`, `BoundDriveID == ""` (or equal to the same drive id — idempotent re-bind is a no-op). All schema-version checks fail closed.
- [ ] **Step 4: In `drive.go`, bump `driveSchemaVersion` to 2 and add to `driveRecord`:**

```go
	// ScopeID links the drive to the recovery scope its owner was dispatched
	// under; GateContextHash links every nested drive to the outer gate
	// (sha256 of the outer child-context token). Both empty for scopeless
	// drives (e.g. finalize's local gate). (schema v2, change 0359)
	ScopeID         string `json:"scope_id,omitempty"`
	GateContextHash string `json:"gate_context_hash,omitempty"`
```

Fix the two tests that pin the schema constant (`drive_test.go`, `store_test.go` write records with `driveSchemaVersion`; `driver_test.go:` and `store_test.go` "+999" unknown-schema cases keep working). Add one new assert in `drive_test.go`: a persisted v1 record read by the v2 store returns `ErrUnknownSchema` (fail closed, never migrated).
- [ ] **Step 5: Run `go test ./internal/gatedrive/ -race -count=1` — expect PASS.**
- [ ] **Step 6: Commit** `feat(gatedrive): recovery scopes with separated parent/child capabilities (drive schema v2)`

---

### Task 2: Scope-bound Start, event-authorized Takeover, and continuation lookups

**Build profile:** premium

**Files:**
- Create: `internal/gatedrive/takeover.go`, `internal/gatedrive/takeover_test.go`
- Modify: `internal/gatedrive/driver.go` (`StartRequest` + scope binding in `Start`), `internal/gatedrive/run_waiting.go` (continuation handle + scope-drive lookup), `internal/gatedrive/driver_transfer_test.go` (claim closes scope)

**Interfaces:**
- Consumes: Task 1's `scopeRecord`, `bindScopeDrive`, `closeScope`, `capHash`, error kinds; existing `ownerCAS`, `verifyOwner`, `ComputeFingerprint`, `randomToken`.
- Produces:

```go
// StartRequest gains (all optional; empty = scopeless drive, existing behavior):
	ScopeID         string
	ChildCapability string // raw; verified against the scope, stored nowhere
	GateContext     string // raw outer child-context token; stored as sha256

// Takeover is the event-authorized exceptional transfer. The CALLER asserts the
// event (its direct child returned without a valid handoff) by calling at all;
// Takeover proves everything else: parent capability, scope identity, exactly
// one candidate drive, no outstanding unclaimed handoff (a valid handoff means
// claim instead), fingerprint/branch/worktree/deadline agreement. On success it
// atomically invalidates the child owner generation and mints a fresh one,
// returned in DriveDoc.Generation. driveID may be "" — then the scope's
// BoundDriveID (task scope) or the unique gate-context match (outer scope)
// resolves it. Any ambiguity, race loss, or identity drift returns a HALTED
// document and never launches, stops, or duplicates a process.
func (d *Driver) Takeover(scopeID, parentCapability, driveID string) (DriveDoc, error)

// FindScopeDriveIDs lists drive ids for changeID whose GateContextHash equals
// gateContextHash and whose LastOutcome is nonterminal OR terminal-unconsumed
// (terminal with a still-set OwnerGeneration). Unreadable records are skipped.
func (s *Store) FindScopeDriveIDs(changeID, gateContextHash string) ([]string, error)

// ContinuationHandle returns the CURRENT unclaimed handoff token of driveID for
// in-process facade use only (never emitted). Fails typed when no unclaimed
// handoff exists.
func (s *Store) ContinuationHandle(driveID string) (string, error)
```

- [ ] **Step 1: Write failing tests in `takeover_test.go`** using the existing fake seams from `driver_test.go` (fake clock/proc/git). Cover, table-driven:

```go
// TestTakeoverInvalidatesChildAndMintsParentOwner: WAITING scope-bound drive;
// Takeover with the exact parent capability returns the drive's recorded
// outcome with a NEW Generation; the old child owner gen now gets
// HALTED/owner-superseded from Advance; the new gen advances normally; the
// scope is Closed afterward; NO Launch/Stop was called on the proc seam
// (assert call counters unchanged).
// TestTakeoverTerminalUnconsumed: drive already PASSED with owner gen still
// set (child died after terminal write); Takeover succeeds; Advance with the
// new gen returns the recorded PASSED with the same Attempt (no relaunch).
// TestTakeoverFailClosedTable: wrong parent capability / child capability
// presented as parent / closed scope / scope-drive identity mismatch (branch,
// worktree, change, task, phase each mutated separately) / fingerprint drift
// (git seam returns different hash) / expired deadline on a live drive /
// outstanding unclaimed handoff (must claim, not take over) / two candidate
// drives for one outer scope / zero candidates / unknown drive schema —
// each returns HALTED with a distinct cause token and mutates neither the
// drive record nor the scope record (reload and compare).
// TestTakeoverRace (-race): two goroutines race Takeover with the same parent
// capability; exactly one gets a fresh Generation, the loser gets HALTED;
// old owner stays invalid either way.
// TestStartBindsScope: Start with ScopeID+ChildCapability binds the new drive
// into the scope (BoundDriveID set) and stamps ScopeID + GateContextHash on
// the record; wrong capability fails BEFORE launch (proc.Launch never called);
// a second Start on the same scope while the first drive is nonterminal fails.
// TestClaimClosesScope: a normal Handoff+Claim of a scope-bound drive marks
// the scope Closed (the nearest-parent chain moves up on normal transfer too).
// TestFindScopeDriveIDs / TestContinuationHandle: behave per doc comments,
// including skip-unreadable and typed no-handoff error.
```

- [ ] **Step 2: Run `go test ./internal/gatedrive/ -run 'Takeover|StartBinds|ClaimCloses|FindScope|ContinuationHandle' -count=1` — expect FAIL.**
- [ ] **Step 3: Implement.** In `Start`: when `ScopeID != ""`, load + verify the scope (capability, identity fields vs request, no live bound drive) *before* `proc.Launch`; after `NewDrive`, `bindScopeDrive` — on bind failure `stopIfOwned` the fresh run and return the error (the drive never existed for the workflow). In `Claim` success path: if the record carries a `ScopeID`, `closeScope` best-effort after the ownership CAS. `Takeover` in `takeover.go`: resolve candidate drive(s); run all fail-closed checks on the loaded record; then `ownerCAS` with a mutate that re-verifies `OwnerGeneration != ""` and `HandoffGeneration == ""` and swaps in the fresh generation; then `closeScope`; build the return doc with `transferDoc`-style shape (Generation = fresh owner gen, Outcome/Cause = recorded). New HALT cause tokens (exported consts in `drive.go` beside the existing ones): `CauseTakeoverAmbiguous = "takeover-ambiguous"`, plus reuse `"identity-mismatch"`, `"handoff-outstanding"`, `string(ErrScopeCapabilityMismatch)` etc. from the ownership kinds.
- [ ] **Step 4: Run `go test ./internal/gatedrive/ -race -count=1` — expect PASS.**
- [ ] **Step 5: Mutation-prove the takeover guard set:** temporarily delete the fingerprint check in `Takeover`, run `go test ./internal/gatedrive/ -run TestTakeoverFailClosedTable -count=1`, confirm RED, restore (use a scratch copy of the file, never `git checkout --` over uncommitted work). Repeat for the parent-capability check. Record both mutations in the commit message body.
- [ ] **Step 6: Commit** `feat(gatedrive): scope-bound Start and event-authorized parent Takeover`

---

### Task 3: App seam — prepare-scope, takeover, and the task-intent (focused argv) drive owner

**Build profile:** standard

**Files:**
- Modify: `internal/app/gate_drive.go`
- Modify: `internal/app/gate_drive_test.go`

**Interfaces:**
- Consumes: Task 1/2 gatedrive surface (`PrepareScope`, `Takeover`, `StartRequest` scope fields).
- Produces:

```go
const (
	OperationGateDrivePrepareScope = "gate.drive.prepare-scope"
	OperationGateDriveTakeover     = "gate.drive.takeover"
)

// GateScopeResult carries the grant. HumanText prints ONLY the scope id — the
// capabilities travel exclusively in the JSON document.
type GateScopeResult struct {
	Envelope
	ScopeID          string `json:"scope_id,omitempty"`
	ChildCapability  string `json:"child_capability,omitempty"`
	ParentCapability string `json:"parent_capability,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

func (s *GateDriveService) PrepareScope(req gatedrive.ScopeRequest) GateScopeResult
func (s *GateDriveService) Takeover(scopeID, parentCap, driveID string) GateDriveResult

// NewTaskGateDriveService composes the seam for TASK-INTENT (focused/ad-hoc)
// drives: the workflow role declares test intent and supplies the argv
// explicitly; there is no config command to resolve. It NEVER sets
// IdempotentSuiteGate (forced false in Start regardless of the request), and
// records provenance "task.argv=agent-supplied". The build/finalize owner
// constructors keep refusing caller argv — the domain boundary moves to "which
// constructor", not away.
func NewTaskGateDriveService(gitCommonDir, exePath string, argv []string) (*GateDriveService, Result, string)

// GateDriveStartRequest gains: ScopeID, ChildCapability, GateContext string.
```

- [ ] **Step 1: Write failing tests** in `gate_drive_test.go` with a fake engine (the file already injects one): `TestPrepareScopeHumanTextRedactsCapabilities` (HumanText contains ScopeID, contains neither capability string); `TestTaskServiceForcesNonIdempotent` (request sets `IdempotentSuiteGate: true`, the engine sees `false` and the exact argv verbatim — no `/bin/sh -c` wrapping for task argv, pass it through as given); `TestTaskServiceRequiresArgv` (empty argv → `ResultInvalidInput`, reason `"missing-argv"`); `TestStartForwardsScopeFields`; `TestTakeoverMapsDoc` (delegates, maps command failure via `mapDriveFailure`). Extend the `driveEngine` interface with `Takeover(scopeID, parentCap, driveID string) (gatedrive.DriveDoc, error)` and `PrepareScope(gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error)`; keep the compile-time `_ driveEngine = (*gatedrive.Driver)(nil)` assertion honest by adding matching methods on `*gatedrive.Driver` (`PrepareScope` delegates to the store — add that thin method in gatedrive if Task 2 did not).
- [ ] **Step 2: Run `go test ./internal/app/ -run 'GateDrive|Scope|Takeover' -count=1` — FAIL, then implement, then PASS.**
- [ ] **Step 3: Commit** `feat(app): gate-drive seam grows prepare-scope, takeover, and the task-intent owner`

---

### Task 4: CLI — `gate drive prepare-scope`, `gate drive takeover`, task-owner argv, scope flags

**Build profile:** standard

**Files:**
- Modify: `internal/cli/gate.go`, `internal/cli/install.go`, `internal/cli/capability_test.go`, `internal/cli/capability_production_test.go`, `internal/cli/gate_test.go`

**Interfaces:**
- Consumes: Task 3's seam methods and operation ids.
- Produces (capability catalog — later skill tasks reference these ids verbatim):
  - `gate.drive.prepare-scope` — `docket gate drive prepare-scope --change-id <id> --task-id <id> --phase <name> --branch <name> --worktree <dir> [--gate-context <token>] [--repo-dir <dir>]`, effects `local-write`.
  - `gate.drive.takeover` — `docket gate drive takeover --scope-id <id> --parent-cap <token> [--drive-id <id>] [--repo-dir <dir>]`, effects `local-write`.
  - `gate.drive.start` gains `[--scope-id <id>] [--child-cap <token>] [--gate-context <token>]`, and `--owner task` which **requires** a `-- <argv...>` boundary (parse exactly as `gate launch` does with `ArgsLenAtDash`); `--owner build|finalize` still **rejects** argv after `--`.

- [ ] **Step 1: Write failing CLI tests** (mirroring `gate_test.go`/`artifact_test.go` registration style): commands registered with required flags; `--owner task` without `--` argv errors; `--owner build` with argv errors; prepare-scope JSON carries all three grant fields while its human text does not carry the capabilities. Update `capability_test.go`'s tree and `capability_production_test.go`'s pinned signature map (the map is exact-match — add/adjust the three rows).
- [ ] **Step 2: FAIL → implement in `gate.go` (thin adapters composing `buildCommandlessGateDriveService` for takeover/prepare-scope — prepare-scope needs only the store: compose commandless; for `--owner task` build the service with `app.NewTaskGateDriveService(repo.CommonDir, exe, argv)`) → PASS.** Add `"gate drive prepare-scope": true` and `"gate drive takeover": true` to `install.go`'s allowlist map.
- [ ] **Step 3: Run `go test ./internal/cli/ -count=1` — PASS.**
- [ ] **Step 4: Commit** `feat(cli): prepare-scope, takeover, and task-owner argv on gate drive`

---

### Task 5: `gate-before` — outer recovery scope + explicit resume attribution (gate record v2)

**Build profile:** premium

**Files:**
- Modify: `internal/app/rungate_before.go`, `internal/app/rungate_store.go`, `internal/cli/run.go`
- Modify: `internal/app/rungate_before_test.go` (or the existing gate-before test file), `internal/cli/capability_production_test.go`

**Interfaces:**
- Consumes: `PrepareScope` via a new small seam so unit tests can fake it; `WorkspaceInspect` (already in app) for branch/worktree identity; existing `PinContext/ReadCorpus/BuildSnapshot` plumbing.
- Produces:

```go
// GateRecord (rungate_store.go) — bump the stored Schema constant to 2, add:
	ScopeID             string `json:"scope_id,omitempty"`
	ParentCap           string `json:"parent_cap,omitempty"`            // raw; record is 0600-private, never printed
	ChildContextHash    string `json:"child_context_hash,omitempty"`   // sha256 of the printed dispatch context
	ContinuationID      string `json:"continuation_id,omitempty"`
	ContinuationDrive   string `json:"continuation_drive,omitempty"`
	ContinuationHandoff string `json:"continuation_handoff,omitempty"`
// Pair rule (0396's shape, extended to a triple): the three Continuation*
// fields are all-empty or all-set; a partial triple is a corrupt record on
// read AND on write. Schema-1 records fail closed as ErrGateCorruptRecord.

// RunGateBefore signature gains resumeID (0 = none):
func RunGateBefore(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, sdeps GateScopeDeps, repoDir, target string, resumeID int) RunGateBeforeResult
type GateScopeDeps struct{ Prepare func(gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error) }
// New reason tokens: ReasonGateResumeUnverified = "resume-unverified",
// ReasonGateScopeFailed = "scope-failed".
```

- Armed report line becomes **`gate-armed <key> <dispatch-context>`** where `<dispatch-context>` is the outer scope's ChildCapability: the parent copies it into the implement-next dispatch prompt; nested drives carry its hash as `GateContextHash`. `RunGateBeforeResult` gains `DispatchContext string`.
- CLI: `docket run gate-before <target> [--resume <id>]` — update the catalog signature row.

- [ ] **Step 1: Write failing tests:** `TestGateBeforePreparesOuterScope` (record carries ScopeID/ParentCap/ChildContextHash; line is `gate-armed <key> <ctx>`; ParentCap appears in the record file but NEVER in HumanText or JSON — add an explicit redaction assert on the marshalled JSON: `ParentCap` gets `json:"-"`? No — the record needs it persisted; the *result* struct simply never carries it: assert the result JSON bytes don't contain the token); `TestGateBeforeResumeBindsOnlyVerifiedInProgress` (resume id whose snapshot status is `in-progress` AND `WorkspaceInspect` applied → record.AttributedID pre-bound and the scope's ChangeID pre-bound; a proposed/implemented id, or a failed inspect, → `gate-unarmed resume-unverified`, no record minted); `TestGateBeforeNoTimestampGames` (resume path never touches DispatchEpoch semantics: BeforeIDs still the fresh set, DispatchEpoch still post-read — a resumed change sits in BeforeIDs and that is fine because AttributedID is already bound); `TestGateRecordContinuationTripleRule` (store rejects partial triples both directions); `TestGateRecordSchema1FailsClosed`.
- [ ] **Step 2: FAIL → implement → PASS (`go test ./internal/app/ -run 'GateBefore|GateRecord' -count=1`).** Production wiring of `GateScopeDeps` happens in `internal/cli/run.go` (compose `gatedrive.OpenStore(repo.CommonDir)` and call `PrepareScope` with `RepoIdentity: repo.CommonDir, GateContext: ""`, Branch/Worktree from the resumed change's inspect when resuming, empty otherwise).
- [ ] **Step 3: Commit** `feat(rungate): gate-before prepares the outer recovery scope and accepts --resume (record schema v2)`

---

### Task 6: `gate-verdict` — nonterminal `gate-continue`, outer takeover, retry preservation

**Build profile:** premium

**Files:**
- Create: `internal/app/rungate_continuation.go`
- Modify: `internal/app/rungate_verdict.go`, `internal/app/rungate_verdict_test.go` (and siblings asserting the old run-waiting→stop mapping), `internal/cli/run.go`

**Interfaces:**
- Consumes: Task 2's `FindScopeDriveIDs`, `ContinuationHandle`, `Driver.Takeover`, `Driver.Handoff`; Task 5's record fields.
- Produces:

```go
const GateDecisionContinue = "gate-continue" // nonterminal; joins Done/RetryOnce/Stop/Observe

// ContinuationSeam is what the verdict path needs from the drive layer; the
// production impl (rungate_continuation.go) composes gatedrive Store+Driver
// from git common dir + exe path; unit tests fake it.
type ContinuationSeam interface {
	// LocateOuterDrive: candidate drives for (changeID, childContextHash);
	// exactly one is required upstream.
	LocateOuterDrive(changeID int, childContextHash string) ([]string, error)
	// TakeoverAndHandoff: event-authorized outer takeover of driveID under
	// (scopeID, parentCap), then an immediate normal Handoff by the fresh
	// owner — returning the single-use handoff token. Exactly this synthesis
	// is what lets RunVerify's EXISTING run-waiting predicate validate the
	// continuation; RunVerify itself is not modified.
	TakeoverAndHandoff(scopeID, parentCap, driveID string) (handoffToken string, halted bool, cause string, err error)
	// ExistingHandoffToken: the unclaimed handoff token of driveID (a worker
	// already handed off; nothing to take over).
	ExistingHandoffToken(driveID string) (string, error)
}
```

Verdict mapping changes in `RunGateVerdict` (order is load-bearing):
1. `VerdictRunWaiting` → **no longer gate-stop.** Mint a fresh `ContinuationID` (`randomToken`-style, add a local helper), read `ExistingHandoffToken(v.HandoffID)`, persist the triple onto the record, report `gate-continue <key> run-waiting <id> <continuation-id> <phase>` with `Terminal: false`. Retry untouched.
2. `VerdictRunIncomplete` with a record carrying a ScopeID → **before any `ConsumeGateRetry` call**: `LocateOuterDrive`. Exactly one candidate → `TakeoverAndHandoff` → re-run `RunVerify`; a resulting `run-waiting` → persist triple → `gate-continue` (Terminal false, retry untouched). Zero candidates → fall through to the existing retry-consumption path unchanged (only a genuinely quiescent incomplete may spend retry). More than one candidate, or a halted/erred takeover → `gate-stop <key> gate-unavailable <cause>` (unsafe ownership never earns retry OR continuation).
3. All other verdict arms unchanged. The observe path (`gateObserveLine`) stays structurally unable to emit `gate-continue` or `gate-retry-once` — extend its header comment to name both exclusions.
4. `HumanText` renders the continue line by adding a `ContinuationID string` field and a `GateDecisionContinue`-aware branch: fields `[gate-continue, key, run-waiting, id, continuation-id, phase]`.

- [ ] **Step 1: Rewrite the failing/changed tests first.** Update every test asserting `run-waiting → gate-stop` to the new mapping. Add: `TestVerdictWaitingIsNonterminalContinue` (decision, Terminal=false, retry marker file NOT created, record triple persisted, line shape exact); `TestVerdictIncompleteWithTrackedDriveContinuesWithoutRetry` (fake seam: one candidate + successful takeover + re-verify run-waiting → gate-continue AND `ConsumeGateRetry`'s O_EXCL marker absent afterward — assert on the filesystem, not on a mock, so the "cannot reach the retry CAS" property is real); `TestVerdictIncompleteQuiescentStillRetriesOnce` (zero candidates → identical to today's behavior, both grant and exhausted arms); `TestVerdictAmbiguousDrivesStops`; `TestVerdictTakeoverHaltStops`; `TestContinueNeverAuthorizesNewClaim` (attribution logic untouched on the continue path — AttributedID unchanged); `TestObservePathStillCannotContinue` (grep-level structural assert already exists for retry; extend to `gate-continue`).
- [ ] **Step 2: FAIL → implement `rungate_continuation.go` + the mapping → `go test ./internal/app/ -run 'Verdict|Continuation' -count=1` PASS.**
- [ ] **Step 3: Mutation-prove the ordering guard:** move the `ConsumeGateRetry` call above the tracked-drive check in a scratch mutation; `TestVerdictIncompleteWithTrackedDriveContinuesWithoutRetry` must redden (the marker file appears). Restore. Note the mutation in the commit body.
- [ ] **Step 4: Run `go test ./internal/app/ -race -count=1` — PASS.**
- [ ] **Step 5: Commit** `feat(rungate): gate-continue keeps the key and the retry; outer takeover synthesizes a normal handoff`

---

### Task 7: Continuation redemption — `run gate-claim`

**Build profile:** standard

**Files:**
- Create: `internal/app/rungate_claim.go`, `internal/app/rungate_claim_test.go`
- Modify: `internal/cli/run.go`, `internal/cli/install.go`, `internal/cli/capability_test.go`, `internal/cli/capability_production_test.go`

**Interfaces:**
- Produces:

```go
const OperationRunGateClaim = "run.gate-claim"
// docket run gate-claim <key> <continuation-id> [--repo-dir <dir>] — local-write.
// Loads the gate record; verifies the presented continuation-id equals the
// stored ContinuationID (constant-time compare, crypto/subtle); performs
// gatedrive Claim(ContinuationDrive, ContinuationHandoff) through the
// commandless drive service; on success CLEARS the triple (single-use at the
// record layer; Claim's CAS makes it single-use at the drive layer) and
// returns {drive_id, generation (fresh owner gen), phase} in JSON — the
// resumed implement-next controller advances with them. Human text names the
// drive id and outcome only, NEVER the generation. Fail closed: no stored
// continuation → "no-continuation"; mismatch → "continuation-mismatch"; a
// HALTED claim (raced, fingerprint drift) → gate-stop-shaped refusal carrying
// the driver's cause; the triple is cleared ONLY on success.
func RunGateClaim(...) RunGateClaimResult
```

- [ ] **Step 1: Failing tests** for each arm above (fake the drive seam like Task 6), including the redaction assert (generation absent from HumanText) and single-use (second call → no-continuation).
- [ ] **Step 2: FAIL → implement → PASS.** CLI leaf under the `run` group; add `"run gate-claim": true` to install.go; update both capability test files.
- [ ] **Step 3: Commit** `feat(rungate): single-use gate-claim redeems a continuation for the resumed controller`

---

### Task 8: `docket-build-task` — every test through the driver; first WAITING always hands off

**Build profile:** premium

**Files:**
- Modify: `skills/docket-build-task/SKILL.md`

This is a contract rewrite; the enforcement teeth land in Task 11's guard — but write the prose so the guard's shape classifier passes it (fenced runnable recipes use `docket gate drive …` only).

- [ ] **Step 1: Rewrite "## The cycle".** Keep the 5-step TDD list, but replace the current conditional-driver paragraph ("When the narrowest honest verification is still a run that may outlast a single foreground call…") with an unconditional routing rule carrying exactly these requirements:
  - **Every test execution this task runs — baseline, RED, GREEN, focused re-run, ad-hoc verification — starts through the native gate driver.** Focused/ad-hoc commands use the task-intent owner: `docket gate drive start --owner task --scope-id <id> --child-cap <token> --run-root <task-scratch-dir> -- <the test command>` (the scope id and child capability arrive in your dispatch prompt; the run root is a scratch dir you choose and read logs from). There is **no duration prediction and no list of test-command spellings**: what makes a command a test is that you are running it as this task's verification, and the 30-second slice is the *maximum* of one observation call, not a minimum — a quick test returns on the driver's next ~250 ms observation.
  - Disposition handling table (verbatim semantics): `PASSED` → continue self-review and the task commit; `FAILED` → the test completed red — read the streams under your `--run-root` and apply the existing repair discretion; `HALTED` → return `BLOCKED` with the typed cause; `WAITING` → **immediately** perform `docket gate drive handoff --drive-id <id> --owner-gen <gen>` and return `WAITING` naming the drive id and single-use handoff token. **After a first `WAITING` you never call `advance` again and never start the test a second time** — the controller owns the drive from here.
  - `WAITING` consumes neither repair nor escalation budget.
  - Delete the sentence permitting continued self-observation; keep the never-yield / never-background / never-raw-verbs sentences verbatim (they are guarded elsewhere).
- [ ] **Step 2: Update the intro** ("handed to you in your prompt along with the branch, the worktree, the selected build profile, and the routing reason") to add "the drive scope id and child capability for this task". Leave the Outcomes/return schema untouched — `WAITING`+`HANDOFF` already carry the right shape.
- [ ] **Step 3: Run the suite's skill-facing checks focused:** `go test ./internal/repoguard/ -count=1` (the existing handoff-sites and gatedriver guards read this file) — PASS; fix wording only if a guard reddens for shape reasons, never by weakening a guard.
- [ ] **Step 4: Commit** `feat(skills): docket-build-task routes every test through the driver and always hands off on first WAITING`

---

### Task 9: `docket-build` controller + caller-loop reference + implement-next continuation clause

**Build profile:** premium

**Files:**
- Modify: `skills/docket-build/SKILL.md`, `skills/docket-build/references/gate-caller-loop.md`, `skills/docket-implement-next/SKILL.md`

- [ ] **Step 1: `skills/docket-build/SKILL.md` — "Dispatching a task":** add: before each worker dispatch, run `docket gate drive prepare-scope --change-id <id> --task-id <task-N> --phase build --branch <branch> --worktree <worktree> --gate-context <dispatch-context>` (the dispatch context arrived in *your* prompt from the gated parent; pass its value through). Hand the worker the **scope id and child capability only**; the parent capability stays in your notes and never enters any prompt, log, or report.
- [ ] **Step 2: "Task-level WAITING and the continuation":** keep the claim-and-advance duty; add the new exceptional branch: **when the worker's dispatch returns without a valid handoff while its scope still binds a nonterminal (or terminal-unconsumed) drive, run `docket gate drive takeover --scope-id <id> --parent-cap <token>` — the authorization is the return event you just observed, never a timer, heartbeat, or quiet log — then drive the same drive to terminal with `advance` slices.** A takeover `HALTED` is a halting condition (unsafe ownership), never repair, escalation, or a fresh worker. A trusted terminal `PASSED` consumed after takeover is not re-run. Neither takeover nor WAITING consumes repair or escalation budget.
- [ ] **Step 3: `gate-caller-loop.md`:** add `prepare-scope` and `takeover` rows to the operations table (one line each, matching Task 4's signatures), and a short "Parent takeover — the event-authorized exception" subsection after "Handoff — the only ownership transfer" stating: normal handoff remains preferred; takeover exists only for a direct child that returned without handing off; it atomically supersedes the child's owner generation; a stale child call thereafter fails owner-superseded; ambiguity fails closed to `HALTED`.
- [ ] **Step 4: `skills/docket-implement-next/SKILL.md`:** in the Step-5/run-verification prose that names `run-waiting` (the paragraph anchored on "`run-waiting <change-id> <handoff-id> <phase>` (change 0342)"), add the continuation-dispatch contract: **a run dispatched with an explicit change id, continuation id, and gate key resumes, not restarts: its FIRST act is `docket run gate-claim <key> <continuation-id>`, claiming the recovered drive before any other work; it never launches a replacement test, and this is a continuation, not `gate-retry-once` — the key stays active until a true terminal disposition.** Also note the dispatch-context token: a gated parent's prompt may carry a dispatch context; pass it into every `gate drive prepare-scope`/`start --gate-context` this run performs.
- [ ] **Step 5: Run `go test ./internal/repoguard/ ./internal/cli/ -count=1`** (prose-contract and skill guards) — PASS without weakening any guard.
- [ ] **Step 6: Commit** `feat(skills): controller scope preparation, event-authorized takeover, continuation resume`

---

### Task 10: Regenerate the dispatch/gate-bracket surfaces from `run-gate.md`

**Build profile:** standard

**Files:**
- Modify: `cursor-rules/run-gate.md`
- Regenerate: `internal/assets/embedded/**` (via `go generate ./internal/assets`), the repo's own installed surfaces (`CLAUDE.md` and any harness rule files `go run ./cmd/docket install --repo-dir .` rewrites)
- Modify: any frozen goldens in `internal/harness/*_test.go` / `internal/install` tests that pin the old interior digest or text

- [ ] **Step 1: Rewrite `cursor-rules/run-gate.md`** keeping the heading and steps 1–5 structure:
  - Step 1: `run.gate-before` now prints `gate-armed <key> <dispatch-context>`; keep both in your notes and include the dispatch context in the implement-next dispatch prompt. `--resume <id>` arms a gate for explicitly resuming an already-in-progress change.
  - Step 4 (the parent contract — this is the run-waiting-to-stop mapping being replaced): `gate-retry-once` keeps its exact current sentence. **Add `gate-continue`:** `gate-continue <key> run-waiting <change-id> <continuation-id> <phase>` is **nonterminal** — the same implement-next attempt owns live or unconsumed tracked work; it keeps the same key, spends no retry, and is structurally distinct from `gate-retry-once` (a continuation of the same attempt, never a second attempt): resume the existing implement-next agent when your harness supports a real resume, otherwise dispatch `docket-implement-next` again with the explicit change id, the continuation id, and the same key. Then run `run.gate-verdict <key>` again after it returns. **Delete** the clause mapping `run-waiting` to "report the handoff id and phase, then stop"; `gate-stop` and `gate-observe` keep forbidding re-dispatch.
- [ ] **Step 2: Regenerate:** `go generate ./internal/assets` (refreshes `internal/assets/embedded/tree/cursor-rules/run-gate.md` + `manifest.json`), then `go run ./cmd/docket install --repo-dir .` from the worktree root (refreshes the committed `CLAUDE.md` block and sibling harness surfaces). `git diff --stat` and stage exactly the regenerated files plus the authored source.
- [ ] **Step 3: Verify:** `grep -c "gate-continue" CLAUDE.md internal/assets/embedded/tree/cursor-rules/run-gate.md` ≥ 1 each; `grep -n "then.*stop" cursor-rules/run-gate.md` shows no run-waiting-stop clause. Run `go test ./internal/harness/... ./internal/install/... ./internal/assets/... -count=1`; update frozen goldens that legitimately pinned the old interior (a golden update is the *expected* diff here, not a guard weakening — the guards pin authored↔installed agreement, which regeneration restores).
- [ ] **Step 4: Commit** `feat(dispatch): run-gate surface recognizes nonterminal gate-continue and the dispatch context`

---

### Task 11: Structural guard — no direct test execution in workflow fixtures

**Build profile:** premium

**Files:**
- Create: `internal/repoguard/testexec_boundary_test.go`

Model on `internal/repoguard/gatedriver_test.go` (same walk helpers, same path-shape classifiers). The guard classifies **workflow-shaped test-execution sites by syntactic shape** and turns RED when a direct test execution is injected into a workflow fixture. It must NOT hand-list filenames or third-party test-command spellings.

- [ ] **Step 1: Write the classifier + guard test:**
  - **Population:** workflow markdown (`isWorkflowMD` — reuse it) fenced blocks, plus workflow shell (`isWorkflowSH`).
  - **Violation shapes (docket-owned closed identifiers only, never runner spellings):**
    - (a) a fenced/runnable line invoking docket's own suite channel directly — derive the spelling from the capability id `development.test`'s argv (`docket development test`, and the `go run ./cmd/docket development test` entry form, both derived by joining the catalog argv, with the `go run ./cmd/docket` prefix as the one documented source-entry variant) — when the same fenced block does not route through `gate drive`;
    - (b) a fenced/runnable line that interpolates a resolved test-command identity into a direct shell invocation: derive the key set by reflecting over `config.Effective` for struct fields whose yaml/json tag is `test_command` (the derive-from-the-consumer rule; today that yields `build.test_command` and `finalize.test_command`) plus their exported-variable forms (uppercase, dots→underscores), again outside a `gate drive` line.
  - **Guard body:** scan the real tree, expect zero violations; every match reports path+line.
  - **Red-injection proof (required by the spec):** feed the classifier synthetic workflow-fixture content containing (i) a fenced `docket development test` line and (ii) a fenced `"$BUILD_TEST_COMMAND"` invocation, and assert both classify as violations; feed it the sanctioned `docket gate drive start --owner build` form and assert it does not. This is the mutation-of-the-population test living inside the guard file itself.
  - **Header comment:** record the residual (inline-backtick evasion, mirroring gatedriver_test.go's recorded residual) — assert the limitation in the header, per the byte-pattern-guard learning.
- [ ] **Step 2: Run `go test ./internal/repoguard/ -run TestExec -count=1` — the real-tree scan must PASS against the Task 8/9/10 state of the skills (this ordering is why the guard lands after them). If it reds on a genuine leftover direct-execution recipe, fix that workflow file — never the guard.**
- [ ] **Step 3: Mutation-prove the guard the other direction:** temporarily reintroduce a fenced direct suite invocation into `skills/docket-build-task/SKILL.md`, run the guard, confirm RED, revert the skill edit (scratch copy, not `git checkout --`).
- [ ] **Step 4: Commit** `test(repoguard): shape-classified guard against direct test execution in workflow fixtures`

---

### Task 12: Real detached-child integration coverage

**Build profile:** premium

**Files:**
- Modify: `internal/gatedrive/integration_test.go` (follow its existing real-process patterns and build/skip conventions)

- [ ] **Step 1: Write the four integration tests** (short driver slices via the unexported `slice`/`pollInterval` overrides — no production-duration sleeps; assert the production constant separately):
  - `TestIntegrationFastCompletionReturnsImmediately`: a real child that exits 0 quickly; a single `Start` returns `PASSED` and the wall clock of the call is well under the (shrunk) slice — fast tests never pay a 30-second floor.
  - `TestIntegrationSliceBoundIsProductionThirtySeconds`: assert `productionSlice == 30*time.Second` and that a live slow child returns `WAITING` within one (shrunk) slice + scheduling margin — together these pin "first slice returns by 30s" without sleeping 30s.
  - `TestIntegrationTakeoverKeepsRunIdentity`: scope-bound drive over a real slow child; record `RawRunDir`, `RawOwnership`, and the supervised PID/session from the run dir's native record before takeover; `Takeover` + `Advance` to terminal; assert the same `RawRunDir`/`RawOwnership`/`Attempt` throughout — same process, no duplicate launch (count run-slot dirs under the run root: exactly one).
  - `TestIntegrationTerminalConsumedFromFreshProcess`: drive a real child to terminal, then build a **new** `Store`+`Driver` (simulating a fresh process over the same git common dir), `Takeover` the terminal-unconsumed drive and `Advance`; assert the recorded verdict is returned with no relaunch.
- [ ] **Step 2: Run `go test ./internal/gatedrive/ -race -count=1` — PASS.** Check the file's wall clock against the suite's budget expectations (`tests/README.md`); if this file sits near a budget ceiling, split the new tests into `integration_takeover_test.go` in the same package.
- [ ] **Step 3: Commit** `test(gatedrive): detached-child integration proof of fast return, slice bound, takeover identity, fresh-process consumption`

---

### Task 13: Prepare the superseding ADR draft (recorded later by docket-adr, not here)

**Build profile:** standard

**Files:**
- Create: `docs/superpowers/plans/2026-09-02-adr-supersede-0098-draft.md`

The ADR itself is recorded by the `docket-adr` agent during `docket-implement-next` Step 6 — this task only prepares the decision text and commits it on the feature branch so the review step can hand it over verbatim.

- [ ] **Step 1: Write the draft** with this exact content skeleton, filled with the final built reality (verify each claim against the code you can see on the branch — verify-the-claim, never from memory):
  - **Title:** "Event-authorized parent takeover extends fingerprinted gate-drive ownership"
  - **Supersedes:** ADR-0098 (structured gate waiting and ownership handoff).
  - **Context:** ADR-0098's cooperative-only transfer (owner-authorized handoff + single-use claim) cannot recover a drive whose owner returned without handing off; the run gate then read healthy tracked work as `run-incomplete`, spent the retry, and terminally stopped a progressing run (changes 0333/0363; change 0359's evidence tables).
  - **Decision:** (1) durable recovery scopes at each dispatch boundary with separately-minted child and parent opaque capabilities, hash-persisted, CAS-transitioned, fail-closed on unknown schema / identity mismatch / second live drive / missing capability; (2) parent takeover authorized only by the direct child's dispatch-return event plus the exact parent capability, atomically superseding the child owner generation — never by timers, heartbeats, log activity, or process-name checks; (3) the outer run gate treats a tracked drive as a nonterminal continuation (`gate-continue`, same key, retry preserved) by synthesizing a normal single-use handoff the unchanged run-waiting predicate validates; (4) explicit resume attribution on `gate-before`.
  - **Preserved from ADR-0098:** structured `WAITING`; fixed-once deadline; fingerprinted single-owner advancement; single-use handoff as the *preferred* transfer; every ambiguity fails closed to `HALTED`, never red.
  - **Consequences:** drive schema v2 / gate record v2 fail closed against old records; a grandparent still cannot skip a live parent; observe mode remains structurally unable to authorize retry or continuation.
- [ ] **Step 2: Commit** `docs: ADR draft — event-authorized parent takeover superseding ADR-0098`

---

### Task 14: Document the four-harness re-probe as the outstanding human merge gate

**Build profile:** economy

**Files:**
- Modify: `skills/docket-build/references/gate-execution.md`

The reconcile decision (change file, 2026-09-02) carves the four-harness real-world acceptance probes out of this autonomous run: they require driving four separately-installed harnesses and **must never be fabricated**. This task documents them as a required pre-merge human verification; it executes nothing.

- [ ] **Step 1: Add a new section** after "Per-harness verdicts", titled `## Change 0359 continuation/takeover acceptance — PENDING HUMAN RE-PROBE (pre-merge gate)`, containing:
  - One row per harness with verdict token `unverified — 0359 re-probe pending (human, pre-merge)`: Claude Code `2.1.251` (interactive AND forked/dispatched implement-next path), Cursor `3.17.21` (registered named-agent + continuation dispatch), Codex `0.150.1` (named dispatch, same-agent resume when available, fresh continuation fallback), OpenCode `1.18.23` (named dispatch + continuation dispatch).
  - The seven probe scenarios verbatim from the spec's "Four-harness contract" (fast return; slice-spanning handoff; worker-return takeover; controller-return top-parent continuation with same gate key; no duplicate process / no new task / no retry while a drive is active; terminal pass and terminal failure consumed by the correct resumed role; explicit resume attributable).
  - The standing rules restated in one line each: a verdict is version-scoped; an interactive-only observation cannot stand in for a dispatched path; a harness that cannot supply the direct-child return event or an explicit continuation is unsupported on that path and the gap is reported, never bridged with a timer.
  - An explicit sentence: **"This section is the outstanding merge-gate verification for change 0359: a human runs these probes and records the evidence here before the PR merges. Do not merge on pending rows, and never write probe evidence that was not observed."** The existing measured verdict sections above it are point-in-time records — leave them untouched.
- [ ] **Step 2: Run `go test ./internal/repoguard/ -count=1`** (prose guards) — PASS.
- [ ] **Step 3: Commit** `docs: record the four-harness 0359 re-probe as the pending human merge gate`

**Note to the controller/reviewer (flows into the results file and PR body, not into more code):** the results artifact and the PR description must both name the four-harness re-probe as the outstanding human verification and link the section added here.

---

## Verification strategy (whole-plan)

1. Per-task focused tests as written above, race-enabled where concurrency is touched.
2. Mutation evidence recorded for: takeover's fingerprint + capability checks (Task 2), the retry-ordering guard (Task 6), the structural guard's population (Task 11), plus the docket-build-task worker contract's standing guard-mutation obligation for anything else guarded.
3. The build gate runs the configured whole suite (`go run ./cmd/docket development test`, resolved from config, driven through the gate driver per docket-build's own policy). Inspect every budget clause line; a `SERIAL CONFIRMED OVER BUDGET:` line is authoritative and acted on.
4. The four-harness acceptance probes are **not** run here — Task 14 records them as the pending human merge gate. No task writes harness-probe PASS evidence.

## Self-review record

Checked against the spec section by section: §1 → Tasks 3/4/8 (task-intent owner + skill routing); §2 → Tasks 8/9 (first-WAITING transfer, controller ownership); §3 → Tasks 1/5 (scopes, capability separation, outer scope at gate-before); §4 → Tasks 2/3/4/9 (event-authorized takeover, nearest-parent rule); §5 → Tasks 6/7/10 (gate-continue, retry preservation, continuation id + redemption, parent contract regeneration); §6 → Task 5 (explicit resume attribution); §7 → constants pinned, asserted in Task 12; Four-harness contract → Task 14 (human gate per the reconcile decision); Verification strategy → Tasks 2/6/11/12 + whole-suite gate; Migration steps 1–7 → task ordering 1-2 / 5 / 3-4+8 / 8-9 / 6-7+10 / 13+10 / 14+build gate. Types cross-checked: `ScopeRequest`/`ScopeGrant`/`capHash` (Tasks 1→2→3→5), `Takeover(scopeID, parentCapability, driveID)` (Tasks 2→3→4→6), the Continuation triple field names (Tasks 5→6→7), `GateDecisionContinue` line shape (Tasks 6→10), dispatch-context flow `gate-before → prompt → prepare-scope/start --gate-context → GateContextHash → LocateOuterDrive` (Tasks 5→9→4→2→6).
