<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0446 — Orphaned halted gate drive blocks every new worktree's first gate admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0446-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first.md)**
<!-- docket:backlink:end -->
# Change 0446 — Historical gate bookkeeping must not block current execution — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An explicitly named change B's gate admission, cancellation, resume, success closeout, and entry into finalize are never blocked by unrelated historical gate records belonging to change A — while every genuine obligation of B's own worktree/run still blocks with an exact locator.

**Architecture:** Correct the authority boundaries of the existing admission/census/ownership paths in `internal/gatedrive` and `internal/app` (rungate_*): historical discovery becomes diagnostic unless a record is positively bound to the requested worktree or run; the epoch launch census attributes obligations through current slot/scope/token references instead of failing closed repo-wide; owner lookup resolves deterministically instead of by directory order; durable completion facts stop being reopened after scratch cleanup. No new stores, commands, schemas, or liveness implementations.

**Tech Stack:** Go (existing `internal/gatedrive`, `internal/app`, `internal/process` packages); the repository's own test suite via the source Go runner (`internal/suiterunner`), gated by the resolved `build.test_command`.

**Spec:** `docs/superpowers/specs/2026-09-23-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first-design.md` (synchronized metadata tree; the change file is `docs/changes/active/0446-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first.md`). Every task below argues from a numbered spec section; workers must read the spec section named in their task before writing code.

## Global Constraints

- **No new machinery:** no new commands, force flags, daemons, background cleanup, registries, persistent schemas, leases/TTLs, age cutoffs, cross-store transactions, configuration keys, retry allowances, or second liveness implementation (spec "Architecture and scope limits").
- **No blanket trust in HALTED:** HALTED is never slot-release proof, completion evidence, or a substitute for slot/participant accounting (spec §4). No cause-name allowlist; a new exception for `stopped-not-initiated` is an unacceptable implementation.
- **Preserve the exclusion guarantee:** two executions never run concurrently in one worktree; an active workflow owns its worktree between sequential drives; an old fenced execution cannot launch after a replacement is admitted; exact-token checks, launch claims, epoch fences, and admission-before-charging are preserved (spec "Required outcome").
- **Fail closed on the target's own records:** a corrupt/missing record positively named by the requested worktree's current slot/scope/epoch still refuses locally with an exact locator. Never infer safety for a named current obligation from a failed read (spec §1).
- **Derive populations by search, never by list:** the pre-reserve refusal sites, the launch-path token checks, and the message sites equating cleanup with admission refusal are each derived from a whole-repo grep at build time; the sites this plan names are known members, not the population (spec §§2–3, AGENTS.md "Never hand-list").
- **Diagnostics are bounded and credential-free:** findings carry drive ids, worktree identity, and disposition tokens only — never reservation tokens, argv, env, or arbitrary directory names (existing contract in `internal/gatedrive/history.go`, `reconcile.go`).
- **Mutation-test every guard** (AGENTS.md; spec AC8): each behavioral test must go red under the named mutation; use `-count=1` to defeat Go's test cache when probing mutations.
- **Excluded:** finalize queue ordering, driver stop/continue policy, named-start maintenance (change 0448), metadata validation/board rendering (change 0449), publication-journal reconciliation (change 0444).
- **ADR discipline:** the changed ADR-0118/ADR-0120 clauses are recorded through the `docket-adr` workflow during implementation (Task 11); Accepted ADR text is never rewritten in place.
- Comments/cross-references anchor on symbol names or verbatim clauses, never line numbers (ADR-0054).

## File Structure

All paths relative to the feature worktree root.

| File | Role in this change |
|---|---|
| `internal/gatedrive/admission.go` | Slot reserve; first-admission legacy inventory (`inventoryLegacyDrives`); stale released-slot `RunEpochID` handling; stored-identity slot addressing. |
| `internal/gatedrive/history.go` | `classifyLegacyDrive` relevance boundary; `LegacyFinding` gains `Worktree`; `HistoryCleanupOutcome` doc correction. |
| `internal/gatedrive/reconcile.go` | `accountEpochLaunches` census: terminal-before-linkage, positive-reference attribution, empty-worktree rule, superseded-epoch slot check. |
| `internal/gatedrive/driver.go` | `Admit` pre-reserve short-circuits; finished-incumbent reconciliation; `releaseAdmissionIfProven` audit; `recordedDoc` RunRoot exposure; discarded release errors. |
| `internal/gatedrive/admission_retire.go` | `RetireWorktreeExecutionEpoch` stored-identity addressing. |
| `internal/app/gate.go` | Raw path: `rawStaleEpochRefusal`, legacy summary on raw refusals. |
| `internal/app/gate_drive.go` | `startBudgetedBuild` advisory refusal; legacy-inventory refusal message; `incumbentRemedyMessage` vacuous `DriveID` branch. |
| `internal/app/rungate_fence.go` | `findEpochByWorktree` deterministic owner selection; slot-named-epoch fail-closed rule in `admitWorkflowMutation`. |
| `internal/app/rungate_cancel.go` | `retireWorktreeSlotOwnership` consolidation; `verifyTerminalEpochQuiescence`, `repairTerminalEpoch`. |
| `internal/app/rungate_before.go` | `validateResumeQuiescence` consolidation onto shared retirement. |
| `internal/app/rungate_complete.go` | `accountCompletionSlot` durable-proof rule; `accountCompletionParticipants` execution proof. |
| `internal/app/rungate_epoch.go` | Epoch-settled lookup consumed by admission's stale-`RunEpochID` settlement. |
| `internal/app/finalize_rebase.go` | `mapDriveOutcome`/`removeGateRunRoot` cleanup ordering. |
| Sibling `_test.go` files of each of the above | Behavioral + paired safety tests; matrix fixtures. |

Tests live beside their packages, following the repo's existing convention (`admission_test.go`, `reconcile_test.go`, `rungate_cancel_test.go`, …). No new test framework; reuse the existing scripted `recoverySeam`/`ProcessSeam` fakes and `cancelSeams` fixtures already present in those files.

**Task order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12. Task 2 (stored-identity addressing) precedes the retirement/cancellation tasks that consume it. Each task is independently buildable and testable (the intermediate states are real states: e.g., after Task 1 admission is relevance-scoped while the census is not yet corrected — the two authorities are independent).

---

### Task 1: Relevance-scoped legacy inventory — unmatched history is diagnostic, not a veto

**Spec:** §1 (discovery vs required evidence), §2 (admission depends on the requested worktree), §6 (diagnostics carry the matched worktree). Grounding bullets #0375/ADR-0118 and #0428/ADR-0120.

**Files:**
- Modify: `internal/gatedrive/history.go` (`classifyLegacyDrive`, `LegacyFinding`, `HistoryCleanupOutcome` doc comment)
- Modify: `internal/gatedrive/admission.go` (`inventoryLegacyDrives`)
- Modify: `internal/app/gate_drive.go` (the refusal message at the `legacyInventoryLocator` consumer — the string `"historical gate drives block this admission; inspect or recover them with docket gate history cleanup (--dry-run first); …"`)
- Modify: `internal/app/gate.go` (`ReserveRawWorktreeExecution` call path: surface the legacy summary / matched worktree on raw refusals — the raw path currently drops the summary entirely)
- Test: `internal/gatedrive/history_test.go`, `internal/gatedrive/admission_test.go`, `internal/app/gate_drive_test.go`

**Interfaces:**
- Consumes: existing `classifyLegacyDrive(h historicalDrive, requestedWorktree string, proc recoverySeam, apply bool) LegacyFinding`, `inventoryLegacyDrives(worktreeRoot string, proc recoverySeam) (*LegacyHistorySummary, error)`.
- Produces: `LegacyFinding` gains a `Worktree string \`json:"worktree,omitempty"\`` field (the matched record's stored worktree identity — a report field, no persisted schema change; drive records are unchanged on disk). `inventoryLegacyDrives` keeps its signature but only returns a non-nil error for findings **positively bound to the requested worktree**; all other retained findings stay in the summary as diagnostics. Tasks 3 and 12 rely on this boundary.

**Behavioral rule to implement (the heart of the change):**

A historical record blocks the requested worktree's admission only when it establishes *same-worktree relevance*: its stored `WorktreePath` is nonempty and canonicalizes (or, when canonicalization fails because the path is gone, string-equals after `filepath.Clean`) to the requested canonical root. Everything else — a record bound to a different worktree, an unresolvable *other* path, an unreadable record, an unknown schema, a stray registry entry, a HALTED record with no run evidence for some other path — is a **diagnostic finding, never a refusal**. A positively-bound record keeps today's full assessment: nonterminal state, unprovable HALTED teardown, and probe errors on *this* worktree's history still retain and block (probe-error-is-not-clean-absence). This relies on ADR-0118's upgrade-quiescence contract for unreadable unreferenced records — an accepted residual, stated in a comment on `inventoryLegacyDrives`.

- [ ] **Step 1: Write the failing history-isolation tests.** In `internal/gatedrive/admission_test.go`, extend the existing legacy-inventory fixture pattern (see the current tests seeding drive records via the store and a scripted `recoverySeam`) with a matrix test:

```go
// TestFirstAdmissionUnrelatedHistoryIsDiagnostic seeds, for a DIFFERENT worktree
// (or with no resolvable binding at all), one record per class:
// PASSED, FAILED, HALTED (several causes; run dirs deleted), WAITING,
// removed-worktree (WorktreePath points at a deleted dir), unsupported-schema
// (SchemaVersion 99), malformed-record (invalid JSON), stray entry (a plain file
// in the registry root), and a schema-2 historical record. A fresh worktree's
// FIRST ReserveWorktreeExecution must SUCCEED, with the retained findings
// carried on the returned summary and no historical record rewritten or deleted.
func TestFirstAdmissionUnrelatedHistoryIsDiagnostic(t *testing.T) { … }

// TestFirstAdmissionOwnBoundHistoryStillBlocks seeds a WAITING (nonterminal)
// record and, separately, an unprovable-HALTED record whose WorktreePath IS the
// requested worktree. Reserve must refuse ErrUnresolvedExecution with the
// drive's locator, and the finding must carry Worktree == the requested root.
func TestFirstAdmissionOwnBoundHistoryStillBlocks(t *testing.T) { … }

// TestFirstAdmissionLiveIncumbentSameWorktreeBlocks: a HALTED record for the
// requested worktree whose scripted seam answers "live" must still refuse.
func TestFirstAdmissionLiveIncumbentSameWorktreeBlocks(t *testing.T) { … }
```

Assert the *mechanism*, not only the outcome: on refusal check `OwnershipError.Op == "inventory-legacy-drive-"+id` for the bound record; on success check `summary.Retained` still lists the unrelated damaged records (history stays inspectable) and that each seeded record file's bytes are unchanged.

- [ ] **Step 2: Run the new tests; confirm they fail** (`go test ./internal/gatedrive/ -run 'FirstAdmission' -count=1` — the diagnostic test fails today because any retained finding refuses).

- [ ] **Step 3: Implement.** In `classifyLegacyDrive`, make step 2 the relevance boundary: resolve the record's binding; when `requestedWorktree != ""` and the record does **not** positively match it, return `LegacyNonblocking` even for an unresolvable path — but record `f.Worktree = h.WorktreePath` and a reason distinguishing `"bound to a different worktree"` from `"worktree binding unresolvable (diagnostic)"`. Matching rule: `admissionKeyFor` success → compare canonical roots; `EvalSymlinks` failure → compare `filepath.Clean(h.WorktreePath) == requestedWorktree` (the stored identity was written canonical, so a byte-equal match is a positive match even after deletion; an unequal unresolvable path is NOT a match). In `inventoryLegacyDrives`, keep gathering every finding into the summary but set `firstLocator` (the refusal trigger) **only** for findings whose record is positively bound to `worktreeRoot` — for unreadable records this binding cannot be established, so they become diagnostic (`Reason` unchanged), with a comment naming the ADR-0118 quiescence contract as the accepted residual. The repository-wide `cleanupHistory` pass (`requestedWorktree==""`) is untouched: it still honestly retains everything.

- [ ] **Step 4: Fix the message sites.** Derive the population: `grep -rn "history cleanup\|Retained > 0\|historical gate drives" --include="*.go" .` (exclude `_test.go` initially, then update tests that pin the old wording). Known members: the `internal/app/gate_drive.go` message string above — reword so it names the *matched* worktree's obligation rather than implying every retained record blocks (e.g. `"a historical gate drive bound to this worktree blocks admission; inspect or recover it with docket gate history cleanup (--dry-run first)…"`), and the `HistoryCleanupOutcome` doc comment `"Retained > 0 means blockers remain visible rather than complete recovery"` → retained findings are honest history, only same-worktree-bound ones can refuse an admission. Also fix `incumbentRemedyMessage`'s vacuous `DriveID` branch note (spec "Admission slot facts": `admissionRecord.DriveID` is never set in production) — either delete the dead branch or comment it as historical-evidence-only; prefer deletion with a test proving no production writer sets it (`grep -rn "\.DriveID = " --include="*.go"`).

- [ ] **Step 5: Raw-path diagnostics.** In `internal/app/gate.go`'s raw admission failure path (where `ReserveRawWorktreeExecution`'s error is mapped through `mapAdmissionFailure`), surface the ownership error's `Legacy` summary and the incumbent/matched-worktree locator on the refusal instead of the current bare `unresolved-execution` with empty cause (spec §6: "Carry the matched worktree identity in the in-memory diagnostic and surface it on raw refusals too"). Add a test in `internal/app/gate_drive_test.go` (or the raw-launch test file the search reveals) asserting the raw refusal's `Cause` is non-empty and names the drive locator.

- [ ] **Step 6: Run the package tests** (`go test ./internal/gatedrive/ ./internal/app/ -count=1` — note `internal/app` may need its build-tagged partition per `tests/README.md`). Fix any test that enshrined the repository-wide veto **only alongside** the stronger paired test (AC8).

- [ ] **Step 7: Mutation-check.** Revert the `firstLocator` scoping (make every retained finding refuse again) and confirm `TestFirstAdmissionUnrelatedHistoryIsDiagnostic` goes red; delete the relevance comparison (treat everything as unbound) and confirm `TestFirstAdmissionOwnBoundHistoryStillBlocks` goes red. Restore.

- [ ] **Step 8: Commit** — `git add` the touched files; message `fix(gatedrive): scope the first-admission legacy inventory to the requested worktree (change 0446)`.

---

### Task 2: Stored-identity slot addressing for removed worktrees

**Spec:** §2 last paragraph ("Cancellation, retirement, and completion of an epoch whose worktree was removed must address the slot through the canonical identity already stored on the epoch/slot rather than re-canonicalizing").

**Files:**
- Modify: `internal/gatedrive/admission.go` (`admissionKeyFor` callers: `LoadWorktreeExecution`, `admissionCAS`), `internal/gatedrive/admission_retire.go` (`RetireWorktreeExecutionEpoch`)
- Test: `internal/gatedrive/admission_retire_test.go`, `internal/gatedrive/admission_test.go`

**Interfaces:**
- Produces: an unexported helper `(s *Store) admissionKeyStored(storedCanonical, op string) (string, error)` — accepts a path *already recorded as canonical* (epoch `Worktree` / slot `WorktreeRoot`): `EvalSymlinks` success → use the resolved root (unchanged behavior for live paths); `EvalSymlinks` failure with `fs.ErrNotExist` → key directly on `filepath.Clean(storedCanonical)` (the identity the slot was keyed under when created); any other canonicalization error → typed `ErrInvalidID` (a probe error is not clean absence). Read/CAS entry points that receive **stored** identities (`LoadWorktreeExecution`, `admissionCAS`, and therefore `RetireWorktreeExecutionEpoch`, release, mark-stopping) route through it. `ReserveWorktreeExecution`/`reserveWorktreeExecution` keep the strict `admissionKeyFor` — admitting a new execution still requires an existing, resolvable worktree.

**Safety note:** keying on the cleaned stored spelling is sound because `reserveWorktreeExecution` only ever persists `EvalSymlinks` output as `WorktreeRoot`, and epoch `Worktree` is bound canonical (`bindEpochWorktree` — verify with a read; if it stores non-canonical input, canonicalize at bind, not at read). A recreated path at the same location resolves to the same key either way.

- [ ] **Step 1: Write the failing test.** In `internal/gatedrive/admission_retire_test.go`:

```go
// TestRetireEpochAfterWorktreeRemoved reserves a slot for a real temp worktree
// with ReserveWorktreeExecutionForEpoch, releases it (ReleaseWorktreeExecution
// with the token), then os.RemoveAll's the worktree directory. Retire must
// still find the slot by the stored identity and clear RunEpochID.
func TestRetireEpochAfterWorktreeRemoved(t *testing.T) { … }

// TestLoadWorktreeExecutionAfterRemovalFindsSlot: same setup;
// LoadWorktreeExecution(storedRoot) returns the record, not ErrInvalidID.
// TestLoadWorktreeExecutionSymlinkAliasStillResolves: a live symlink alias of
// the worktree still resolves to the same slot (canonicalise-every-symlink-hop).
```

- [ ] **Step 2: Run; confirm failure** (today `admissionKeyFor` returns `ErrInvalidID` on the removed path).
- [ ] **Step 3: Implement `admissionKeyStored` and reroute the stored-identity entry points.** Keep `reserve` strict; add a doc comment stating the invariant ("stored identities are EvalSymlinks output by construction; a missing path keys on its recorded spelling — a canonicalization failure on a missing path is not proof the slot is absent").
- [ ] **Step 4: Run `go test ./internal/gatedrive/ -count=1`; all green.**
- [ ] **Step 5: Mutation-check:** make the `ErrNotExist` branch return `ErrInvalidID` again; both new tests go red. Restore.
- [ ] **Step 6: Commit** — `fix(gatedrive): address worktree slots by stored canonical identity after path removal (change 0446)`.

---### Task 3: Finished-incumbent reconciliation at the normal admission boundary

**Spec:** §3 entire; grounding bullet #0439. This is the largest task; its worker must read §3 verbatim first.

**Files:**
- Modify: `internal/gatedrive/admission.go` (`reserveWorktreeExecution`), `internal/gatedrive/driver.go` (new `reconcileFinishedIncumbent`; `Admit`/`admitScopedWorktree` flow), `internal/app/gate_drive.go` (`startBudgetedBuild`), `internal/app/gate.go` (`rawStaleEpochRefusal`, raw completed-slot release)
- Test: `internal/gatedrive/admission_test.go`, `internal/gatedrive/driver_test.go`, `internal/app/gate_drive_test.go`

**Interfaces:**
- Produces: `(d *Driver) reconcileFinishedIncumbent(worktreeRoot string) (settled bool, finding string, err error)` — inspects the exact incumbent slot **outside** the slot lock (probe), then applies release under the existing CAS with the *expected* reservation token and state; returns `settled=true` only when the release write succeeded (or the slot became admissible concurrently). The reserve path then simply retries once. Consumed by `Admit` (before mapping a busy/unresolved refusal), by the raw launch path, and — read-only — by `WorktreeAdmissionRefusal`'s callers.

**Behavioral rules (from §3, condensed to the decision table the code implements):**

| Incumbent shape | Proof required to settle | Action |
|---|---|---|
| slot executing/stopping/unresolved, `DriveID`-less scoped/scopeless slot whose **current-token** drive (`ReservationToken == drive.AdmissionToken`, found via the drive named by the slot's scope or by scanning for the token match) is PASSED/FAILED | the drive record itself (supervisor-committed) | `ReleaseWorktreeExecution(root, slot.ReservationToken)`, continue admission |
| raw slot, run proven terminal (`proc.Observe`/`ClassifyRun` → `stopProvesTeardown`) | positive process teardown proof | release, continue (this deliberately narrows ADR-0118's explicit-stop-only raw release — record in Task 11's ADR) |
| reserved-never-confirmed slot | `proc.ResolveReservation` → `never-launched` **and** the nonblocking relaunch claim (`tryRelaunchClaim`) acquired for the slot's drive (a delayed `StartAdmitted` ticket retains launch authority — settle through the fenced/terminal path first) | settle drive terminal via existing path, release, continue |
| HALTED drive, no positive teardown proof | — | refuse (unchanged); never released on the label |
| busy claimant, live process, unresolved establishment, pending relaunch, failed release write | — | refuse with the existing typed error + finding; a failed release write is **never** reported as successful admission |
| successor won the race (token changed between probe and CAS) | — | re-evaluate that actual incumbent once, then refuse or settle; never clear a slot using the newly observed successor token |

- [ ] **Step 1: Derive the pre-reserve refusal population.** `grep -rn "WorktreeAdmissionRefusal\|ReserveWorktreeExecution\|ReserveRawWorktreeExecution\|ReserveWorktreeExecutionForEpoch\|rawStaleEpochRefusal" --include="*.go" . | grep -v _test` — record the caller list in the task's commit message body. Known members: `startBudgetedBuild` (advisory refusal before `suiteBudgetPrecheck`/`Admit`), `GateLaunch` → `rawStaleEpochRefusal` → `ReserveRawWorktreeExecution`, `Driver.Admit` → `epochGated` → `admitScopeless`/`admitScoped` → `admitScopedWorktree` → `reserveWorktreeExecution`. Every member must reach reconciliation before a busy refusal is final; request-validation checks (bad command, budget, fingerprint) may keep refusing first.
- [ ] **Step 2: Write the failing tests** (driver level, using the existing scripted `ProcessSeam` in `driver_test.go`):

```go
// TestAdmitSettlesFinishedIncumbent: drive a scopeless start to PASSED but
// interrupt the release (simulate by manually rewriting the slot back to
// "executing" with its token, or by a seam that fails the first release), then
// call Admit for a NEW start on the same worktree. It must settle the finished
// incumbent, admit once, and not send any stop signal (assert the seam recorded
// zero Stop calls for the settle leg).
// TestAdmitRefusesLiveIncumbent: seam answers live → typed ErrWorktreeBusy.
// TestAdmitHaltedNoProofNeverReleases: HALTED incumbent, seam errors → refusal,
// slot state unchanged.
// TestAdmitFailedReleaseWriteRefuses: release write forced to fail (read-only
// record file) → refusal, never a success document.
// TestRawStartSettlesFinishedRawIncumbent (internal/app): a completed raw run's
// occupied slot admits the next GateLaunch/gate-drive start without a manual
// GateStop.
// TestBudgetedBuildReconcilesBeforeRefusal (internal/app): startBudgetedBuild
// with a finished incumbent starts once and charges exactly one suite attempt.
```

- [ ] **Step 3: Run; confirm failures.**
- [ ] **Step 4: Implement `reconcileFinishedIncumbent` in `driver.go`** following the decision table; wire it: (a) in `Admit`, on an `ErrWorktreeBusy`/`ErrUnresolvedExecution` from the reserve, run reconciliation and retry the reserve **once**; (b) in `startBudgetedBuild`, change the advisory `WorktreeAdmissionRefusal` refusal to defer to the authoritative `Admit` when the slot *might* be a finished incumbent — simplest correct form: drop the early return for busy/unresolved and let `Admit` (which now reconciles) decide, keeping the advisory check only for the no-charge fast path it was built for (admission still precedes `reserveBuildSuiteAttempt`, so the no-charge property is preserved by the existing `Admit`-before-charge order — verify by reading `startBudgetedBuild` and keep charge-after-admit); (c) in the raw path, run reconciliation before `rawStaleEpochRefusal`'s refusal and before `ReserveRawWorktreeExecution`. Reuse only existing helpers: `LoadWorktreeExecution`, `store.Load`, `tryRelaunchClaim`, `proc.ResolveReservation`/`Observe`/`ClassifyRun`, `stopProvesTeardown`, `ReleaseWorktreeExecution`, `RetireWorktreeExecutionEpoch`. Probe outside the slot lock; apply under `admissionCAS` with the expected token.
- [ ] **Step 5: Concurrency tests** (AC7, in `driver_concurrency_test.go` following its existing race-test pattern): race two `Admit`s over one finished incumbent → exactly one launch; race reconciliation with a successor reservation → the successor is untouched and the loser re-evaluates once.
- [ ] **Step 6: Run `go test ./internal/gatedrive/ ./internal/app/ -count=1 -race`.**
- [ ] **Step 7: Mutation-check (AC8):** bypass the reconciliation from `startBudgetedBuild` (restore the unconditional early refusal) → `TestBudgetedBuildReconcilesBeforeRefusal` red; accept HALTED as release proof (add `|| out == HALTED` to the settle predicate) → `TestAdmitHaltedNoProofNeverReleases` red. Restore.
- [ ] **Step 8: Commit** — `fix(gatedrive): reconcile a proven-finished incumbent at the admission boundary (change 0446)`, body listing the derived caller population.

---

### Task 4: Stale released-slot RunEpochID settlement at admission

**Spec:** §2 ("Its surviving RunEpochID is not a live owner either… settled through the existing exact-token retirement, not refused with ErrStaleRunEpoch"), §5 last-but-one paragraph.

**Files:**
- Modify: `internal/gatedrive/admission.go` (`reserveWorktreeExecution` run-epoch fence), `internal/gatedrive/driver.go` (seam field), `internal/app/rungate_epoch.go` + the wiring site where `SetEpochRevokedResolver` is installed (find via `grep -rn "SetEpochRevokedResolver\|epochRevokedResolver" --include="*.go"`)
- Test: `internal/gatedrive/admission_test.go`, `internal/app/rungate_epoch_test.go`

**Interfaces:**
- Produces: a driver/store seam `EpochSettledFunc func(epochID string) (settled bool, err error)` — answers whether an epoch is durably **completed**, or **cancelled with confirmed accounting** (terminal state in the epoch record; readable). Wired from the app layer beside the existing `epochRevokedResolver` using `findEpochByID` over the rungate root. `reserveWorktreeExecution`'s fence changes: a **released** slot whose `RunEpochID` differs from the incoming epoch consults the seam; `settled=true` → retire via `RetireWorktreeExecutionEpoch(root, slot.RunEpochID, slot.ReservationToken)` and readmit; `settled=false`, seam absent, or seam error → today's `ErrStaleRunEpoch` (fail closed, live between-drive ownership preserved). A non-released slot never consults the seam — the epoch fence on busy slots is untouched.

- [ ] **Step 1: Write the failing tests:**

```go
// TestReleasedSlotWithCompletedEpochReadmits: reserve-for-epoch E1, release,
// script the seam settled(E1)=true; a fresh reserve (epoch E2 or raw) succeeds
// and the slot's RunEpochID afterwards is E2/"".
// TestReleasedSlotWithLiveEpochStillFenced: seam settled=false → ErrStaleRunEpoch.
// TestReleasedSlotSeamErrorFailsClosed: seam error → ErrStaleRunEpoch, slot untouched.
// TestBusySlotNeverConsultsSettledSeam: executing slot + different epoch →
// ErrStaleRunEpoch and the scripted seam records zero calls.
// TestRecreatedWorktreePathInheritsAndSettles (AC2): delete + recreate the
// worktree dir at the same path with a properly released slot naming a
// settled epoch; the next reserve succeeds (no legacy inventory re-run —
// assert LegacyInventoried stays false on the new record).
```

- [ ] **Step 2: Run; confirm failures** (today the released-slot fence refuses unconditionally on epoch mismatch — verify against the `stored.Record.RunEpochID != rec.RunEpochID` check that precedes the state switch).
- [ ] **Step 3: Implement** the seam field + fence change in `reserveWorktreeExecution` (only the `admissionReleased` arm consults it, keeping the check-before-state-switch shape for busy slots), the app-layer resolver (epoch state `completed`/`cancelled` from the durable record — do **not** treat `cancelling` as settled), and its wiring at every `NewDriver`/store construction site the grep reveals. Do not ask a user to cancel a completed run: the seam accepting `completed` is exactly this rule.
- [ ] **Step 4: Run the two packages' tests; green.**
- [ ] **Step 5: Mutation-check:** make the seam treat `cancelling` as settled → `TestReleasedSlotWithLiveEpochStillFenced` (scripted with a cancelling epoch) red; drop the seam consultation and always readmit → the fence test red. Restore.
- [ ] **Step 6: Commit** — `fix(gatedrive): settle a released slot's terminal RunEpochID at admission instead of refusing (change 0446)`.

---

### Task 5: Epoch launch census — attribute obligations, don't inherit all history

**Spec:** §4 entire; grounding bullet #0437.

**Files:**
- Modify: `internal/gatedrive/reconcile.go` (`accountEpochLaunches`, `reconcileEpochDrive`), `internal/gatedrive/driver.go` (only if `resolveDriveEpoch` needs a census-mode sibling — launch-authority behavior is untouched)
- Test: `internal/gatedrive/reconcile_test.go`

**Interfaces:**
- Consumes: `store.Load`, `loadHistoricalDrive` (Task 1's package), `resolveDriveEpoch`, `LoadScope`, `LoadWorktreeExecution`, `tryRelaunchClaim`.
- Produces: same exported signatures (`ReconcileEpochLaunches`, `ObserveEpochLaunches`); changed semantics consumed by Tasks 6 and 8. New finding vocabulary: unreferenced unreadable/unlinked records emit informational `history-unattributed:<id>` findings that do **not** clear `Accounted`.

**Behavioral rules:**

1. **Terminal before linkage:** check `isTerminalOutcome(rec.LastOutcome)` on the walked record **before** `resolveDriveEpoch` — a terminal drive's launch axis is settled regardless of lost linkage (HALTED included; teardown stays the slot/participant checks' job — comment this, citing §4's "no blanket trust in HALTED" reconciliation).
2. **Unreadable via `store.Load`:** retry through `loadHistoricalDrive`; a supported schema-2 historical record with a terminal outcome is settled history; nonterminal or still-unreadable falls to rule 3.
3. **Positive reference decides blocking:** an unreadable or linkage-lost record blocks (`record-unreadable:`/`linkage-unresolved:`, `Accounted=false`) only when a **current reference names it**: the target worktree's slot (`slot.ReservationToken == rec.AdmissionToken`, or the slot's scope's current/pending drive ids include it) or a scope carrying `RunEpochID == epochID` names it as current/pending. Otherwise it is `history-unattributed:<id>`, informational. Resolving the reference never depends on reading the unreadable record itself: for an unreadable record the reference is established from the slot/scope side (scope `Current`/`Pending` drive-id fields — read `scope.go` for the exact field names before coding).
4. **Corrupt required scope:** a scope named by the target epoch's current slot that cannot be read keeps the epoch unaccounted (unchanged fail-closed behavior; add the explicit test).
5. **Empty worktree is not vacuous:** when `epochID != ""` and `worktreeRoot == ""` (a superseded epoch), do **not** return accounted early: still walk the registry and account scope-linked drives by `RunEpochID`; skip only the slot-side checks — the caller supplies the replacement's worktree for those (Task 6 threads it; until then the slot check is skipped with a `slot-check-deferred` finding that keeps behavior honest — remove the finding in Task 6).
6. **Token rotation:** an older nonterminal scopeless drive whose `AdmissionToken` no longer matches the slot's token is historical **only if every launch/relaunch path verifies the token** — verify by search (`grep -rn "verifyAdmissionToken\|AdmissionToken" --include="*.go" internal/gatedrive/ | grep -v _test`) and record the result in the commit body; any unverified launch path keeps such drives candidates.

- [ ] **Step 1: Write the failing tests** in `reconcile_test.go` (reuse its existing driver+seam fixture):

```go
// TestCensusTerminalSettledBeforeLinkage: a HALTED scoped drive whose scope file
// is deleted → accounted (settled by outcome), no linkage-unresolved finding.
// TestCensusUnreferencedCorruptRecordInformational: corrupt an unrelated drive
// record → Accounted=true, finding "history-unattributed:<id>".
// TestCensusReferencedCorruptRecordBlocks (strengthened 0437 regression, spec §4):
// create a REAL admitted drive whose AdmissionToken matches the current slot,
// then corrupt its scope (scoped case) or the record (scopeless case) →
// Accounted=false with the exact locator.
// TestCensusSchema2HistoricalTerminalSettles: seed a schema-2 record with
// LastOutcome PASSED (bytes written directly, mirroring history_test.go's
// fixture) → accounted, not record-unreadable.
// TestCensusSupersededEpochStillEnumerates: worktreeRoot "", epochID E with an
// unaccounted scope-linked nonterminal drive → Accounted=false (today's code
// returns accounted vacuously — this is AC5's superseded-branch case).
// TestCensusRotatedTokenScopelessIsHistorical: scopeless nonterminal drive whose
// token the slot no longer holds → informational, current token holder still
// accounted separately.
```

- [ ] **Step 2: Run; confirm each fails for the stated reason** (in particular the vacuous `worktreeRoot == ""` early return and the unconditional `linkage-unresolved`).
- [ ] **Step 3: Implement** rules 1–6 in `accountEpochLaunches`. Keep `resolveDriveEpoch` itself untouched (launch authority unchanged — spec: "The correction is to census attribution, not to launch authority"); the census gains its own attribution wrapper that first tries `resolveDriveEpoch` and, on `ok=false`, applies the positive-reference test before deciding blocking vs informational.
- [ ] **Step 4: Run `go test ./internal/gatedrive/ -count=1`; green, including the untouched claimant/delayed-ticket/exact-reservation tests from #0437.**
- [ ] **Step 5: Mutation-check (AC8):** restore linkage-before-terminal ordering → `TestCensusTerminalSettledBeforeLinkage` red; restore the empty-worktree accounted shortcut → `TestCensusSupersededEpochStillEnumerates` red; make every unreadable record block again → `TestCensusUnreferencedCorruptRecordInformational` red. Restore.
- [ ] **Step 6: Commit** — `fix(gatedrive): epoch launch census attributes obligations through current references (change 0446)`, body recording the launch-path token-verification search result.

---

### Task 6: Consolidate slot retirement; thread the superseded epoch's replacement worktree

**Spec:** §4 final paragraph, §5 ("Reconcile a stale RunEpochID on a released slot using existing exact-token retirement"); AC5.

**Files:**
- Modify: `internal/app/rungate_cancel.go` (`retireWorktreeSlotOwnership`, `repairTerminalEpoch`, `verifyTerminalEpochQuiescence`), `internal/app/rungate_before.go` (`validateResumeQuiescence`)
- Test: `internal/app/rungate_cancel_test.go`, `internal/app/rungate_before_resume_test.go`

**Interfaces:**
- Consumes: Task 2's stored-identity addressing (retirement of a removed worktree's slot), Task 5's census semantics.
- Produces: `retireWorktreeSlotOwnership(seams cancelSeams, ep EpochRecord) (bool, string)` becomes the **single** retirement implementation; `repairTerminalEpoch` and `validateResumeQuiescence` call it instead of their inline copies. One defined successor outcome across all three sites: the finding `"slot-replaced-by-successor"` with the operation still accounted/quiescent (successor untouched). For a **superseded** epoch (empty `ep.Worktree`), quiescence checks resolve the replacement epoch's worktree (follow `ReplacementReserved` from the epoch record via `findEpochByID`) and confirm the predecessor's token/epoch no longer hold that slot; the launch census is called with the predecessor's `EpochID` and the replacement's worktree (closing Task 5's `slot-check-deferred`).

- [ ] **Step 1: Diff the three retirement copies against each other first** (consolidation-flattens-caller-variance): read `retireWorktreeSlotOwnership`, `repairTerminalEpoch`'s inline block, and `validateResumeQuiescence`'s inline block side by side; the known variance is the raced-CAS re-read outcome (`slot-replaced-by-successor` vs already-cancelled vs neutral) — that variance is the defect being unified, so it flattens *deliberately*; document any **other** variance found before folding it in.
- [ ] **Step 2: Write the failing tests:**

```go
// TestRetirementSitesConverge: for each of runCancel / repairTerminalEpoch /
// validateResumeQuiescence, seed a released slot whose RunEpochID a SUCCESSOR
// now holds; assert all three report the successor finding, leave the slot
// byte-identical, and none regresses the epoch or refuses.
// TestResumeSupersededBranchAccountsScopeLinkedDrives (AC5): supersede an epoch
// (empty Worktree), add an unaccounted scope-linked nonterminal drive for it,
// arm resume → refused with the launch finding, not "accounted".
// TestRepairTerminalEpochRemovedWorktree: terminal epoch whose worktree dir was
// removed; repair retires the slot via stored identity (Task 2) and reports
// already-cancelled/cancelled, not slot-unreadable.
```

- [ ] **Step 3: Run; confirm failures** (the superseded branch today skips both launch worktree and slot checks; the copies report divergent successor outcomes).
- [ ] **Step 4: Implement** the consolidation and the replacement-worktree threading. `verifyTerminalEpochQuiescence` gains the replacement resolution for superseded epochs; `repairTerminalEpoch` and `validateResumeQuiescence` delete their inline classify/release/retire/re-read blocks in favor of `retireWorktreeSlotOwnership`.
- [ ] **Step 5: Run `go test ./internal/app/ -count=1` (with the package's build-tag partition per tests/README.md); green, including all existing cancel/resume tests.**
- [ ] **Step 6: Mutation-check:** re-inline a divergent successor outcome in one site → `TestRetirementSitesConverge` red; restore the superseded early-skip → the AC5 test red. Restore.
- [ ] **Step 7: Commit** — `fix(app): one slot-retirement path for cancel, terminal repair, and resume (change 0446)`.

---

### Task 7: Deterministic worktree owner selection and the slot-named-epoch rule

**Spec:** §5 selection rules; §1 ("when the requested worktree's slot names a RunEpochID, ambient owner lookup must resolve that epoch to a readable record"); AC3, AC6.

**Files:**
- Modify: `internal/app/rungate_fence.go` (`findEpochByWorktree`, `admitWorkflowMutation`)
- Test: `internal/app/rungate_fence_test.go`

**Interfaces:**
- Produces: `findEpochByWorktree(repoDir, canon string) (gateKey string, found bool, err error)` — same signature, deterministic semantics: collect **all** matching non-completed epochs bound to `canon`; exactly one active/completing match → it wins regardless of directory order and of cancelled/cancelling records; no active match but ≥1 cancelled/cancelling non-superseded match → return one (they all fence identically via `ErrRunCancelled`; pick the deterministic first by sorted key for stability); ≥2 active/completing matches → return a typed error (fail closed locally, the ambiguous-contradiction arm — reuse the shape `FindEpochByChange` uses for `ErrEpochAmbiguous`; mint `ErrEpochOwnerAmbiguous` in `rungate_epoch.go` if no fitting sentinel exists); completed never matches (unchanged). Superseded epochs cannot match because supersession clears `Worktree` (verify `SupersedeCancelledEpoch` still does; do not re-filter). `admitWorkflowMutation` additionally enforces: when `findEpochByWorktree` finds nothing, load the worktree's slot (open the gatedrive store from the repo's common dir — mirror how `productionCancelSeams` opens it); a readable slot with nonempty `RunEpochID` that `findEpochByID` cannot resolve to a readable epoch → **refuse** with a locator naming the epoch id and worktree (fail closed) instead of admitting unfenced. An absent/unreadable slot or empty `RunEpochID` keeps today's unfenced admit; the caller's own uncanonicalizable `repoDir` keeps its fail-open (spec accepted residual — comment it and cite the spec's decision).

- [ ] **Step 1: Write the failing tests** (fixture: write epoch.json files directly under a temp rungate root, as `rungate_fence_test.go` / `rungate_epoch_test.go` already do):

```go
// TestOwnerSelectionActiveBeatsCancelledRegardlessOfOrder: cancelled epoch in a
// gate-key dir sorting BEFORE the active epoch's dir, both bound to one path →
// the active key is returned. (Today first-match returns the cancelled one.)
// TestOwnerSelectionSoleCancelledStillFences: only a cancelled non-superseded
// epoch → found, and admitWorkflowMutation returns ErrRunCancelled.
// TestOwnerSelectionTwoActiveOwnersAmbiguous: two active epochs, one path →
// typed ambiguity error; mutation refused, not silently one-of.
// TestOwnerSelectionCompletedNeverOwns: completed epoch only → not found.
// TestSlotNamedEpochUnreadableRefusesLocally (AC3): slot names epoch E; no
// readable epoch record carries E (corrupt it, and separately chmod 000 it) →
// admitWorkflowMutation fails closed with E in the error; the same corruption
// of an UNREFERENCED epoch record does not affect a different worktree.
```

- [ ] **Step 2: Run; confirm failures.**
- [ ] **Step 3: Implement** the two-pass selection and the slot-named-epoch check. Keep the epoch-carrying fences (`epochLaunchGate`, `epochRevokedResolver`, keyed cancel/completion/participant/fence ops) untouched — add `TestEpochCarryingFencesUnchangedByOwnerSelection`: after a new owner binds the path, a stale cancelled epoch's launch-gate call still refuses (AC6's separate proof).
- [ ] **Step 4: Run the package tests; green.**
- [ ] **Step 5: Mutation-check (AC8):** restore first-match selection → the order test red (seed the order so first-match picks wrong); drop the cancelled-sole-owner arm (skip terminal epochs entirely) → `TestOwnerSelectionSoleCancelledStillFences` red; restore skip-on-unreadable for the slot-named epoch → the AC3 test red. Restore.
- [ ] **Step 6: Commit** — `fix(app): deterministic worktree owner selection; slot-named epoch must resolve (change 0446)`.

---

### Task 8: Durable completion facts survive scratch cleanup

**Spec:** §5 ("A durably released execution … must not become unknown merely because optional scratch evidence later disappears"); AC6.

**Files:**
- Modify: `internal/app/rungate_complete.go` (`accountCompletionSlot`, `accountCompletionParticipants`)
- Test: `internal/app/rungate_complete_test.go`

**Interfaces:**
- Consumes: `classifySlotOwnership`, `observeTerminalProof`, gatedrive store loads.
- Produces: `accountCompletionSlot` drops the process re-observation of a matching **released** owned slot (the `slot.RawRunDir != "" → observeTerminalProof` leg on `slotOwned`+released): a released owned slot is itself the sufficient durable proof of *that slot's* execution; the re-observation proved nothing about unaccounted participants or launches and turned deleted scratch into a false blocker. `accountCompletionParticipants` accepts, for an **execution** participant, an exact matching released slot record or a persisted PASSED/FAILED drive record as terminal proof when direct observation fails on missing evidence; native participants keep their recorded terminal-status requirement; HALTED is never accepted as that proof.

- [ ] **Step 1: Write the failing tests:**

```go
// TestCompletionSlotReleasedOwnedNoReobservation: released slotOwned slot with a
// RawRunDir whose directory is DELETED → accounted (today: process-unobserved
// blocker). Also assert the observer seam records zero calls for this leg.
// TestCompletionParticipantDurableProof: execution participant whose run dir is
// gone but whose handle matches the released slot's RawRunDir (and, second case,
// a PASSED drive record) → accounted; a HALTED drive record → NOT accepted.
// TestCompletionUnreleasedOwnedSlotStillBlocks: slotOwned + state executing →
// blocked (unchanged safety).
// TestCompleteThenScratchCleanupThenFinalizeAdmits (AC6, cross-task, may live in
// an integration-style test file): complete a run, close out ownership, remove
// scratch, seed a cancelled never-superseded predecessor epoch bound to the same
// path in a dir that sorts first, plus unrelated damaged history → finalize's
// gate admission on the same worktree succeeds via the production paths.
```

- [ ] **Step 2: Run; confirm failures.**
- [ ] **Step 3: Implement**; keep genuinely pending journals and contradictory current references blocking (existing arms untouched); do not synthesize terminal evidence from missing files — the accepted proof is the *existing durable record*, and its absence still blocks.
- [ ] **Step 4: Run the package tests; green.**
- [ ] **Step 5: Mutation-check:** accept HALTED as participant execution proof → the durable-proof test red; restore the released-slot re-observation → the no-reobservation test red. Restore.
- [ ] **Step 6: Commit** — `fix(app): durable release and completion facts are sufficient closeout proof (change 0446)`.

---

### Task 9: Terminal-result and cleanup audit — release errors, RunRoot exposure, cleanup ordering

**Spec:** §5 final paragraph ("Audit the existing terminal-result and cleanup paths as part of this change").

**Files:**
- Modify: `internal/gatedrive/driver.go` (`driveAndPersistClaim`'s two `_ = d.releaseAdmissionIfProven(cur)` sites and `Advance`'s, `recordedDoc`, `releaseAdmissionIfProven` stale comment), `internal/app/finalize_rebase.go` (`mapDriveOutcome` / `removeGateRunRoot` ordering)
- Test: `internal/gatedrive/driver_test.go` or `driver_faults_test.go`, `internal/app/finalize_rebase_test.go`

**Interfaces:**
- Produces: (a) release errors surface — `driveAndPersistClaim`/`Advance` capture the `releaseAdmissionIfProven` error and carry a bounded finding on the returned `DriveDoc` (add a `ReleaseFinding string` field to `DriveDoc`, or reuse an existing findings channel if `DriveDoc` has one — read the struct first; never silently drop). (b) `recordedDoc` exposes `RunRoot` for a HALTED outcome only when the slot's release/teardown evidence is settled: thread a boolean from the caller (which just ran `releaseAdmissionIfProven`) — `recordedDoc` keeps its signature and a sibling `recordedDocWithRoot(id, ownerGen, rec, exposeRoot bool)` carries the gate; PASSED/FAILED exposure unchanged. (c) `releaseAdmissionIfProven`'s doc comment "A drive with no admission token (a scopeless start) has no slot to free" is corrected — `admitScopeless` sets a token; the empty-token case is a legacy/raw-history record. (d) Finalize: `mapDriveOutcome`'s `removeGateRunRoot(doc.RunRoot)` and the caller's orphaned-root removal run only after the terminal document's release/persistence evidence succeeded; on a failed release, retain the root and surface a local persistence/teardown finding.

- [ ] **Step 1: Read `DriveDoc` (in `internal/gatedrive/drive.go` or `driver.go`) and `mapDriveOutcome` end-to-end before designing the finding channel.**
- [ ] **Step 2: Write the failing tests:**

```go
// TestReleaseFailureSurfacesOnTerminalDoc: force ReleaseWorktreeExecution to
// fail (read-only slot record) after a PASSED slice → the returned doc carries
// the release finding and the slot is NOT freed.
// TestHaltedDocWithholdsRunRootWhenSlotUnsettled: HALTED with slot marked
// stopping/unresolved → doc.RunRoot == "" ; with proven release → RunRoot set.
// TestFinalizeCleanupRetainsRootOnUnsettledRelease (internal/app): interrupt at
// the release boundary; the run root survives and the finding is reported —
// "test interruption at this boundary so normal cleanup cannot recreate the
// reported time bomb" (spec).
```

- [ ] **Step 3: Run; confirm failures. Step 4: Implement (a)–(d). Step 5: package tests green (`-count=1`).**
- [ ] **Step 6: Mutation-check:** restore `_ =` on one release site → the surfacing test red; expose RunRoot unconditionally on terminal → the withholding test red. Restore.
- [ ] **Step 7: Commit** — `fix(gatedrive,app): terminal cleanup waits for settled release evidence (change 0446)`.

---

### Task 10: Acceptance matrices — quarantine fixture, generations, corruption, concurrency sweep

**Spec:** AC1, AC2, AC3, AC7 residue not already covered task-by-task; the quarantine fixture note.

**Files:**
- Test: `internal/gatedrive/admission_test.go`, `internal/gatedrive/integration_sequence_test.go` (follow its existing end-to-end pattern), `internal/app/rungate_cancel_test.go`
- Create: a fixture copy of the sanitized quarantine record inside the test tree (e.g. `internal/gatedrive/testdata/quarantine-0368-record.json`)

**Interfaces:** consumes everything above; produces no new production symbols.

- [ ] **Step 1: Import the quarantine fixture.** Copy `/Users/homer/dev/docket-quarantine/gate-drives/b66ce1405cd813e5519c344360f61bd4/record.json` into `testdata/` **verbatim** (never into any live registry; the original and its epoch backup are preserved untouched). Add `TestQuarantinedHaltedRecordIsNonblockingForUnrelatedWorktree`: seed it into an isolated fixture store, admit a fresh unrelated worktree → success; and `…StillInspectable`: `CleanupHistory` still reports it retained. Note in the test comment: the manually edited live cancelled state is not a test oracle; only this frozen record is.
- [ ] **Step 2: AC1 mixed-orderings sweep:** one table-driven test seeding *many* mixed records (every class from Task 1's matrix, several of each, shuffled insertion order, before/after deleting scratch files) asserting: unrelated worktree admits; no record's bytes change; `cleanupHistory` counts partition exactly.
- [ ] **Step 3: AC2 same-worktree generations:** end-to-end in `integration_sequence_test.go`: run drive→release→re-admit cycles on one path; delete/recreate the path (Task 4's test extended through the driver); symlink alias admission; several superseded epochs then a fresh run; cancellation/retirement/completion of a removed-worktree epoch (Task 2/6 joins); and the live-incumbent counter-case (a genuinely executing slot still refuses).
- [ ] **Step 4: AC3 reserved-before-launch / replacement-before-attach windows:** corrupt the exact drive named by a current reservation during the `Admit`→`StartAdmitted` window and during a reserved relaunch → the target refuses with the right locator while a companion worktree's start succeeds in the same test.
- [ ] **Step 5: AC7 sweep:** in `driver_concurrency_test.go`, race same-worktree starts across scoped/scopeless/raw and a symlink alias → exactly one launch; independent worktrees progress concurrently; race cancellation with a delayed `StartAdmitted` → pre-admitted work never first appears after `cancelled` is reported (extend the existing 0437 race tests rather than duplicating their harness).
- [ ] **Step 6: Run `go test ./internal/gatedrive/ ./internal/app/ -count=1 -race`; green.**
- [ ] **Step 7: Commit** — `test(gatedrive,app): acceptance matrices for history isolation, generations, corruption, and races (change 0446)`.

---

### Task 11: ADRs — record the changed authority boundaries

**Spec:** "Architecture and scope limits"; §2, §3.

**Files:** ADRs are authored through the **docket-adr workflow** (dispatch the `docket-adr` agent per AGENTS.md "dispatch, don't run inline"), which owns `docs/adrs/` on the metadata branch — not hand-written files in the feature tree.

- [ ] **Step 1: Dispatch docket-adr** to record one new ADR: *historical gate discovery has no global veto* — relevance to the requested worktree/run decides whether uncertainty blocks; authoritative local uncertainty stays protected; positive teardown can settle a finished raw incumbent during admission (the deliberate narrowing of ADR-0118's explicit-stop-only raw release). The ADR **supersedes the conflicting clauses** of ADR-0118 (worktree-wide admission inventory scope; raw explicit-stop-only release) and ADR-0120 (an unresolvable historical path as possible ownership of every worktree), preserving their other guarantees and ADR-0087/0095 process-evidence rules; ADR-0124's observation-only closeout stays binding and is cited, not changed. Provide the agent the change id (446) so the change file's `adrs:` list is extended and the supersession is linked both ways.
- [ ] **Step 2: Verify** the ADR ids land in the change file's `adrs:` frontmatter (the docket-adr workflow does this; confirm, don't hand-edit).
- [ ] **Step 3:** No feature-branch commit unless the workflow requires an index touch in-tree; otherwise this task produces metadata-branch artifacts only.

---

### Task 12: Whole-suite gate and budget review

**Spec:** AC9; AGENTS.md "Run the whole suite at the build gate".

- [ ] **Step 1: Run the full suite** through the resolved `build.test_command` (read it from config — never a second copy), entered from source via the Go runner (`internal/suiterunner`); under docket-build this is the standard end-of-build gate slice-driven via `docket gate drive` — do not substitute a hand-rolled `go test ./...`.
- [ ] **Step 2: Read the budget report even on green:** any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding; a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach — the new race/matrix tests in Tasks 3, 5, 10 are the likely candidates; confirm serially per `tests/README.md` before acting.
- [ ] **Step 3: Sweep the branch against its own thesis** (fix-reintroduces-its-own-defect-class): grep the branch's *additions* for the defect class — any new code path that folds a probe **error** into the same branch as clean absence, or that refuses on a record it cannot positively bind to the target (`git diff main... | grep -n "err != nil"` reviewed by hand around every new read). Fix in place.
- [ ] **Step 4: Commit** any gate-driven fixes; the results file and evidence recording are owned by the enclosing docket-implement-next workflow, not this plan.

---

## Self-Review (performed)

**Spec coverage:** §1 → Tasks 1, 5, 7. §2 → Tasks 1, 2, 4. §3 → Task 3. §4 → Tasks 5, 6. §5 → Tasks 7, 8, 9. §6 → Tasks 1 (messages, raw-path locator), 3 (typed refusals). AC1 → Tasks 1, 10. AC2 → Tasks 2, 4, 10. AC3 → Tasks 5, 7, 10. AC4 → Task 3. AC5 → Task 6. AC6 → Tasks 7, 8. AC7 → Tasks 3, 10. AC8 → per-task mutation steps. AC9 → Task 12. ADRs → Task 11. Change-file item "verify change 444's recovery after install" is post-merge human/workflow verification, outside the build (record it in the results file's Verify-human section — noted here for the results author).

**Known deliberate deferrals (spec-sanctioned):** the path fence's missing epoch input and unreadable-unreferenced residuals are accepted limits (spec "Final design logic check") — commented at their sites, not "solved".

**Type consistency:** `LegacyFinding.Worktree` (Task 1) is what Task 10's assertions read; `EpochSettledFunc` (Task 4) is only consumed inside gatedrive; `retireWorktreeSlotOwnership`'s unchanged signature (Task 6) matches its existing callers; `recordedDocWithRoot` (Task 9) is additive. Workers must verify exact struct/field spellings against the tree at implementation time — the tree, not this plan, is authoritative for pre-existing symbols.
