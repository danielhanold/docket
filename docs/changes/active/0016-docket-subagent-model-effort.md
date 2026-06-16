---
id: 16
slug: docket-subagent-model-effort
title: docket skills as model/effort-pinned subagents — foundation
status: implemented
priority: medium
created: 2026-06-15
updated: 2026-06-16
depends_on: []
related: [15, 17]
adrs: [8]
spec: docs/superpowers/specs/2026-06-15-docket-subagent-model-effort-design.md
plan: docs/superpowers/plans/2026-06-16-docket-subagent-model-effort.md
results:
trivial: false
auto_groomable:
branch: feat/docket-subagent-model-effort
pr: https://github.com/danielhanold/docket/pull/30
blocked_by:
reconciled: true
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

- ~~Exact harness agent-dir list `sync-agents.sh` writes into~~ — **resolved
  (reconcile 2026-06-16):** mirror `link-skills.sh`'s `HARNESS_SKILL_DIRS`,
  swapping `skills` → `agents`; write only into harness roots that already exist;
  per-repo overrides target `<repo>/.claude/agents/` (the only project-over-user
  harness).
- Whether the agent-layer convention warrants its own ADR (decide at build).

## Reconcile log

### 2026-06-16 — reconcile before build (implement-next)

Verified the spec against `origin/main` (`e5ca467`); the design holds and nothing
was built elsewhere. Notes:

- **Code assumptions confirmed.** `link-skills.sh` exists with `HARNESS_SKILL_DIRS`
  (6 entries: `.claude`/`.codex`/`.cursor`/`.agents`/`.kiro`/`.windsurf` `+/skills`)
  and the `DOCKET_HARNESS_ROOT` test seam; `tests/test_link_skills.sh` exercises
  that seam exactly as §9 describes; all 5 autonomous + 2 interactive skills exist
  under `skills/`; there is **no `agents/` dir** and **no `agents:` block** in
  `.docket.yml` yet — clean slate.
- **Open question #1 resolved (harness agent-dir list).** Mirror `link-skills.sh`'s
  `HARNESS_SKILL_DIRS` swapping `skills`→`agents`: `~/.claude/agents`,
  `~/.codex/agents`, `~/.cursor/agents`, `~/.agents/agents`, `~/.kiro/agents`,
  `~/.windsurf/agents` — write only into harness roots that already exist. The
  per-repo override pass targets `<repo>/.claude/agents/` specifically (the only
  harness with project-over-user precedence).
- **Scope tightened: 5 wrapper files in 0016.** One wrapper per autonomous skill
  (`docket-implement-next`, `docket-auto-groom`, `docket-finalize-change`,
  `docket-status`, `docket-adr`). The §4 table's separate auto-groom *critic* row
  is **not** a distinct file in 0016 — 0016 never spawns it; per spec §10 the
  critic-file-vs-variant question is settled in 0017's rewiring.
- **Related/archived: no conflict.** 0015 (proposed) consumes the abort-and-report
  + fan-out patterns later; 0017 (proposed, `depends_on: 16`) is the explicit
  composition follow-on, out of scope here. Most-recent archive 0011 (github
  mirror) is orthogonal — the `agents:` addition to `.docket.yml`/convention is
  purely additive alongside `board_surfaces`.
- **ADR (open question #2) deferred to build/review** per the spec — likely
  warranted for the separate-generator and committed-project-override-for-
  reproducibility decisions; recorded via `docket-adr` at step 6 if so.

### 2026-06-16 — post-review addition (install.sh)

Adding `sync-agents.sh` made machine install a two-script step (it was one,
`link-skills.sh`, before this change) — a DX regression flagged in review. Added an
umbrella **`install.sh`** that runs both primitives in order (idempotent), plus
`tests/test_install.sh` and a rewritten README Install section leading with the
one command. `migrate-to-docket.sh` stays out (per-repo migration, not machine
setup). The two primitives remain callable directly. Pushed to PR #30.
