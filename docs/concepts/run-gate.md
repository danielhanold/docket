# The run gate and attribution

## The problem it solves

You start a build run and walk away. Nobody is watching it. When it
finishes it reports back — and that is exactly where the danger lives: a
run that quit halfway through prints a report that reads like success,
and the "it's done" notification is the launched worker's own word about
itself, not an independent fact. Trust that word and you launch the same
work again while it is still in flight, or you mark a change — one unit of
planned work, roughly one pull request, tracked as one markdown file — as
finished when it actually halted and needs a human.

The **run gate** is the bookkeeping around a launched build run: who
launched it, whether it finished, whether it may be retried. It keeps
those facts in durable state of its own, outside the worker's prose, so
the decision to launch again rests on something the worker cannot fake by
wording its report well.

Attribution is the second half of the same problem. When two build loops
run at once, a finish has to be tied back to the exact launch that
produced it — otherwise one loop's completed work gets credited to the
other loop's launch, and the wrong change is retried or marked done. The
gate answers "did *this* launch finish?" mechanically, by a key it minted
when the launch was armed, never by matching on timing or names.

## The moving parts

The parent arms the gate, launches a worker — an agent, a separately
launched worker with its own context, pinned to a model and effort —
carrying the gate's key, and later asks the gate for a verdict. The
verdict, not the worker's report, says what may happen next.

```
  gate-before ──arms──► <key> + dispatch-context
       │                         (both handed to the launch)
       ▼
  launch the worker (carries the key)
       │
       ▼
  worker runs, then a completion notification arrives
       │                         (the worker's own claim)
       ▼
  gate-verdict <key> ──reads the gate's durable state, not the prose──►
       │
       ├─ gate-retry-once ──► exactly one more launch, same key
       ├─ gate-continue ───► the same attempt resumes; spends no retry
       ├─ gate-stop / gate-observe ─► no re-launch is authorized
       └─ run-halted ──────► the run needs a human
```

- **Arming** mints the key and a dispatch-context string; the launch
  carries both, so the finish can later be matched to this arming and no
  other.
- **The worker** is put to work by a dispatch — launching a named agent
  to do a step and waiting for it to return — but the run gate itself
  does not sit and wait: it launches, then observes, so a long run does
  not pin the parent.
- **The verdict** reads the gate's own record of the run and emits one
  report line. Only one line authorizes another launch; the rest forbid
  it.
- A run can end in a **halt** — a state the gate reports with its own
  exit code, distinct from a plain pass or fail, because a runner that
  exits non-zero for its own reasons has not necessarily failed the work.

## The invariants

- A completion notification is the worker's claim, never the parent's
  verdict; only the gate's durable state authorizes a re-launch.
- Every verdict is read against the key that armed the launch; with no
  key, the gate falls back to an unattributed read against a named change
  id and can never authorize a re-launch from that fallback.
- The gate attributes a claim conservatively: when it cannot mechanically
  tie a finish to this launch, it declines to credit it rather than
  guessing.
- A halt is reported with its own exit code, and that exit code is a
  property of the run's state, not of how the gate learned the run had
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
  — had the run gate attribute a claim conservatively and report a halt
  with its own exit code.
- [ADR-0078](../adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md)
  — settled the parent-facing gate surface for Claude Code and its
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
  path by which the gate discovered it.
- [ADR-0095](../adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md)
  — replaced the per-platform detachment contract with a native
  supervisor that delivers a genuine session and an exact terminal record
  (supersedes ADR-0081).
