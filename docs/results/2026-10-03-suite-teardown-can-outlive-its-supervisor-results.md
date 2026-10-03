<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0492 — Suite teardown can outlive its supervisor](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-03-0492-suite-teardown-can-outlive-its-supervisor.md)**
<!-- docket:backlink:end -->
# Suite teardown can outlive its supervisor — Results

**Human action:** No action is required. The change only reports; it never signals or blocks. The optional walkthrough below shows the new finding on a real gate.

## Outcome

Before this change, when a gate's supervisor was killed on its own (SIGKILL or a crash), docket counted the suite as gone. `run.cancel` reported `cancelled` even though `go run` and the test runner were still running in the supervisor's process group.

Now docket checks the dead supervisor's process group and reports what it finds, without changing any outcome:

- `process.Service.ProbeLeftover(runDir)` is a new read-only check. It returns one of three answers. **none** means the group is empty. **leftover** means the group still has members and its leader (the supervisor's pid) is gone. **unclear** covers everything else: a zombie supervisor, a reused pid, a held or unprovable `live.lock`, a `pgid` that is ≤ 1 or differs from `supervisor_pid`, or a probe error. It never signals and never writes.
- The launch census (`supervisorGone` in `internal/gatedrive`) runs the check for every exited supervisor. On **leftover** it reports `tree-survives:<drive>:<pgid>` and still counts the drive as torn down. That finding outranks `replacement-stopped` and `run-terminal`. On **none** or **unclear** it reports `run-terminal`, as before.
- `run.cancel` and the guardian still report `cancelled`, and the success closeout still reports `run-complete`. The finding appears in their `findings`.
- The glossary has a `tree-survives` entry that tells a human how to confirm (`pgrep -lg <pgid>`) and stop (`kill -TERM -<pgid>`) a leftover. `docs/concepts/run-tracker.md` no longer says these gaps are tracked by 0492.

Gaps 2 (a KILL escalation leaving targets in their own groups) and 4 (the worktree freeing milliseconds before teardown ends) remain accepted losses. Gap 3 (finalize's relaunch) is left to change 0493.

## Human actions and testing

### Optional — see `tree-survives` on a real gate

Prerequisites: a docket checkout with this branch's binary installed, and a test command that runs for at least a minute.

1. Start a tracked run with `run.start implement-next`, then start a gate with `gate.drive.start --owner build ...`.
2. Find the supervisor pid in the run dir's manifest (`supervisor_pid`) and run `kill -KILL <pid>`. Kill only that pid, not the group.
3. Run `run.cancel --key <key> --reason test`.
   Expected: disposition `cancelled`, and `findings` contains `tree-survives:<drive>:<pgid>`.
4. Cleanup: confirm the group with `pgrep -lg <pgid>`, then stop it with `kill -TERM -<pgid>`.

## Verification performed

- Task 1: `ProbeLeftover` tests in `internal/process`, using real processes. A killed and reaped supervisor gives leftover, and then none once its child exits. A zombie supervisor gives unclear. Seam tests cover every unclear branch. Four mutations each turned the expected tests red, including dropping the leader probe.
- Task 2: census tests in `internal/gatedrive`. They cover the leftover finding, its ranking, and none, unclear and probe-error keeping `run-terminal`. Mutations for the posture, the ranking, unclear and the error path each turned red.
- Task 3: app-layer integration tests. A real-process `run.cancel` stays `cancelled`, reports the finding, and never signals the group. The closeout verdict stays `run-done run-complete`. Both posture mutations turned red.
- Task 4: docs and comments only. `go generate ./internal/assets` changed nothing, and `TestCommentAnchorStyle` passed.
- The full suite gate result is recorded in the PR body's build-evidence block.

## Known issues and follow-ups

- **Plan mutation typo (confirmed, harmless).** The plan's census "error-ignored" mutation, which deletes `lerr == nil && `, does not compile. The worker used a variant that compiles, and that variant turned red. The merged plan is a frozen record, so nothing needs to change.
- **Targets of a killed runner stay invisible (accepted).** If the runner itself is killed, its targets live in their own process groups and the check cannot see them. This is documented in the spec.
