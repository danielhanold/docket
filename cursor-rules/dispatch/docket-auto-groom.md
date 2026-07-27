## docket-auto-groom — dispatch only

Trigger when asked to drain the auto-groomable needs-brainstorm queue with no human (e.g. "auto-groom
the backlog", "design the auto-groomable stubs").

Dispatch to the subagent `docket-auto-groom`, foreground, using this mode's subagent-launch
mechanism. The prompt must include any explicit stub id, and that kill/defer are never autonomous
(the agent abstains back to the human queue instead).

Do NOT run the grooming inline or make kill/defer decisions in the parent.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-auto-groom", run_in_background: false,
         prompt: "Drain the auto-groomable needs-brainstorm queue.")
