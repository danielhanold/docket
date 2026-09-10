<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0405 — Investigate the gate.drive.prepare-scope -> gate.drive.start handshake rejecting a build-task worker's focused gate](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-10-0405-investigate-the-gate-drive-prepare-scope-gate-drive-start-ha.md)**
<!-- docket:backlink:end -->
# Investigate the gate.drive.prepare-scope -> gate.drive.start handshake rejecting a build-task worker's focused gate — Results

## Outcome

A build-task recovery scope can now carry a *sequence* of task-owned test drives — baseline,
failing regression (RED), fix verification (GREEN) — instead of permanently binding the single
drive its first scoped start created. Previously a prepared recovery scope bound exactly one drive,
so a completed test wedged the worker: its next `gate.drive.start` under the same parent-issued
scope was rejected.

The fix reworks the scope record into schema v2, a durable single-slot lifecycle:

- The scope record is the serialization authority for its one drive slot. A start persists a
  durable *reservation* under the scope CAS **before** any process launch, retires the predecessor's
  recovery authority (clears its drive-record owner generation) as the journaled second half of the
  transition, and only then launches — `reserve → retire predecessor → launch → confirm`.
- A successor start acknowledges exactly the received predecessor receipt (its durable
  PASSED/FAILED result) and reuses the scope slot; a new test is always a new drive, never a
  relaunch. Only durably PASSED/FAILED predecessors permit a successor (WAITING, HALTED, pending
  handoff, transferred ownership, closed scope, corrupt or unresolved-launch state all fail closed).
- A new cataloged `gate.drive.acknowledge` operation is the final drive's successor: it consumes the
  last result and closes the scope. Takeover and enumeration now resolve the scope's *current* slot
  rather than timestamps; acknowledged predecessors drop out of recovery-candidate enumeration
  because their owner generation is cleared.
- Ownership diagnostics that previously collapsed to a generic `invalid-request` now surface typed
  `OwnershipErrorKind` reason tokens (`scope-busy`, `stale-predecessor`, `predecessor-not-reusable`,
  `unresolved-launch-transition`, and the existing scope-identity/capability/closed kinds), each
  paired with a bounded valid-next-action message and never leaking wrapped free text, argv, or
  environment contents.
- CLI (`gate.go`): `start` gains `--predecessor-drive-id` / `--predecessor-owner-gen`; a new
  `acknowledge` subcommand is added. The caller contracts (`docket-build-task`, `docket-build`,
  `gate-caller-loop.md`) and their embedded mirrors are updated together for the sequential-start
  receipt and terminal acknowledgement.

The scope schema bumps to v2 and fails closed (`ErrUnknownSchema`) on a v1 or unknown record —
never reinterpreted. The design decision is recorded in ADR-0117; ADR-0107 (event-authorized parent
takeover) is preserved. The change builds on 416's identity-bundle fix, which is unchanged.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) driven through the native build gate to a
  PASSED disposition; durable build evidence recorded and verified against the certified head.
- The change adds substantial focused coverage in `internal/gatedrive`: the single-slot lifecycle
  and reservation/pending-ack journal (`scope_test.go`), reserve→launch→confirm ordering and
  successor retire/clear (`driver_test.go`), fault-injection and restart transitions
  (`driver_faults_test.go`), concurrent linked-worktree scope sequences over real git
  (`driver_concurrency_test.go`, `integration_sequence_test.go`), terminal acknowledgement
  (`acknowledge_test.go`), takeover current-slot resolution (`takeover_test.go`), and typed
  ownership mapping (`ownership_test.go`, `app/gate_drive_test.go`). Guards carry mutation evidence
  per the plan's discipline.
- Whole-branch review at the deep rung (diff > 1500 lines) returned two findings, both fixed
  in-branch (see below); the suite was re-certified after the fixes.

## Findings and limitations

### Orphaned reserved drive when a scoped start loses the slot (important — fixed)

A reserved drive record minted before a losing `reserveScopeDrive` / `retirePredecessor` /
`clearPendingAck` was never bound to a scope and never cleaned up, so `FindScopeDriveIDs` counted it
as a spurious outer-recovery candidate and could make outer takeover fail closed on ambiguity —
degrading the very crash-recovery path scopes provide. Fixed in commit `72b198e1` by best-effort
deleting the just-minted reserved drive directory on all three failure legs of `Driver.startScoped`
(new unexported `Store.removeReservedDrive`, which validates the id and refuses a symlinked dir
before `os.RemoveAll`). Deletion is safe on the retire/clear legs: the slot is left reserved +
pending-ack, which `Start`, `Takeover`, and `Acknowledge` all reject without loading the removed
record. A focused test reproduces the orphan and carries mutation evidence.

### Owner-generation asymmetry in Acknowledge post-retirement paths (minor — fixed)

The idempotent-repeat (`scope.Closed && scope.FinalAcked`) and resumable-half branches of
`Driver.Acknowledge` accept the caller-supplied `ownerGen` without verifying it, unlike the normal
path's `retirePredecessor` check. This is an intentional, safe asymmetry: the `ownerGen` check
protects a *live, owned* drive from a stale owner, but once the final drive's owner is retired the
drive is consumed history, the scope record does not persist the retired owner generation, and
`childCapability` (checked first) is the sufficient scope authority. Commit `c166fc5d` documents the
rationale on the branches (anchored on symbol names) and adds a characterization test locking the
behavior in, so a future switch to a persisted-owner-gen check would surface as a deliberate
contract change rather than a silent one.

No other limitations were identified. Coverage of the concurrency, fault, and restart paths is
extensive (barrier-controlled fakes and a deterministic clock rather than sleeps), so no additional
human-only functional scenario is warranted.
