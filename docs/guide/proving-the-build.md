# Test gate: Proving the build

By the end of this page you will know how a finished branch earns the right to be reviewed and
merged: the one test run that certifies it, the durable record that run leaves behind, how the run is
driven when the suite takes longer than a single command call, how a rebased branch is made green
again before it merges, and where you tell docket which command counts as "the tests."

## The build gate

After every task on a plan (the task-by-task breakdown a build follows, written on the feature
branch) has committed, the build runs the whole test suite exactly once. This single run is the
**build gate** (the full test-suite run at the end of a build that must be green before review), and
it answers the build's own question — "does everything I assembled actually work together?" — before
anyone spends effort reading the diff.

The outcome forks two ways:

- **Green** hands off to review. The gate mints a record of the run (the next section) and the
  bounded reviewer reads the branch trusting that record, exactly the way a human reviewer trusts a
  green status check on a pull request instead of re-running the suite themselves.
- **Red** does not reach review. Instead it becomes one synthetic repair task, routed on the
  `premium → max → halt` ladder — meaning it is handed to a strong worker (a **build profile** being
  one of four worker tiers — economy, standard, premium, max — a plan task is routed to by risk),
  escalated once to the strongest, and if that still cannot green the suite, the build halts for a
  human rather than merging a broken integration.

Running the suite here — on the build side, owned by the worker that can fix a failure — rather than
inside the reviewer is deliberate: the suite is the boundary between building and reviewing, and it
belongs to the side that can act on a failure. The reasoning behind that split is
[Reviewing before the human does](./reviewing-before-the-human.md).

Two `build:` config keys shape this step, both settable in any config layer:

- `build.gate` — `local` (the default) runs the suite once after all tasks and mints exact-head
  evidence on green; `off` declares that this repository has no build test gate, and truthful
  `skipped` evidence is recorded instead of running anything (quote the value `off`).
- `build.checkpoint` — `false` (the default) keeps only the per-task code commits as the durable
  record of progress, so a resumed run reconstructs where it was from the plan, commits, code, and
  tests. `true` additionally writes a compact resume ledger recording each task's profile,
  escalation, and commit, so a resumed run can skip work already proven complete. Anything other
  than `true`/`false` is a config error, not a silent fallback.

## Build evidence

A green gate does not just pass silently — it leaves **build evidence** (the committed record of that
gate run, read by the reviewer). The record captures the command that ran, its result, the exact
branch head it ran against, and a timestamp. The reviewer verifies that this record is present,
green, and pinned to the exact head it is reviewing; if it is missing, malformed, or stale, the
reviewer returns a blocker and refuses to certify — running the suite itself is never the reviewer's
job.

That same record carries forward into the pull request body, where the close-out sequence reads it
back. This is what lets a clean merge skip a redundant test run: when the pre-merge rebase changes
nothing and the recorded evidence is green and still pinned to the head being merged, the post-rebase
run is skipped and the skip is logged. The concrete payoff is that the whole path from build to merge
runs the suite **once** when nothing has to be fixed or rebased — the record, not a re-run, carries
the proof between steps.

## The gate driver and wall-clock budgets

The gate does not assume the whole suite finishes inside one command call. It executes *durably*: the
run records its outcome where a later look can read it, and the agent establishes completion from
that record rather than from the completion signal of whatever command kicked it off. This is why a
gate run can go quiet for a while, and why a stale "still running" report is not evidence that it
crashed. A gate driver advances the run in bounded slices and reads the durable record for the
verdict; it never simply launches the suite in the background and walks away.

Two things bound and shape that run:

- **The observation budget.** `gate_observation_budget` (default `30`, in minutes, settable in any
  layer) caps how long docket will keep watching for a terminal result. Exhausting it with no result
  **fails closed**: the build halts for a human rather than guessing either success or a red suite,
  because a run that has not finished is not a run that has failed. This is docket's own policy value,
  deliberately independent of whatever single-call timeout your harness imposes.
- **Wall-clock budgets per test file.** The suite runs its files in parallel with per-job isolation
  and measures each file against its own wall-clock budget. Because a parallel wall-clock number
  depends on the machine and the load around it, a budget line is a *screening* signal, not an
  automatic failure. A `BUDGET WATCH:` (or `PARALLEL-SENSITIVE:`) line is exactly that screening
  finding — a heads-up to record and look at, nothing that reds the run. A
  `SERIAL CONFIRMED OVER BUDGET:` line is different: it is an authoritative breach, confirmed by
  re-running that file on its own away from the parallel load, and it is the one you act on. Neither
  line fails the run by default, so nothing else will catch a real breach for you — reading them is
  part of reading the gate result.

## Re-greening after a rebase

The build gate certifies the branch as it stood when the build finished. Closing the change out
re-checks it against the *current* integration branch. **Finalize** (the close-out sequence: rebase
onto the integration branch, retest, merge, archive) rebases the feature branch onto the
**integration branch** (the branch code lands on, usually `main`) and re-runs the suite, so a branch
that was green in isolation but conflicts with work merged since cannot land a broken integration.

If that post-rebase run reds, the rebase surfaced a real integration failure, and an integration-
repair step is what makes it green again. It root-causes the newly-red tests and writes a *minimal*
fix — bounded to at most two attempts, and never by weakening or deleting a test to force green —
then hands a structured report back to the close-out sequence, which gates the merge behind sign-off
on that repair. The full close-out flow, and what it does when repair cannot succeed, is
[Landing changes safely](./landing-changes.md); the mechanism view is
[Finalize as a sequencer](../concepts/finalize-sequencer.md).

## Configuring the suite command

Two keys name the suite, one per gate, and they are **read from config, never from a second copy**:

- `build.test_command` — the command the build gate runs.
- `finalize.test_command` — the command finalize's merge gate runs before it merges.

Both default to the empty string, which means *unconfigured*: a `local` gate with no command halts
with a typed remedy pointing you at `docket repository configure-tests` rather than trying to guess a
command at runtime. They are **independent** — the two may diverge if a repo wants a lighter suite at
build time than at merge time — but in this repository both resolve to the same command today. The
one rule that matters whichever they resolve to: each gate reads its own key from config, so there is
exactly one source for each and no drifting duplicate to keep in sync. `finalize.gate` is the
matching on/off switch for the merge gate — `local` (the default), `ci`, `both`, or `off`, where
`off` merges trusting the pull request's own continuous-integration checks with no local
rebase-and-retest.

For the exact shape of every one of these keys and the layers each may be set in, the reference is
the shipped `.docket.example.yml` — see [Config keys](../reference/config-keys.md) for where that
lives, rather than copying values from here.
