## docket-rebase-resolver — dispatch only

This agent wraps no skill and is normally dispatched **by** docket-finalize-change when its
rebase-onto-base gate hits a conflict, not invoked directly. If asked to resolve rebase conflicts for a
finalize, dispatch it.

Dispatch to the subagent `docket-rebase-resolver`, foreground, using this mode's subagent-launch
mechanism. The prompt must include the conflicted rebase state and the feature worktree the rebase
is running in — this agent is feature-scoped, and a delegated dispatch that names no worktree is
refused. The agent reconciles each hunk by merge intent and continues the rebase to completion (it
never runs tests).

Do NOT resolve the conflicts inline in the parent.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-rebase-resolver", run_in_background: false,
         prompt: "Resolve the rebase conflicts by merge intent and continue the rebase.")
