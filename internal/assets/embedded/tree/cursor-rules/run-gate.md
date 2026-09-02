## Run gate — bracket a dispatched implement-next run with the gate facade

A dispatched run that stops early returns a report that reads as success, and a completion
notification is the CHILD's claim, not your report. The gate facade owns attribution, durable
state, and retry accounting — never hand-reimplement them. The `docket` binary is on `PATH`;
resolve each operation below from the capability catalog and run it with the flags shown. If the
binary is not found, the install is broken — stop and surface it; never reconstruct the gate by hand.

1. Before dispatching `docket-implement-next`, run the `run.gate-before` operation with the
   `implement-next` argument. It prints `gate-armed <key> <dispatch-context>` — keep both in your own
   notes (a shell variable does not survive the next tool call) and copy the `<dispatch-context>` into
   the implement-next dispatch prompt. Add `--resume <id>` to arm a gate for explicitly resuming an
   already-in-progress change. If it prints `gate-unarmed`, you may still dispatch, but the return is
   keyless (step 2's fallback) and can never authorize a re-dispatch.
2. After the run returns — or its detached completion notification arrives — run
   the `run.gate-verdict` operation with `<key>`. Without a key, run the `run.gate-verdict` operation
   with `--unattributed`, adding any change id the notification names as a trailing hint argument.
3. Obey the facade's `gate-*` report line exactly — never its exit code, and never the child's
   prose.
4. Only `gate-retry-once` authorizes another dispatch: the same `docket-implement-next`, once, for
   the id and unmet conjuncts it names, keeping the same key. `gate-continue <key> run-waiting
   <change-id> <continuation-id> <phase>` is **nonterminal** — the same implement-next attempt owns
   live or unconsumed tracked work; it keeps the same key, spends no retry, and is structurally
   distinct from `gate-retry-once` (a continuation of the same attempt, never a second attempt):
   resume the existing implement-next agent when your harness supports a real resume, otherwise
   dispatch `docket-implement-next` again with the explicit change id, the continuation id, and the
   same key. Then run the `run.gate-verdict` operation with `<key>` again after it returns. Every
   `gate-stop` and every `gate-observe` forbids re-dispatch — `run-halted` means a human is needed.
5. Never hand-reimplement attribution or infer permission from child prose, launch shape,
   timestamps, ids, or process exit codes.
