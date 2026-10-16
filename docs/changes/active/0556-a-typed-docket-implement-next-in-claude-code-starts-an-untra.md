---
id: 556
slug: 'a-typed-docket-implement-next-in-claude-code-starts-an-untra'
title: 'A typed /docket-implement-next in Claude Code starts an untracked run'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-16'
updated: '2026-10-16'
depends_on: []
stacked_on:
related: [541, 503]
discovered_from: [345]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Typing `/docket-implement-next` in Claude Code starts an implement run that the run tracker does not track. A live check on 2026-10-16 (Claude Code 2.1.301, throwaway repositories, one run each) showed:

- **Typed `/docket-implement-next 1`:** Claude Code hands the command straight to a new `docket-implement-next` agent, because the skill declares `context: fork` and `agent: docket-implement-next`. The main session never takes a turn, so it never runs `run.start`. The agent's prompt is the skill text plus `ARGUMENTS: 1`, with no run context, and the claim commit records an empty `gate_context_hash`.
- **`/loop docket-implement-next 1`:** the main session reads the prompt as ordinary text, runs `run.start`, and dispatches the agent with the run context. The claim records a context hash, so the run is tracked. Prose prompts ("implement 345") take the same path.

An untracked run still builds to a PR. But if it dies partway, docket cannot authorize a retry, resume a paused run, or stop it with `run.cancel`; the human recovers by hand. Change 0345 described this gap in a four-harness design that is now outdated; this change replaces it with the Claude Code part only.

## What changes

Make a typed `/docket-implement-next` in Claude Code either start a tracked run or say clearly that it did not. Directions to weigh at grooming (hypotheses, not decisions):

1. Let the typed command reach the main session first: a short entry that runs `run.start`, dispatches the agent with the run context, and runs `run.verdict` afterwards. This means dropping the automatic fork for this skill and keeping the long worker procedure out of the main session's context.
2. Keep the fork, and have an untracked run say so plainly in its report (visibility only, no new stop).
3. Document prose and `/loop` as the ways to start implement-next, and leave the typed command untracked.

Reuse the existing run tracker: no new token format, retry rule, or attribution mechanism. Prove the result with a live typed-command run in a fresh Claude Code session, checking the claim commit's context hash.

## Out of scope

- Codex, Cursor, and OpenCode. Cursor has no skill fork; Codex is unsupported; OpenCode ships untested.
- A child that receives the run context but drops it at claim time (change 0541).
- Prose and `/loop` starts, which are already tracked.
- Recovery after the main session loses its run key.
