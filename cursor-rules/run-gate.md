## Run gate — verify a dispatched implement-next run before you report it

A dispatched run that stops early returns a report that reads as success. Do not trust it; read git.
Docket's helper facade is not on `PATH`: run each command below verbatim, expansion included.
`verify-run` only reads local metadata, so both snapshots must be taken from FRESH ORIGIN state —
re-sync on BOTH sides, or a claim abandoned by an earlier session shows up only in the after-read
and is attributed to this run.

1. **Before dispatching** `docket-implement-next`, re-sync the metadata worktree with
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight`, then snapshot the claimed
   set: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids`.
2. Dispatch and block on the return, as above.
3. **After the return**, re-sync again with
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` and re-run
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids`. Any id
   absent from the snapshot is this run's claim; an empty diff (drained, or a lost claim race) ends
   the gate.
4. Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run <id>` and key on its
   report line, never its exit code:
   - `run-complete` / `run-unclaimed` — done.
   - `run-halted` — done; **never re-dispatch** a halt, which means a human is needed.
   - `run-incomplete` — re-dispatch the same agent **once**, passing the id and the unmet
     conjuncts; verify again; if still incomplete, stop and report loudly. Never a third dispatch.
