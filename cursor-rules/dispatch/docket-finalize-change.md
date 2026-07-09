## docket-finalize-change — dispatch only

Trigger when asked to close out a change whose PR is approved or merged (e.g. "finalize change 48",
"close out the merged PR").

Dispatch prompt must include the change id, and that it merges (if approved) through the rebase-retest
gate, archives, cleans up the branch/worktree, and refreshes the board.

Do NOT merge or archive inline; let the agent run its gate (it may itself dispatch the rebase-resolver
or integration-repair subagents).

    Task(subagent_type: "docket-finalize-change", run_in_background: false,
         prompt: "Finalize change 48: merge through the gate, archive, clean up, refresh the board.")
