<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0489 — Delete the task-owned gate-drive machinery; the outer takeover recovers only live drives](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0489-delete-the-task-owned-gate-drive-machinery-once-no-skill-dri.md)**
<!-- docket:backlink:end -->
# Delete the task-owned gate-drive machinery; the outer takeover recovers only live drives — Results

**Human action:** Nothing is needed beyond the usual reinstall of the `docket` binary after merge. One optional check is listed below: confirm that an implement-next run which dies after its gate finished gets a retry instead of a terminal stop.

## Outcome

Change 0488 moved build-task workers off the gate driver, which left the code behind task-owned test drives with no callers. This change deletes that code, about 10,000 lines (production code and tests). What went:

- the three operations `gate.drive.prepare-scope`, `gate.drive.acknowledge` and `gate.drive.takeover`;
- `gate.drive.start`'s `--owner task` and its four scope and predecessor flags;
- the task-intent owner, scoped starts, and slot rotation;
- the per-test scope lifecycle and terminal acknowledgement;
- the cancel census's scope walk;
- the takeover's run-revocation check, which nothing could reach.

`gate.drive.start` now accepts only `--owner build|finalize`, with no `--` argv.

The run tracker still keeps one small recovery scope per run, which `run.start`, `run.verdict` and `run.cancel` use. Its records keep their on-disk shape, so a run already in flight when you upgrade keeps working.

**Behavior fix.** Before this change, when implement-next died the run tracker treated every finished build gate as something it could take over. So an implement-next that died after a red-then-green gate ended in a terminal `run-stop … takeover-ambiguous`. One that died after a later commit ended in a fingerprint halt. Now only a still-running drive can be taken over:

- a run that dies mid-gate still gets `run-continue`;
- a run that dies after its gate finished gets `run-retry-once` and runs the suite again. This costs one build attempt; that is the accepted loss.

The decision is recorded as ADR-0131.

Old task-scope and task-drive files on disk are never read again; they are not migrated or deleted. Old drive records still decode, and legacy `scoped` admission slots still settle.

## Human actions and testing

### Optional — a dead run after a finished gate retries

This is the one behavior change you can see. Automated tests cover it at unit level and through the real `run.verdict` continuation path (`TestIntegrationRunVerdictFinishedBuildDrivesAreNotTakeoverCandidates`). Wait for it to happen naturally rather than forcing it.

1. The next time an implement-next run stops early after its full-suite gate has finished (during review, or after the results commit), run `docket run verdict <key>` with that run's key.
   Expected: the report line is `run-retry-once …` (or a halt for a human if `build.max_attempts` is already used up). It must not be `run-stop … takeover-ambiguous`.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) through the build gate: green at `985b514` after the nine plan tasks. The final certification run, after the review fixes and this results file, is recorded in the PR's build-evidence block.
- Every plan task and review fix ran its focused tests directly, and each new guard was mutation-tested: it went red with its premise removed and green again after restore. The guards cover live-only candidates, explicit-id takeover, accepting a drive that finished during the takeover, the retired CLI surface, run-context hash storage, old-record tolerance, scope-store write serialization, and the advisory reconcile run id.
- Whole-branch review (deep tier): 4 findings (0 blocker, 2 important, 2 minor), all fixed in-branch:
  - F1: the `test_go_race.sh` budget row went back to 60 (commit `0bdf4c5`);
  - F2: a test now covers the advisory reconcile under the presented run id (commit `6ce1805`);
  - F3 and F4: the old-task-drive comment and fixture now match reality, and the verdict test gained a positive control (commit `8aba6a7`).

## Known issues and follow-ups

### `test_go_race.sh` has little parallel budget headroom

The first gate on this branch printed `BUDGET WATCH: tests/test_go_race.sh — 134s under -j11`, after task 9 had lowered that row from 60 to 45. Review fix F1 put the row back to 60, which makes the parallel screen threshold 150s, 16s above what was measured. Nothing fails today. As the race shard grows, though, a BUDGET WATCH line may come back. Next step: if it does, confirm serially before changing the row (see `tests/README.md`). `test_go_toolchain.sh` was lowered from 55 to 45, based on two serial runs.

### Change 0491's spec should note the takeover revocation check is gone

0491 (retire the run id) lists "the revocation checks at launch, recovery, and takeover". This change already deleted the takeover check, because no outer scope ever carried a run id. When 0491 is groomed, its spec should drop that item. The launch and recovery checks are untouched.
