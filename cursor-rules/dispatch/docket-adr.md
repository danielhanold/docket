## docket-adr — dispatch only

Trigger when asked to record, supersede, reverse, or index an architecture decision (e.g. "record an
ADR for this decision", "supersede ADR-0015", "regenerate the ADR index").

Dispatch prompt must include the decision (context / decision / consequences) or the index operation;
the agent assigns the number and updates the index.

Do NOT hand-write the ADR file or pick the number in the parent.

    Task(subagent_type: "docket-adr", run_in_background: false,
         prompt: "Record an ADR for <decision>: context, decision, consequences.")
