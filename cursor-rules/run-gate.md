## Run gate — bracket a dispatched implement-next run with the gate facade

A dispatched run that stops early returns a report that reads as success; a completion
notification is the CHILD's claim, not your report. The gate facade owns attribution, durable state,
and retry accounting: never hand-reimplement them, and never infer permission from child prose,
launch shape, timestamps, ids, or exit codes. The `docket` binary is on `PATH`; resolve each
operation below from the capability catalog. If it is missing, the install is broken: surface it,
never rebuild the gate by hand.

1. Before dispatching `docket-implement-next`, run `run.gate-before` with `implement-next`. It prints
   `gate-armed <key> <dispatch-context>`; keep both (it won't survive the next tool call) and copy
   the `<dispatch-context>` into the dispatch prompt. Add `--resume <id>` to arm for
   resuming an already-in-progress change. `gate-unarmed` still lets you dispatch, but keyless
   (step 2's fallback) and can never authorize a re-dispatch.
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
