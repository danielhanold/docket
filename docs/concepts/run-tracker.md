# The run tracker and attribution

## The problem it solves

You start a build run and walk away. Nobody is watching it. When it
finishes it reports back — and that is exactly where the danger lives: a
run that quit halfway through prints a report that reads like success,
and the "it's done" notification is the dispatched worker's own word about
itself, not an independent fact. Trust that word and you dispatch the same
work again while it is still in flight, or you mark a change — one unit of
planned work, roughly one pull request, tracked as one markdown file — as
finished when it actually halted and needs a human.

The **run tracker** is the bookkeeping around a dispatched build run: who
started it, whether it finished, whether it may be retried. It keeps
those facts in durable state of its own, outside the worker's prose, so
the decision to dispatch again rests on something the worker cannot fake by
wording its report well.

Attribution is the second half of the same problem. When two implement-next
runs build at once, a finish has to be tied back to the exact run that
produced it — otherwise one run's completed work gets credited to the
other, and the wrong change is retried or marked done. The tracker answers
"did *this* run finish?" mechanically, by a key it minted when the run was
started, never by matching on timing or names.

## The moving parts

The parent starts a tracked run, dispatches a worker — an agent, a separately
launched worker with its own context, pinned to a model and effort —
carrying the run context, and later asks the tracker for a verdict. The
verdict, not the worker's report, says what may happen next.

```
  docket run start implement-next
       │  prints run-started <key> <run-context>
       │  (or run-untracked <reason>: dispatch keyless)
       ▼
  the parent dispatches implement-next (the prompt carries the run context)
       │
       ▼
  worker runs, then a completion notification arrives
       │                         (the worker's own claim)
       ▼
  docket run verdict <key> ──reads the tracker's durable state, not the prose──►
       │
       ├─ run-done ────────► finished; nothing more to dispatch
       ├─ run-retry-once ──► exactly one more dispatch, same key
       │                      (granted at most run.max_attempts - 1 times)
       ├─ run-continue ────► the same attempt is still working: resume it or
       │                      dispatch again with its continuation id;
       │                      same key, spends no retry
       ├─ run-stop ────────► no re-dispatch; run-stop … run-halted means the
       │                      change needs a human
       └─ run-observe ─────► an unattributed read (no key); never authorizes
                              a re-dispatch
```

- **Starting** (`docket run start implement-next`) re-reads the backlog
  from fresh state, writes the run record, and prints the key and a
  run-context string. It launches nothing: the parent dispatches the
  worker itself and copies the run context into the dispatch prompt, so
  the finish can later be matched to this start and no other. If the
  record cannot be written, start prints `run-untracked <reason>`
  instead; the parent may still dispatch, but without a key, so that run
  can only ever be read unattributed and can never earn a re-dispatch.
- **The verdict** (`docket run verdict <key>`) reads the tracker's own
  record of the run and prints exactly one decision line. It always exits
  0: the decision is in the line, never in the exit code. Without a key,
  `docket run verdict --unattributed [<id>...]` checks the named changes
  (or every in-progress change) and prints `run-observe` lines, holding
  no key and writing nothing.
  A keyed `run-done` line also retires the run, so it no longer counts as
  live; if something the run started is still settling, the verdict reports
  `run-stop … completion-unaccounted` instead, and repeating the same verdict
  once it settles retires the run.
- **Only two lines authorize another dispatch.** `run-retry-once` grants
  one more attempt for the change id and unmet work it names.
  `run-continue` is nonterminal: the same attempt still owns tracked work,
  so the parent resumes the existing worker, or dispatches implement-next
  again with the change id and the continuation id it names, keeping the
  same key and spending no retry. Every `run-stop` and every `run-observe`
  forbids a re-dispatch.
- **Verifying a change** (`docket run verify --id <id>`) checks one
  change's run against its postconditions — workspace and remote heads,
  build evidence, the pull request, the plan and results files, the claim
  — straight from git, GitHub, and the evidence, and reports one verdict
  (`run-complete`, `run-incomplete` with every unmet condition, or
  `run-unclaimed`). It never reads the worker's report.
- **Resuming** a change already in progress is
  `docket run start implement-next --resume <id>`. Because one worktree
  carries at most one live run, start refuses rather than opening a
  second run over one that has not verifiably stopped:
  `resume-active-run` means the prior run is still active (cancel it or
  continue it through its verdict), and `cancellation-pending` means a
  cancel is still finishing. Once the prior run is confirmed cancelled,
  start reserves exactly one replacement and reports
  `resume-replacement-reserved`; asking again returns that same reserved
  key instead of minting a second run.
- **Cancelling** is explicit: nothing stops a dispatched run when a tab
  closes or a process dies. `docket run cancel --key <key> --reason <why>`
  fences the run, stops its registered tasks and processes, and reports
  `cancelled`, `cancellation-pending` (the fence holds but teardown is
  still resolving; run the same cancel again), `already-cancelled`, or
  `refused` (the key or repository did not match; nothing was touched).
  A cancel never counts as a test failure, never earns a retry, and never
  rolls back completed work.

## The worktree lock

A canonical worktree runs at most one gate at a time. Every gate start — a
drive's one launch and a raw `gate launch` — takes that worktree's
**lock** without waiting, and hands it to the gate's supervisor, which
holds it for the gate's whole life. The kernel releases the lock when the
supervisor exits, so a finished, crashed, or killed gate frees the
worktree on its own: there is no recovery step and nothing to settle
afterward.

So a `worktree-busy` refusal means exactly one thing: a live supervisor
holds the lock. The refusal never queues the start, never joins the
running gate, and never stops it. It names the holder (its drive and
change, or a raw run dir) only after confirming that gate is still
running; otherwise it says the holder is unknown. Freeing the worktree is
the operator's act — wait for the holder to finish, or stop it
(`docket run cancel` for a tracked run, `docket gate stop <run-dir>
--reason <why>` for a raw launch).

`docket gate` commands exit 0 on success, 1 on failure, and 2 on invalid
input. A halted gate run exits like a failed one; the run's own outcome
is in its output, never only in the exit code.

`run cancel` finds a run's drives by the run context those drives record,
and counts a drive torn down once its supervisor is gone. When a supervisor
died alone (SIGKILL or a crash) and its process group still has members,
cancel still reports `cancelled` and adds a `tree-survives:<drive>:<pgid>`
finding: the suite is still running and finishes on its own. Docket never
stops it; the glossary's `tree-survives` entry shows how to. Two smaller
gaps stay accepted: a KILL escalation can leave test targets running in
their own process groups, and after a graceful stop the worktree frees a
moment before the suite runner finishes stopping its targets.

A publish killed mid-flight does not wedge a run. `pr.publish` and
`workspace.publish` journal each remote call in the run record before they
make it, and hold a lock file beside the record until they have written the
outcome. If the process dies in between, the lock frees with it. Cancel,
resume, and the success closeout then read the entry as abandoned: cancel
reports `cancelled`, resume admits its replacement, and the closeout
completes, each with a `mutation-abandoned:<op>` finding (see the glossary).
While the lock is still held, or when docket cannot prove it free, the entry
blocks as `mutation-pending:<op>`.

## The invariants

- A completion notification is the worker's claim, never the parent's
  verdict; only the tracker's durable state authorizes a re-dispatch.
- Every verdict is read against the key that started the run; with no
  key, the tracker falls back to an unattributed read against named change
  ids and can never authorize a re-dispatch from that fallback.
- The tracker attributes a claim conservatively: when it cannot
  mechanically tie a finish to this run, it declines to credit it rather
  than guessing.
- Only `run-retry-once` and `run-continue` authorize another dispatch;
  every stop or observe line forbids it.
- The verdict always exits 0; the decision lives in the report line, not
  the exit code.
- A probe that cannot prove a process is alive is not evidence that it
  died — only a failed existence check is.
- A resume admits exactly one replacement, and only after the prior run's
  cancellation is confirmed.

## Decided in

- [ADR-0074](../adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md)
  — made the build gate's verdict three-valued (green, red, or halt), so an
  exit the test runner defines as a non-failure halts instead of reading as
  red.
- [ADR-0078](../adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md)
  — put the parent-facing run-tracker rule for Claude Code in one physical
  instructions file.
- [ADR-0084](../adrs/0084-re-dispatch-permission-gated-on-attribution-capability-not-launch-shape.md)
  — gated re-dispatch permission on mechanical attribution rather than the
  shape of the launch or the worker's own report.
- [ADR-0087](../adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md)
  — ruled that a liveness probe's non-zero answer is not evidence of
  death.
- [ADR-0095](../adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md)
  — made the gate supervisor deliver a genuine session and an exact
  terminal record (supersedes ADR-0081).
