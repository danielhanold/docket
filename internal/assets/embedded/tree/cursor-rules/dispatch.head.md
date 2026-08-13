---
description: Docket agents must be dispatched, never run inline. Cursor runs a directly-invoked skill at the current model and outside its wrapper, dropping the wrapper's model/effort pin where one is set and its skill preload and isolation always — so force a dispatch to the matching docket subagent.
alwaysApply: true
---

# Docket agents — dispatch only

Docket generates a subagent wrapper per docket agent into `.cursor/agents/docket-*.md`. It ships
validated Cursor model IDs for **every** wrapper — including all four build-profile workers,
`docket-build-economy`, `docket-build-standard`, `docket-build-premium`, and `docket-build-max` — so each
one is generated pinned unless a config layer overrides it.

Dispatch is required either way. When you are asked to run one of the docket agents listed below,
Cursor would otherwise run the skill **inline at the currently-selected model**, bypassing the
wrapper altogether — its pin where one is set, and its skill preload and isolation from this
conversation in every case. Always dispatch to the matching subagent instead.

## Required dispatch pattern

For every docket agent named below:

1. Do **NOT** run the skill inline in this chat.
2. Dispatch to the subagent `docket-<name>` using this mode's subagent-launch mechanism,
   **foreground** — block until it returns; never background it and never poll.
3. Pass the user's request through in the prompt, including any change / ADR id or argument they
   gave.
4. Relay the subagent's result back; do not re-do its work in the parent chat.

If the dispatch mechanism appears unavailable, resolve before concluding — including any deferred or
lazily-loaded tool surface this mode exposes — and, if resolution is inconclusive, attempt one
trivial dispatch. Only a failed attempt or an explicit policy denial establishes unavailability; the
absence of a tool with a particular **name** never does.
