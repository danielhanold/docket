## docket-build-economy — dispatch only

Trigger only from the `docket-build` controller, when it has routed a plan task to the ECONOMY
profile. Never trigger this agent from a human request directly.

Dispatch to the subagent `docket-build-economy`, foreground, using this mode's subagent-launch
mechanism. The prompt must carry the plan task, the branch and its feature worktree, the selected
profile and its routing reason, and the completion schema from the docket-build-task skill. This
agent is feature-scoped: a delegated dispatch that names no worktree is refused.

Do NOT implement the task in the parent, and do NOT dispatch a reviewer after it.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-build-economy", run_in_background: false,
         prompt: "Task 3 of <plan path>. Profile: economy (fully specified, established pattern, no cross-file reasoning). <task text>")
