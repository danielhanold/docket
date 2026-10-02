<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0490 — Replace the durable worktree admission slot with a supervisor-held kernel lock](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0490-replace-the-durable-worktree-admission-slot-with-a-superviso.md)**
<!-- docket:backlink:end -->
# Replace the durable worktree admission slot with a supervisor-held kernel lock — Results

**Human action:** Yes. After merging, reinstall the `docket` binary only while no gate (test suite) is running, on every machine that runs Docket. The deletion and the cancel check below are optional.

## Outcome

Before this change, "one test suite per worktree" was enforced by a JSON state machine (the "worktree slot") under `.git/docket/gate-admission/`. Whenever an outcome was ambiguous (a lost launch response, a crash between steps, an interrupted release, a stop that couldn't be proven), the slot stayed closed. From then on every later build or finalize gate in that worktree was refused until someone recovered it, and several of those states had no recovery path at all.

Now every gate launch takes a non-blocking exclusive file lock (`flock`) at `.git/docket/worktree-locks/<sha256 of the worktree root>/busy.lock` and hands it to the gate supervisor process. The supervisor holds it for its whole life and releases it last, after its terminal record. If the supervisor dies, the kernel releases the lock, so a dead run frees the worktree with no recovery step.

- A start that finds the lock held is refused `worktree-busy`. It charges no suite attempt and leaves nothing behind. The refusal names the holder only after confirming that the holder's run is still running; otherwise it says "holder unknown".
- The lock key is the worktree root as git reports it, with symlinks resolved. `/tmp` vs `/private/tmp`, a symlinked root, a different-case spelling on a case-insensitive disk, and a subdirectory all map to one lock.
- Finalize's single automatic relaunch takes the lock again. If another gate holds it, the drive halts `worktree-busy` instead of relaunching.
- Deleted: the slot store and its states, `launch-unconfirmed` as an admission refusal, finished-incumbent reconciliation, the first-admission legacy inventory, and the `gate.history.cleanup` command.
- `run.cancel`, the death guardian, the success closeout, and resume now find a run's suites by the run context recorded on each drive, not through the slot. A suite counts as torn down once its supervisor is gone.
- ADR-0132 supersedes ADR-0118. ADR-0120, ADR-0124 and ADR-0125 have update notes.

Departures from the spec, all deliberate:
- The relaunch retries the lock briefly (bounded, about 2s, never blocking). Its own just-exited supervisor may still be releasing the lock.
- `ErrUnresolvedLaunchTransition` is kept. Two launch paths that have nothing to do with the slot still raise it.
- A drive start whose working directory is not inside a git worktree is now refused with the typed reason `worktree-unresolved`, not reported as an internal error.
- Finalize keeps a halted gate's run directory when that gate's supervisor has not been proven to have exited.
- `process.Service.ClassifyRun` is deleted, since its only caller was the legacy inventory.

## Human actions and testing

### Important — reinstall only while no gate is running

A supervisor started by the old binary holds no worktree lock, so the new binary would not see it as busy and could start a second suite in the same worktree. This applies on every machine that runs Docket gates.

1. Make sure no build or finalize gate is running: no `docket` gate supervisor in `ps -ax | grep 'docket'`, and no `implement-next` or `finalize` run in progress.
   Expected: no supervisor processes listed.
2. Reinstall per AGENTS.md ("Rebuild the binary after a merge to main").
   Expected: `docket version` reports the merged `main` commit.

### Optional — delete the old slot directory

Nothing reads `.git/docket/gate-admission/` any more. Once the new binary is installed you can remove it with `rm -rf "$(git rev-parse --git-common-dir)/docket/gate-admission"`. This step cannot be undone, but it only matters if you roll back to an older binary.

### Optional — re-run a cancel that was stuck pending

If any `run.cancel` was stuck at `cancellation-pending` because of a slot finding, re-run the same command (`docket run cancel --key <key> --run-id <id> --reason <why>`) once the new binary is installed.
Expected: disposition `cancelled` (or `already-cancelled`).

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed through the build gate on the pre-review head 18db6d09d. The final head is certified by the evidence block in the PR.
- Each task worker ran focused unit, integration and race tests and mutation-tested its new guards. Examples: dropping close-on-exec on the lock descriptor, dropping the lock hand-over at a launch site, skipping the run-context comparison in the cancel census, and treating a probe error as absence each turn the matching test red.
- New tests cover the spec's L1–L10 list. For example, a supervisor killed with SIGKILL frees the worktree; path aliases and a case variant map to one lock (the case-variant subtest ran on this case-insensitive APFS volume); and cancel ignores another run's drive in the same worktree.
- A deep whole-branch review found 1 important and 7 minor issues. Fixes:
  - F1 (cancel could report `cancelled` over a live first launch whose launch response was lost): fixed in c5a85a67e.
  - F2, F4, F5, F7, F8: fixed in ab6bead26.
  - F6 (glossary entry): fixed in c8d0c9853.
  - F3: not changed; recorded below.
- Runtime budgets were re-measured after the deletions. The `gatelifecycle` shard was over budget (22s against 10) and was sped up to 2–4s by reaping test supervisors, not by raising the ceiling. The build gate printed two `PARALLEL-SENSITIVE` screening lines (`test_go_finalize_e2e.sh`, `test_go_integration_app_merge.sh`). Both were already there and neither is a confirmed breach.
- Real-process tests ran only on macOS. Linux behaviour rests on the documented `flock(2)` semantics.

## Known issues and follow-ups

### A suite can outlive its supervisor (change 0492)

This is confirmed and is the same as today's real guarantee. If a supervisor is killed by itself, or a stop escalates to KILL, test processes in their own process groups can keep running after the worktree is marked free. On a graceful stop the worktree frees while the test runner spends up to about 5s stopping its targets. Change 0492 tracks this.

A related suspected gap, found during the review fixes: a relaunch that halted without attaching (cause `launch-unconfirmed` or `relaunch-failed`) is cleared by cancel based on the first run's directory only. A replacement supervisor that came up anyway would not be stopped by cancel. This belongs with 0492.

### An orphaned reserved drive blocks a run's success closeout (review F3, recorded)

This is confirmed. If a CLI is killed between admitting a drive and launching it, the run's drive stays reserved. `run.verdict`'s success closeout then reports `completion-unaccounted` (finding `launch-pending:<drive>`) every time it runs. The workaround is `run.cancel` on that run, which settles the never-launched drive but marks the run cancelled, not complete. Naming `run.cancel` in the verdict output would change the strict report-line contract, so this is left for 0491's run-tracker redesign.

### Behaviour losses accepted by the spec

- Between two gates of a live run, another start (finalize, a raw `gate.launch`) is no longer refused `stale-run-id`. The run's next gate is refused `worktree-busy` instead.
- A metadata write in a worktree whose owning run record is unreadable is no longer refused through the slot.
- Cancel does not attribute or stop a drive whose record cannot be read. The worktree lock still keeps a second suite out while it runs.
- Resume no longer reads a torn or corrupt replacement chain, so it returns the existing reserved key where it used to refuse `replacement-run-record-unreadable`.
- A zero-budget drive slice now makes two process observations (the slice and the holder-note check), not one.
