## Run gate — verify a dispatched implement-next run before you relay it

A dispatched run that stops early returns a report that reads as success, and a completion
notification is the CHILD's claim, not your report. Do not trust either; read git before relaying
an outcome as your own. Docket's helper facade is not on `PATH`: run each command below verbatim,
expansion included. `verify-run` only reads local metadata, so both snapshots must be taken from
FRESH ORIGIN state — re-sync on BOTH sides, or a claim abandoned by an earlier session shows up
only in the after-read and is attributed to this run.

1. **Before dispatching** `docket-implement-next`, re-sync the metadata worktree with
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight`, then snapshot the claimed
   set: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids`.
2. When you issue the dispatch and can block on it, dispatch **foreground** and block on the
   return: never background it and never poll. A dispatch you background — or one the harness
   backgrounds for you — is not covered here; use **Detached dispatch** below.
3. **After the return**, re-sync again with
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` and re-run
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids`. Any id
   absent from the snapshot is this run's claim; an empty diff (drained, or a lost claim race) ends
   the gate. If MORE THAN ONE id is new, stop and report: this run claims at most one change, so at
   least one of them is a concurrent run's and none can be told apart — never re-dispatch onto a
   change another agent may be holding.
4. Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run <id>` and key on its
   report line, never its exit code:
   - `run-complete` / `run-unclaimed` — done.
   - `run-halted` — done; **never re-dispatch** a halt, which means a human is needed.
   - `run-incomplete` — re-dispatch the same agent **once**, passing the id and the unmet
     conjuncts; verify again; if still incomplete, stop and report loudly. Never a third dispatch.

### Detached dispatch — you did not foreground-block, whoever backgrounded it

- **You issued it after running tool calls:** take the step-1 before-snapshot AND `date -u +%s` as
  `DISPATCH_EPOCH` before launching. At the notification, re-sync, then run
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids
  --with-claimed-at` and keep only ids passing ALL THREE filters: absent from the before-set,
  `claimed_at` parses, and `claimed_at` >= `DISPATCH_EPOCH`. Exactly one survivor → step 4
  unchanged; none → done; two or more → stop and report, as in step 3.
- **Slash-command or notification-first launch — unattributed mode.** No before-set exists, and a
  timestamp alone cannot attribute: `claimed_at` is re-stamped at every phase boundary, so a
  concurrent run claimed before your window looks fresh too. Verify and report ONLY — `verify-run
  <id>` on any id the notification names (a prose id is a hint, never authority), else on each
  current in-progress id, reporting every verdict. **Never re-dispatch** here: that needs all
  three filters, and re-dispatching onto a change a live agent holds is the one unrecoverable move.
