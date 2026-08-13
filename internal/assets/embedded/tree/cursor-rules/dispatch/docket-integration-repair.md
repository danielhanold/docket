## docket-integration-repair — dispatch only

This agent wraps no skill and is normally dispatched **by** docket-finalize-change when the rebased
suite is red, not invoked directly. If asked to make a suite pass after a finalize rebase, dispatch it.

Dispatch to the subagent `docket-integration-repair`, foreground, using this mode's subagent-launch
mechanism. The prompt must include the red-test output, the base it was rebased onto, and the
feature worktree the rebased branch is checked out in — this agent is feature-scoped, and a
delegated dispatch that names no worktree is refused. The agent writes a minimal fix in at most two
attempts and never weakens tests.

Do NOT weaken or delete tests in the parent to force green.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-integration-repair", run_in_background: false,
         prompt: "The rebased suite is red — root-cause and write a minimal fix.")
