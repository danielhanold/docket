# Change 0402 — docs coverage table

This is the completeness audit for the three-tier docs restructure (change 0402). Every heading of
the pre-change `docs/guide/README.md` body (read from the merge-base
`git merge-base HEAD origin/main`) and every file that lived under `docs/cursor/`, `docs/codex/`
(fixtures tree included), and `docs/opencode/` gets exactly one row, naming the page and section
that now carries its content. A row says *dropped* only where the content described the retired
`scripts/docket.sh` facade, corrected under change 385.

## Source-body headings (`docs/guide/README.md`)

| Source heading/file | Now carried by (page § section) | Notes |
|---|---|---|
| `## Table of contents` | `docs/README.md`, and the three tier indexes `docs/guide/README.md`, `docs/concepts/README.md`, `docs/reference/README.md` | The single flat TOC is replaced by the docs entry point plus one index per tier. |
| `## How it works` | `docs/README.md` (three-tier framing); `docs/concepts/change-lifecycle.md` § The moving parts | The high-level overview becomes the docs-index framing and the change-lifecycle concept page. |
| `### The change lifecycle` | `docs/guide/capturing-work.md` § How a change moves, and the board; `docs/concepts/change-lifecycle.md` | State list and board as a user meets them (guide); the state machine and its ADRs (concepts). |
| `## Why docket` | `docs/guide/building-without-supervision.md` § Why hand work to the loop at all | |
| `### The reconcile superpower` | `docs/guide/building-without-supervision.md` § Reconcile: killing stale work before any code; `docs/concepts/reconcile.md` | How-to framing on the guide page; the mechanism and its comparison-brief row on the concept page. |
| `## Install` | `docs/guide/running-on-your-harness.md` § Install docket | Install/update lead the harness page. |
| `### Prerequisites` | `docs/guide/running-on-your-harness.md` § What you need first | |
| `### 1. Install docket on your machine` | `docs/guide/running-on-your-harness.md` § 1. Install docket on your machine | |
| `### 2. Set up your global config` | `docs/guide/running-on-your-harness.md` § 2. Set up your global config | |
| `## Updating docket` | `docs/guide/running-on-your-harness.md` § Keep docket current | |
| `## Quickstart: the daily loop` | `docs/guide/daily-loop.md` | The one-screen day's cycle; links out to the page that owns each step. |
| `### Draining hands-free with `/loop`` | `docs/guide/building-without-supervision.md` § Draining the queue hands-free | |
| `### Closing out hands-free with `/loop`` | `docs/guide/landing-changes.md` § Closing out hands-free with `/loop` | |
| `## Configuration — `.docket.yml`, global config, and machine-local overrides` | `docs/guide/governing-through-configuration.md` (whole page); `docs/concepts/config-layers.md` | The config chapter is the whole governing-through-configuration page; the layer model is a concept page. |
| `### `.docket.yml` — per-repo settings` | `docs/guide/governing-through-configuration.md` § The per-repo file: `.docket.yml` | Points at `.docket.example.yml` for exact shape rather than copying it. |
| `### Reclaiming stale claims (`reclaim`)` | `docs/guide/keeping-the-backlog-honest.md` § Reclaiming stale claims | |
| `### Capturing discovered work (`auto_capture`) and typing it (`change_types`)` | `docs/guide/capturing-work.md` § Capturing an idea: designed, rough, or discovered; § Typing your work; § Where discovered work lands | Carries the guarded phrase `untyped set can only shrink`. |
| `#### The taxonomy (`change_types`)` | `docs/guide/capturing-work.md` § Typing your work → The taxonomy (`change_types`) | |
| `#### Migrating to typed changes` | `docs/guide/capturing-work.md` § Migrating to typed changes | |
| `### Speaking your language (`dummy_mode`)` | `docs/guide/designing-before-building.md` § Shaping the conversation to your reader (`dummy_mode`) | |
| `#### Persona gallery` | `docs/guide/designing-before-building.md` § A gallery of personas | |
| `### Workflow roles — the `skills:` map` | `docs/guide/governing-through-configuration.md` § Workflow roles — the `skills:` map | Names the `skills:` map generally; the guarded example `brainstorm: docket-brainstorm` is carried separately on `designing-before-building.md`. |
| `### Global config — `~/.config/docket/config.yml`` | `docs/guide/governing-through-configuration.md` § Cross-repo defaults: global config | |
| `### `.docket.local.yml` — the machine-local layer` | `docs/guide/governing-through-configuration.md` § Machine-local overrides: `.docket.local.yml` | |
| `### Coordination keys are per-repo-only` | `docs/guide/governing-through-configuration.md` § The coordination fence; `docs/concepts/config-layers.md` | The fence rule (guide) and the why (concept). |
| `### When a file is misplaced or malformed` | `docs/guide/governing-through-configuration.md` § When a config file is misplaced or malformed | |
| `### Migrating from `agents.yaml`` | `docs/guide/governing-through-configuration.md` § Migrating from `agents.yaml` | |
| `## docket-mode: where metadata lives` | `docs/guide/where-the-metadata-lives.md` (whole page); `docs/concepts/two-branches.md` | The metadata chapter is the whole page; the two-branch model is also a concept page. |
| `### The two-branch model` | `docs/guide/where-the-metadata-lives.md` § The two-branch model; `docs/concepts/two-branches.md` | |
| `### Where each artifact lives` | `docs/guide/where-the-metadata-lives.md` § Where each artifact lives | |
| `### `integration_branch` and GitFlow` | `docs/guide/where-the-metadata-lives.md` § `integration_branch` and GitFlow | |
| `### The `.docket/` metadata worktree` | `docs/guide/where-the-metadata-lives.md` § The `.docket/` metadata worktree | |
| `### Finalize → selective publish` | `docs/guide/landing-changes.md` § Selective publish on close-out | Moved to the landing-changes page (finalize owns selective publish). |
| `### Publishing terminal records to the integration branch (`terminal_publish`, opt-in)` | `docs/guide/where-the-metadata-lives.md` § Publishing terminal records to your code branch (`terminal_publish`, opt-in) | |
| `### `main`-mode: the single-branch opt-out` | `docs/guide/where-the-metadata-lives.md` § Single-branch mode: the opt-out | |
| `### git-hook frameworks (pre-commit, husky, lefthook)` | `docs/guide/where-the-metadata-lives.md` § git-hook frameworks (pre-commit, husky, lefthook) | |
| `## Tuning agent models & effort` | `docs/guide/running-on-your-harness.md` § Tuning models and effort per task; § Claude Code (fork/dispatch mechanics) | Carries the guarded phrases `Fork-exclusion principle` and `completed (forked execution)`. |
| `## Skills` | `docs/reference/skills-and-agents.md` § Skills (inventory); `docs/guide/remembering-why.md` § Architecture decisions (ADRs) (the `docket-adr` portion) | Guarded phrase `## Skills` now lives on the reference inventory page (also asserts absence of `#the-eight-skills`). |
| `## Learnings — the loop's memory` | `docs/guide/remembering-why.md` § The learnings ledger; `docs/concepts/memory.md` | |
| `## Customization` | Umbrella heading — its subsections are distributed to the pages below | The parent heading carries no standalone prose; each subsection rehomed individually. |
| `### Consultant-authored brainstorm (opt-in)` | `docs/guide/designing-before-building.md` § Having a consultant write the spec | Carries the guarded phrase `brainstorm: docket-brainstorm`. |
| `### docket-build — the lean, profile-routed build` | `docs/guide/building-without-supervision.md` § Build profiles and the one escalation (routing/escalation half); `docs/guide/proving-the-build.md` § The build gate (gate half); `docs/concepts/build-profiles-and-gate.md` | The one source section splits routing (building-without-supervision) from the gate (proving-the-build), per the plan. |
| `### docket-review — the bounded whole-branch reviewer` | `docs/guide/reviewing-before-the-human.md` (whole page) | |
| `### Runner delegation — running docket agents on another harness` | `docs/guide/running-on-your-harness.md` § Delegating an agent to another harness | |
| `### Running under Cursor Auto-run` | `docs/guide/running-on-your-harness.md` § Cursor | Also draws on the moved `docs/cursor/permissions.md` prose; see the file rows below. |
| `### Hands-off finalize — what blocks it, and the recipe that works` | `docs/guide/landing-changes.md` § When finalize is blocked; § The prerequisite: branch protection that permits an unattended merge | Carries the guarded phrase `auto-mode classifier`. |
| `## Status` | `docs/guide/keeping-the-backlog-honest.md` § Status versus the terminal sweep; § The health checks and what to do about each | |
| `## Migration` | `docs/guide/where-the-metadata-lives.md` § Migrating an existing repo onto docket | |
| `### Migrating an existing repo to docket-mode` | `docs/guide/where-the-metadata-lives.md` § Moving a single-branch repo to the two-branch layout | |
| `### Migrating a pre-0051 repo` | `docs/guide/where-the-metadata-lives.md` § Carrying a pre-0051 repo forward | |

## Harness files (`docs/cursor/`, `docs/codex/`, `docs/opencode/`)

| Source heading/file | Now carried by (page § section) | Notes |
|---|---|---|
| `docs/cursor/validation.md` | `docs/reference/harness/validation.md` | `git mv`'d in Task 7; self-reference and `permissions.md` links rewritten; sentinel `test_cursor_contract_docs` repointed. |
| `docs/cursor/permissions.example.json` | `docs/reference/harness/permissions.example.json` | `git mv`'d in Task 7; linked from the harness guide page and reference index. |
| `docs/cursor/sandbox.example.json` | `docs/reference/harness/sandbox.example.json` | `git mv`'d in Task 7. |
| `docs/cursor/permissions.md` | `docs/guide/running-on-your-harness.md` § Cursor (surviving prose); **partly dropped** | Prose absorbed into the guide's Cursor section. The `## Trust tiers — docket's shell surface, classified` taxonomy and the facade-specific troubleshooting items (`$DOCKET_SCRIPTS_DIR`/`docket.sh` variable-expansion cases: the `:?}` short-spelling and compound-command-demotion entries) are **dropped as stale** — they described the retired `scripts/docket.sh` facade, corrected under change 385 (the binary is now on `PATH`). The three-gates model, the run-outside-the-sandbox story, the invalid-JSON and allowlist-a-helper cautions survive, re-voiced for the native binary. |
| `docs/codex/setup.md` | `docs/guide/running-on-your-harness.md` § Codex | Prose absorbed in Task 6 (wrapper generation, the committed AGENTS.md dispatch block, both entry paths, restart-after-regeneration); file deleted in Task 7. |
| `docs/codex/validation-runbook.md` | `docs/reference/harness/validation-runbook.md` | `git mv`'d in Task 7; `docs/codex/setup.md` and `docs/cursor/permissions.md` references rewritten to the harness guide page; fixtures references repointed; sentinel `test_codex_runbook` repointed. |
| `docs/codex/fixtures/nested-launch/README.md` | `docs/reference/harness/fixtures/nested-launch/README.md` | `git mv`'d with the fixtures tree in Task 7; relative fixtures link verified to still resolve. |
| `docs/codex/fixtures/nested-launch/certification.md` | `docs/reference/harness/fixtures/nested-launch/certification.md` | `git mv`'d with the fixtures tree. |
| `docs/codex/fixtures/nested-launch/decision.md` | `docs/reference/harness/fixtures/nested-launch/decision.md` | `git mv`'d with the fixtures tree. |
| `docs/codex/fixtures/nested-launch/probe-coordinator.toml` | `docs/reference/harness/fixtures/nested-launch/probe-coordinator.toml` | `git mv`'d with the fixtures tree. |
| `docs/codex/fixtures/nested-launch/probe-leaf.toml` | `docs/reference/harness/fixtures/nested-launch/probe-leaf.toml` | `git mv`'d with the fixtures tree. |
| `docs/codex/fixtures/nested-launch/probe-log.md` | `docs/reference/harness/fixtures/nested-launch/probe-log.md` | `git mv`'d with the fixtures tree. |
| `docs/opencode/setup.md` | `docs/guide/running-on-your-harness.md` § opencode | Prose absorbed in Task 6 (same treatment as Codex); file deleted in Task 7. |

## Glossary extensions

Terms added beyond the spec's glossary table:

| Term | Gloss | Added by |
|---|---|---|
| worktree | an isolated working copy of the repo, on its own branch | Task 2 (`docs/guide/building-without-supervision.md`, in the Plan step of the end-to-end run) |

No other terms were added beyond the spec's approved glosses.
