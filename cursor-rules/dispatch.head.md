---
description: Docket agents must be dispatched, never run inline. Cursor runs a directly-invoked skill at the current model, which defeats docket's model/effort pins — so force a Task dispatch to the matching subagent_type.
alwaysApply: true
---

# Docket agents — dispatch only

Docket ships model/effort-pinned subagent wrappers in `.cursor/agents/docket-*.md`. When you are
asked to run one of the docket agents listed below, Cursor would otherwise run the skill **inline at
the currently-selected model**, which defeats the pin. Always dispatch to the matching subagent
instead.

## Required dispatch pattern

For every docket agent named below:

1. Do **NOT** run the skill inline in this chat.
2. Launch a **Task** with `subagent_type: "docket-<name>"` and `run_in_background: false`
   (foreground — wait for it). Pass the user's request through in the prompt, including any change /
   ADR id or argument they gave.
3. Relay the subagent's result back; do not re-do its work in the parent chat.
