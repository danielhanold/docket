## Run gate — bracket a dispatched implement-next run with the gate facade

A dispatched run that stops early returns a report that reads as success; a completion
notification is the CHILD's claim, not your report. The gate facade owns attribution, durable state,
and retry accounting: never hand-reimplement them, and never infer permission from child prose,
launch shape, timestamps, ids, or exit codes. The `docket` binary is on `PATH`; resolve each
operation below from the capability catalog. If it is missing, the install is broken: surface it,
never rebuild the gate by hand.

1. Before dispatching `docket-implement-next`, run `run.gate-before` with `implement-next`. It prints
   `gate-armed <key> <epoch> <dispatch-context>`; keep all three (they won't survive the next tool
   call) and copy the `<dispatch-context>` into the dispatch prompt. The `<epoch>` is the run epoch id
   you thread into `run.cancel --epoch` (below) and every `--run-epoch` dispatch flag (`agent.enter`,
   `gate drive start`, `gate drive prepare-scope`). Add `--resume <id>` to arm for resuming an
   already-in-progress change. `gate-unarmed` still lets you dispatch, but keyless (step 2's fallback)
   and can never authorize a re-dispatch.
2. After the run returns, or its completion notification arrives, run `run.gate-verdict`
   with `<key>`; without a key, run it with `--unattributed` plus any change id the notification
   names. Obey the resulting `gate-*` report line exactly, never its exit code or the child's prose.
3. Only `gate-retry-once` authorizes another dispatch: the same `docket-implement-next`, once, for
   the id and unmet conjuncts it names, keeping the same key. `gate-continue <key> run-waiting
   <change-id> <continuation-id> <phase>` is **nonterminal**: the same attempt still owns tracked
   work, so it keeps the same key, spends no retry, and is distinct from `gate-retry-once` (a
   continuation, not a second attempt). On it, resume the existing implement-next agent,
   or dispatch `docket-implement-next` again with the explicit change id, the continuation id, and the
   same key, and run `run.gate-verdict` with `<key>` again. Every `gate-stop` and every
   `gate-observe` forbids re-dispatch; `run-halted` means a human is needed.

## Stopping a dispatched run — there is no automatic Stop button

A run you dispatched has **no automatic Stop**: closing a tab, interrupting the coordinator, or
killing a process does not tell the gate the run is over, and the arm says so (it reports the
honest owner-lifecycle caveat). To stop a dispatched run deliberately, invoke the explicit
`run.cancel` operation (argv resolved from the capability catalog) with the key and epoch the arm
gave you, plus a human reason — `--key <key> --epoch <id> --reason <why>`.

It fences the run so nothing new can attach to it, then stops the run's registered native tasks and
processes and reports one disposition:

- `cancelled` — the run was fenced and everything the cancel tracks is accounted for.
- `cancellation-pending` — the fence is durably held but teardown is not fully accounted for yet
  (a process or in-flight action still resolving). Cleanup is safe to resume: **re-run the same
  cancel** to finish it; a repeat never restores the run.
- `already-cancelled` — the run was already cancelled (or the cancel already completed); a no-op.
- `refused` — the key, epoch, or repository did not match; nothing was touched.

Cancelling is never a test failure and never earns a retry: it charges no suite attempt and resets
no deadline, budget, or retry state. Completed work is never rolled back.

## Resuming after a stop or interruption

Arm a resume with `run.gate-before … --resume <id>`. Because one worktree carries at most one live
run, the arm refuses to start a second run over one that has not verifiably stopped, and tells you
what to do instead:

- `resume-active-run` — the prior run's epoch is still **active** (nothing has confirmed it
  stopped). The arm prints a locator naming the change, epoch, and key, plus the exact remedy:
  **cancel it** via the `run.cancel` operation (`--key <key> --epoch <id> --reason <why>`) and
  resume after confirmed cancellation, **or** continue the live run via `run.gate-verdict`. Do not
  force a fresh claim over a run that may still be live.
- `cancellation-pending` — a cancellation is still finishing. The resume only observes that
  cleanup; it does not admit a replacement. Finish the cancel (re-run it until `cancelled`), then
  resume.
- `resume-replacement-reserved` — the prior epoch was already confirmed-cancelled and superseded,
  and **exactly one** replacement dispatch is reserved. A repeat arm (or one recovering a lost
  response) returns that same reserved key rather than minting a second run — dispatch the reserved
  replacement; never start a parallel one.

Resume admits **exactly one** replacement, and only after cancellation is confirmed. `gate-continue`
is unchanged by any of this: it stays nonterminal, keeps the same key, spends no retry, and resumes
the existing agent (or re-dispatches with the change id and continuation id) as in step 3 above.
