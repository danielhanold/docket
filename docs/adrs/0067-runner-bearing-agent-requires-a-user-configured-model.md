---
id: 67
slug: runner-bearing-agent-requires-a-user-configured-model
title: A runner-bearing agent must carry a user-configured model, runner-wide
status: Accepted
date: 2026-08-04
supersedes: []
reverses: []
relates_to: [15, 37, 38]
change: 205
---

## Context

Change 0079 built cross-harness runner delegation, and change 0168 established that only a
*user-configured* value is ever baked into a child-runner flag: a shipped
`agents/harness-defaults.yml` entry is a default **for this harness**, so forwarding it could send a
Claude model ID to a Codex process.

The consequence was a hole. A `runner:`-bearing agent with no model configured anywhere simply
emitted no model flag, and the child process fell through to its **own** default model.
`scripts/runners/codex.md` documented this as a supported shape — "Omitted ⇒ the child's own default
model."

Change 0205 adds the `opencode` runner, which reaches models through OpenRouter. There the child's
own default is pay-per-token and of unknown identity, so the failure is silent in the run and
surfaces on the bill instead. The framework's existing posture (ADR-0037) is that explicit config is
never silently ignored and never silently degraded; a model-less `runner:` is the mirror case —
absent config silently resolving to something nobody chose.

## Decision

A `runner:`-bearing docket agent **must** carry a user-configured model, or wrapper generation fails
loudly. The rule is **runner-wide** — codex, cursor, opencode alike — not per-adapter.

In `sync-agents.sh`'s `emit_wrapper`, after the registration check and after change-0168's
provenance flags are computed, an empty `flag_model` logs an ERROR naming the agent and the runner
and exits nonzero. The literal `inherit` — docket's own no-pin sentinel, which every adapter
normalizes to "no flag" — is rejected on the same leg; without that it would be a one-word bypass of
the guard.

The error is raised at **generation** time, where the config was just written and the person who
wrote it is present, rather than mid-dispatch.

Runner-wide rather than opencode-only because "is a model required?" should not be an
adapter-by-adapter fact a user has to learn twice. For a subscription-billed child the guard is
milder in value, but it costs nothing there.

## Consequences

- This **reverses the documented posture** in `scripts/runners/codex.md` and
  `scripts/runners/cursor.md` that an omitted model means the child's own default. It is a breaking
  change for any existing model-less codex or cursor configuration.
- Docket cannot reach or migrate the config layers that may carry such a configuration: a
  machine-local `.docket.local.yml` or the global `~/.config/docket/config.yml` sit outside the reach
  of any repo-committed migration. Affected users get a generation-time error and must add a
  `model:`. That is the accepted cost of the loud failure.
- The rule is loud but **not fail-before-write**. `emit_wrapper`'s call sites redirect into the
  target wrapper path, so the offending agent is left with a zero-length wrapper, and agents later in
  glob order are not regenerated until the config is fixed. This was a deliberate trade: making it
  fail-before-write would require resolving every (harness, agent) pair twice.
- Model resolution for runner-bearing agents becomes a single, uniform rule across adapters, so a new
  adapter inherits the guard rather than re-deciding it.
