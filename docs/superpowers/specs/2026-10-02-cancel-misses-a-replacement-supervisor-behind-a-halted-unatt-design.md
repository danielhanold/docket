<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0493 — Retire the automatic gate relaunch](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-03-0493-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt.md)**
<!-- docket:backlink:end -->

# Retire the automatic gate relaunch — design

## Summary

A gate drive whose supervisor dies mid-run can currently earn one automatic relaunch, if the drive opted in with `IdempotentSuiteGate`. Only finalize's local gate opts in, and `evidence.recertify` shares that gate. This change removes the relaunch completely. When a supervisor dies, every drive now halts, exactly as build drives already do, and a human re-runs finalize. Re-running finalize re-runs the suite anyway.

Removing the relaunch also removes the bug class this change was opened for: a relaunch whose launch response was lost could leave a replacement supervisor that the run census never accounts. With no relaunch, a drive has at most one launch, and the census already handles both ways that launch can be recorded. 0492's item 3 (`proveNoTreeSurvives` trusting `vanished` before a relaunch) goes away for the same reason.

**Hard constraint (set at grooming): this change must add no other risk.** Build drives must behave exactly as they do today, apart from the renamed halt cause. Finalize and recertify gain only the halt that replaces the relaunch. The *Risk analysis* section checks each point.

## Background

### Why the direction changed

0493 was captured to fix the census for a relaunch that halted without attaching its run dir: `run.cancel` and `run.verdict`'s success closeout prove only the run dirs a drive records, so a replacement supervisor that started despite a lost launch response would be missed. Grooming traced that gap on `main` (`756fea9`) and found that no production run can reach it:

- only a drive with `IdempotentSuiteGate` can relaunch;
- only `processFinalizeGate.RunLocalGate` sets that flag;
- that drive carries no run context;
- the census attributes only drives that carry a run context.

ADR-0132 (problem fact 6) already records that "no production drive both carries a run and can relaunch". The 2026-10-02 backlog review then decided to retire the relaunch instead of hardening around it (recorded on 0493 in the metadata commit `fce79ea3f`).

### The relaunch has never fired here

On this machine the drive registry (`<git-common-dir>/docket/gate-drives/v2`) holds 216 drive records since 2026-09-29. Five are finalize or recertify drives. None has a non-zero `relaunch_count`, a `relaunch_reserved: true`, or a `prior_raw_run_dir`.

### What the relaunch carries today (all on `main` at `756fea9`)

- **Driver (`internal/gatedrive/driver.go`).**
  - `driveSlice`'s death branch (`StateSignaled`, `StateVanished`): `proveNoTreeSurvives`, then the relaunch conditions (`relaunchUsed`, `relaunchRefusal`), `reserveRelaunch`, the bounded worktree-lock re-take (`relaunchLockTries`, `relaunchLockPause`), `Launch`, token resolution on a launch error, `attachReservedRelaunch`, and the holder-note rewrite (`PriorHolder`, `ownerIf`).
  - `Advance`'s `RelaunchReserved` branch: `recoverReservedRelaunch` and `haltReservedRelaunch`/`haltReservedRelaunchCause`.
  - The sentinel `errRelaunchRaceLost` and `sliceResult.relaunchRaceLost`.
- **Halt causes only the relaunch emits:**
  - `relaunch-exhausted`, `relaunch-failed`, and `launch-unconfirmed`;
  - `worktree-busy` as a HALT cause (`CauseWorktreeBusy`). The admission refusal spelled `worktree-busy` (`ErrWorktreeBusy`) is a different mechanism and stays;
  - for dead finalize drives only: `relaunchRefusal`'s `deadline-expired`, `fingerprint-error`, and `worktree-changed`.

  Build drives, which never opt in, halt `not-idempotent` when their supervisor dies, or `uncertain-ownership` when the death cannot be confirmed.
- **Record (`internal/gatedrive/drive.go`):** `IdempotentSuiteGate`, `RelaunchCount`, `RelaunchReserved`, `RelaunchToken`, and `PriorRawRunDir`.
- **Opt-in surface:**
  - `StartRequest.IdempotentSuiteGate` in `gatedrive`;
  - `GateDriveStartRequest.IdempotentSuiteGate` in `internal/app/gate_drive.go`;
  - the `--idempotent-suite-gate` flag on `gate drive start` in `internal/cli/gate.go`;
  - finalize's `IdempotentSuiteGate: true` in `RunLocalGate`.

  No skill, agent, or script passes the flag.
- **Census (`internal/gatedrive/reconcile.go`):** `reconcileRunDrive`'s non-terminal `RelaunchReserved` branch (`reconcileReservation`, `settleNeverLaunchedCancelled`), `proveRunDirsGone`'s `PriorRawRunDir` leg, and the `!r.RelaunchReserved` clause in `settleNeverLaunchedFirstLaunch`'s guard.
- **Holder note (`internal/gatedrive/worktree_lock.go`):** `WorktreeLock.PriorHolder`. Its only caller is the relaunch.
- **Shared, not relaunch-specific:** the per-drive claim. The `relaunch.lock` file is opened through `tryRelaunchClaim` and the `relaunchClaim` type. `revalidateAdmittedLaunch`, which `StartAdmitted` calls, and the census's claim-busy probe also use it.
- **Prose:**
  - `skills/docket-build/SKILL.md`: "a bounded relaunch of a proven-dead **idempotent** suite gate … a non-idempotent gate earns no relaunch";
  - `skills/docket-finalize-change/references/gate-failure.md`: the sentence about the gate's "single automatic relaunch" halting `worktree-busy`;
  - `docs/reference/glossary.md`: the relaunch sentence under `worktree-busy`, and the remark that `launch-unconfirmed` "survives only as a gate-drive HALT cause".
- **ADRs:**
  - ADR-0098 (status `Superseded by ADR-0107`) permitted "at most one non-overlapping relaunch";
  - ADR-0107's "Preserved from ADR-0098" list does not carry the relaunch;
  - ADR-0132 (Accepted) describes the relaunch's lock re-take, its holder-note rewrite, and problem fact 6;
  - ADR-0087 mentions "one bounded relaunch" only as historical context about `gate-run.sh`.

## Design

### 1. A supervisor death always halts

In `driveSlice`, the `StateSignaled`/`StateVanished` branch keeps its first step unchanged. `proveNoTreeSurvives` still runs, and a failure still halts `uncertain-ownership`. Every other outcome then halts with a new cause, **`supervisor-died`**, exported as `CauseSupervisorDied` beside the other cause constants in `drive.go`. Nothing after that runs: no relaunch conditions, reservation, lock re-take, launch, attach, or holder-note write.

- **Build drives** keep their exact steps. Only the token changes, from `not-idempotent` to `supervisor-died`.
- **Finalize and recertify drives** halt where they would have relaunched. Finalize's `mapDriveHaltCause` maps `supervisor-died` to `GateHaltUnavailable` through its existing default, so finalize reports reason `gate-halted`, the same as for any other unavailable halt.
- `proveNoTreeSurvives` keeps its behaviour. Its comment now says it decides between `uncertain-ownership` and `supervisor-died` rather than gating a relaunch.

### 2. Delete the relaunch machinery

- **Driver:**
  - `reserveRelaunch`, `recoverReservedRelaunch`, `haltReservedRelaunch`, `haltReservedRelaunchCause`, and `attachReservedRelaunch`;
  - `Advance`'s `RelaunchReserved` branch. `Advance` calls `driveAndPersist` directly;
  - the slice's claim plumbing. The first launch (`launchScopeless`) already releases its claim before the slice and calls `driveAndPersist` with none, so after this change nothing passes a claim into a slice. Fold `driveAndPersistClaim` into `driveAndPersist` and drop `driveSlice`'s `claim` parameter. `launchScopeless` keeps releasing its claim at the same point; only its comment, which says the slice "can reserve its own single relaunch", changes;
  - `relaunchRefusal`, `relaunchLockTries`, `relaunchLockPause`, `ownerIf`, `errRelaunchRaceLost`, `sliceResult.relaunchRaceLost`, and its branch in `driveAndPersistClaim`;
  - the `relaunchUsed` local.
- **Halt-cause emitters:** `relaunch-exhausted`, `relaunch-failed`, `launch-unconfirmed`, `not-idempotent`, and the `CauseWorktreeBusy` constant.
- **Record fields:** `IdempotentSuiteGate`, `RelaunchCount`, `RelaunchReserved`, `RelaunchToken`, and `PriorRawRunDir`.
  - `Attempt` and the output document's `attempt` stay; the value is now always 1.
  - The `RunRoot` field stays. Its comment no longer says a relaunch replays it.
- **Opt-in surface:**
  - `StartRequest.IdempotentSuiteGate` and `GateDriveStartRequest.IdempotentSuiteGate`;
  - the `--idempotent-suite-gate` flag. Its catalog signature drops it automatically, since `buildSignature` derives signatures from the cobra flags;
  - finalize's `IdempotentSuiteGate: true`.
- **Holder note:** `WorktreeLock.PriorHolder`.

The historical schema-2 reader (`internal/gatedrive/history.go`) is **not** edited. It describes a frozen historical schema, and the census reads only its terminal outcome.

### 3. The census handles at most one launch

In `internal/gatedrive/reconcile.go`:

- **`reconcileRunDrive`.** The non-terminal `cur.RelaunchReserved` branch, `reconcileReservation`, and `settleNeverLaunchedCancelled` go. A non-terminal drive then has two outcomes: an unattached first launch resolves its admission token (`reconcileFirstLaunch`, unchanged), and an attached drive proves its run dir gone.
- **`proveRunDirsGone`** proves only `RawRunDir`.
- **`settleNeverLaunchedFirstLaunch`'s CAS guard** becomes `r.RawRunDir == ""`. `settleNeverLaunched` keeps its shared shape for that one caller.
- **`reconcileHaltedDrive` is unchanged.** A HALTED drive either never attached (its admission token is resolved) or has its one run in `RawRunDir`. That makes 0493's original gap impossible by construction.
- **Comments.** The package comment and the docs for `ReconcileRunLaunches`, `reconcileRunDrive`, and `proveRunDirsGone` drop every mention of a relaunch, a replacement, or a prior run dir.

### 4. The per-drive claim keeps its file name

- **Go identifiers are renamed** for accuracy: `relaunchClaim` becomes `driveClaim`, `tryRelaunchClaim` becomes `tryDriveClaim`, and `relaunchLockFileName` becomes `driveClaimLockFileName`. The compiler checks every site.
- **The file stays `relaunch.lock` on disk.** A CLI process still running across the upgrade opens that path, and a renamed file would let an old and a new process each hold "the" claim. A comment on the constant records this.

### 5. Old drive records

- The store decodes records with plain `json.Unmarshal`, which ignores unknown fields. An existing record that still carries the deleted fields therefore loads, and those fields are dropped.
- `driveSchemaVersion` stays 4, and the v3 legacy read is unchanged.
- A pre-0493 binary reading a new record sees `idempotent_suite_gate` missing, so false, and never relaunches. Both directions are safe.
- The schema comment in `drive.go` records that change 0493 stopped writing the relaunch fields and that a new store ignores them on read.

**The one upgrade edge.** A record left mid-relaunch (`relaunch_reserved: true`, non-terminal) exists only if a CLI crashed between reserving and attaching. After the upgrade, `Advance` treats it as an ordinary drive: its first run is dead, so it halts `supervisor-died`. If a replacement had actually launched, it runs to completion holding the worktree lock. Finalize's `haltedRunRootHoldsUnexitedRun` scans every slot under the root, so the root and logs are kept. Every such record is a finalize or recertify drive with no run context, so the census never attributes it before or after this change. There are no such records on this machine (see *Rollout*).

### 6. Prose

- **`skills/docket-build/SKILL.md`:** the relaunch sentence becomes "the caller never relaunches, stops a raw run, or composes the raw verbs; a gate whose supervisor dies halts `supervisor-died`, never relaunched." The existing rule that the caller never relaunches is kept. Re-baseline its budget row in `internal/repoguard/budgets_test.go`.
- **`skills/docket-finalize-change/references/gate-failure.md`:** remove the relaunch sentence and keep the admission-refusal sentence. If it reads naturally, add that a supervisor death halts the gate and the fix is re-running finalize. Re-baseline its budget row.
- **`docs/reference/glossary.md`:** remove the relaunch sentence from the `worktree-busy` entry. Rewrite the `launch-unconfirmed` remark to say the token is retired: no driver path emits it since change 0493, and it was the drive HALT cause for a relaunch whose launch could not be established. Add a short `supervisor-died` entry next to the other gate-drive HALT causes, if the glossary lists them.

### 7. The decision record

- **A new ADR, "Gate drives never relaunch automatically".** It records:
  - the decision: a supervisor death halts every drive, and a human re-runs the workflow;
  - why: the relaunch never fired in practice; it was the only source of the lost-replacement class; it is the reason 0492's item 3 exists; and its crash-window machinery is a large share of the driver;
  - the accepted cost: a rare supervisor death in finalize now needs a human re-run;
  - the alternatives rejected: hardening the census (the original 0493 scope), and forbidding a relaunch only for run-attributed drives.

  Set `relates_to: [87, 98, 107, 132]`. It supersedes and reverses no ADR whole. The `docket-adr` dispatch records it at build time.
- **ADR-0132 gets a dated `## Update` note** pointing to the new ADR. Its *Relaunch* admission bullet and the holder-note sentence "A relaunch rewrites it" no longer apply, and problem fact 6 now holds by construction.
- **No other edits.** ADR-0098 is already superseded, its status line is not edited again, and ADR-0107 never carried the relaunch. ADR-0087's mention is historical context about a retired script.

## Before and after

| Situation | Today | After |
|---|---|---|
| A build gate's supervisor dies | HALTED `not-idempotent` (or `uncertain-ownership`) | HALTED `supervisor-died` (or `uncertain-ownership`) |
| A finalize or recertify gate's supervisor dies, within its deadline, worktree unchanged | one automatic relaunch on the same deadline | HALTED `supervisor-died`; finalize reports `gate-halted` and a human re-runs finalize |
| Same, after the deadline | HALTED `deadline-expired` (finalize: `running-at-budget`) | HALTED `supervisor-died` (finalize: `unavailable`) |
| Same, worktree changed or fingerprint unreadable | HALTED `worktree-changed` / `fingerprint-error` | HALTED `supervisor-died` |
| A live run passes its deadline | HALTED `deadline-expired` | unchanged |
| `gate drive start --idempotent-suite-gate` typed by hand | accepted | refused as an unknown flag |
| Census, cancel, success closeout, for any production drive | — | unchanged (no production drive it attributes ever carried relaunch state) |

## Risk analysis

For each risk: what could go wrong, and why it does not.

1. **Build drives.** Their death branch runs the same `proveNoTreeSurvives` step and halts at the same point. Only the token differs (`supervisor-died` instead of `not-idempotent`). No code reads either token: finalize maps both through its default, the build skills and their `gate-caller-loop.md` treat HALTED by outcome, and the cause is reported, never branched on. Every other build path (start, admission, the suite-attempt budget, WAITING slices, PASSED/FAILED, deadline expiry with a live run, handoff and claim) is untouched.
2. **Finalize and recertify.** The only change is that a supervisor death halts instead of relaunching. A halt is finalize's existing fail-closed outcome: the PR stays open, the change stays `implemented`, and `## Finalize blocked` records the reason as for any `gate-halted`. The recorded history shows zero relaunches, so this costs a human re-run only on a rare death. The label shift for a death after the deadline (`running-at-budget` to `unavailable`) touches only the report field `HaltCause`. Nothing branches on it, and the new label is accurate, since nothing is running.
3. **Old records.** These are covered under Design §5. Loading cannot break, because unknown JSON fields are ignored and the schema version is unchanged. A pre-0493 binary reading a new record never relaunches. The one upgrade edge is a record left mid-relaunch by a crashed CLI. It halts instead of being recovered, and finalize keeps its root. There are zero such records here, and *Rollout* gives a one-line check.
4. **The per-drive claim.** Its on-disk name is unchanged, so `StartAdmitted`'s revalidation and the census's claim-busy probe keep excluding each other, across the upgrade too. The Go rename is mechanical and compiler-checked.
5. **The census.** For every production drive it attributes (build drives that carry a run context), `RelaunchReserved` was always false and `PriorRawRunDir` always empty. The removed branches never ran for them, so cancel and the success closeout behave identically. No finding token is added or removed in practice: `launch-pending` and `replacement-stopped` remain emitted by the first-launch and stop paths.
6. **Locks and lock order.** No lock is added. The relaunch's worktree-lock re-take, the only place a drive took the worktree lock outside admission, goes away. That removes a lock acquisition and adds none.
7. **The CLI flag.** No skill, agent, or script passes `--idempotent-suite-gate`. A hand-typed use now fails loudly as an unknown flag rather than silently. `TestRepresentativeSignatures` pins the `gate.drive.start` signature and is updated to match.
8. **Vocabulary guard.** The retired-vocabulary guard (`internal/repoguard/retired_vocabulary_test.go`) forbids *old* tokens and never requires a replacement to exist. Removing `launch-unconfirmed`'s last emitter is safe, and its fixture strings stay valid. `supervisor-died` is a new token, not a rename recorded in ADR-0129, so it needs no guard row.
9. **Budgets.** Two skill files change, so their rows in `budgets_test.go` are re-baselined to the new exact counts, as every prose change does. No Go-side budget guard references the touched symbols.
10. **Other repos on this machine.** Each repo that uses docket has its own drive registry. The upgrade edge in item 3 applies per repo, so the *Rollout* check runs in each.

## Tests

Derive the test sites from a whole-repo grep (AGENTS.md: never hand-list the sites of an operation you are gating). The grep should cover `RelaunchReserved`, `RelaunchToken`, `RelaunchCount`, `PriorRawRunDir`, `IdempotentSuiteGate`, `idempotent-suite-gate`, `reserveRelaunch`, `recoverReservedRelaunch`, `relaunch-failed`, `relaunch-exhausted`, `not-idempotent`, `launch-unconfirmed`, `CauseWorktreeBusy`, `PriorHolder`, and `ownerIf`. On `756fea9` it finds 15 test files across `internal/gatedrive`, `internal/app`, `internal/cli`, and `internal/repoguard`. Sort them into three groups:

- **Tests that assert relaunch behaviour** are deleted or rewritten as halt tests. Examples: `TestSignaledDeathRelaunchAdmittedOnce`, `TestDeathRelaunchRefusals`, `TestRelaunchCrashBetweenReserveAndLaunchRecovers`, `TestRelaunchReservationNotRefundedOnUncertainty`, `TestRelaunchFindsWorktreeHeldHaltsWorktreeBusy`, `TestRelaunchWaitsOutItsOwnSupervisorsExit`, `TestRelaunchRewritesHolderKeepingOwner`, `TestConcurrentSameOwnerAdvanceRelaunchesOnce`, `TestRelaunchReservationHolderCannotBeStolenBeforeLaunch`, `TestRelaunchCrossesNoRunGate`, `TestCensusReservedRelaunchResolvesRelaunchToken`, `TestReconcileNeverLaunchedSettlesTerminalClosingRecoveryLaunchWindow`, and `TestIntegrationGatedriveProcessDeathPermitsAtMostOneRelaunch`.
- **Tests that set the flag or a field incidentally** drop it and keep every assertion. Examples: the finalize-mirroring and verdict fixtures in `internal/app` that set `IdempotentSuiteGate: true`.
- **Pins** are updated: `TestRepresentativeSignatures` and the two budget rows.

New and rewritten tests:

1. **Death halts, for both kinds of drive.** A signaled death and a vanished death halt `supervisor-died`, for a drive started as finalize starts one and for a build drive. `Launch` is called exactly once (the first launch), and the worktree lock is never taken again.
2. **Unproven death.** When `proveNoTreeSurvives` fails, the drive halts `uncertain-ownership`, as today.
3. **Integration (`supervisor_integration_test.go`).** Kill a real supervisor mid-run. The drive halts `supervisor-died`, no second run dir appears under the run root, and the worktree lock is free afterwards.
4. **Finalize mapping.** A `supervisor-died` drive maps to `GateHaltUnavailable`, and finalize reports `gate-halted`.
5. **Old-record compatibility (`store_test.go`).** A v4 record carrying `idempotent_suite_gate: true`, `relaunch_count: 1`, `relaunch_reserved: true`, `relaunch_token`, and `prior_raw_run_dir` loads without error, and its next write omits them. A non-terminal one advanced over a dead run halts `supervisor-died` and launches nothing.
6. **Census without relaunch.** An attributed HALTED drive whose recorded run dir is live is stopped by cancel and reported `run-live` in observe mode, and only `RawRunDir` is probed. The existing first-launch census tests pass unchanged.
7. **The claim file name.** The per-drive claim is still created at `relaunch.lock`, which pins the on-disk compatibility of Design §4.

**Mutation checks** (AGENTS.md: a guard is code). Each of these must turn a test red:

- re-adding any launch after a death: test 1 or test 3;
- changing the claim file name: test 7;
- making the store decode strictly (`DisallowUnknownFields`): test 5;
- dropping the finalize default mapping for the new cause: test 4.

**Must pass unmodified:** every test outside the three groups above, in particular every census, admission, worktree-lock, handoff and claim, suite-budget, and run-tracker test that does not set a removed field. If such a test needs an assertion changed, the change has moved production behaviour beyond this design. The build must stop and report it rather than adjust the test. The build gate runs the whole suite.

## Rollout

Before installing the new binary, check each repo that uses docket for a drive left mid-relaunch:

```sh
grep -lE '"relaunch_reserved": ?true' "$(git rev-parse --git-common-dir)"/docket/gate-drives/v2/*/record.json
```

No output means there is nothing to handle. On this repo the check returned no files over 216 records at grooming time. If any file is listed, let that drive's finalize finish, or re-run it, before installing. The results file carries this check under *Human actions and testing*.

## Effect on follow-ups

- **0492:** item 3 (`proveNoTreeSurvives` trusting `vanished` before a relaunch) has no relaunch left to protect and is dropped, as 0492's own open question already anticipates. `proveNoTreeSurvives` still runs before a death halt, but it only chooses between two halt causes. Items 1, 2, and 4 are unaffected.
- **0491:** unaffected. Census attribution by run context is unchanged.

## Acceptance criteria

- No driver path launches a second run for a drive. A supervisor death halts `supervisor-died`, or `uncertain-ownership` when the death cannot be proven.
- `IdempotentSuiteGate`, the relaunch record fields, the `--idempotent-suite-gate` flag, and every relaunch-only function, constant, and halt-cause emitter listed in Design §2 are gone. The per-drive claim file is still `relaunch.lock`.
- Records written before this change load and advance under the rules in Design §5.
- The census proves only `RawRunDir` and has no relaunch branch. `reconcileHaltedDrive` is unchanged.
- The prose in Design §6 is updated, and the two budget rows are re-baselined.
- The new ADR is recorded, and ADR-0132 has its Update note.
- Every test outside the three groups in *Tests* passes without modification, and the mutation checks hold.

## Out of scope

- Tearing down a suite after its supervisor is gone: process groups outliving the supervisor, KILL escalation, and graceful-stop timing. That is 0492's items 1, 2, and 4.
- Any automatic re-run in finalize after a `supervisor-died` halt. That would reintroduce a relaunch.
- A never-launched drive blocking a successful run's closeout (0490 review finding F3, recorded under 0491).
- The run id and its fences (0491).
- The worktree lock and its holder model (0490, ADR-0132), apart from deleting the relaunch's re-take and `PriorHolder`.
- Deleting the `relaunch.lock` files or the old relaunch fields from records already on disk.
