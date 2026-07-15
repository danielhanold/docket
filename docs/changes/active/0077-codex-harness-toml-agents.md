---
id: 77
slug: codex-harness-toml-agents
title: Codex harness — TOML agent generation + AGENTS.md dispatch block
status: proposed
priority: high
created: 2026-07-15
updated: 2026-07-15
depends_on: []
related: [45, 46, 51, 57]
adrs: [15]
spec: docs/superpowers/specs/2026-07-15-codex-harness-toml-agents-design.md
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
| Spec | [2026-07-15-codex-harness-toml-agents-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-15-codex-harness-toml-agents-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md) |
<!-- docket:artifacts:end -->

## Why

`codex` is already a valid `agent_harnesses` token and `link-skills.sh` already links
docket skills into `~/.codex/skills`, but the agent layer emits the same
markdown-frontmatter wrapper for every harness — and OpenAI Codex CLI reads standalone
**TOML** agent files (`.codex/agents/*.toml` with `name` / `description` /
`developer_instructions` / `model` / `model_reasoning_effort`) instead. A repo opting
into `agent_harnesses: [claude, codex]` today generates dead `.md` files Codex silently
ignores, and Codex has no analog of Cursor's dispatch rule, so a directly-invoked docket
skill would run inline at the session model, defeating the pin. Claude and Cursor now
have first-class subagent support; Codex is the confirmed gap.

## What changes

- A per-harness emitter registry in `sync-agents.sh`: `codex` → `.toml` +
  `emit_codex_toml()` (built-in wrapper → TOML field mapping; model/effort passthrough
  verbatim per ADR-0015), every other harness → the existing markdown emitter unchanged.
- A managed, marker-bounded dispatch block in the repo's `AGENTS.md` (committed;
  machine-neutral content) telling Codex to delegate directly-invoked docket skills to
  the matching pinned agent — reusing the hardened managed-block pattern from the
  .gitignore lib.
- Housekeeping: the managed .gitignore block gains `.codex/agents/docket-*.toml`;
  orphan pruning and both `--check` legs extend to TOML wrappers and the AGENTS.md
  block; new `tests/` cases (TOML validity, field mapping, byte-identical non-codex
  regression, block create/idempotence/prune).

## Out of scope

- Live Codex CLI verification (change 0078, which depends on this).
- User-level `~/.codex/AGENTS.md` dispatch instructions (pending 0078 findings).
- Extra TOML fields (`sandbox_mode`, `mcp_servers`, `skills.config`,
  `nickname_candidates`) and any `link-skills.sh` changes.

## Open questions

- Exact TOML field names/paths were taken from one fetch of the Codex subagent docs —
  re-verify against the live documentation at plan time before coding.

## Reconcile log
