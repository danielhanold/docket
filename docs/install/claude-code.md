# Claude Code: running docket under Claude Code

Claude Code is docket's reference harness, and everything in [Installing docket](install.md) and
[Models](models-and-effort.md) applies to it directly. Two Claude-Code-specific details are worth
calling out.

**Forking preserves the pin but hides the run.** As the *How the pin survives a direct invocation*
table in [Models](models-and-effort.md) shows, a directly-invoked forked skill returns as
`completed (forked execution)`, with no box to drill into in the TUI. A forked run is not lost, only
unobservable there: Claude Code still writes its full transcript to
`~/.claude/projects/<project-slug>/<session-id>/subagents/agent-<id>.jsonl`. Treat that path as an
**observed internal, not an interface** — it was accurate on Claude Code 2.1.207, it may move, and
docket depends on it for nothing. When you want to watch a long run live, dispatch it with
`@docket-<name>` instead, which routes through a real, drillable dispatch.

**Restart after changing an agent or a skill.** Skills and agents are **registered at process
start**. After an install, or after you edit a skill's frontmatter, an already-open session keeps
running the *old* definitions — so a freshly-added fork appears to do nothing, and a healthy pin
looks broken. Restart the harness process (a new session — clearing the context is not enough) and
re-invoke.
