## docket-plan-writer — dispatch only

This agent wraps no skill and is normally dispatched **by** docket-implement-next at its Step 4
plan step, not invoked directly. If asked to author the implementation plan for a claimed change,
dispatch it.

Dispatch to the subagent `docket-plan-writer`, foreground, using this mode's subagent-launch
mechanism. The prompt must include the change id, title, and synchronized change-file path, the
synchronized spec path, the resolved plan and build skill names, the learnings index path when
learnings are enabled, and the feature worktree the branch is checked out in together with its
pre-dispatch HEAD — this agent is feature-scoped, and a delegated dispatch that names no worktree
is refused. The agent commits the plan artifact with its backlink and a `Docket-Plan-Path` trailer
on the feature branch, performs no docket metadata mutation, and returns only the plan's
repo-relative path as `PLAN_PATH=<repo-relative-path>`.

Do NOT write the plan inline in the parent.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-plan-writer", run_in_background: false,
         prompt: "Author the implementation plan and commit it on the feature branch.")
