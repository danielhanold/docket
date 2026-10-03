<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0493 — Retire the automatic gate relaunch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0493-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt.md)**
<!-- docket:backlink:end -->
# Retire the automatic gate relaunch — Results

**Human action:** Before installing this build, run the one-line rollout check below in every repo that uses docket. It takes a few seconds. Nothing else is required.

## Outcome

A test-suite gate whose supervisor process died mid-run could get one automatic relaunch. Only finalize's local gate, and `evidence.recertify` which shares it, had opted in. That relaunch carried a lot of crash-window machinery, and it was the only way to hit the bug this change was first opened for: the run census missing a replacement supervisor. The relaunch had never fired on this machine.

The relaunch is now gone:

- **Every supervisor death halts.** The gate drive halts with the new cause `supervisor-died`, or with `uncertain-ownership` when the death can't be confirmed. Build gates behave as before, except the halt token is renamed from `not-idempotent`. Finalize now reports `gate-halted` where it used to relaunch, and a human re-runs finalize.
- **The relaunch machinery is deleted.** This covers the reservation and token, crash recovery, the worktree-lock re-take, the relaunch-only halt causes, the `IdempotentSuiteGate` opt-in and its `--idempotent-suite-gate` flag (typing that flag is now refused as unknown), and the relaunch fields in the drive record.
- **The census proves one run directory per drive** and has no relaunch branch, so the original gap can no longer happen.
- **Old drive records still load.** The retired fields are ignored on read and dropped on the next write. The per-drive claim file keeps its on-disk name, `relaunch.lock`, so an old and a new binary still exclude each other.

Departures from the spec, all found in the code and checked in review:

- `launch-pending` is no longer produced. Its only producer was the deleted relaunch-reservation branch, and no production code branches on it.
- The admission-coverage pin `TestGateLaunchAdmissionCoverage` now expects 2 launch sites instead of 3.
- `censusFindingRank` is deleted. It only ranked findings across two run directories.
- `historicalView`, the projection in `internal/gatedrive/history.go`, no longer copies the deleted `PriorRawRunDir` field. The spec said not to edit `history.go`, but this projection would not compile otherwise. The frozen schema-2 decoder itself is unchanged.
- `docs/concepts/run-tracker.md` was also updated.

## Human actions and testing

### Important — check for a drive left mid-relaunch before installing

If a CLI crashed while a relaunch was in progress, the record it left would halt instead of being recovered once the new binary is installed. No record on this machine was in that state when this change was groomed. Run the check per repo, because each repo has its own drive registry.

From the root of each repo that uses docket:

1. `grep -lE '"relaunch_reserved": ?true' "$(git rev-parse --git-common-dir)"/docket/gate-drives/v2/*/record.json`
   Expected: no output. A "no matches" message from the shell glob means the same thing.
2. If a file is listed, let that drive's finalize finish, or re-run finalize for it, before installing.

## Verification performed

- Each of the six plan tasks was built test-first by its own worker and ran its focused tests, including the gatedrive integration tests and the app run-cancel, completion and verdict integration tests.
- Mutation checks turned the guarding tests red, as the spec requires:
  - re-adding a launch after a death;
  - re-adding a worktree-lock re-take after a death;
  - halting an unproven death as `supervisor-died`;
  - re-adding the census's prior-run-dir leg or its reservation branch;
  - strict JSON decoding of drive records;
  - re-adding a relaunch field to the record;
  - re-registering the CLI flag;
  - renaming the claim file;
  - mapping the new cause away from finalize's default;
  - a one-word budget change.
- No test outside the relaunch-asserting, incidental-field and pin groups needed an assertion change.
- The full suite is certified by the build gate on the branch head. The evidence is in the PR body.
