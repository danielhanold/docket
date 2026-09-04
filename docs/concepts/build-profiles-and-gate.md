# Build profiles and the test gate

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
list of tasks, and each is routed to a **build profile**: one of four worker
tiers (economy, standard, premium, max) a plan task is routed to by risk. After
the last task lands, the **build gate** — the full test-suite run at the end of a
build that must be green before review — runs the whole suite once and records
**build evidence**, the committed record of that gate run, read by the reviewer.

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
   build evidence      the build does not
   committed;          reach review
   review may begin
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
- The gate measures each test file against a wall-clock budget. A parallel run's
  number is machine-dependent, so a budget-watch line is a screening finding to
  record, while a serially-confirmed breach is the one to act on — neither fails
  the run by itself.
- The build evidence is committed, so the reviewer reads a durable record of the
  gate run instead of trusting a worker's word that the suite passed.

## The invariants

- Every plan task is routed to exactly one profile by its risk; standard is the
  default and absorbs anything the router cannot confidently place.
- A worker owns one task end to end and records it with exactly one commit; work
  outside that task belongs to another worker.
- A task escalates up the ladder at most once, and only for genuine
  under-capacity — not for an expected failing test or a round of debugging.
- The full suite runs once at the build gate, after the last task lands, never
  only the tests a single task enumerated.
- The gate's suite command is read from configuration (`build.test_command`),
  never from a second copy, so it tests the exact checkout under review.
- A wall-clock budget line never fails the run on its own: a screening finding is
  recorded, a serially-confirmed breach is acted on.
- The build evidence is committed before review begins, so review rests on a
  recorded gate run rather than a claim.

## Decided in

- [ADR-0063](../adrs/0063-docket-owns-the-build-role-profile-routed-workers.md) —
  had docket own the build role as profile-routed workers, with model and effort
  pinned on named agents (supersedes ADR-0023's per-role build-model surface).
- [ADR-0064](../adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md)
  — moved the shipped model and effort defaults for those workers into a
  harness-indexed sidecar.
- [ADR-0066](../adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md)
  — had docket own the review role and fixed that the suite runs in the build
  gate, before review, not inside the review.
- [ADR-0070](../adrs/0070-fix-loop-profile-envelope-blocker-floor-and-max-ceiling.md)
  — bounded the fix loop's profile envelope with a blocker floor at standard and
  a ceiling below max.
- [ADR-0074](../adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md)
  — made the build gate's verdict tri-state, so a runner-defined non-failure exit
  reads as a halt rather than a pass.
