---
id: 535
slug: 'load-a-private-repository-s-agent-instructions-without-repos'
title: 'Load a private repository''s agent instructions without repository files'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-06'
updated: '2026-10-06'
depends_on: [531, 534]
stacked_on:
related: [532, 533, 334, 351]
discovered_from: []
adrs: [36, 78]
spec: 'docs/superpowers/specs/2026-10-06-load-a-private-repository-s-agent-instructions-without-repos-design.md'
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
| Spec | [2026-10-06-load-a-private-repository-s-agent-instructions-without-repos-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-06-load-a-private-repository-s-agent-instructions-without-repos-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0078](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md) |
<!-- docket:artifacts:end -->

## Why

Docket's dispatch and run-tracker rules reach the parent agent only through the repository's own CLAUDE.md or AGENTS.md. Those rules say: dispatch the named docket agent, and bracket every implement-next run with the run tracker. The user-level copy was retired because two copies drifted apart.

A private-visibility repository can't carry those files, so docket must not write them there. Without another delivery path, agents in a private repository would run docket workflows inline and untracked. Promoted lessons have the same problem: they normally graduate into the repository's AGENTS.md, which a restricted repository doesn't allow.

## What changes

- In private repositories, the dispatch block and promoted lessons live in a private instructions file under `.git/dckt/`. Docket writes no AGENTS.md, CLAUDE.md, or docket-named rule file in the worktree.
- A new read-only `docket instructions` command prints that file in a private repository and nothing anywhere else. `--hook` emits Claude Code's session-start hook shape.
- `docket install` sets up delivery once per machine. Neither user-level surface contains rule text or the word "docket":
  - Claude Code: a user-level `SessionStart` hook running `dckt instructions --hook`.
  - Codex and OpenCode: a static pointer block (markers `dckt:`) in their user-level AGENTS.md.
- Cursor, which reads rules only from the project, gets `.cursor/rules/dckt-dispatch.mdc`, excluded through `.git/info/exclude`.
- The first plan task checks each harness in a fresh session.

## Out of scope

- Shared repositories, whose repository-level dispatch blocks are unchanged.
- What ships through PRs and commits: writing rules and the leak check (#532).
- Installing the `dckt` alias itself (#534).
- Moving the instructions when switching modes (#533).
