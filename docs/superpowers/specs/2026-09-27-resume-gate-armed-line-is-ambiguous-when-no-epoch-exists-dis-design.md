<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0463 — Resume gate-armed line is ambiguous when no epoch exists — dispatch context gets passed as --run-epoch](../../changes/archive/2026-09-28-0463-resume-gate-armed-line-is-ambiguous-when-no-epoch-exists-dis.md)**
<!-- docket:backlink:end -->

# Resume arm always binds a run epoch — design

**Change:** 0463 · **Date:** 2026-09-27 · **Status:** groomed

## Problem

`run.gate-before implement-next --resume <id>` has an epochless path. When `FindEpochByChange` finds no prior epoch for the change (because the earlier run was unarmed, pre-epoch, or never bound one), `RunGateBefore` arms a gate and an outer recovery scope but mints no epoch. `RunGateBeforeResult.HumanText` then omits the epoch slot and prints `gate-armed <key> <dispatch-context>`.

The documented parent procedure (AGENTS.md and `cursor-rules/run-gate.md` with its embedded copy) reads the line positionally as `gate-armed <key> <epoch> <dispatch-context>`. The epoch and the dispatch context are both 32-hex tokens, so on this path the parent takes the dispatch context (a child capability) to be the epoch. In change 0382's resumed run, it was passed as `--run-epoch` and the dispatch context was never passed as `--gate-context`. `epochLaunchGate` rejected the unknown epoch with a typed `EpochError` (`epoch-not-found`). `mapDriveFailure` has no case for `EpochError`, so the refusal collapsed to the catch-all `invalid-request`. The child dropped the flag, and the whole run completed with no epoch linkage: no `run.cancel` fence, no worktree-ownership protection, and no warning.

## Decisions

1. **Every armed gate carries an epoch.** A resume with no prior epoch mints one, rather than staying epochless with a relabelled output line. This makes the three-token positional contract true on every armed path and restores cancel/fence coverage for the resumed run.
2. **The positional contract stays.** `run gate-before --json` already reports `epoch` and `dispatch_context` as separate named fields. Once decision 1 holds, the positional line is unambiguous, so retiring it would churn a cross-harness contract for no gain (YAGNI).
3. **An unknown epoch gets its own refusal token (`unknown-run-epoch`).** There is no credential-hash detection of a dispatch context passed as an epoch. Decision 1 removes the parse ambiguity that produced the misuse, and scanning gate records for credential hashes on every refusal adds credential-handling code for a now-unreachable mistake.
4. **The concurrent epochless-resume race is accepted as fail-safe.** Epoch locks are per gate key, not per change, so two simultaneous epochless resumes of one change can each mint an active epoch. `FindEpochByChange` then returns `ErrEpochAmbiguous`, and later resumes refuse with `resume-epoch-unreadable`. Nothing is overwritten, and it is strictly better than today's equivalent race, which yields two epochless runs. No per-change lock is added.

## Design

### 1. Mint an epoch on the epochless resume branch (`internal/app/rungate_before.go`)

In `RunGateBefore`, step (6a) mints only when `resumeID == 0`. Change the condition so it also mints when this is a resume whose `FindEpochByChange` returned `found == false`. The replacement path (`armResumeReplacement`) already mints its own epoch and returns early, so it is unaffected.

For the epochless resume mint:

- **`ChangeID` = the resumed change id** (`scopeChangeID`), as a fresh arm does. A later resume of the same change then finds this epoch `EpochActive` and refuses with `resume-active-run` and the cancel/continue locator. That is the single-live-run protection the stub asked about.
- **Bind `Worktree` = the verified feature worktree** (`insp.Path`) immediately after the mint, via `epochCAS`, exactly as `armResumeReplacement` does. This is required, not optional. `epochLaunchGate` refuses an active epoch whose `Worktree` is empty or different (`ErrStaleRunEpoch`). A fresh arm's epoch acquires its worktree later at claim time, but a resume has already claimed, so an unbound resume epoch would be refused on first use.
- **A mint or bind failure returns `gateUnarmed(ReasonGateMintFailed)`.** This is the existing fail-closed channel. The orphan gate record is inert because no key is returned.

### 2. Armed ⇒ exactly three tokens

- After decision 1, no armed return path produces an empty `Epoch`. `HumanText` drops its `if r.Epoch != ""` conditional and always renders `gate-armed <key> <epoch> <dispatch-context>`, followed by the owner-lifecycle line when present.
- Fix the stale text that describes the epochless line:
  - the `Epoch` field comment ("Empty only on a legacy resume arm…")
  - the `HumanText` doc comment ("the epoch is omitted only on a legacy resume arm…")
  - the step (4a) branch comment ("falls through to the existing resume arm, which shares no epoch…")
  - the step (6a) comment ("A resume arm does NOT mint here…")
  - the `gate-before` cobra `Short` in `internal/cli/run.go`, which still reads `gate-armed <key> <dispatch-context>`
- The parent-facing prose (AGENTS.md run-gate rule, `cursor-rules/run-gate.md`, `internal/assets/embedded/tree/cursor-rules/run-gate.md`) already documents the three-token form and needs no edit. The build must grep every `gate-armed` prose site repo-wide to confirm that none documents an optional or omitted epoch. Point-in-time records (archived changes, results, specs) are excluded.
- The JSON result shape is unchanged.

### 3. A named refusal for an unknown or unresolvable run epoch

- **`mapDriveFailure` (`internal/app/gate_drive.go`)** gains an `AsEpochError` case, placed before the catch-all:
  - `ErrEpochNotFound` → `ResultInvalidInput`, reason **`unknown-run-epoch`** (new stable token).
  - Other epoch kinds that indicate a readable-but-unusable registry, such as `ErrEpochAmbiguous`, → `ResultInvalidInput` with the kind as the reason.
  - IO or corrupt kinds → `ResultInternalError` with the kind as the reason, mirroring the existing `gatedrive.AsStoreError` split.
  - The reason is always a fixed vocabulary token, never argv, a path, or record content.
- **Next-action message.** Where `mapDriveFailure`'s callers attach a next action (the `ownershipNextAction` / `fenceNextAction` pattern), `unknown-run-epoch` gets a one-line, credential-free message along the lines of: *"the --run-epoch value names no run epoch in this repository; pass the second token of the gate-armed line (the dispatch context is the third and goes to --gate-context)."* The message never echoes the presented value.
- **Consumers covered:** `gate drive start` and `gate drive prepare-scope` both route through `mapDriveFailure`. The plan must also trace `agent.enter --run-epoch`, whose participant registration validates the epoch against the gate key's record (`RegisterEpochParticipant`). If an unknown or mismatched epoch there currently surfaces as a generic or unclassified refusal, it must surface a named token instead: `unknown-run-epoch` for a not-found epoch, and the existing stale-linkage kind for a mismatch.
- **Register the token wherever reason tokens are declared or enumerated.** For example, if a closed vocabulary or schema lists gate-drive reasons, add `unknown-run-epoch` there so the catalog stays truthful.
- **Existing tokens are unchanged:** `stale-run-epoch` (the epoch exists but does not own this worktree, or was superseded), `run-cancelled`, `run-completed`.

## Testing

Every test is mutation-checked per repo rules: strip the corresponding fix and watch the test go red.

1. **Epochless resume arms with an epoch.** Seed an in-progress change with a verified worktree and no epoch record. Then `RunGateBefore(..., resumeID)` → `Armed`, non-empty `Epoch`, and the epoch record exists with `ChangeID == id`, `State == EpochActive`, and `Worktree ==` the verified worktree. Mutation: restore the `resumeID == 0` condition, and separately drop the worktree bind.
2. **Human-line shape invariant.** For every armed result the tests can produce (fresh arm, epochless resume, cancelled-replacement resume), `HumanText`'s first line has exactly four space-separated fields, field 3 equals `Epoch`, and field 4 equals `DispatchContext`. Mutation: reintroduce the conditional epoch omission.
3. **Repeat epochless resume is refused as active.** A second `--resume` of the same change after test 1 → `gate-unarmed resume-active-run` with the locator. Mutation: leave `ChangeID` unset on the resume mint.
4. **Unknown epoch refusal token.** `gate drive start` and `gate drive prepare-scope` with a well-formed but unknown `--run-epoch` → `invalid-input` / `unknown-run-epoch`, never `invalid-request`. The output must not contain the presented value. Mutation: remove the `AsEpochError` case.
5. **agent.enter unknown epoch.** An unknown `--run-epoch` yields a named token, per the plan's trace.
6. **End-to-end 0382 repro.** Claim a change with no armed gate (so no epoch exists), then run `gate-before --resume`, parse the printed line positionally, and call `gate drive start --run-epoch <field 3> --gate-context <field 4>` for the resumed change → admitted, and the drive's worktree slot carries that epoch id.
7. **Race stays fail-safe (documenting test).** Two epochless-resume epochs minted for one change → a subsequent `--resume` returns `gate-unarmed resume-epoch-unreadable`, not an arm.

## Out of scope

- Enforcing arm-before-dispatch for slash-command or agent launches (change 0345).
- Minting epochs retroactively for completed runs, or repairing gate records from earlier runs.
- Retiring the positional `gate-armed` line in favour of `--json`.
- Detecting a dispatch-context or credential hash presented as `--run-epoch`.
- A per-change lock serializing concurrent epochless resumes.
- Redesigning the gate-drive refusal vocabulary beyond the epoch cases above.
- The child's call-site spelling in 0382's resumed run (omitted `--change-id`/`--phase`/`--gate-context`).
