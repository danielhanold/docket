<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0488 — Run task-worker tests directly in the foreground, not through gate drives](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-02-0488-run-task-worker-tests-directly-in-the-foreground-not-through.md)**
<!-- docket:backlink:end -->
# Run task-worker tests directly in the foreground, not through gate drives — Results

**Human action:** One important check after merge: reinstall the `docket` binary, then confirm the next real implement-next run's build workers make no `gate.drive.*` calls and wrap each test in `timeout --kill-after=10s 10m`. macOS machines also need GNU coreutils installed (`brew install coreutils`).

## Outcome

Build-task workers used to run every focused test (baseline, RED, GREEN, re-runs, mutation probes) through a task-owned gate drive inside a recovery scope. That protocol could refuse a start about 25 ways, several of them permanent, and was behind roughly two-thirds of recent run halts. It is now gone from the workflow:

- **Workers run tests directly.** The worker skill (`docket-build-task`) tells workers to run each test in the foreground in the feature worktree as `timeout --kill-after=10s 10m <command>` (`gtimeout` as the fallback; neither present means the worker returns `BLOCKED`). The verdict comes from the exit status: 0 is green, 124 or 137 means the 10-minute limit was hit (not red), 125–127 means the command could not run (not red), anything else is red. Workers never run the full suite and call no gate operation. Their outcomes are `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`.
- **The controller runs every full-suite attempt.** The build controller (`docket-build`) no longer prepares scopes or handles worker handoffs and takeovers. After a repair worker's fix, the controller starts the next counted suite attempt itself.
- **A non-blocking audit.** The controller checks each worker's reported test commands for the `timeout` wrapper and notes any omission as an informational line here; it never halts on one.
- **Run id.** Worker prompts no longer carry a run id. Implement-next still passes it on its own full-suite starts, because `run.cancel` depends on it until change 0491.
- **Docs.** The run-tracker rule, glossary, install prerequisites (GNU coreutils), gate references, and user guide match the new contract. The managed `AGENTS.md` block was regenerated on the branch (a guard requires it to match the generator byte for byte), and the embedded skill mirror was regenerated.
- **Guards.** A new absence guard fails if any maintained workflow markdown instructs a task-owned drive operation, or pairs `WAITING` with the worker outcomes. New sentinels pin the 10-minute `timeout` rule and the audit's never-a-halt clause. Guards for retired behavior were removed; size ceilings were lowered to the new file sizes.
- **Decision record.** ADR-0130 supersedes ADR-0117 and narrows ADR-0107 to the boundary between the coordinator and implement-next.

One departure from the spec, from review: full-suite build starts now always pass `--change-id` (and `--run-context` when present). Without it, the driver did not count the attempt against `build.max_attempts`, and the run tracker could not find the drive to continue it. The worker drives being retired here were the last ones that carried both flags, so this gap would otherwise have widened.

The task-drive Go machinery is unchanged and unused by any skill; change 0489 removes it.

## Human actions and testing

### Important — confirm the new worker contract in a real run

The guards prove the skill text says the right thing; nothing automated proves a live worker follows it. Do this after the PR merges and the installed binary is rebuilt (see the repo's "Rebuild the binary after a merge to main" rule). Skipping it leaves open whether workers on your harness actually wrap tests in `timeout` and avoid the gate driver.

Prerequisites: GNU coreutils installed (`timeout --version` or `gtimeout --version` prints a version); a build-ready change in the backlog.

1. Run `docket version` and check its commit equals the merged `main` HEAD.
   Expected: the full commit id matches.
2. Dispatch the next implement-next run as usual.
   Expected: the run reaches its PR or a normal disposition.
3. Read the build workers' transcripts or returns.
   Expected: each `VERIFICATION` line shows test commands prefixed with `timeout --kill-after=10s 10m` (or `gtimeout`), and no worker runs `docket gate drive …`.
4. Read that run's results file.
   Expected: any worker test that ran without the wrapper appears as an informational line under Verification performed; the build was not halted for it.

## Verification performed

- **Full suite (build gate):** `go run ./cmd/docket development test` passed through the build-owned gate driver at e594921a, before the review fixes. A second full run over the final head is recorded in the PR body's build-evidence block. The budget report showed three screening-only `PARALLEL-SENSITIVE` lines for shell tests this branch does not touch (`test_go_finalize_e2e.sh`, `test_go_integration_app_merge.sh`, `test_go_race.sh`) and no serial-confirmed breach.
- **Focused checks by the workers:** each ran `go test` on `./internal/repoguard` and `./internal/assets` (plus `./internal/harness/...` and `./internal/install/...` for the core task) directly under `timeout --kill-after=10s 10m`. Every worker test command carried the wrapper; the time-limit audit found no omissions.
- **Mutation checks:** each new guard was reddened by re-inserting one forbidden token of each class (8 classes), by breaking its paragraph extractor (non-vacuity check), by weakening the audit and exit-status clauses, by stripping `--run-id` and `--change-id` from build starts, and by lowering each size ceiling by one. All restored to green.
- **Review:** a deep-tier whole-branch review returned 6 findings (1 important, 5 minor); all were fixed in-branch (commits 9973dc5e and 909d9bed).

## Known issues and follow-ups

### Nothing proves a worker really used `timeout`

The audit only reads what a worker reports. A worker that forgets the wrapper and misreports its command is not caught. In practice most harnesses cap a single shell call, and `go test` stops a hung run after 10 minutes by default. The case left open is a non-Go repo, on a harness with no call limit, where a worker forgets the wrapper and a test hangs: the worker blocks until a human notices. Accepted residual; no action planned.

### `--change-id` help text is misleading

`docket gate drive start --help` describes `--change-id` as "recorded only". For build-owned starts it decides whether the attempt is counted against `build.max_attempts` and whether the run tracker can find the drive. Confirmed; documentation only. Suggested next action: correct the help text, which fits naturally into change 0489's cleanup of the gate-drive CLI.

### The run tracker still depends on recovery scopes

`run.start` prepares an outer scope and `run.verdict` can take over a drive through it, so the scope operations are not fully dead even though no skill calls them. Change 0489 needs to decide whether the outer scope survives before deleting the machinery.
