<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0437 — Reject revoked run epochs before gate-start admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0437-reject-revoked-run-epochs-before-gate-start-admission.md)**
<!-- docket:backlink:end -->
# Reject revoked run epochs before gate-start admission — Results

## Outcome

Every epoch-backed gate-drive launch route now validates run-epoch liveness before it launches a
process, closing the defect where the epoch-revocation resolver was consulted only on the takeover
path. Implements ADR-0118's "live epoch ownership between sequential drives" requirement.

Delivered:

- A new injected seam `gatedrive.EpochLaunchGate` (`internal/gatedrive/driver.go`): the app-owned
  authoritative liveness read that runs the driver's durable reservation body while the epoch
  registry's per-key lock is held, so a concurrent cancellation fence either lands before the read
  (reservation never runs) or observes the durable reservation. `Driver.Admit` now runs its
  scoped/scopeless reservation through this gate; a nil gate or an empty epoch id preserves the
  genuinely epoch-less standalone behavior unchanged.
- `Driver.StartAdmitted` re-validates the epoch and the exact durable reservation under the gate and
  holds the existing per-drive claimant flock across launch/attach (released after
  `ConfirmWorktreeExecution`, before the drive slice, because flock is per-open-fd). A delayed
  ticket refused between Admit and launch is settled `HALTED run-cancelled`, launching nothing.
- Automatic relaunch (`driveSlice`) and reserved-relaunch recovery (`Advance`) are epoch-gated via
  `authorizeRelaunch` / `resolveDriveEpoch`; a lost/inconsistent epoch linkage refuses new execution
  (never demotes to standalone). Recovery may still identify and stop an already-created process
  after revocation (teardown, not permission).
- A same-scope successor gets its own execution reservation
  (`admission.go:rotateWorktreeExecutionForSuccessor`): a fresh `ReservationToken` + bumped
  `ExecutionGen`, predecessor raw-run identity cleared, `RunEpochID` preserved; the predecessor's
  stale token loses authority so a late predecessor release/abandon cannot free the successor.
- The production `epochLaunchGate` (`internal/app/rungate_gate.go`) is a read-only liveness read
  under the existing `epoch.lock` (no epoch write), failing closed on cancelling/cancelled →
  `ErrRunCancelled`, superseded/unknown, and empty/wrong worktree binding → `ErrStaleRunEpoch`. It
  is wired at every production gate-drive constructor: `newOwnedGateDriveService` (build + finalize),
  `NewTaskGateDriveService`, `NewCommandlessGateDriveService`, and `NewContinuationSeam`.
- Cancellation now accounts for pending and replacement launches
  (`gatedrive.ReconcileEpochLaunches`, consumed by `reconcileEpochTeardown` for both `run.cancel`
  and the death guardian): it censuses epoch-linked scope/drive reservations and the relaunch
  journal, takes the per-drive claim nonblocking (busy is pending, never waited on), and keeps
  cancellation pending on any unproven stop, busy claim, unresolved resolution, unreadable record,
  or unresolved epoch linkage.
- A syntactic AST launch-site guard (`launch_sites_guard_test.go`) binds every `proc.Launch` call
  site to the `epochGated` boundary via the package-internal call graph with a computed population
  floor.

No new daemon, background loop, persistent store, schema, lifecycle state, CLI command,
configuration, or coordination framework was added — only the existing epoch registry, `epochCAS` /
epoch lock, per-drive `relaunchClaim`, worktree admission slot, and scope/drive reservation records.

## Verification performed

- TDD throughout: each of the 9 plan tasks and both review fixes drove RED → GREEN through the
  native gate driver with `go test -count=1` (never a cached verdict), and every new guard was
  mutation-tested (strip the guard, watch its named test redden, restore).
- Deterministic concurrency proofs (Task 7): the cancel-vs-launch barrier matrix and lock-order /
  no-deadlock tests use channel/done-channel oracles only — no timing sleeps as oracles — and pass
  under `go test -race`.
- Full source-resolved build suite (`go run ./cmd/docket development test`) run from the feature
  checkout via the gate driver. The initial run went red on a single cross-task defect: Task 8's new
  guard file used bare `t.TempDir()`, which the repo-wide `TestRealProcessPackagesUseFixtureTempDir`
  guard (`internal/repoguard`) rejects; it was repaired to `testsupport.TempDir(t)` and the re-run
  is green (52/52 suite files, 0 failures). Build evidence recorded green and verified against the
  branch HEAD.
- Budget report: only advisory `BUDGET WATCH:` screening findings (parallel-overrun streak 1/5) on
  pre-existing large shards (`test_go_race`, `test_go_toolchain`, and three integration shards) —
  no `SERIAL CONFIRMED OVER BUDGET:` breach, so nothing to act on.
- Independent deep whole-branch review returned 0 blockers and 2 important findings; both were
  about this branch's own core guarantee and were fixed in-branch (see Findings and limitations).

## Findings and limitations

### Launch-after-completed-cancellation window in reserved-relaunch recovery (fixed)

Deep review found a TOCTOU: `Advance`'s reserved-relaunch recovery read epoch liveness (releasing
the epoch lock) and then took the per-drive claim as a separate step using a stale `revoked` value,
so a concurrent cancellation could settle the never-launched reservation and complete before
recovery launched the replacement — violating the change's central "no process after completed
cancellation" invariant (spec AC4). Fixed (commit e70e1d01): `reconcile.go`'s never-launched arm now
settles the drive terminal `HALTED run-cancelled` under the held claim via `ownerCAS`, so a later
recovery sees the terminal record and never launches. Covered by a deterministic test that is green
under `-race` and mutation-checked.

### Fail-open skip in the cancellation census (fixed)

`ReconcileEpochLaunches` silently `continue`d over a drive whose epoch linkage was unresolvable
(`resolveDriveEpoch` ok==false), deviating from the spec's fail-closed census. Fixed
(commit 98671b39): a lost/unreadable linkage now emits a bounded `linkage-unresolved:<id>` finding
and keeps `Accounted=false`, mirroring the existing `record-unreadable:<id>` leg; a clean
other-epoch/epoch-less resolution is still correctly skipped.

### Design note — no per-drive worktree filter in the census (reviewer-confirmed sound)

`ReconcileEpochLaunches` matches drives by the globally-unique epoch id and deliberately omits a
per-drive worktree exclusion filter, because such a filter is a fail-open risk (it could only
wrongly exclude a real obligation). The reviewer confirmed this cannot over- or under-account given
the exact epoch association.

## Follow-ups

### Change 435 owns stale-slot retirement and terminal-cancel repair (out of scope here)

Per the spec's implementation order, 437 lands first and deliberately leaves stale-slot refusal in
place; it clears no stale `RunEpochID` fields. Stale epoch-ownership retirement, cancellation-
specific epoch detachment, terminal-cancel repair and its resume check, and the existing slot
teardown ownership/error audit remain assigned to change 435 (`depends_on: [437]`, already proposed).
No new change minted; this is the existing tracked dependency.
