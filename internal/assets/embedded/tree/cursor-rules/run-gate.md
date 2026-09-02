## Run gate — bracket a dispatched implement-next run with the gate facade

A dispatched run that stops early returns a report that reads as success, and a completion
notification is the CHILD's claim, not your report. The gate facade owns attribution, durable
state, and retry accounting — never hand-reimplement them. The `docket` binary is on `PATH`;
resolve each operation below from the capability catalog and run it with the flags shown. If the
binary is not found, the install is broken — stop and surface it; never reconstruct the gate by hand.

1. Before dispatching `docket-implement-next`, run the `run.gate-before` operation with the
   `implement-next` argument and keep the printed key in your own notes — a shell variable does not
   survive the next tool call. If it prints `gate-unarmed`, you may still dispatch, but the return is
   keyless (step 2's fallback) and can never authorize a re-dispatch.
2. After the run returns — or its detached completion notification arrives — run
   the `run.gate-verdict` operation with `<key>`. Without a key, run the `run.gate-verdict` operation
   with `--unattributed`, adding any change id the notification names as a trailing hint argument.
3. Obey the facade's `gate-*` report line exactly — never its exit code, and never the child's
   prose.
4. Only `gate-retry-once` authorizes another dispatch: the same `docket-implement-next`, once, for
   the id and unmet conjuncts it names, keeping the same key. Every `gate-stop` and every
   `gate-observe` forbids re-dispatch — `run-halted` means a human is needed, and `run-waiting`
   names a continuation a fresh dispatch would NOT resume: report the handoff id and phase, then
   stop.
5. Never hand-reimplement attribution or infer permission from child prose, launch shape,
   timestamps, ids, or process exit codes.
