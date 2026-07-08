## docket-rebase-resolver — dispatch only

This agent wraps no skill and is normally dispatched **by** docket-finalize-change when its
rebase-onto-base gate hits a conflict, not invoked directly. If asked to resolve rebase conflicts for a
finalize, dispatch it.

Dispatch prompt must include the conflicted rebase state; the agent reconciles each hunk by merge
intent and continues the rebase to completion (it never runs tests).

Do NOT resolve the conflicts inline in the parent.

    Task(subagent_type: "docket-rebase-resolver", run_in_background: false,
         prompt: "Resolve the rebase conflicts by merge intent and continue the rebase.")
