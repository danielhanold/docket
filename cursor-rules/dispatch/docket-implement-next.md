## docket-implement-next — dispatch only

Trigger when asked to implement the next build-ready change, drain the backlog, or build a specific
change id end-to-end (e.g. "implement the next change", "build change 48", "drain the docket backlog").

Dispatch prompt must include the explicit change id if the user named one (otherwise let the agent
select), and that it runs autonomously to an open PR and stops at the human merge gate.

Do NOT run the build inline, merge the PR, or re-brainstorm the design (the agent reconciles but never
re-brainstorms).

    Task(subagent_type: "docket-implement-next", run_in_background: false,
         prompt: "Implement change 48 end-to-end to an open PR; stop at the merge gate.")
