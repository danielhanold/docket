---
id: 16
slug: docket-subagent-model-effort
title: docket skills as model/effort-pinned subagents — foundation
status: in-progress
priority: medium
created: 2026-06-15
updated: 2026-06-16
depends_on: []
related: [15, 17]
adrs: []
spec: docs/superpowers/specs/2026-06-15-docket-subagent-model-effort-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/docket-subagent-model-effort
pr:
blocked_by:
reconciled: false
---

## Why

Every docket skill runs inline in the main conversation at whatever model and
effort the session is on. The skills differ enormously in blast radius — the two
fully autonomous, no-backstop skills (`implement-next`, `auto-groom`) versus
near-mechanical board rendering — yet all inherit one undifferentiated session
model. We want per-skill control of **model** and **effort**, configurable
without editing skill bodies, with sane built-in defaults driven by autonomy and
git-state-corruption risk.

Harness facts were verified before designing: subagent frontmatter natively
supports both `model` and `effort`; nested subagents work; project-level
`.claude/agents/` overrides user-level and is committable; `skills:` injects skill
content; frontmatter is static (so configurability needs a generator). See the
spec for the full rationale and the model/effort table.

## What changes

This is the **foundation** half (0017 does the composition wiring):

- **Subagent wrappers** for the 5 autonomous skills (`implement-next`,
  `auto-groom`, `finalize-change`, `status`, `adr`): thin `agents/docket-*.md`
  files pinning model+effort and loading the existing skill via `skills:` — the
  skill body stays the single source.
- **Advisory model/effort** for the 2 interactive skills (`new-change`,
  `groom-next`): they stay inline skills (they brainstorm with the human and
  can't be fire-and-forget), surfacing a recommended model/effort at startup.
- **Layered config** — built-in defaults (the table) ⊕ global
  `~/.config/docket/agents.yaml` ⊕ per-repo `.docket.yml` `agents:` block;
  precedence per-repo > global > built-in. Per-repo overrides generate
  *committed* project-level agent files, preserving the clone-identical
  reproducibility guarantee.
- **`sync-agents.sh`** — a new idempotent generator (separate from
  `link-skills.sh`) that resolves config and writes the agent files, with a
  `--check` CI mode that fails on drift.
- **`docket-convention`** doc update — the contract gains the `agents:` block,
  the agent layer + precedence, the `auto` ⇒ omit-effort rule, and the
  abort-and-report semantics for autonomous subagents.

After this change, standalone invocation of each autonomous skill runs at its
pinned model/effort.

## Out of scope

- Rewiring sub-invocations to nested subagents — that is **0017**. Until it
  lands, `implement-next`'s status/adr calls and `auto-groom`'s critic run inline
  at the parent's model.
- The TDD build's model — that is `superpowers:subagent-driven-development`'s own
  config, where most token spend lands. Pinning `implement-next` to Opus governs
  its reconcile/escalation, not the build.
- Forcing the session model for the interactive skills (not possible from a
  skill; advisory only).

## Open questions

- Exact harness agent-dir list `sync-agents.sh` writes into (mirror
  `link-skills.sh`'s `HARNESS_SKILL_DIRS`, swapping `skills` → `agents`).
- Whether the agent-layer convention warrants its own ADR (decide at build).

## Reconcile log
