<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0517 — Make evidence.record certify a finalize re-test with the finalize gate settings](../../changes/archive/2026-10-04-0517-make-evidence-record-certify-a-finalize-re-test-with-the-fin.md)**
<!-- docket:backlink:end -->

# evidence.record certifies a finalize run with the finalize settings

## Problem

`EvidenceRecord` (`internal/app/evidence_ops.go`) turns a passed gate run into the build-evidence
record. It reads only `build.gate` and `build.test_command`. Two finalize paths call it after a run
of `finalize.test_command`:

- **`finalize.rebase`'s built-in gate.** `processFinalizeGate.mapTerminalDrive`
  (`internal/app/finalize_rebase.go`) calls `EvidenceRecord` on every PASSED drive, including the
  drives built by `NewFinalizeGateDriveService`.
- **The re-test after an integration repair.** The `docket-finalize-change` skill starts
  `gate.drive.start --owner finalize`, then calls the `evidence.record` operation with the PASSED
  run dir.

In both, the record is built from build settings while the finalize command ran:

| Config | Today | Correct |
|---|---|---|
| `build.gate: off`, finalize local | `skipped` / `build-gate-off`, no command | green, names `finalize.test_command` |
| both local, commands differ | green, names `build.test_command` | green, names `finalize.test_command` |
| build local, `build.test_command` empty | refused `unconfigured-gate-command`; the built-in gate maps that to a halt after a passing suite | green, names `finalize.test_command` |

This contradicts change 0374's spec, which said "Finalize's in-process gate continues to validate
against finalize configuration." It went unnoticed because docket's own config sets both commands to
the same value, and so does every existing fixture.

## Design

### Operation: an owner, mirroring the gate driver

`gate drive start` already splits by owner: `--owner build|finalize` selects
`NewBuildGateDriveService` or `NewFinalizeGateDriveService`, and each reads only its own
`<owner>.test_command`. `evidence.record` follows the same split.

- `EvidenceRecordRequest` gains `Owner` (`json:"owner"`). The CLI adds `--owner build|finalize`,
  **optional**. Empty means `build`, so every existing caller keeps its exact behavior.
- Any other value is refused as invalid input with its own stable reason (for example
  `invalid-owner`), before config is read.
- **`build` (or empty): unchanged.** `build.gate: off` mints skipped evidence; a local gate records
  `build.test_command`; an empty `build.test_command` refuses `unconfigured-gate-command`.
- **`finalize`:** reads only `finalize.test_command`.
  - No skipped path, and `finalize.gate` is not consulted. `gate drive start --owner finalize`
    does not consult it either: the gate setting decides whether finalize runs its suite, and once a
    finalize run has passed, the record states what ran. A skipped record's only reason is
    `build-gate-off`, which would be false here.
  - An empty `finalize.test_command` refuses `unconfigured-gate-command`, with a message naming
    `finalize.test_command`.
  - Then the existing local-gate steps, unchanged: a run dir is required (`missing-run-dir`), the
    run is observed and must be `passed`, `verifyFeatureHead` checks the head, and
    `evidence.NewRecord` builds the green record with the finalize command.
- Messages and help name the owner's key instead of hard-coding `build.*`: the `--run` flag help,
  the `unconfigured-gate-command` and `missing-run-dir` messages, the `EvidenceRecord` doc comment,
  and the file header (which today says the operation "no longer re-resolves
  finalize.test_command").
- The evidence record format (`internal/evidence`) is unchanged. A green record already carries its
  command, which is what finalize's skip permit compares with the resolved `finalize.test_command`.

### Callers

- **`processFinalizeGate.mapTerminalDrive`** passes the seam's own owner into
  `EvidenceRecordRequest`, applying the seam's existing zero-value rule (an unset seam owner is
  treated as finalize), so an empty owner never reaches `EvidenceRecord` from this site. The
  finalize rebase gate certifies with `finalize`; `evidence.recertify`'s drive, built with owner
  `build`, stays `build`. `EvidenceRecertify`'s gate-off branch calls `EvidenceRecord` with no owner
  and stays build.
- **Finalize skill text.** `skills/docket-finalize-change/SKILL.md` (the paragraph beginning "Then
  re-gate the repaired head through the gate driver") and
  `skills/docket-finalize-change/references/gate-failure.md` add `--owner finalize` to their
  `evidence.record` call.
- **Implement-next skill text.** `skills/docket-implement-next/SKILL.md`, under "Create the durable
  evidence", says `evidence.record` "reads the observed gate command and outcome from the run
  directory". Correct it: the outcome comes from the run directory, the command from the build
  configuration. Build and implement-next calls take no new flag.
- Regenerate the embedded asset tree after the skill edits.

### Tests

Every fixture configures build and finalize with **different** values (for example `build-cmd` and
`finalize-cmd`). With identical commands the old and new code produce the same record, so only a
mixed fixture can tell them apart (learning `shared-resource-keeps-first-owner-assumptions`).

`EvidenceRecord`, owner `finalize`, with a passed run at the current feature head:

1. `build.gate: off` → green naming `finalize-cmd`, not skipped.
2. Both gates local → green naming `finalize-cmd`.
3. Build local with `build.test_command` empty → green naming `finalize-cmd`, not refused.
4. `finalize.test_command` empty → `unconfigured-gate-command`.

Owner `build` and owner omitted, same mixed fixture: green naming `build-cmd`, and skipped under
`build.gate: off`. An unknown owner is refused before any read.

Built-in gate: a `finalize.rebase` gate pass with the mixed config yields evidence naming
`finalize-cmd`, and green (not skipped) under `build.gate: off`. A recertify drive still names
`build-cmd`.

CLI: `--owner` reaches the request; an unknown value is refused.

Prose guard: add `TestAlignmentContracts` entries for `skills/docket-finalize-change/SKILL.md` and
`references/gate-failure.md` keyed on the `evidence.record` call carrying `--owner finalize`. The
existing `align_0502_finalize_regate` entries look for a bare `` `--owner finalize` ``, which the
`gate.drive.start` clause already satisfies, so they cannot catch a dropped evidence flag.

Mutation-test each: remove the owner switch in `EvidenceRecord` (tests 1–3 go red), remove the owner
hand-off in `mapTerminalDrive` (the built-in gate test goes red), and strip `--owner finalize` from
each finalize skill file's `evidence.record` call (the guard goes red).

### Not changed

- The evidence record format; the record gains no owner field.
- `finalize.gate: off` behavior; finalize still runs no suite then.
- `evidence.recertify`'s policy, which stays build-owned.
- How `docket schema` publishes `evidence.*` flags (change 0520).
- The wider coordination-tax and evidence items in change 0360.

## Verification

- The tests above, each mutation-tested.
- The full suite through the resolved `build.test_command`.
