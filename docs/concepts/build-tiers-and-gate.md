# Build tiers and the suite gate

## The problem it solves

A build works through a list of tasks, and the tasks are not alike. Some are
mechanical — follow a pattern that already exists in the file next door. Some
carry real weight — a data migration you cannot walk back, an architecture call
that is still unsettled. Hand every task to your most capable, most expensive
worker and you pay premium rates to rename a variable. Hand every task to the
cheapest worker and it will eventually wreck something that could not be undone.
The two failure modes pull in opposite directions, and one fixed worker cannot
serve both.

And a build that *looks* finished is not the same as one that is correct. A
worker can make its own task's narrow test pass and still have broken a test
three files away that it never ran. Somebody has to run every test, once, after
all the pieces are assembled — and leave proof that they did, so the next reader
trusts a record rather than a claim.

Docket answers both halves with routing and a gate. The **plan** — the
task-by-task breakdown a build follows, written on the feature branch — is a
list of tasks, and each is routed to a **build tier**: one of four workers
(economy, standard, premium, max) chosen by risk. After
the last task lands, the **build gate** — the full test-suite run at the end of a
build that must be green before review — runs the whole suite once and records
**build evidence**, the immutable record of that gate run, read by the reviewer.

## The moving parts

```
   plan (task-by-task breakdown)
      │
      ▼  router reads each task's risk
  ┌──────────┬──────────┬──────────┬──────────┐
   economy    standard   premium    max
  (pattern-   (normal    (named     (mistakes
   following)  work; the  risk)      cannot be
               uncertainty            walked
               sink)                  back)
  └──────────┴─────┬────┴──────────┴──────────┘
                   │  one worker per task, one commit
                   │  under-capacity → ONE bounded escalation up
                   ▼
           assembled feature branch
                   │
              (all tasks done)
                   ▼
           build gate: run the WHOLE suite once
                   │
         ┌─────────┴─────────┐
       green                 red
         │                   │
         ▼                   ▼
   build evidence      repair tasks, then the
   recorded;           suite again — until
   review may begin    build.max_attempts is
                       spent, then a halt
```

- The router sends the bulk of the work to **standard** — the default, and the
  sink for any task it cannot confidently place — and reserves premium and max
  for tasks whose mistakes are costly or irreversible.
- Each task is one worker's whole assignment. The worker runs its own focused
  tests, makes exactly one commit, and returns; work outside that task boundary
  belongs to a different worker.
- A worker that finds its task materially harder or riskier than its tier can
  carry does not soldier on. It returns under-capacity, and the controller
  escalates that one task up the ladder exactly once. An expected failing test or
  ordinary debugging is not an escalation.
- The build gate is not the per-task focused tests. It is the entire suite, run
  once after the branch is assembled, because a task that passed in isolation can
  still have reddened a test it never looked at.
- A red gate does not reach review — the build turns the failure into repair
  tasks instead (routed premium, then max, then a halt). `build.max_attempts`
  (default 4) caps how many full-suite runs the phase may spend — the initial
  run plus a repair-and-rerun for each red result — and once those runs are
  spent a still-red suite halts for a human. A value of 1 disables repair: the
  first red run halts.
- The gate runs the command in `build.test_command` when `build.gate` is
  `local`. With `build.gate: off` it runs nothing and records truthful skipped
  evidence (`skipped`, reason `build-gate-off`) before review. An empty
  `build.test_command` under `local` is a configuration gap, not a red suite: the
  build halts with the remedy `docket repository configure-tests`.
- Build evidence is minted by `docket evidence record` from the passed run and
  checked by `docket evidence verify`. It lives in the pull request body's
  build-evidence block and is never committed, so the reviewer reads a durable
  record of the gate run instead of trusting a worker's word that the suite
  passed.

## The invariants

- Every plan task is routed to exactly one tier by its risk; standard is the
  default and absorbs anything the router cannot confidently place.
- A worker owns one task end to end and records it with exactly one commit; work
  outside that task belongs to another worker.
- A task escalates up the ladder at most once, and only for genuine
  under-capacity — not for an expected failing test or a round of debugging.
- The full suite runs once at the build gate, after the last task lands, never
  only the tests a single task enumerated.
- The gate's suite command is read from configuration (`build.test_command`),
  never from a second copy, so it tests the exact checkout under review.
- Build evidence is recorded in the pull request body before review begins, so
  review rests on a recorded gate run rather than a claim.
- An empty test command never reads as a red suite: it halts with a remedy
  instead of manufacturing a repair task.

## Decided in

- [ADR-0063](../adrs/0063-docket-owns-the-build-role-profile-routed-workers.md) —
  had docket own the build role as tier-routed workers, with model and effort
  pinned on named agents.
- [ADR-0064](../adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md)
  — indexed the shipped model and effort defaults for those workers by harness.
- [ADR-0066](../adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md)
  — had docket own the review role and fixed that the suite runs in the build
  gate, before review, not inside the review.
- [ADR-0070](../adrs/0070-fix-loop-profile-envelope-blocker-floor-and-max-ceiling.md)
  — bounded the fix pass's tiers: a blocker's fix starts no lower than
  standard, and no fix task runs at max.
- [ADR-0074](../adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md)
  — made the build gate's verdict three-valued (green, red, or halt), so an exit
  the test runner defines as a non-failure halts instead of reading as red.
