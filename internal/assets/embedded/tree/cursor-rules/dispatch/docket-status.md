## docket-status — dispatch only

Trigger when asked to see or refresh the docket backlog / board (e.g. "show the docket board",
"refresh the board", "run the docket health checks", "sweep merged changes").

Dispatch to the subagent `docket-status`, foreground, using this mode's subagent-launch mechanism.
The prompt must include which pass is wanted (board regen, merge sweep, or health checks) if the
user specified.

Do NOT regenerate the board or run the sweep inline.

One concrete call, as an illustration of the shape — not the contract:

    Task(subagent_type: "docket-status", run_in_background: false,
         prompt: "Refresh the docket board: regenerate BOARD.md, sweep, run health checks.")
