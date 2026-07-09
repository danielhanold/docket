## docket-auto-groom — dispatch only

Trigger when asked to drain the auto-groomable needs-brainstorm queue with no human (e.g. "auto-groom
the backlog", "design the auto-groomable stubs").

Dispatch prompt must include any explicit stub id, and that kill/defer are never autonomous (the agent
abstains back to the human queue instead).

Do NOT run the grooming inline or make kill/defer decisions in the parent.

    Task(subagent_type: "docket-auto-groom", run_in_background: false,
         prompt: "Drain the auto-groomable needs-brainstorm queue.")
