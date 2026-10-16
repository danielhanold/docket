---
id: 556
slug: 'a-typed-docket-implement-next-in-claude-code-starts-an-untra'
title: 'A typed /docket-implement-next in Claude Code starts an untracked run'
status: 'in-progress'
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
plan: 'docs/superpowers/plans/2026-10-16-a-typed-docket-implement-next-in-claude-code-starts-an-untra.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/a-typed-docket-implement-next-in-claude-code-starts-an-untra'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-16T23:38:06Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-16-a-typed-docket-implement-next-in-claude-code-starts-an-untra-design.md](../../superpowers/specs/2026-10-16-a-typed-docket-implement-next-in-claude-code-starts-an-untra-design.md) |
| Plan | [2026-10-16-a-typed-docket-implement-next-in-claude-code-starts-an-untra.md](../../superpowers/plans/2026-10-16-a-typed-docket-implement-next-in-claude-code-starts-an-untra.md) |
| ADRs | [ADR-0024](../../adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0026](../../adrs/0026-fork-dispatch-opacity-two-invocation-paths.md), [ADR-0084](../../adrs/0084-re-dispatch-permission-gated-on-attribution-capability-not-launch-shape.md), [ADR-0133](../../adrs/0133-the-run-key-is-the-run-tracker-s-only-handle-gate-starts-car.md) |
<!-- docket:artifacts:end -->

## Why

Typing `/docket-implement-next` in Claude Code starts an implement run that the run tracker does not track. A live check on 2026-10-16 (Claude Code 2.1.301, throwaway repositories, one run each) showed:

- **Typed `/docket-implement-next 1`:** Claude Code hands the command straight to a new `docket-implement-next` agent, because the skill declares `context: fork` and `agent: docket-implement-next`. The main session never takes a turn, so it never runs `run.start`. The agent's prompt is the skill text plus `ARGUMENTS: 1`, with no run context, and the claim commit records an empty `gate_context_hash`.
- **`/loop docket-implement-next 1`:** the main session reads the prompt as ordinary text, runs `run.start`, and dispatches the agent with the run context. The claim records a context hash, so the run is tracked. Prose prompts ("implement 345") take the same path.

An untracked run still builds to a PR. But if it dies partway, docket cannot authorize a retry, resume a paused run, or stop it with `run.cancel`; the human recovers by hand. Change 0345 described this gap in a four-harness design that is now outdated; this change replaces it with the Claude Code part only.

## What changes

Document the tracked ways to start implement-next. This is a docs-only change: the typed command keeps working as it does today, stays visible, and no skill or agent instructions change.

- The maintained docs say to start implement-next with a plain request in the session or `/loop`, both of which the run tracker tracks. They say a typed `/docket-implement-next` in Claude Code goes straight to the implementer, so that run is untracked, with no automatic retry, resume, or cancel if it stops partway.
- That covers:
  - the install guide's invocation-paths table, whose "skill-invoke for everything else" steers implement-next onto the untracked path today;
  - the unattended-run guide;
  - the run-tracker concept page;
  - the README quick start;
  - any other maintained mention found by grep.
- Docs describe current behavior only, and existing doc sentinels stay green.

A note inside the implementer's report was designed and then dropped after a behavioral-risk review. Every implement run would read it, but it could only help a run that already finished. The linked spec records that analysis, the live-check evidence, and the other rejected alternatives.

## Out of scope

- Any change to skills, agent wrappers, dispatch blocks, harness adapters, the run tracker, or `change.claim`. That includes an untracked-run note in the skill and a run-tracker warning.
- Making a typed `/docket-implement-next` tracked, hiding it from the `/` menu, or changing its fork frontmatter.
- Codex, Cursor, and OpenCode launch mechanics.
- A child that receives the run context but drops it at claim time (change 0541).

## Reconcile log

### 2026-10-16

2026-10-16 — Re-read against current main (ffa75abfa) and the named docs: README quick-start drain line, building-without-supervision /loop section, models-and-effort invocation-paths table and run-tracker concept page still match the spec description. Spec groomed today; no scope change, no relation change.
