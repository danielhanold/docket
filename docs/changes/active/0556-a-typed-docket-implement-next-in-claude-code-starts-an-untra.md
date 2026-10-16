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
related: [541, 503, 345]
discovered_from: [345]
adrs: [24, 26, 84, 133]
spec: 'docs/superpowers/specs/2026-10-16-a-typed-docket-implement-next-in-claude-code-starts-an-untra-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-10-16-a-typed-docket-implement-next-in-claude-code-starts-an-untra-design.md](../../superpowers/specs/2026-10-16-a-typed-docket-implement-next-in-claude-code-starts-an-untra-design.md) |
| ADRs | [ADR-0024](../../adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0026](../../adrs/0026-fork-dispatch-opacity-two-invocation-paths.md), [ADR-0084](../../adrs/0084-re-dispatch-permission-gated-on-attribution-capability-not-launch-shape.md), [ADR-0133](../../adrs/0133-the-run-key-is-the-run-tracker-s-only-handle-gate-starts-car.md) |
<!-- docket:artifacts:end -->

## Why

Typing `/docket-implement-next` in Claude Code starts an implement run that the run tracker does not track. A live check on 2026-10-16 (Claude Code 2.1.301, throwaway repositories, one run each) showed:

- **Typed `/docket-implement-next 1`:** Claude Code hands the command straight to a new `docket-implement-next` agent, because the skill declares `context: fork` and `agent: docket-implement-next`. The main session never takes a turn, so it never runs `run.start`. The agent's prompt is the skill text plus `ARGUMENTS: 1`, with no run context, and the claim commit records an empty `gate_context_hash`.
- **`/loop docket-implement-next 1`:** the main session reads the prompt as ordinary text, runs `run.start`, and dispatches the agent with the run context. The claim records a context hash, so the run is tracked. Prose prompts ("implement 345") take the same path.

An untracked run still builds to a PR. But if it dies partway, docket cannot authorize a retry, resume a paused run, or stop it with `run.cancel`; the human recovers by hand. Change 0345 described this gap in a four-harness design that is now outdated; this change replaces it with the Claude Code part only.

## What changes

Make an untracked implement run visible, and point people at the tracked ways to start one. The typed command keeps working as it does today and stays visible.

- The implementer's final report gains a short note when its dispatch prompt carried neither a run context nor a run key. The note says the run is untracked, so if it stops partway docket can't retry, resume, or cancel it. It says to start implement-next with a plain request or `/loop` to get a tracked run. It only informs: it never changes the disposition, stops the run, or touches the claim.
- The maintained docs say to start implement-next with a plain request or `/loop`, and that a typed `/docket-implement-next` in Claude Code runs untracked. That covers the unattended-run guide, the run-tracker concept, the invocation-paths table, the README quick start, and any other maintained mention found by grep.
- A prose-contract sentinel pins the note, and the skill's size ceiling moves by exactly the added words.

The linked spec holds the live-check evidence, the rejected alternatives, and the acceptance details.

## Out of scope

- Making a typed `/docket-implement-next` tracked, hiding it from the `/` menu, or changing its fork frontmatter.
- Codex, Cursor, and OpenCode launch mechanics.
- A child that receives the run context but drops it at claim time (change 0541).
- Changes to the run tracker, `change.claim`, the parent-facing dispatch blocks, or the agent wrappers.
- Recovery after the main session loses its run key.
