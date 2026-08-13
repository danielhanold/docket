## docket-review-standard — dispatch only

Trigger only from the `docket-implement-next` controller at its review step, when it has selected
the STANDARD reviewer rung. Never trigger this agent from a human request directly.

Dispatch to the subagent `docket-review-standard`, foreground, using this mode's subagent-launch
mechanism. The prompt must carry the branch and its base ref, the feature worktree the branch is
checked out in, the change's title and scope sections, the relevant learnings hooks, and the
current build-evidence record. This agent is feature-scoped: a delegated dispatch that names no
worktree is refused.

The reviewer is read-only: it returns findings and never fixes, never commits, and never runs the
test suite. Do NOT dispatch a second reviewer afterwards.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-review-standard", run_in_background: false,
         prompt: "Review branch feat/<slug> against origin/main. Rung: standard (build routed or escalated a task to its standard profile). Evidence: <block>. <context>")
