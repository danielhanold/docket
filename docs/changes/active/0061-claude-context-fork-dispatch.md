---
id: 61
slug: claude-context-fork-dispatch
title: Claude Code skill-invocation parity — context:fork dispatch to pinned wrappers
status: proposed
priority: medium
created: 2026-07-11
updated: 2026-07-11
depends_on: []
related: [16, 45, 46, 48]
adrs: [8]
spec: docs/superpowers/specs/2026-07-11-claude-context-fork-dispatch-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-11-claude-context-fork-dispatch-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-11-claude-context-fork-dispatch-design.md) |
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md) |
<!-- docket:artifacts:end -->

## Why

docket pins each autonomous skill's `model`/`effort` via a generated subagent wrapper. That pin only applies when the skill runs as a **dispatched** subagent — not when it is **invoked directly**. `sync-agents.sh` assumes "only cursor exhibits the inline quirk," but Claude Code has the same quirk: `/docket-status` (or a model-auto-invoked skill) runs **inline at the session model**, so a session on the wrong tier silently defeats the pin — `docket-status` meant for `haiku` runs at `opus`, or `docket-implement-next` meant for `opus/xhigh` runs at whatever the session happens to be. Cursor already fixes this with a generated `alwaysApply` dispatch rule; Claude Code has been left exposed on a false assumption.

## What changes

Add the native Claude Code `context: fork` + `agent: docket-<name>` frontmatter to the committed `SKILL.md` of the four **headless-safe** autonomous skills — `docket-status`, `docket-adr`, `docket-implement-next`, `docket-auto-groom` — so a direct invocation forks into the existing pinned wrapper and runs at its `model`/`effort`. Edit-once in the shared symlinked skill source; inert in Cursor/Codex/other harnesses (unknown frontmatter is ignored, and Cursor keeps its own dispatch rule). No new generation, hooks, or CLAUDE.md routing.

Correct the stale "only cursor" comment in `sync-agents.sh` and the README harness/dispatch docs to the real **two-mechanism** story (Cursor dispatch rule vs. Claude `context: fork`), and document the **fork-exclusion principle**: fork only skills that never need the human mid-run, because a forked subagent cannot reach the human (Claude Code excludes `AskUserQuestion`/`EnterPlanMode`/etc. from subagents). Add a test asserting the four forked skills carry the frontmatter and the three interactive/excluded ones do not.

Full design, selection table, composition (no-recursion) argument, and plan-time verifications are in the linked spec.

## Out of scope

- Forking `docket-finalize-change` / autonomous merge — blocked on Claude Code's auto-mode "Merge Without Review" classifier; tracked as change 0062.
- `docket-new-change` and `docket-groom-next` — interactive brainstorm skills; must stay inline with the human.
- Dispatch-only helper agents (`docket-integration-repair`, `docket-rebase-resolver`, `docket-auto-groom-critic`, `docket-brainstorm-consultant`) — no `SKILL.md`, invoked only via `Task`, never subject to the inline quirk.
- Consolidating/replacing the wrapper + Cursor-rule machinery with `context: fork` (considered and declined in favor of the minimal parity fix).

## Open questions

- Should the fork invariant be enforced by `sync-agents.sh --check` (advisory), so a newly-added autonomous skill isn't silently left un-forked? (Leaning yes — mirrors the Cursor dispatch-fragment warning.)
- Confirm empirically that `context: fork` + the wrapper's `skills:` preload compose without double-running, and that nested forks (implement-next invoking adr/status) degrade to inline-within-the-pinned-subagent rather than breaking.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
