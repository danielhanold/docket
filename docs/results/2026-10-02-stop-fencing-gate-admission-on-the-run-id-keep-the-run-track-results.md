<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0491 — Retire the run id; the run key becomes the run tracker's only handle](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0491-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track.md)**
<!-- docket:backlink:end -->
# Retire the run id; the run key becomes the run tracker's only handle — Results

**Human action:** Yes. After merging, install the new binary only while no docket run is in flight, then restart every agent session. The run-tracker instructions changed, and a session still holding the old instructions will pass `--run-id`, which now fails as an unknown flag.

## Outcome

The run tracker used to hand out three tokens per run: a run key, a run id, and a run context. The run id added nothing the key didn't already give, but gate starts were refused on it. Those refusals all said "superseded by a resume", even when the real cause was something else, such as an unbound worktree or a differently spelled `--repo-dir`. This change retires the run id:

- `run.start` prints `run-started <key> <run-context>`. Its JSON result no longer has `run_id`, and it declares the `process-control` effect.
- `run.cancel` takes `--key <key> --reason <why>`.
- `gate.drive.start` and `agent.enter` no longer accept `--run-id`. `agent.enter --run-key` alone now drives the dormant Codex lifecycle linkage, and nothing documents passing it.
- The run check on gate starts is gone. A gate start is now refused only by the worktree lock (`worktree-busy`) and the drive protocol's own checks.
- Renames: `stale-run-id` is now `run-superseded`. The non-cancel `run-id-mismatch` is now `run-record-conflict`. `unknown-run-id` is retired, and an unknown key reports `run-not-found`.

Two stuck states are fixed:

- **Never-launched drive.** A `gate.drive.start` killed between writing its drive record and launching the suite used to wedge the run. A successful run stayed stuck forever at `completion-unaccounted`, and an incomplete run got `continuation-unverified` instead of a retry. The keyed `run.verdict` now closes such a drive as HALTED `launch-abandoned` and stops nothing. A successful run then completes, and an incomplete one earns its `run-retry-once`.
- **Missing run root.** A drive whose run root directory is missing now counts as never launched in cancel and in the verdict, instead of `resolution-unresolved` forever.

Two departures from the plan, both from review:

- `run.start` now refuses an empty run-scope capability as `run-untracked scope-failed` before it mints anything. Before, the problem was caught only after the run record had been written.
- Comments that called the closeout "observation-only" were reworded, because the closeout now writes the `launch-abandoned` settle.

## Human actions and testing

### Important — install after merge, then restart sessions

This matters because every running agent session carries the run-tracker block it loaded at start. A session that still holds the old block copies a run id and passes `--run-id`, and the new binary rejects that flag. If you skip the restart, the next dispatched implement-next run fails at its first gate start or cancel.

1. Confirm no docket run is in flight. No implement-next or finalize agent should be running.
2. Follow the AGENTS.md rebuild rule: sync main, confirm the merge landed, run the `development.install` operation with `--source /Users/homer/dev/docket`, and check that `docket version` reports the merged HEAD.
3. Restart every Claude/Cursor/Codex session.
   Expected: the new session's CLAUDE.md block shows `run-started <key> <run-context>` and `--key <key> --reason <why>`, with no `<run-id>`.

### Optional — a run left `completing` by a never-launched drive completes

Use this if a run is already stuck at `run-stop <key> run-tracker-unavailable completion-unaccounted` from before the upgrade.

1. Run the `run.verdict` operation with that `<key>` on the new binary.
   Expected: `run-done <key> run-complete`. The leftover drive record is HALTED with cause `launch-abandoned`.

### Optional — a cancel stuck `cancellation-pending` on a missing run root finishes

1. Re-run the `run.cancel` operation with `--key <key> --reason <why>`.
   Expected: disposition `cancelled`.

## Verification performed

- Full suite via `build.test_command` (`go run ./cmd/docket development test`) through the gate driver. The pre-review head 1db1e214c was green. After the review fixes the head moved, and it is re-certified by the final gate recorded in the PR body.
- Each task ran its own tests and its mutation probes:
  - the T1 regression, which returns red when the verdict arm is put back to `launch-pending`;
  - the missing-root rule, and the rule that any other probe error stays `resolution-unresolved`;
  - the T2 settle-before-scan ordering;
  - the key-only linkage and preflight;
  - every new retired-vocabulary row, plus a negative control showing the rows do not match the gate supervisor's `run_id`.
- Whole-branch deep review: 3 minor findings, 0 blockers.
  - F1 (stale "observation-only" comments) was fixed in b80016065.
  - F2 (the capability check came after the mint) was fixed in e139a0dfb.
  - F3 (an accepted loss the spec did not list) was recorded in the new ADR's Consequences.
- The gate printed three `PARALLEL-SENSITIVE` screening lines: `test_go_finalize_e2e.sh`, `test_go_integration_app_merge.sh`, and `test_go_race.sh`. None of those shards is a file this change touched, and there was no `SERIAL CONFIRMED OVER BUDGET` line.
- The `runcompletion` and `runverdict` integration shards ran at 13 s and 17.6 s against ceilings of 15 s and 20 s, so there is little headroom left.

## Known issues and follow-ups

### A live child's gate start can be abandoned by a concurrent keyed verdict

This happens if a keyed `run.verdict` runs while the dispatched child is still alive and sitting in the milliseconds between admitting a drive and launching it. The verdict settles that drive `launch-abandoned`, the charged build attempt is not refunded, and the child's launch refuses. The run then gets a normal retry. This is confirmed by reading the code and has not been observed. The window is tiny, and coordinators only run the verdict after the child returns. It is accepted and recorded in the new ADR. No action is suggested.

### `startedRunResult` keeps an unreachable fail-closed check

Two existing tests assert the formatter's empty-input check directly, so the check was kept. Its comment says it can no longer be reached. Impact: none. A later cleanup could drop the check together with those two assertions.
