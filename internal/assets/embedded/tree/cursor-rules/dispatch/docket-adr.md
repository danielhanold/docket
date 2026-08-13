## docket-adr — dispatch only

Trigger when asked to record, supersede, reverse, or index an architecture decision (e.g. "record an
ADR for this decision", "supersede ADR-0015", "regenerate the ADR index").

Dispatch to the subagent `docket-adr`, foreground, using this mode's subagent-launch mechanism. The
prompt must include the decision (context / decision / consequences) or the index operation; the
agent assigns the number and updates the index.

Do NOT hand-write the ADR file or pick the number in the parent.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-adr", run_in_background: false,
         prompt: "Record an ADR for <decision>: context, decision, consequences.")
