---
id: 26
slug: fork-dispatch-opacity-two-invocation-paths
title: Accept fork-dispatch opacity; document two invocation paths; add no tooling
status: Accepted
date: 2026-07-12
supersedes: []
reverses: []
relates_to: [8, 17, 20, 24]
change: 65
---

## Context

ADR-[[0024]] made `context: fork` + `agent:` frontmatter the Claude-Code-native dispatch
mechanism, so that a **directly invoked** autonomous skill still lands in its pinned wrapper.
It reasoned about the mechanism but never exercised it. Change 0065 did: four probe skills
plus one live in-session invocation, on **Claude Code 2.1.207 (2026-07-12)**. Five findings —
inputs to the decision below, not the decision:

1. **`context: fork` is honored**, including when the skill is reached through the Skill tool.
   A real subagent is spawned, with its own `subagents/agent-<id>.jsonl` transcript and its own
   `agentType` metadata.
2. **The wrapper's `model`/`effort` pin holds inside the fork.** Haiku-pinned wrappers ran at
   `claude-haiku-4-5` beneath an opus/sonnet parent — the pin is not silently inherited from
   the session.
3. **The self-preloading cycle is safe.** An agent whose `skills:` preloads the very skill that
   forks into it neither recurses nor degrades to inline. **This closes the open question
   ADR-[[0024]] left**: 0024 *argued* no-recursion (preload is startup content injection; the
   fork fires on invocation) but never tested it. It is now evidence-backed.
4. **A forked run is not drillable in the TUI** — unlike an Agent-tool dispatch, which is. The
   fork's transcript is reachable only on disk, at
   `~/.claude/projects/<project-slug>/<session-id>/subagents/agent-<id>.jsonl`.
5. **Skills and agents register at process start.** A session that predates a frontmatter or
   wrapper change keeps running the *old* definition. This is the failure mode that makes a
   perfectly healthy fork look broken — and it is what cost a user an afternoon on 2026-07-12.

## Decision

**Fork-dispatch's opacity is accepted, not fixed.** A forked run is unobservable in the TUI by
design of the harness, and docket will **not** add tooling to compensate — no log-tailer, no
wrapper-side progress protocol, no status file.

Instead docket **documents two first-class invocation paths** into the same pinned wrapper:

- **skill-invoke** (`/docket-status`) — forked, pinned, cheapest, **opaque**.
- **agent-dispatch** (`@docket-status`) — the *identical* pinned run, **drillable** in the TUI,
  costs one dispatch turn.

and it **names the on-disk transcript path** as the escape hatch for the opaque path.

The rule a reader needs: **observability is a choice the caller makes at invocation time, not a
property docket engineers.** Pick agent-dispatch when you want to watch; pick skill-invoke for
everything else.

## Consequences

- **Both paths produce an identical pinned run.** They differ only in **observability**
  (agent-dispatch is drillable) and **cost** (agent-dispatch spends a dispatch turn; the fork
  does not). Rule of thumb: **agent-dispatch to watch a long run, skill-invoke for everything
  else.**
- **The self-preload cycle is verified safe** → ADR-[[0024]]'s no-recursion argument is now
  evidence-backed, not merely argued, and the composition wiring
  (`docket-implement-next` → `docket-status` / `docket-adr`) needs **no guard**.
- **The on-disk transcript path is an observed Claude Code internal, not a contract.** It is
  version-stamped (2.1.207) and may move. docket depends on it for **nothing** — it appears in
  **prose only**, never in a script. This is precisely why the escape hatch is documentation
  rather than tooling: prose can carry a caveat that code cannot. A stale sentence misleads; a
  stale script breaks.
- **Process-start registration is now a documented operating rule.** After `sync-agents.sh`, or
  any skill-frontmatter edit, an existing session keeps running the **old** definition —
  **restart the harness process**. (This is the same machine-local, regenerate-don't-trust
  posture as ADR-[[0020]].)
- **No new machinery.** No `sync-agents.sh` change, no new script, no new generated file — the
  whole decision lands as documentation.
- **Cursor is unaffected.** Its generated dispatch rule (ADR-[[0017]]) already routes a direct
  invocation through a real `Task`, so Cursor users are always on the drillable path; the
  two-path choice is a Claude-Code-only consideration.

This ADR is **parallel and additive** — it supersedes and reverses no ADR. It relates to
ADR-[[0008]] (the agent layer whose pin the fork carries), ADR-[[0017]] (the Cursor dispatch
rule, the other half of the two-mechanism story), ADR-[[0020]] (machine-local generated agent
artifacts — the registration-at-process-start consequence), and ADR-[[0024]] (the fork-dispatch
decision whose open composition question this closes).
