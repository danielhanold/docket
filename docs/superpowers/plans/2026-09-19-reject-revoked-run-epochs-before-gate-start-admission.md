<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0437 — Reject revoked run epochs before gate-start admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-19-0437-reject-revoked-run-epochs-before-gate-start-admission.md)**
<!-- docket:backlink:end -->
# Reject Revoked Run Epochs Before Gate-Start Admission — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fence every epoch-backed gate-drive launch route — fresh admission, delayed `StartAdmitted` tickets, automatic relaunch, and reserved-relaunch recovery — against a cancelled/superseded/unbound run epoch, give a same-scope successor its own execution reservation, and make pending launch obligations visible to cancellation so no process can first appear after a completed cancellation.

**Architecture:** Three layers change. (1) `internal/gatedrive` gains one injected seam, `EpochLaunchGate` (the app-owned authoritative liveness read that runs the driver's durable reservation body while the epoch registry lock is held), plus: epoch-gated `Admit`, a revalidating `StartAdmitted` that holds the existing per-drive launch claimant lock across launch/attach, epoch-gated relaunch authorization (automatic and recovered), successor slot rotation (fresh `ReservationToken` + `ExecutionGen`), and a `ReconcileEpochLaunches` accounting read cancellation consumes. (2) `internal/app` implements the gate over the existing epoch registry (`findEpochByID` extended to a unique-match locator, read-only under the existing `epoch.lock`, mapped to the existing `MutationFenceError` tokens) and wires it at every production gate-drive constructor. (3) `reconcileEpochTeardown` (run.cancel + the death guardian) additionally reconciles epoch-linked drive/relaunch reservations through the new gatedrive accounting. No new daemon, store, schema, lifecycle state, CLI command, or coordination framework — only existing records, locks, and atomic writers.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), stdlib `testing`, `go/ast`+`go/parser` for the launch-site guard, the package's existing fake-seam concurrency test patterns (`internal/gatedrive/driver_concurrency_test.go`).

**Spec:** `docs/superpowers/specs/2026-09-19-reject-revoked-run-epochs-before-gate-start-admission-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…`). The change file is `docs/changes/active/0437-reject-revoked-run-epochs-before-gate-start-admission.md`.

## Global Constraints

- **Complexity limit (spec, verbatim intent):** reuse existing epoch, admission, scope, drive, and relaunch records, their states, lock files, and atomic writers. No new daemon, background loop, persistent store, schema, lifecycle state, configuration, CLI command, generic coordination framework, or additional retry layer. Small private helpers and reuse of the existing claimant lock are allowed; a second ownership protocol is not. If the required launch/cancel serialization cannot fit this record-and-lock model, STOP and report the concrete design conflict instead of adding machinery.
- **Lock order (spec, mandatory):** epoch lock → nonblocking per-drive launch claim (when needed) → worktree-admission → scope → drive locks, in that order. No path acquires the epoch lock while retaining an inner lock/claim (refactor recovery entry accordingly). Never hold the epoch or admission lock across process launch, stop, network I/O, or suite execution. The per-drive claimant lock MAY span the bounded launch/attach or reservation-resolution operation, as `relaunchClaim` does today. Keep fingerprint computation outside epoch critical sections where possible.
- **No epoch write from the liveness path:** the admission liveness wrapper reads under the epoch lock and never rotates the generation or rewrites the record. Never wrap side-effecting admission in `epochCAS` (its trailing rewrite can fail after admission succeeded).
- **Failure channels:** reuse the existing tokens only — app-side `MutationFenceError` (`run-cancelled`, `stale-run-epoch`; already mapped by `mapDriveFailure` and `fenceNextAction` in `internal/app/gate_drive.go`), gatedrive ownership kinds (`ErrUnresolvedExecution`, `ErrUnresolvedLaunchTransition`, `ErrNotOwner`, `ErrStaleRunEpoch`), and `HALTED` causes. Locators stay bounded (drive ids, disposition tokens) — never a capability, reservation token, argv, or env value.
- **Budget behavior:** an epoch refused by `Admit` consumes no suite attempt; an attempt reserved before a later cancellation stays charged even when its delayed launch is refused (no refunds). Relaunch denial adds no attempt or relaunch allowance and resets no deadline/budget/retry state. Do not retry launch automatically after a refusal or persistence failure.
- **Out of scope (change 435 owns):** stale-slot retirement, cancellation-specific epoch detachment, terminal-cancel repair and its resume check, and the existing slot-teardown helpers' broader foreign-slot/error audit. Also out: relaunch policy changes, raw standalone launch/recover, run attribution, coordinator lifecycle, normal successful-run epoch retirement. Preserve takeover and ordinary release semantics; do not weaken ordinary release's preservation of `RunEpochID`.
- **TDD + mutation:** every new guard/conjunct is mutation-tested — strip it, watch a targeted test redden — always with `go test -count=1` (learning `cached-runner-serves-a-mutated-tree`; a `(cached)` verdict is absence of evidence). Deterministic barriers (channels, injected fakes) are the concurrency oracles — never timing sleeps.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054). `gofmt` every touched file. Commit per task.
- **Build gate:** run the whole suite via the configured `build.test_command` (read it from config — the Go runner `internal/suiterunner` is the sole channel; see `tests/README.md`), from this feature checkout. Read the budget report even on green.
- New Go tests in `internal/gatedrive` and untagged `internal/app` tests run under the existing `go`-category wrappers automatically. Any NEW `//go:build integration` test in `internal/app` must be named so an existing shard's `-run` prefix matches it (see `tests/lib/go-integration-shard.sh` and the `tests/test_go_integration_app_*.sh` shards — the shard library enforces a completeness contract); prefer untagged unit tests with fake seams wherever they suffice.

## File Structure

- `internal/gatedrive/driver.go` — `EpochLaunchGate` type + `SetEpochLaunchGate`; epoch-gated `Admit`/`StartAdmitted`; claim-held launch; epoch-gated relaunch authorization; epoch linkage resolution.
- `internal/gatedrive/admission.go` — successor slot rotation (`rotateWorktreeExecutionForSuccessor`).
- `internal/gatedrive/reconcile.go` (new) — `ReconcileEpochLaunches` + `EpochLaunchReport` (cancellation's pending-launch accounting read).
- `internal/gatedrive/store.go` — claimant-lock reuse helper (if any refactor is needed; the lock file and `tryRelaunchClaim` stay as-is).
- `internal/gatedrive/epoch_gate_test.go`, `driver_epochfence_test.go`, `admission_successor_test.go`, `reconcile_test.go`, `driver_concurrency_test.go` (extended), `launch_sites_guard_test.go` — new/extended coverage.
- `internal/app/rungate_epoch.go` — unique-match epoch-by-id locator + the read-only locked liveness check.
- `internal/app/rungate_gate.go` (new, small) — `epochLaunchGate(gitCommonDir) gatedrive.EpochLaunchGate` production implementation.
- `internal/app/gate_drive.go`, `internal/app/rungate_continuation.go` — wire `SetEpochLaunchGate` beside the existing `SetEpochRevokedResolver` at all four production constructors.
- `internal/app/rungate_cancel.go` — `cancelSeams` gains a launch-reconciler seam; `reconcileEpochTeardown` consumes `ReconcileEpochLaunches`; `productionCancelSeams` composes it. `internal/app/agent_guardian.go` — same composition for the death guardian.
- `internal/app/rungate_gate_test.go`, `rungate_cancel_test.go` (extended), `gate_drive_test.go` (extended) — app coverage.

---

### Task 1: `EpochLaunchGate` seam and epoch-gated `Admit`

**Files:**
- Modify: `internal/gatedrive/driver.go`
- Test: `internal/gatedrive/epoch_gate_test.go` (new)

**Interfaces:**
- Produces (used by Tasks 2, 3, 5, 7, 8):

```go
// EpochLaunchGate is the app-injected authority that validates a run epoch is
// LIVE (active, uniquely resolved in this repository's registry, and bound to
// worktree) and, while the registry's per-key epoch lock is held, runs reserve —
// the driver's durable admission/reservation body — so a concurrent cancellation
// fence either lands before the liveness read (reserve never runs) or observes
// the durable reservation reserve produced. A validation failure returns a typed
// error and reserve is NEVER called. The gate performs no epoch write. A nil gate
// or an empty epochID runs reserve directly (a genuinely epoch-less standalone
// gate keeps its existing behavior).
type EpochLaunchGate func(epochID, worktree string, reserve func() error) error

// SetEpochLaunchGate injects the gate at composition, before any concurrent
// start, so it needs no lock (mirrors SetEpochRevokedResolver).
func (d *Driver) SetEpochLaunchGate(g EpochLaunchGate)

// epochGated runs reserve under the injected gate when both the gate and the
// epoch id are present, else directly. Every epoch-backed reservation/launch
// authorization in this package flows through this ONE helper (the launch-site
// guard in Task 8 keys on it).
func (d *Driver) epochGated(epochID, worktree string, reserve func() error) error
```

- `AdmissionTicket` gains an unexported in-memory field `runEpochID string` (never persisted — the durable linkage stays the slot/scope records; spec "Retain an admitted ticket's epoch identity in memory").

**Steps:**

- [ ] **Step 1: Write failing tests** in `epoch_gate_test.go` (fake gate = closure recording calls; fake proc/git/clock per the package's existing `driver_test.go` fixtures):
  - `TestAdmitConsultsEpochGateWithIDAndWorktree` — a scoped and a scopeless `Admit` with `RunEpochID: "e1"` each call the gate exactly once with `("e1", req.Worktree)`; the returned ticket carries the epoch id (assert via the Task-2 launch path later; here assert the gate was called and admission succeeded).
  - `TestAdmitEpochGateRefusalReservesNothing` — gate returns a sentinel error without calling `reserve`; `Admit` returns that exact error (via `errors.Is`), NO worktree slot exists (`LoadWorktreeExecution` → `ErrNotFound`), no reserved drive record was minted (drive registry dir empty), and for the scoped case the scope slot is untouched.
  - `TestAdmitReservationRunsInsideGate` — the fake gate asserts, around its `reserve()` call, that the worktree slot is absent before and `reserved` after (the durable decision happens while the gate is held).
  - `TestAdmitNilGateOrEmptyEpochUnchanged` — nil gate with an epoch id, and a set gate with `RunEpochID: ""`, both admit exactly as today (no gate call for the empty-epoch case).
- [ ] **Step 2: Run to verify failure** — `go test ./internal/gatedrive/ -count=1 -run 'TestAdmit(Consults|EpochGate|Reservation|NilGate)'` → FAIL (`SetEpochLaunchGate` undefined).
- [ ] **Step 3: Implement** — add the type, setter, field `epochLaunch EpochLaunchGate` on `Driver`, and `epochGated`. In `Admit`, after the fingerprint/record construction, wrap the tail dispatch:

```go
var ticket *AdmissionTicket
err = d.epochGated(req.RunEpochID, req.Worktree, func() error {
    var aerr error
    if req.ScopeID == "" {
        ticket, aerr = d.admitScopeless(rec, ownerGen, req.RunEpochID)
    } else {
        ticket, aerr = d.admitScoped(req, rec, ownerGen)
    }
    return aerr
})
if err != nil {
    return nil, err
}
ticket.runEpochID = req.RunEpochID
```

  The fingerprint (`ComputeFingerprint`) and `precheckScopedStart` stay OUTSIDE the gate (fingerprints out of epoch critical sections; the precheck is advisory). Lock order inside `reserve` is unchanged: admission flock → scope → drive.
- [ ] **Step 4: Run to verify pass** — same command, PASS; then `go test ./internal/gatedrive/ -count=1` (whole package still green).
- [ ] **Step 5: Mutation-check** — temporarily make `epochGated` skip the gate (call `reserve` directly); `TestAdmitEpochGateRefusalReservesNothing` must redden (`-count=1`). Restore.
- [ ] **Step 6: Commit** — `git add internal/gatedrive/driver.go internal/gatedrive/epoch_gate_test.go && git commit -m "feat(gatedrive): epoch launch gate seam fences Admit (change 0437 task 1)"`.

---

### Task 2: `StartAdmitted` revalidation, per-drive claim across launch, delayed-ticket refusal

**Files:**
- Modify: `internal/gatedrive/driver.go`
- Test: `internal/gatedrive/driver_epochfence_test.go` (new)

**Interfaces:**
- Consumes: `epochGated`, `AdmissionTicket.runEpochID` (Task 1); `tryRelaunchClaim` (existing per-drive claimant flock in `store.go` — the spec designates this SAME lock file as the launch claimant for initial launches too).
- Produces (relied on by Tasks 6 and 7): between final authorization and attach, the drive's claimant flock is HELD, so `tryRelaunchClaim(id)` from cancellation reports busy — "a busy claim is pending work, never proof of a crashed caller". The revalidation body (private):

```go
// revalidateAdmittedLaunch re-reads, under the epoch gate, the EXACT durable
// reservation this ticket minted — the worktree slot must still carry the
// ticket's ReservationToken in the state the ticket expects (reserved when the
// ticket owns the slot, executing when it reuses a peer's), and the RESERVED
// drive record must still exist under the ticket's owner generation — and
// acquires the drive's claimant flock NONBLOCKING. Any mismatch, a busy claim,
// or a revoked epoch refuses with a typed error and launches nothing.
func (d *Driver) revalidateAdmittedLaunch(t *AdmissionTicket) (*relaunchClaim, error)
```

**Steps:**

- [ ] **Step 1: Write failing tests:**
  - `TestStartAdmittedRevalidatesEpoch` — Admit under a permissive fake gate; flip the fake to refuse (simulating a cancellation fence between the two phases); `StartAdmitted` returns the gate's typed error, `proc.Launch` was NEVER called (fake proc counts), the reserved drive record is settled `HALTED` (fail-closed durable invalidation of the delayed ticket), and the freshly reserved worktree slot is released (never-launched is proven by construction — no `Launch` call happened; a slot the ticket merely reused is left untouched).
  - `TestStartAdmittedRefusesForeignReservation` — after Admit, release the slot and reserve it anew (a rotated/foreign reservation); `StartAdmitted` refuses `ErrUnresolvedLaunchTransition` without launching.
  - `TestStartAdmittedRefusesBusyClaim` — hold the drive's claimant flock from the test (via `tryRelaunchClaim`); `StartAdmitted` refuses without launching and without waiting (returns promptly — assert with a done-channel, not a timer oracle).
  - `TestStartAdmittedHoldsClaimAcrossLaunch` — fake `proc.Launch` blocks on a barrier channel; from another goroutine at the barrier, `tryRelaunchClaim(id)` reports busy; after launch+attach return, the claim is free again.
  - `TestStartAdmittedEpochlessUnchanged` — empty `runEpochID`: no gate call, launch proceeds exactly as today (still under the claim — the claim discipline is unconditional; only the epoch validation is conditional).
- [ ] **Step 2: Run to verify failure** — `go test ./internal/gatedrive/ -count=1 -run TestStartAdmitted` → FAIL.
- [ ] **Step 3: Implement.** `StartAdmitted` becomes:

```go
claim, err := d.revalidateAdmittedLaunch(t)   // epoch gate held only inside
if err != nil { return DriveDoc{}, err }
defer claim.close()                            // released after attach or failure reconciliation
if t.scoped { return d.launchScoped(t) }
return d.launchScopeless(t)
```

  `revalidateAdmittedLaunch` runs `d.epochGated(t.runEpochID, t.rec.WorktreePath, func() error { … })` whose body: (a) `tryRelaunchClaim(t.id)` — busy → `ownershipErr(ErrUnresolvedLaunchTransition, "start-admitted")`; (b) `LoadWorktreeExecution` and verify `slot.ReservationToken == t.token` with the state the ticket expects; (c) `Load(t.id)` and `verifyOwner` + still-nonterminal. On epoch refusal from the gate: close any claim taken, settle the drive `HALTED` with cause `"run-cancelled"` under `ownerCAS` (mirror the `launch-failed` CAS blocks in `launchScopeless`), and for `t.reservedFresh || !t.scoped` release the slot (`ReleaseWorktreeExecution`) — nothing launched, provably idle — then return the gate's error unchanged so the app surfaces the fence token. The epoch lock is released when the gate returns; `proc.Launch` runs OUTSIDE it, holding only the claim. Note `errors.Is`/`AsOwnershipError` mapping stays intact through the wrap.
- [ ] **Step 4: Run to verify pass** — `go test ./internal/gatedrive/ -count=1` (whole package; the concurrency and faults suites exercise these paths heavily — fix any legitimately changed expectations, never weaken an assert).
- [ ] **Step 5: Mutation-check** — (a) skip the revalidation (return a fresh claim unconditionally): `TestStartAdmittedRevalidatesEpoch` and `TestStartAdmittedRefusesForeignReservation` redden; (b) drop the claim acquisition: `TestStartAdmittedHoldsClaimAcrossLaunch` reddens. `-count=1`. Restore.
- [ ] **Step 6: Commit** — `git commit -m "feat(gatedrive): StartAdmitted revalidates epoch and holds the launch claim (change 0437 task 2)"` (add only the two files).

---

### Task 3: Epoch linkage resolution; fence automatic relaunch and reserved-relaunch recovery

**Files:**
- Modify: `internal/gatedrive/driver.go`
- Test: `internal/gatedrive/driver_epochfence_test.go` (extend)

**Interfaces:**
- Consumes: `epochGated` (Task 1), `reserveRelaunch`/`recoverReservedRelaunch`/`driveSlice` (existing).
- Produces:

```go
// resolveDriveEpoch resolves the run epoch a durable drive is linked to, from
// existing records only. A scoped drive answers from its scope's RunEpochID; a
// scopeless drive with an AdmissionToken answers from the worktree slot ONLY
// when the slot's ReservationToken still equals that token (an exact-reservation
// match). ok=false with cause set means the linkage is LOST or inconsistent —
// the drive can no longer prove whether it is epoch-backed, so new execution is
// refused (never demoted to standalone). ("", true, "") is a genuinely
// epoch-less drive (legacy empty token, or a slot recording no epoch).
func (d *Driver) resolveDriveEpoch(rec driveRecord) (epochID string, ok bool, cause string)

// authorizeRelaunch validates, under the epoch gate, that the drive's epoch (if
// any) is live and reserves the single automatic replacement while the gate is
// held. It returns the held claim; the caller launches OUTSIDE the gate.
func (d *Driver) authorizeRelaunch(id, ownerGen string, rec driveRecord) (*relaunchClaim, string /*halt cause*/, error)
```

**Steps:**

- [ ] **Step 1: Write failing tests** (fixtures: drive through fake-proc death → relaunch leg, as `driver_test.go`'s existing relaunch tests do):
  - `TestRelaunchRefusedWhenEpochRevoked` — scoped drive whose scope carries `RunEpochID: "e1"`; fake gate refuses; the death-relaunch leg HALTs with cause `"run-cancelled"`, `proc.Launch` is called exactly once (the original), and no relaunch is reserved (`RelaunchReserved` false).
  - `TestRelaunchAuthorizedUnderGateThenLaunchedOutside` — permissive fake gate records enter/exit; fake `proc.Launch` asserts (via the recorded state) the gate is NOT held during the replacement launch, and `reserveRelaunch`'s CAS committed while it WAS held.
  - `TestRecoveredRelaunchValidatesEpochBeforeClaim` — drive with `RelaunchReserved: true` + token; revoked epoch: `Advance` never takes the claim path to a new launch — a `never-launched` resolution settles `HALTED` (cause `"run-cancelled"`) with no `Launch`; an `identified` resolution still attaches and the run is observed/stopped normally (reconcile is teardown, not permission — spec AC4). Assert the fake gate is consulted BEFORE `tryRelaunchClaim` (lock order: never epoch while holding the claim) — have the fake gate probe `tryRelaunchClaim` availability.
  - `TestRelaunchLostLinkageRefuses` — scopeless drive with an `AdmissionToken` whose slot now carries a different `ReservationToken`; the relaunch leg HALTs `"unresolved-execution"` without launching (lost linkage never demotes to standalone).
  - `TestRelaunchStandaloneUnchanged` — `AdmissionToken` present, slot matches, slot `RunEpochID == ""`: relaunch proceeds exactly as today; and a legacy drive with empty `AdmissionToken` and no scope likewise.
- [ ] **Step 2: Run to verify failure** — `go test ./internal/gatedrive/ -count=1 -run 'TestRelaunch|TestRecoveredRelaunch'` → FAIL.
- [ ] **Step 3: Implement.**
  - `resolveDriveEpoch`: `rec.ScopeID != ""` → `LoadScope` (load error → `(_, false, "epoch-unreadable")` using the existing `CauseEpochUnreadable`) → `(scope.RunEpochID, true, "")`. Else `rec.AdmissionToken == ""` → `("", true, "")`. Else `LoadWorktreeExecution(rec.WorktreePath)`: `ErrNotFound` or token mismatch → `("", false, "unresolved-execution")`; match → `(slot.RunEpochID, true, "")`.
  - In `driveSlice`'s `StateSignaled/StateVanished` leg, replace the bare `claim, err = d.store.reserveRelaunch(id, ownerGen)` branch with `claim, haltCause, err := d.authorizeRelaunch(id, ownerGen, rec)`; a non-empty `haltCause` → `return halt(&res, haltCause)`; the race-lost/terminal sentinel handling is unchanged. `authorizeRelaunch` = `resolveDriveEpoch` (lost → halt cause), then `d.epochGated(epochID, rec.WorktreePath, func() error { claim, err = d.store.reserveRelaunch(id, ownerGen); return err })`, mapping a gate refusal to halt cause `"run-cancelled"` (bounded token, matches the fence vocabulary). The replacement `proc.Launch` stays where it is — after the gate returned, claim held.
  - In `Advance`'s `rec.RelaunchReserved` entry, BEFORE calling `recoverReservedRelaunch`, resolve+validate the epoch via a read-only pass through the gate with a no-op reserve body; on refusal, do NOT take the claim for a new launch: run `recoverReservedRelaunch` in a teardown-bounded mode — concretely, thread a `revoked bool` parameter: when revoked, the `never-launched` arm settles `HALTED "run-cancelled"` (reuse the `haltReservedRelaunch` CAS shape with that cause) instead of returning a live claim, while the `identified`, busy, and ambiguous arms keep their existing behavior (attach/report/halt — reconciliation, not authorization).
- [ ] **Step 4: Run to verify pass** — `go test ./internal/gatedrive/ -count=1`.
- [ ] **Step 5: Mutation-check** — strip the `authorizeRelaunch` gate call (reserve directly): `TestRelaunchRefusedWhenEpochRevoked` reddens; strip the lost-linkage arm (treat mismatch as standalone): `TestRelaunchLostLinkageRefuses` reddens. `-count=1`. Restore.
- [ ] **Step 6: Commit** — `git commit -m "feat(gatedrive): fence automatic and recovered relaunches on epoch liveness (change 0437 task 3)"`.

---

### Task 4: A successor gets its own execution reservation

**Files:**
- Modify: `internal/gatedrive/admission.go`, `internal/gatedrive/driver.go` (`admitScopedWorktree`)
- Test: `internal/gatedrive/admission_successor_test.go` (new)

**Interfaces:**
- Consumes: `admissionCAS`, `verifyAdmissionToken`, `admitScopedWorktree` (existing).
- Produces (Task 6 and 7 rely on the rotation semantics):

```go
// rotateWorktreeExecutionForSuccessor transitions an EXECUTING slot the same
// scope+epoch still owns to a FRESH reservation for the sequence's next drive:
// it verifies oldToken, requires state executing, bumps ExecutionGen, mints a
// new ReservationToken, clears RawRunID/RawRunDir (the predecessor's raw-run
// identity never rides the successor's reservation), preserves
// RepoIdentity/WorktreeRoot/ScopeID/RunEpochID/Kind, and lands in "reserved".
// Any other state, a token mismatch, or an unreadable record refuses typed and
// writes nothing. After rotation the predecessor's oldToken has NO authority:
// its late release/unresolve/stopping calls fail ErrNotOwner (existing
// verifyAdmissionToken), so a stale predecessor cleanup can never free or
// poison the successor's slot.
func (s *Store) rotateWorktreeExecutionForSuccessor(worktreeRoot, oldToken string) (newToken string, err error)
```

- `admitScopedWorktree` change: the same-scope `admissionExecuting` reuse arm (today `return slot.ReservationToken, false, false, nil, nil`) becomes a rotation — it returns the FRESH token with `reservedFresh=false, ownsSlot=true` (the successor confirms its own slot and owns its post-launch failure legs). The same-scope `admissionReserved` arm (a concurrent first-start peer's provisional reservation) is UNCHANGED — scope arbitration still picks exactly one winner and losers must not release the winner's adopted reservation (`isSameScopeRaceLoss` untouched).

**Steps:**

- [ ] **Step 1: Write failing tests:**
  - `TestSuccessorRotatesExecutingSlot` — drive a scoped first start to executing and a terminal PASSED verdict, but do NOT release the slot (the terminal-before-release window: skip/stub the release); start the successor with the predecessor receipt; assert the successor's slot has a NEW `ReservationToken`, `ExecutionGen` bumped, empty `RawRunID/RawRunDir` before launch, and after launch-confirm carries the SUCCESSOR's run identity; the scope/epoch fields survived.
  - `TestLatePredecessorReleaseCannotFreeSuccessor` — after rotation, `ReleaseWorktreeExecution(worktree, oldToken)`, `MarkWorktreeExecutionUnresolved(worktree, oldToken)`, and `MarkWorktreeExecutionStopping(worktree, oldToken)` each return `ErrNotOwner` and leave the successor's record byte-stable (compare generation).
  - `TestSuccessorAdmissionFailureLegsReleaseRotatedSlot` — force `reserveScopeDrive`/`retirePredecessor` failures after rotation (stale receipt fixture); the rotated reservation is released (not leaked, not left blocking), and the scope state is unchanged from the failure's existing contract. NOTE: with `ownsSlot=true` after rotation, audit each `admitScoped` failure leg — a rotated (not freshly minted) reservation must be released on a genuine failure exactly like a fresh one; extend the `reservedFresh` handling to a `releasable` notion covering both, without disturbing the same-scope-race-loss adoption rule.
  - `TestFirstStartPeerSharingUnchanged` — the existing concurrent first-start contention behavior (reserved-arm reuse + `isSameScopeRaceLoss`) is pinned unchanged (extend/duplicate from the existing suite if already covered — verify by running the existing tests and add the pin only if absent).
  - `TestOrdinaryReleasePreservesRunEpoch` — `ReleaseWorktreeExecution` on the successor's own token still preserves `RunEpochID` on the released record (regression pin; likely exists — verify, add if absent).
  - `TestPendingPredecessorTransitionRefusesSuccessor` — a scope with `PendingAckDriveID` set refuses the successor (`ErrUnresolvedLaunchTransition`) BEFORE any rotation (rotation must not run on an ambiguous predecessor transition — order the rotation after the busy-arm checks that already exist in the precheck plus the executing-state requirement).
- [ ] **Step 2: Run to verify failure** — `go test ./internal/gatedrive/ -count=1 -run 'TestSuccessor|TestLatePredecessor|TestFirstStartPeer|TestOrdinaryRelease|TestPendingPredecessor'` → FAIL.
- [ ] **Step 3: Implement** `rotateWorktreeExecutionForSuccessor` via `admissionCAS` (one locked read-modify-write; the mutate rejects non-executing states and token mismatches typed), and rewire `admitScopedWorktree`'s executing arm to call it, returning `(newToken, false, true, nil, nil)`. Thread the release-on-failure audit through `admitScoped` (Step 1's third test defines it). Existing handoff-journal crash-recovery behavior (`reserveScopeDrive` → `retirePredecessor` → `clearPendingAck` order and their fail-closed arms) is byte-preserved.
- [ ] **Step 4: Run to verify pass** — `go test ./internal/gatedrive/ -count=1` (the integration_sequence and takeover suites exercise successor flows; expectations that legitimately observed token REUSE may need updating to rotation — update only assertions that encoded the defect, never protections).
- [ ] **Step 5: Mutation-check** — make the executing arm return the OLD token again (the pre-change defect): `TestSuccessorRotatesExecutingSlot` and `TestLatePredecessorReleaseCannotFreeSuccessor` redden. `-count=1`. Restore.
- [ ] **Step 6: Commit** — `git commit -m "feat(gatedrive): successor drives get a fresh execution reservation (change 0437 task 4)"`.

---

### Task 5: App-side epoch launch gate + wiring at every production constructor

**Files:**
- Modify: `internal/app/rungate_epoch.go`; Create: `internal/app/rungate_gate.go`
- Modify: `internal/app/gate_drive.go` (`newOwnedGateDriveService`, `NewTaskGateDriveService`, `NewCommandlessGateDriveService`), `internal/app/rungate_continuation.go` (`NewContinuationSeam`)
- Test: `internal/app/rungate_gate_test.go` (new), `internal/app/gate_drive_test.go` (extend)

**Interfaces:**
- Consumes: `gatedrive.EpochLaunchGate` + `SetEpochLaunchGate` (Task 1); existing `readStoredEpoch`, `acquireEpochLock`, `epochOwnsWorktree` (`rungate_fence.go`), `canonicalWorktree`, `ErrRunCancelled`/`ErrStaleRunEpoch` (`rungate_fence.go`), `findEpochByID`.
- Produces:

```go
// findEpochDirByID resolves the UNIQUE gate-key directory holding the epoch
// whose public EpochID is epochID. Zero matches → ErrEpochNotFound; more than
// one → ErrEpochAmbiguous; corrupt/unreadable siblings are skipped for matching
// but the scan error contract mirrors findEpochByID. (Extend findEpochByID's
// scan; do not duplicate it — one walker, two shapes.)
func findEpochDirByID(rungateRoot, epochID string) (dir string, rec EpochRecord, err error)

// epochLaunchGate builds the production gatedrive.EpochLaunchGate over this
// repository's registry. It locates the epoch by id (unique match), acquires
// that key's epoch.lock, RE-READS the record under the lock (the unlocked scan
// only located the directory), validates — state == EpochActive, and
// epochOwnsWorktree(rec.Worktree, canonicalWorktree(worktree)) — and only then
// runs reserve while still holding the lock. It NEVER writes the epoch record.
// Refusal mapping (all fail closed): cancelling/cancelled → ErrRunCancelled;
// superseded → ErrStaleRunEpoch; missing/ambiguous/corrupt/unreadable →
// the typed EpochError (surfaced as an internal refusal, never a free pass);
// wrong or empty worktree binding → ErrStaleRunEpoch (the epoch does not own
// the worktree the start names — omission/substitution is the same refusal).
func epochLaunchGate(gitCommonDir string) gatedrive.EpochLaunchGate
```

**Steps:**

- [ ] **Step 1: Write failing tests** in `rungate_gate_test.go` (fixture: a temp gate-key dir + `MintEpochRecord` + `bindEpochWorktree`, as `rungate_epoch` tests do):
  - `TestEpochLaunchGateAdmitsActiveBoundEpoch` — active epoch bound to the fixture worktree: `reserve` runs exactly once, nil error.
  - `TestEpochLaunchGateRefusalMatrix` — table over: missing id, ambiguous id (two dirs, same EpochID), corrupt record, cancelling, cancelled, superseded, bound to a DIFFERENT worktree, unbound (empty `Worktree`). Each: `reserve` never called, error is the mapped token above (`errors.Is(err, ErrRunCancelled)` / `ErrStaleRunEpoch` / `AsEpochError` kind).
  - `TestEpochLaunchGatePerformsNoWrite` — read the epoch file bytes (and generation) before and after both an admitted and a refused call; byte-identical (spec AC6).
  - `TestEpochLaunchGateSerializesWithFence` — barrier test: gate acquires the lock and parks inside `reserve` on a channel; a concurrent `epochCAS` active→cancelling from another goroutine blocks until `reserve` returns (assert with a done-channel ordering, not a sleep). Then the reverse: fence first → the gate refuses.
  - In `gate_drive_test.go`: `TestProductionConstructorsWireEpochLaunchGate` — for each of `NewBuildGateDriveService`, `NewFinalizeGateDriveService`, `NewTaskGateDriveService`, `NewCommandlessGateDriveService`, and `NewContinuationSeam`, prove the composed driver carries a non-nil gate that reaches the registry. Follow the file's existing constructor-test pattern; if the driver's gate is not observable from outside `gatedrive`, add a tiny exported probe in gatedrive (e.g. `func (d *Driver) EpochLaunchGateWired() bool`) — a read-only accessor, not new machinery. Deleting any ONE constructor's `SetEpochLaunchGate` line must redden this test (learning `fix-reintroduces-its-own-defect-class`: the wiring is where the takeover-only defect lived).
  - `TestBuildStartEpochRefusalChargesNoAttempt` — fake engine whose `Admit` returns `ErrRunCancelled`: `startBudgetedBuild` refuses with reason `run-cancelled` and `SuiteBudgetUsage` is unchanged (admission precedes charging). And `TestBuildStartChargedAttemptNotRefundedOnFencedLaunch` — `Admit` succeeds, `StartAdmitted` returns `ErrRunCancelled`: the attempt stays charged, `AbandonAdmission` NOT called (pin with the fake engine's call log).
- [ ] **Step 2: Run to verify failure** — `go test ./internal/app/ -count=1 -run 'TestEpochLaunchGate|TestProductionConstructorsWire|TestBuildStart(Epoch|Charged)'` → FAIL.
- [ ] **Step 3: Implement** `findEpochDirByID` + `epochLaunchGate` per the interface block, and add `engine.SetEpochLaunchGate(epochLaunchGate(gitCommonDir))` immediately beside each existing `SetEpochRevokedResolver` call (all four sites). The gate canonicalizes the worktree argument once, outside the lock. It must not call `epochCAS` (no write).
- [ ] **Step 4: Run to verify pass** — `go test ./internal/app/ -count=1` and `go test ./internal/gatedrive/ -count=1`.
- [ ] **Step 5: Mutation-check** — (a) make the gate skip the state check: refusal-matrix rows redden; (b) make it skip the worktree-binding check: the wrong-worktree row reddens; (c) delete one constructor wiring line: the wiring test reddens. `-count=1`. Restore.
- [ ] **Step 6: Commit** — `git commit -m "feat(app): production epoch launch gate wired at every gate-drive constructor (change 0437 task 5)"`.

---

### Task 6: Cancellation accounts for pending and replacement launches

**Files:**
- Create: `internal/gatedrive/reconcile.go`; Test: `internal/gatedrive/reconcile_test.go`
- Modify: `internal/app/rungate_cancel.go`, `internal/app/agent_guardian.go`
- Test: `internal/app/rungate_cancel_test.go` (extend)

**Interfaces:**
- Consumes: `tryRelaunchClaim`, `ProcessSeam.ResolveReservation`/`Stop`, `LoadWorktreeExecution`, `LoadScope`/scope enumeration (`FindScopeDriveIDs` exists; add a sibling scan if scope→epoch enumeration needs one — reuse the existing registry walkers, never a new store), drive registry walk (mirror `inventoryLegacyDrives`' deterministic-id-order walk).
- Produces:

```go
// EpochLaunchReport is the bounded accounting of one epoch's launch obligations
// on one canonical worktree. Findings carry drive ids + disposition tokens only
// (e.g. "launch-pending:<id>", "claim-busy:<id>", "replacement-stopped:<id>",
// "resolution-unresolved:<id>") — never a token, argv, or env value.
type EpochLaunchReport struct {
    Accounted bool
    Findings  []string
}

// ReconcileEpochLaunches reconciles, for an ALREADY-FENCED epoch, every
// epoch-linked launch obligation the durable records name: scoped drives whose
// scope carries epochID, and scopeless drives whose AdmissionToken matches a
// worktree slot recording epochID (exact repository/worktree/epoch/drive/
// reservation associations — an obsolete slot RawRunDir is not an inventory,
// and an empty participant list proves nothing). For each nonterminal drive:
// take the claimant flock NONBLOCKING — busy is pending work (never waited on);
// free → re-read the record and resolve the exact reservation
// (AdmissionToken, or RelaunchToken when RelaunchReserved): a proven
// never-launched accounts it; an identified run is stopped through proc and
// accounts only on proven teardown; unresolved, a resolution/read/stop error,
// or a missing/corrupt required record keeps Accounted=false. It launches
// nothing, mutates no drive verdict, and preserves unresolved evidence — it
// must never erase a reservation that may have launched.
func (d *Driver) ReconcileEpochLaunches(worktreeRoot, epochID string) (EpochLaunchReport, error)
```

- App side: `cancelSeams` gains `launches epochLaunchReconciler` where `type epochLaunchReconciler interface { reconcile(worktree, epochID string) (gatedrive.EpochLaunchReport, error) }`; production impl wraps a `gatedrive.NewSystemDriver(store, proc)` composed in `productionCancelSeams` from the same store + the gate service's process seam (follow `appGateStopper`'s `gateService()` resolution; a nil/unavailable reconciler is a FINDING + `accounted=false`, fail closed, mirroring the nil-stopper rule). `reconcileEpochTeardown` calls it between step (5b) and (6), merging findings and and-ing `accounted`. Verify `agent_guardian.go`'s call site composes seams through the same production path (it shares `reconcileEpochTeardown`; confirm its seam composition and extend identically).

**Steps:**

- [ ] **Step 1: Write failing gatedrive tests** (`reconcile_test.go`, fake proc):
  - `TestReconcileSeesPendingReservedDrive` — Admit (no StartAdmitted): report `Accounted=false` with a `launch-pending:<id>` finding, even though the worktree slot is merely `reserved` and no participant was ever registered.
  - `TestReconcileBusyClaimIsPending` — hold the drive's claim from the test: `claim-busy:<id>`, `Accounted=false`, and the call returns without waiting (done-channel promptness, no timer oracle).
  - `TestReconcileProvenNeverLaunchedAccounts` — free claim, fake `ResolveReservation` → never-launched: accounted, and NOTHING was launched or mutated (drive record generation unchanged).
  - `TestReconcileIdentifiedReplacementStopped` — a drive whose single relaunch attached a replacement (`RawRunDir` ≠ slot's recorded `RawRunDir` — the slot still names the predecessor): the replacement is found via the DRIVE record, stopped through proc, accounted on proven stop; unproven stop → pending (spec AC4: "Cancellation sees the replacement even when the slot still names the predecessor").
  - `TestReconcileReservedRelaunchResolved` — `RelaunchReserved` + token: resolves the RELAUNCH token (never the admission token); never-launched/identified/unresolved arms as above.
  - `TestReconcileFailuresPreserveEvidence` — resolution error / unreadable drive record / stop error: `Accounted=false`, records untouched (AC5: publication/read/stop failures preserve unresolved evidence).
  - `TestReconcileReplayConverges` — first call pending (unproven stop); make the fake prove teardown; second call accounts with no second launch (AC5 replay convergence).
  - `TestReconcileReleasedSlotWithPendingDriveNotAccounted` — released/absent slot + a pending epoch-linked drive: `Accounted=false` (a released slot alone never settles launch obligations).
- [ ] **Step 2: Failing app tests** (`rungate_cancel_test.go`, fake reconciler seam): `TestCancelPendingWhileLaunchObligationUnresolved` (reconciler pending → disposition `cancellation-pending`, findings surfaced), `TestCancelCompletesWhenLaunchObligationsSettle` (accounted + existing accounting green → `cancelled`), `TestCancelReconcilerUnavailableFailsClosed` (nil reconciler → finding + pending). Verify existing cancel tests still pass with the seam defaulting appropriately in fixtures.
- [ ] **Step 3: Run to verify failure** — `go test ./internal/gatedrive/ ./internal/app/ -count=1 -run 'TestReconcile|TestCancel(Pending|Completes|Reconciler)'` → FAIL.
- [ ] **Step 4: Implement** per the interface blocks. Enumeration: walk the drive registry in deterministic id order (mirror `inventoryLegacyDrives`); for each readable record bound to the canonical worktree, resolve its epoch via the same linkage rule as `resolveDriveEpoch` (Task 3 — reuse it; do not restate the predicate, per learning `duplicated-gate-copies-the-whole-predicate`) and take only exact-epoch matches. Skip terminal drives whose teardown the slot/participants already account. No epoch lock is taken anywhere in this path (the epoch is already fenced; cancellation holding epoch while probing claims is forbidden by the lock order).
- [ ] **Step 5: Run to verify pass** — both packages, `-count=1`.
- [ ] **Step 6: Mutation-check** — (a) make the reconciler skip scopeless slot-linked drives: `TestReconcileSeesPendingReservedDrive` (scopeless variant) reddens; (b) treat a busy claim as accounted: `TestReconcileBusyClaimIsPending` reddens; (c) drop the app-side and-ing of `accounted`: `TestCancelPendingWhileLaunchObligationUnresolved` reddens. `-count=1`. Restore.
- [ ] **Step 7: Commit** — `git commit -m "feat: cancellation accounts for pending and replacement launches (change 0437 task 6)"`.

---

### Task 7: Deterministic race barriers and lock-order proofs

**Files:**
- Test: `internal/gatedrive/driver_concurrency_test.go` (extend; follow its existing channel-barrier idioms), `internal/gatedrive/driver_epochfence_test.go` (extend)

**Interfaces:** Consumes everything above; produces no production code. This task is AC2 + AC6's concurrency half, end to end. Every ordering assertion is a channel/done-ordering fact; no timing sleep is an oracle anywhere.

**Steps:**

- [ ] **Step 1: Write the barrier matrix** (each a test; a fake `EpochLaunchGate` backed by a mutable fake registry map + mutex stands in for the app gate, so "cancel" = flip the fake's state + run `ReconcileEpochLaunches`):
  - `TestBarrierCancelBeforeAdmit` — fence first: `Admit` refuses, nothing durable exists, reconcile reports accounted (nothing to account).
  - `TestBarrierCancelBetweenAdmitAndStartAdmitted` — fence lands after Admit's return: `StartAdmitted` refuses, no `Launch`, reconcile sees the reserved drive pending until the refusal settles it, then a replay accounts.
  - `TestBarrierCancelBetweenAuthorizationAndLaunch` — fake gate parks AFTER a successful `reserve` (final authorization won) and before `proc.Launch`; concurrent reconcile at the barrier reports `claim-busy` pending; after the launch+attach completes and is stopped, replay accounts. Asserts the process CAN be created after the fence yet cancellation cannot complete until it is identified and stopped (the spec's second race outcome).
  - `TestBarrierCancelBetweenLaunchAndAttach` — fake `proc.Launch` returns, park before `attachLaunch`: reconcile is pending (busy claim); after attach + stop, replay accounts. Assert no launch after a COMPLETED (accounted) reconcile: once a replay reports accounted, a subsequent `StartAdmitted`/relaunch attempt on the same epoch refuses and `proc.Launch`'s call count is final.
  - `TestBarrierSameScopeFirstStartContention` and `TestBarrierSuccessorUnderCancel` — the initial-start peer race and the successor path under a mid-flight fence: exactly one winner ever launches; the loser releases nothing the winner adopted; a fenced successor start refuses before rotation OR after rotation with the rotated slot released (both legal; assert no launch and no leaked non-released reservation either way).
- [ ] **Step 2: Lock-order and no-deadlock proofs:**
  - `TestNoEpochAcquisitionWhileClaimHeld` — instrument the fake gate to record enter/exit; drive the recovery entry (`Advance` on `RelaunchReserved`) and `StartAdmitted`; assert every gate ENTER happens while `tryRelaunchClaim` (probed from the gate callback itself) can still be acquired — i.e., the claim is never already held by this caller when the gate is entered.
  - `TestClaimContentionBounded` — N goroutines race `StartAdmitted`/`Advance`/`ReconcileEpochLaunches` over one drive with a blocking fake launch: all return (done-channels), exactly one launch, contenders report busy/typed refusals — never block indefinitely (bounded by the barrier release, not a timeout oracle).
- [ ] **Step 3: Run** — `go test ./internal/gatedrive/ -count=1` and `go test -race -count=1 ./internal/gatedrive/` (the race detector run is part of this task's evidence; `-count=1` per the cached-runner learning).
- [ ] **Step 4: Commit** — `git commit -m "test(gatedrive): deterministic cancel/launch race barriers and lock-order proofs (change 0437 task 7)"`.

---

### Task 8: Launch-site guard (syntactic, computed) + guard mutation sweep

**Files:**
- Test: `internal/gatedrive/launch_sites_guard_test.go` (new)

**Interfaces:** Consumes `epochGated` / `authorizeRelaunch` / `revalidateAdmittedLaunch` symbol names (Tasks 1–3). Produces the standing guard AC7 requires: "Derive any launch-site guard from syntactic executable call sites, not a hand-maintained spelling list."

**Steps:**

- [ ] **Step 1: Write the guard.** Parse every non-test `.go` file in `internal/gatedrive` with `go/parser`. Compute:
  1. **Population:** every call expression whose selector is `.Launch(` on the driver's process seam (match the receiver by the package's own field name `proc` via AST, not a regex — a spelling guard matches a spelling; bound it syntactically per learning `byte-pattern-guard-matches-a-spelling`).
  2. **Boundary set:** the functions that invoke the epoch authorization boundary (`epochGated`) — computed from the AST, not written down.
  3. **Reachability:** build the package-internal call graph (function decl → called same-package functions) and assert every Launch call site is reachable ONLY from exported entry points whose path passes through a boundary-set function. Concretely: for each function containing a Launch site, walk callers transitively to the package's exported methods; every such path must include a boundary-set member.
  - **Population floor (learning `marker-scoped-guard-needs-a-population-floor`):** assert the population is non-empty AND assert the count is COMPUTED-and-reported, with a hard failure if it is zero (a guard that finds no launch sites is broken, not green).
- [ ] **Step 2: Run to verify it passes against the finished tree**, then **mutation-test the guard itself both ways**: (a) add a bare `d.proc.Launch(rec.launchRequest())` in a new unguarded helper → guard reddens (detects addition); (b) delete the `epochGated` call from `authorizeRelaunch` → guard reddens (the boundary set loses the path). Restore; `-count=1` on every probe.
- [ ] **Step 3: Full guard-family mutation sweep** (one sitting, recorded in the commit message body): re-run the Task 1–6 mutation checks against the FINAL tree — liveness check, claim exclusion, successor ownership (token rotation), pending-launch accounting — each strip must redden a named test under `go test -count=1`. Any probe that stays green is a defect to fix now (learning `assert-detects-removal-not-replacement`).
- [ ] **Step 4: Commit** — `git commit -m "test(gatedrive): AST launch-site guard binds every launch to the epoch boundary (change 0437 task 8)"`.

---

### Task 9: Retained-behavior pins, full-suite gate, and the 435 boundary note

**Files:**
- Test: verify/extend `internal/gatedrive/takeover_epoch_test.go`, `internal/gatedrive/suitebudget_test.go`, `internal/gatedrive/admission_test.go`, `internal/app/gate_drive_test.go`

**Steps:**

- [ ] **Step 1: Audit the retained-behavior tests AC7 names** — between-drive epoch ownership (`ErrStaleRunEpoch` fence in `reserveWorktreeExecution`), takeover revocation protection (`takeover_epoch_test.go`), no-budget-reset on cancellation, and ordinary-release `RunEpochID` retention. Run each suite; where a named protection has no existing test, add the pin now (grep first — never duplicate an existing assert; extend the file that owns the behavior).
- [ ] **Step 2: Verify the epoch-less standalone path end to end** — one test (likely existing, verify) proving a standalone gate (empty `RunEpochID`, finalize-style) admits, launches, relaunches, and releases exactly as before this change, with the gate wired but never consulted (AC1's second half).
- [ ] **Step 3: `gofmt -l` clean check** over all touched files; fix any drift.
- [ ] **Step 4: Full-suite gate.** Resolve the build test command from config (never a second copy) and run it from this checkout: `go run ./cmd/docket development test`. Green is required. **Read the budget report**: a `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line is a screening finding to note; a `SERIAL CONFIRMED OVER BUDGET:` line must be acted on (serial-confirm per `tests/README.md`).
- [ ] **Step 5: Record the boundary** — in the build's results notes (for the results file at completion): stale ownership retirement (releasing/repairing pre-existing stale `RunEpochID` slots) and historical terminal-cancel repair remain assigned to change 435; this change deliberately leaves stale-slot refusal in place and clears no stale `RunEpochID` fields (AC8).
- [ ] **Step 6: Commit** any pins added — `git commit -m "test: pin retained epoch/takeover/budget/release behavior (change 0437 task 9)"`.

---

## Self-Review (performed)

- **Spec coverage:** §Validate-at-admission-and-every-launch → Tasks 1–3, 5; §Serialize-with-existing-locks → Tasks 1–2 (gate-wrapped reservation, claim across launch), 6 (no epoch while probing claims), 7 (proofs); §Successor-reservation → Task 4; §Cancellation-accounting → Task 6; §Refusals-and-budget → Tasks 2, 5; AC1→T1/T5/T9, AC2→T7, AC3→T4, AC4→T3/T6, AC5→T6, AC6→T5/T7, AC7→T8/T9, AC8→T9.
- **Type consistency:** `EpochLaunchGate(epochID, worktree string, reserve func() error) error` is uniform across Tasks 1, 3, 5, 7; `rotateWorktreeExecutionForSuccessor(worktreeRoot, oldToken string) (string, error)` across 4, 6, 7; `ReconcileEpochLaunches(worktreeRoot, epochID string) (EpochLaunchReport, error)` across 6, 7; `resolveDriveEpoch(rec driveRecord) (string, bool, string)` across 3, 6.
- **Known judgment points left explicit for the builder:** the unbound-epoch (empty `Worktree`) refusal in Task 5 is the spec's fail-closed rule — if an existing production flow legitimately starts a gate before `bindEpochWorktree`, that is a concrete design conflict to REPORT, not to paper over; Task 4's `ownsSlot`/release-leg audit is spelled out in its third test rather than assumed.
