---
id: 334
slug: claude-md-global-and-project-copies-out-of-sync-on-finalize-a
title: Make Docket dispatch minimal, non-recursive, and mechanically gated
status: 'in-progress'
priority: high
type: fix
created: 2026-08-21
updated: '2026-08-26'
depends_on: []
stacked_on:
related: [294]
discovered_from: [317]
adrs: []
spec: docs/superpowers/specs/2026-08-25-consolidate-dispatch-block-subagent-guard-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'feat/claude-md-global-and-project-copies-out-of-sync-on-finalize-a'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-26T02:19:30Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-25-consolidate-dispatch-block-subagent-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-25-consolidate-dispatch-block-subagent-guard-design.md) |
<!-- docket:artifacts:end -->

## Why

The parent-facing dispatch rule can reach the named agent it just dispatched, causing that
agent to dispatch another instance of itself. Change 0317 observed a three-deep
`docket-status` tree before the actual status charter ran.

The same always-loaded block repeats every native agent description and asks the model to
execute the run gate's attribution and retry procedure by hand. A separate personal-global
Claude copy adds another independently drifting source. The result is recursive execution,
avoidable context weight, and safety policy spread across generated prose and scripts.

## What changes

- Inject an exact-name self-recursion guard through the shared wrapper generator while
  preserving every required dispatch to a different agent.
- Replace the generated 17-agent description roster with a compact rule that treats each
  harness's native agent registry as authoritative.
- Move foreground, detached, and unattributed `docket-implement-next` run-gate mechanics
  behind a durable `gate-before` / `gate-verdict` facade with opaque per-dispatch state and
  atomic one-retry accounting.
- Retire the hand-authored global Claude dispatch block at the human merge gate, leaving the
  generated per-repository surface authoritative for Claude, Codex, and OpenCode; Cursor
  keeps its distinct routing rule while consuming the same compact gate facade.
- Absorb change 0294's dispatch-roster and run-gate slimming scope into this change.

## Out of scope

- Changing `verify-run` predicates, run-gate safety policy, model/effort selections, or
  cross-agent composition.
- Unifying Cursor's routing surface with `AGENTS.md`/`CLAUDE.md`.
- Automating edits to the user's personal global `~/.claude/CLAUDE.md`; its removal is a
  human acceptance prerequisite.
- Pruning unrelated always-loaded instructions.

## Open questions

None. Detailed mechanics, failure postures, assumptions, and acceptance criteria are settled
in the linked spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
