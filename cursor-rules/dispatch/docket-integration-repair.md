## docket-integration-repair — dispatch only

This agent wraps no skill and is normally dispatched **by** docket-finalize-change when the rebased
suite is red, not invoked directly. If asked to make a suite pass after a finalize rebase, dispatch it.

Dispatch prompt must include the red-test output and the base it was rebased onto; the agent writes a
minimal fix in at most two attempts and never weakens tests.

Do NOT weaken or delete tests in the parent to force green.

    Task(subagent_type: "docket-integration-repair", run_in_background: false,
         prompt: "The rebased suite is red — root-cause and write a minimal fix.")
