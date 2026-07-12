---
id: 24
slug: claude-context-fork-skill-dispatch
title: Claude Code uses `context: fork` frontmatter as its inline-skill dispatch mechanism; fork only human-non-interactive skills
status: Accepted
date: 2026-07-11
supersedes: []
reverses: []
relates_to: [8, 17]
change: 61
---

## Context

docket pins each autonomous skill's model/effort via a generated subagent wrapper (the
ADR-[[0008]] agent layer). That pin holds **only when the skill is reached via a `Task`
dispatch** — not when the skill is **invoked directly** (a human typing `/docket-status`, or
the model auto-invoking it). Claude Code runs a direct invocation **inline at the session
model**, silently defeating the pin.

`sync-agents.sh` had encoded the assumption that "only Cursor exhibits the inline quirk" —
**false**: Claude Code exhibits it too. Cursor solves it with a generated `alwaysApply`
dispatch rule (ADR-[[0017]]); Claude Code has a **native mechanism** the earlier design did
not account for.

## Decision

For Claude Code, add native **`context: fork`** + **`agent: docket-<name>`** frontmatter to
the committed `SKILL.md` of each headless-safe autonomous skill, forking a direct invocation
into the existing pinned wrapper — **no generated file, no hook, no CLAUDE.md routing**.

This is the Claude-Code half of a deliberate **two-mechanism** story: Cursor uses a generated
dispatch rule, Claude Code uses native `context: fork`. `HARNESS_HAS_DISPATCH_RULES` stays
**Cursor-only**.

**Fork-exclusion principle:** fork **only** skills that never need the human mid-run, because
a forked subagent has **no channel to the human** (Claude Code withholds `AskUserQuestion`,
`EnterPlanMode`, etc. from subagents).

- **Forked (4):** `docket-status`, `docket-adr`, `docket-implement-next`, `docket-auto-groom`.
- **Not forked (3):** `docket-new-change`, `docket-groom-next` (interactive brainstorm), and
  `docket-finalize-change` — the last excluded **not for interactivity** but because its
  headless merge is blocked by Claude Code's auto-mode "Merge Without Review" classifier
  (making finalize forkable/autonomous is a separate permissions decision, tracked as change
  0062).

## Consequences

- **One edit-once change** in the shared symlinked skill source. The frontmatter is **inert**
  (ignored) in Cursor/Codex/other harnesses, and on a Claude Code too old to know the field it
  **degrades to today's inline behavior** — strictly no worse.
- **Composition terminates without recursion:** wrappers preload the skill via `skills:`
  (content injection at startup), while `context: fork` fires **on invocation** — so a forked
  agent that *starts* with the skill preloaded does not re-fork.
- The invariant (**exactly the 4 forked, the 3 not**) is guarded by a dedicated structural
  test, `tests/test_skill_fork_dispatch.sh`.
- **Deferred:** enforcing the invariant additionally in `sync-agents.sh --check` (a change
  0061 open question). Declined for now because `--check` cannot derive "should be forked"
  purely from "is autonomous-wrapped" — `docket-finalize-change` is autonomous-wrapped yet
  deliberately unforked — so it would need an explicit fork-allowlist: more standing machinery
  than this minimal parity fix warrants, and a second place for the 4/3 split to drift. The
  dedicated test is the guard.

This ADR is **parallel and additive** — it supersedes or reverses no ADR; it relates to
ADR-[[0008]] (the agent layer this fork-dispatch pins into) and ADR-[[0017]] (the Cursor
dispatch rule / full agent set that this is the Claude-Code-native counterpart to).

## Update — 2026-07-12

The **fork-exclusion principle**'s no-channel fact extends beyond the human: a forked/subagent
skill also has **no channel to receive a task-notification**. A fork cannot be resumed by the
notification it is waiting on the way a main-loop session can — awaiting one hands control back
to the caller. So a dispatched/forked parent may **never** background a child and *yield* to
await a notification: that returns a **half-done run the caller reads as `completed`** (observed
live on 2026-07-12 grooming #0065 — `docket-auto-groom` backgrounded its critic re-check and
yielded, and the parent then committed the still-live agent's uncommitted working-tree files).
The general **never-yield rule** now lives in `docket-convention`'s *Composition* paragraph
(a dispatched/forked parent actively blocks, and a caller never treats a bare `completed` as
proof nor adopts a child's uncommitted files), and is applied at `docket-auto-groom` §3's
re-check. See #0066.
