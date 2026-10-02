<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0488 — Run task-worker tests directly in the foreground, not through gate drives](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0488-run-task-worker-tests-directly-in-the-foreground-not-through.md)**
<!-- docket:backlink:end -->

# Run task-worker tests directly in the foreground, not through gate drives — design

## Problem

Change 0359 (ADR-0107) put every test a build-task worker runs onto the gate driver: baseline, RED, GREEN, the focused re-run, ad-hoc checks, and mutation probes. Changes 0405 (ADR-0117), 0416, 0459, and 0467 extended it. Each test is a `gate.drive.start --owner task` inside a recovery scope that the controller prepared with `gate.drive.prepare-scope`. The worker:

- carries an eight-value identity bundle;
- presents a predecessor receipt on every later start (the previous drive's id and owner generation, captured from its `--json` reply);
- hands the drive off on its first `WAITING`;
- closes the scope with `gate.drive.acknowledge`.

That protocol is now the main cause of stuck builds:

- **A start can be refused about 25 ways, and several are permanent.** A HALTED drive turns every later start into `predecessor-not-reusable`; one example is `worktree-changed` from an in-place mutation check (change 0486). A failed launch leaves the slot reserved, so every later start is `scope-busy`. The worker returns `BLOCKED`, the build halts, and a human has to run `change.resume-halted`.
- **Small slips break the identity checks.** The scope compares the worktree path as a raw string, and `--repo-dir` defaults to cwd, so a different spelling of the same worktree is refused `scope-identity-mismatch`. A start without the scope flags hits a worktree slot stamped with a run id the worker never receives, and is refused `stale-run-id`.
- **Agents can't see the state they need.** No read-only operation shows a scope's current drive, so agents dig through `<git-common-dir>/docket/` and docket's source to recover a receipt. A recent Cursor build needed about 19 steps for one task's checks.
- **The halt record bears this out.** The metadata branch holds 27 `run halted` reports since 2026-09-09. Read individually, roughly two-thirds trace to gate ownership machinery, not red tests.

For focused tests, the protocol buys almost nothing:

- Nothing reads a task drive. docket-build accepts `COMPLETE` by commit ancestry, and `evidence.record` accepts only a PASSED run of `build.test_command`.
- Nothing enforces it except skill prose.
- The full-suite build gate re-runs everything anyway.
- 0359's four-harness acceptance probes for worker handoff and takeover (in `gate-execution.md`, "PENDING HUMAN RE-PROBE (before merge)") were never run. Every row is still `unverified`.

The incident behind 0359 (change 0333) was a full-package `go test -race ./internal/app` run of about four minutes, treated as a focused test while its caller returned. Putting a time limit on the command addresses that; tracking every test was never needed. The 0405 stub recorded the direct path working: workers "correctly fell back to running their fast, sub-second focused test … directly in the foreground, so no build was harmed."

Separately, `docket-implement-next/references/fix-loop.md` dispatches docket-build-task fix workers without preparing a scope, so under the current contract their tests are either refused or run outside the contract.

## Decision

### 1. Workers run every test directly, in the foreground, under GNU `timeout`

docket-build-task runs each test (baseline, RED, GREEN, focused re-run, ad-hoc check, mutation probe) directly in the feature worktree as:

```
timeout --kill-after=10s 10m <test command>
```

- **Block until it exits.** Never background a test, and never end the turn while one runs. If the harness hands back a still-running session, keep waiting on that same session, and never start a second copy. ADR-0024 is unchanged.
- **Make the harness wait at least as long.** Raise the harness's own shell-call timeout to at least 10 minutes where the harness allows it, so a shorter default doesn't become the real limit. If the harness still cuts the call off, treat that as hitting the limit. Because `timeout` runs the test in its own process group, that run still ends at the 10-minute deadline. Don't start another test until it's gone; stop it yourself if you can see it.
- **Fallback.** Use `gtimeout` when `timeout` isn't on `PATH`. If neither exists, return `BLOCKED` naming the missing prerequisite (GNU coreutils). Never run a test without the limit.
- **Read the result from the exit status, never from output text:**

| Exit status | Meaning | Worker action |
|---|---|---|
| 0 | green | continue |
| 124 or 137 | the 10-minute limit was hit (TERM, or KILL after the 10-second grace) | not red, and not by itself a reason to escalate: narrow the command, or return `BLOCKED` naming it |
| 125, 126, 127 | `timeout` or the command could not run | not red: fix the invocation, or return `BLOCKED` |
| any other non-zero | red | the existing repair discretion |

- **Never run the configured full-suite command.** The full suite is the controller's gate. "Focused" means the narrowest command that exercises the task.
- **Mutation evidence works as before:** mutate, run, restore, run again. With no drive, there's nothing to halt as `worktree-changed`.

This was checked on macOS 27.0 with GNU coreutils 9.12. A wrapped `bash -c 'sleep 60 & …; wait'` exits 124 at the deadline, and its child process is gone afterwards. A command that ignores TERM exits 137 after the grace period.

### 2. The worker contract loses every gate operation

From `skills/docket-build-task/SKILL.md`, remove:

- the drive-scope bundle in the intro;
- the `gate.drive.start --owner task` paragraph, with its JSON-capture and `gate_reply` instructions;
- *The one run-id exception*;
- the `worktree-busy` paragraph;
- *Sequential drives within your scope* (predecessor receipts and `gate.drive.acknowledge`);
- *Continued after a `WAITING` handoff*.

Outcomes become `COMPLETE | NEEDS_ESCALATION | BLOCKED`, and the `HANDOFF:` line leaves the return block. `VERIFICATION` lists every test command exactly as it ran, including its `timeout` wrapper, with its exit status. The controller's audit (Decision 3) reads that line.

The integration-repair worker follows the same contract. It fixes the failure, re-runs the failing tests directly as its focused check, commits, and returns `COMPLETE`. It never runs the full suite.

### 3. The controller runs every full-suite attempt

In `skills/docket-build/SKILL.md`:

- **Dispatching a task.** No `gate.drive.prepare-scope`. The payload is:
  - the `Feature worktree:` line, kept verbatim, because `feature_worktree_dispatch_test.go` requires it inside the `docket:feature-dispatch` markers;
  - the task text and the applicable repository instructions;
  - the tier and routing reason;
  - the return schema.

  No run context, run id, or capability goes into a worker prompt.
- **Reading a worker's return.** Valid outcomes are `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`. Any token outside these three is malformed and halts the build. `COMPLETE` is still checked against git ancestry. The text deliberately doesn't name the retired outcome; see the absence guard under Tests.
- **Time-limit audit, which is visibility only.** For every return, check each test command in `VERIFICATION` for the `timeout --kill-after=10s 10m` wrapper (or `gtimeout`).
  - A command without it, or a `VERIFICATION` line too unclear to tell, becomes one informational line in the results file's `## Verification performed` section, through the existing worker-surfaced-findings path: "task N: `<command>` ran without the 10-minute `timeout` wrapper — informational, no action required; the full-suite gate certified the branch."
  - It is never a halting condition and never makes a return malformed. It never re-dispatches, escalates, or re-runs a test, and never casts doubt on the task's commit. It adds no operation, state, transaction, or commit of its own; it rides on the results checkpoint the coordinator already writes.
  - It catches an honest omission. It cannot catch a worker misreporting what it ran.
- **Delete** *Task-level WAITING and the continuation*, including its *Exceptional branch* (takeover).
- **The build gate, red path.**
  1. Each red result becomes one repair task: premium, then one escalation to max.
  2. When the repair worker returns `COMPLETE`, the controller starts the next counted attempt itself, with the same build-owned `gate.drive.start` (`--run-id` when its prompt carried one), and drives it to a final result.
  3. Green ends the phase. Red becomes the next repair task while attempts remain. Exhaustion halts.

  Attempt counting is unchanged, and `build.max_attempts: 1` still means a red first run halts with no repair. The repair dispatch payload no longer carries the run id.
- **Unchanged:** routing, escalation, the halting conditions (minus the takeover halt, which goes away), the gate run posture, the build gate's start/advance, evidence, and *Abandoning a live drive* (handoff of the build-owned drive).

### 4. The run id leaves the worker path only

Implement-next keeps passing `--run-context` and `--run-id` on its own build-owned starts (Step 5's suite gate, post-repair attempts, Step 6's evidence re-mint and re-gates), and `--run-context` on `change.claim`, exactly as today.

Removing the run id from the build gate here would break cancellation. `run.cancel` matches the run by its run id (`runCancel`'s `expectRunID`), decides slot ownership by the slot's `RunID`, and finds the run's launched suites by run id (`ReconcileRunLaunches`). A suite started without a run id would survive a cancel, and the resume would then be refused `worktree-busy`.

Change 0491 retires the run id entirely once 0490 and 0491 remove that dependency. That is the human's direction from this groom: the run key becomes the run tracker's only handle.

## Unchanged

- **The build-owned full-suite gate:** `gate.drive.start --owner build`, `advance`, handoff and claim at the run boundary, fingerprinting, the observation budget, evidence (`evidence.record` and `evidence.verify`), and the suite-attempt budget.
- **Implement-next's fix loop.** Fix workers already run the docket-build-task contract without a scope, and the loop's full-suite gate is unchanged.
- **Finalize's gate and its `docket-integration-repair` agent,** which already runs tests directly.
- **All Go code.** The task-intent owner, scopes, predecessor receipts, `acknowledge`, and `takeover` stay in the catalog, unused by any workflow, until change 0489 deletes them.
- **The run tracker:** run key, run-context binding, retry accounting, and `run-continue`.
- **ADR-0024's never-yield rule.**

## Prose sites

Derive the full site list from a whole-repo grep (AGENTS.md rule) for the retired vocabulary: `--owner task`, `prepare-scope`, `predecessor`, `acknowledge`, `takeover`, `child-cap` / child capability, scope bundle, `scope-id`, `WAITING` as a worker outcome, and the repair worker's run-id exception. Sort prose from executable. The known sites:

| File | Change |
|---|---|
| `skills/docket-build-task/SKILL.md` | Decisions 1 and 2. Also fix the ambiguous "the controller runs the full suite once after every task" to read "once, after every task has committed". |
| `skills/docket-build/SKILL.md` | Decision 3. "A worker's passed *task* gate" becomes "a worker's focused tests". |
| `skills/docket-implement-next/SKILL.md` | The run-context sentence names only `change.claim` and build-owned `gate.drive.start`. The run-id sentence names only build-owned starts. Drop the sentence "Scoped task-owned starts inherit the run id from their scope, so a build-task worker is handed it only for that repair re-run.", and state that build-task workers receive neither value. |
| `skills/docket-implement-next/references/fix-loop.md` | Confirm the fix-worker wording matches the new contract; no structural change. |
| `skills/docket-build/references/gate-caller-loop.md` | The callers become the build controller, implement-next's re-mint and re-gates, and finalize. Remove the `prepare-scope`, `takeover`, and `acknowledge` rows from both the operations and JSON-capture tables, the scope and successor text in the `start` row, and *Parent takeover*. "A build-task worker returns `BLOCKED`" becomes the caller's own halt. Keep start, advance, handoff, and claim, plus the dispositions, JSON capture, the shell-safe capture names, and worktree admission. |
| `skills/docket-build/references/gate-execution.md` | In *Change 0359 continuation/takeover acceptance*, drop the worker scenarios 2 (worker-to-controller handoff) and 3 (takeover after a worker returns), and reword scenario 1 to the build-owned gate. Keep scenarios 4–7 as the outstanding human verification of the run-boundary continuation. Retitle the section to say so, since 0359 has merged and "before merge" no longer applies. |
| `cursor-rules/run-tracker.md` | Drop `gate drive prepare-scope` from the `--run-id` flag list. The managed AGENTS.md and CLAUDE.md blocks regenerate through docket's install path, never by hand. |
| `docs/reference/glossary.md` | *docket-build*: worker outcomes lose `WAITING`. *Start / run key / run id / run context*: drop the scope inheritance and the repair-worker exception. *Gate drive*: used by the build and finalize suite gates; build-task workers never start a drive. Drop any claim that a workflow uses `prepare-scope`, `takeover`, or `acknowledge` for workers. |
| `docs/install/install.md` | Add GNU coreutils `timeout` to *What you need first*: standard on Linux; on macOS, `brew install coreutils` (it may install as `gtimeout`). |
| `internal/assets/embedded/tree/…` | Regenerate with `go generate ./internal/assets`. `TestEmbeddedMatchesAuthored` enforces the match. |

## Tests

Move each guard's property to where it now lives. Never restore deleted prose to keep a grep green (learnings: restatement-accumulates-its-own-guards and test-premise-deleted-not-regated).

- **New absence guard.** Convert `internal/repoguard/gatedrive_scope_identity_test.go`, reusing its corpus scanner and paragraph splitter.
  - It scans the whole maintained workflow-markdown corpus (`isWorkflowMD`, which includes the embedded mirrors).
  - It fails if any paragraph instructs `--owner task`, `gate.drive.prepare-scope`, `gate.drive.acknowledge`, `gate.drive.takeover`, `--predecessor-drive-id` / `--predecessor-owner-gen`, or `--scope-id` / `--child-cap`.
  - It also fails if any paragraph pairs `WAITING` with the worker outcomes `COMPLETE`, `NEEDS_ESCALATION`, or `BLOCKED`. That reuses `gatedriver_test.go` detector D's existing `waitFwd`/`waitRev` shape, inverted from "must name a handoff" to "must not occur".
  - The forbidden token set is the asserted property. Sites are found by scanning the whole corpus, never from a per-file list.
  - A non-vacuity companion, using the same extractor, must still find the build-owned `gate.drive.start` site and docket-build-task's outcome list.
  - Mutation-test it: re-insert one token of each class and watch it turn red; break the extractor and watch the companion turn red.
- **Time-limit pin.** `prose_contracts_test.go` sentinels require two things:
  - `skills/docket-build-task/SKILL.md` carries `timeout --kill-after=10s 10m`, the exit-status reading (124 and 137 mean the limit was hit), and the rule that `VERIFICATION` lists each command exactly as run;
  - `skills/docket-build/SKILL.md` carries the time-limit audit and its clause that the audit is never a halting condition and never makes a return malformed.

  Deleting any of these turns the suite red.
- **Retire** (their subject is gone):
  - `gatedrive_scope_identity_test.go` prongs A–C, absorbed into the new guard;
  - `gatedrive_run_id_thread_test.go` prongs A (prepare-scope sites), C (task-owned starts carry no run id), and D (the repair re-run floor);
  - `gatedriver_test.go` detector D (a task-level `WAITING` must name a handoff), whose shape moves into the absence guard, inverted;
  - the `change_0459_scope_transferred` sentinels in `prose_contracts_test.go`.
- **Keep and narrow:**
  - `testexec_boundary_test.go` still forbids the full suite outside `gate drive`. Narrow its header rationale from "every test-intent command" to the full-suite channel.
  - `gatedrive_json_capture_test.go` and `gatecapture_reserved_param_test.go` keep their contract prongs. Re-check any population floor that counted worker sites.
  - `gatedrive_run_id_thread_test.go` keeps prong B and `TestRunTrackerCopiesRunIDIntoDispatchPrompt`.
  - `budgets_test.go`: rewrite the comments, and lower the size ceilings to the new file sizes.
- Run the whole suite through `build.test_command` at the build gate.

## Prose and decision record

- **New ADR, recorded through docket-adr:** *Build-task workers run focused tests directly under a fixed time limit; the gate driver serves only full-suite gates.*
  - `supersedes: [117]` and `relates_to: [107, 24]`. ADR-0107's recovery scopes and parent takeover stop applying between the build controller and its workers. They still apply between the coordinator and implement-next. Precedent: ADR-0126 narrowed ADR-0101 and ADR-0106 through `relates_to`.
  - **Rejected alternatives:**
    - keeping the driver for task tests without scopes, which stays exposed to the worktree slot, the run fence, and the 30-second slice collision from change 0412;
    - 0359's earlier rule of using the driver only when a test looks long, which is a prediction and is replaced by a fixed limit;
    - having the repair worker drive or hand off the full suite, which keeps the driver and `WAITING` at the worker level;
    - a configurable limit, which nothing enforces;
    - a docket-owned bounded runner, which puts docket back in the test path while GNU `timeout` is enough;
    - hard enforcement, where the controller halts a return whose command lacks the wrapper. A forgotten prefix on a passing test would stop the build and need a human, which is a new stuck mode;
    - harness hooks, which work on one harness only and can't tell a test from any other command.
  - **Enforcement chosen:** a static pin keeps the rule in the worker contract, and the controller's audit reports omissions in the results file without halting.
  - **Accepted losses:**
    - nothing can take over a focused test that is still running after its worker died;
    - there is no durable record of worker test runs, and nothing ever read one;
    - GNU coreutils becomes a macOS prerequisite.
- **Results file, *Human actions and testing* (Important):** after merge and the binary reinstall, check that the next real implement-next run shows workers making no `gate.drive.*` calls and wrapping each test in `timeout --kill-after=10s 10m`.
- **Accepted residual, recorded in the results file:** nothing mechanically proves a worker used `timeout`. The controller's audit reports honest omissions, but a worker that misreports its command isn't caught. Even then, most harnesses cap a single shell call, and `go test` ends a hung run after 10 minutes by default. The case left open is a non-Go repo, on a harness with no call limit, with a forgotten wrapper and a hung test: the worker blocks until a human notices.

## Out of scope

- Deleting the Go machinery for task-owned drives (change 0489).
- Replacing the worktree admission slot (change 0490).
- Retiring the run id entirely, per the human's direction at this groom, along with its gate fence (change 0491).
- Changing the build-owned full-suite gate, evidence, finalize's gate, or the run tracker's attribution and retry model.
- The controller-side background-and-yield cases in change 0412.
