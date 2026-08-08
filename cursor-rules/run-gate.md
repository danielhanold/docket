## Run gate — verify a dispatched implement-next run before you report it

A dispatched run that stops early returns a report that reads as success. Do not trust it; read git.

1. **Before dispatching** `docket-implement-next`, snapshot the claimed set:
   `docket.sh verify-run --in-progress-ids`.
2. Dispatch and block on the return, as above.
3. **After the return**, re-run `docket.sh verify-run --in-progress-ids`. Any id absent from the
   snapshot is this run's claim; an empty diff (drained, or a lost claim race) ends the gate.
4. Run `docket.sh verify-run <id>` and key on its report line, never its exit code:
   - `run-complete` / `run-unclaimed` — done.
   - `run-halted` — done; **never re-dispatch** a halt, which means a human is needed.
   - `run-incomplete` — re-dispatch the same agent **once**, passing the id and the unmet
     conjuncts; verify again; if still incomplete, stop and report loudly. Never a third dispatch.
