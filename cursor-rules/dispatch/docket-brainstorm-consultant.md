## docket-brainstorm-consultant — dispatch only

This agent wraps no skill and injects no convention (a deliberate deviation from the
ADR-0009 critic pattern). It is an opt-in consultant-author for the brainstorm role,
handed a settled design and returning exactly one of an authored spec or critique
concerns. Dispatch it rather than reviewing inline.

Dispatch to the subagent `docket-brainstorm-consultant`, foreground, using this
mode's subagent-launch mechanism. The prompt must include the stub/idea,
neighbouring changes, relevant ADRs, and relevant learnings findings, drawn from
the learnings index, the consultant needs to author or critique.

It performs zero docket operations — no git, no file writes, no board updates — and
returns prose in-context only.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-brainstorm-consultant", run_in_background: false,
         prompt: "Author a spec or return critique concerns for this settled design.")
