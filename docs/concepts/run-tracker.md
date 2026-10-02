# The run tracker and attribution

## The problem it solves

You start a build run and walk away. Nobody is watching it. When it
finishes it reports back — and that is exactly where the danger lives: a
run that quit halfway through prints a report that reads like success,
and the "it's done" notification is the launched worker's own word about
itself, not an independent fact. Trust that word and you launch the same
work again while it is still in flight, or you mark a change — one unit of
planned work, roughly one pull request, tracked as one markdown file — as
finished when it actually halted and needs a human.

The **run tracker** is the bookkeeping around a launched build run: who
launched it, whether it finished, whether it may be retried. It keeps
those facts in durable state of its own, outside the worker's prose, so
the decision to launch again rests on something the worker cannot fake by
wording its report well.

Attribution is the second half of the same problem. When two build loops
run at once, a finish has to be tied back to the exact launch that
produced it — otherwise one loop's completed work gets credited to the
other loop's launch, and the wrong change is retried or marked done. The
tracker answers "did *this* launch finish?" mechanically, by a key it minted
when the launch was started, never by matching on timing or names.

## The moving parts

The parent starts a tracked run, launches a worker — an agent, a separately
launched worker with its own context, pinned to a model and effort —
carrying the run key, and later asks the tracker for a verdict. The
verdict, not the worker's report, says what may happen next.

```
  run start ──mints─► <key> + run-context
       │                         (both handed to the launch)
       ▼
  launch the worker (carries the key)
       │
       ▼
  worker runs, then a completion notification arrives
       │                         (the worker's own claim)
       ▼
  run verdict <key> ──reads the tracker's durable state, not the prose──►
       │
       ├─ run-retry-once ──► exactly one more launch, same key
       │                      (granted at most run.max_attempts - 1 times)
       ├─ run-continue ───► the same attempt resumes; spends no retry
       ├─ run-stop / run-observe ─► no re-launch is authorized
       └─ run-halted ──────► the run needs a human
```

- **Starting** mints the key and a run-context string; the launch
  carries both, so the finish can later be matched to this start and no
  other.
- **The worker** is put to work by a dispatch — launching a named agent
  to do a step and waiting for it to return — but the run tracker itself
  does not sit and wait: it launches, then observes, so a long run does
  not pin the parent.
- **The verdict** reads the tracker's own record of the run and emits one
  report line. Only one line authorizes another launch; the rest forbid
  it.
- A run can end in a **halt** — a state the tracker reports with its own
  exit code, distinct from a plain pass or fail, because a runner that
  exits non-zero for its own reasons has not necessarily failed the work.

## The worktree lock

A canonical worktree runs at most one gate at a time. Every gate start
— a drive's first launch, its single relaunch, and a raw `gate launch` —
takes that worktree's **lock** without waiting, and hands it to the gate's
supervisor, which holds it for the gate's whole life. The kernel releases
the lock when the supervisor exits, so a finished, crashed, or killed gate
frees the worktree on its own: there is no recovery step and nothing to
settle afterward.

So a `worktree-busy` refusal means exactly one thing: a live supervisor
holds the lock. The refusal never queues the start, never joins the
running gate, and never stops it. It names the holder (its drive and
change, or a raw run dir) only after confirming that gate is still
running; otherwise it says the holder is unknown. Freeing the worktree is
the operator's act — wait for the holder to finish, or stop it
(`run.cancel` for a tracked run, `docket gate stop <run-dir> --reason
<why>` for a raw launch).

`run.cancel` finds a run's drives by the run context those drives record,
and counts a drive torn down once its supervisor is gone. The known
process-tree gaps — a supervisor that dies alone while its children keep
running, a KILL escalation, and TERM ending `go run` while the suite runner
is still stopping its targets — are tracked by change 0492.

## The invariants

- A completion notification is the worker's claim, never the parent's
  verdict; only the tracker's durable state authorizes a re-launch.
- Every verdict is read against the key that started the launch; with no
  key, the tracker falls back to an unattributed read against a named change
  id and can never authorize a re-launch from that fallback.
- The tracker attributes a claim conservatively: when it cannot mechanically
  tie a finish to this launch, it declines to credit it rather than
  guessing.
- A halt is reported with its own exit code, and that exit code is a
  property of the run's state, not of how the tracker learned the run had
  stopped.
- A non-zero liveness probe is not evidence the run died — only a failed
  existence check is.
- Exactly one report line authorizes another launch, and it names the
  change id and the still-unmet work; every stop or observe line forbids
  re-launching.

## Decided in

- [ADR-0074](../adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md)
  — made a build run's verdict tri-state, so a runner-defined non-failure
  exit is read as a halt, not a pass.
- [ADR-0075](../adrs/0075-run-gate-attributes-a-claim-conservatively-and-reports-a-halt-with-its-own-exit-code.md)
  — had the run tracker attribute a claim conservatively and report a halt
  with its own exit code.
- [ADR-0078](../adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md)
  — settled the parent-facing run-tracker surface for Claude Code and its
  one-physical-instructions-file policy.
- [ADR-0080](../adrs/0080-detached-delegation-execution-posture-launch-then-observe.md)
  — set the detached-run posture to launch-then-observe.
- [ADR-0084](../adrs/0084-re-dispatch-permission-gated-on-attribution-capability-not-launch-shape.md)
  — gated re-dispatch permission on mechanical attribution capability
  rather than the shape of the launch.
- [ADR-0087](../adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md)
  — ruled that a liveness probe's non-zero answer is not evidence of
  death.
- [ADR-0088](../adrs/0088-halt-exit-code-is-a-property-of-run-state-not-discovery-path.md)
  — fixed a halt's exit code as a property of the run's state, not of the
  path by which the tracker discovered it.
- [ADR-0095](../adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md)
  — replaced the per-platform detachment contract with a gate
  supervisor that delivers a genuine session and an exact terminal record
  (supersedes ADR-0081).
