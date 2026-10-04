---
id: 464
slug: 'align-guide-install-docs-and-docket-example-yml-with-the-go'
title: 'Align the human-facing docs and example config with the docket binary'
status: 'proposed'
priority: 'medium'
type: 'docs'
created: '2026-09-27'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [363, 371, 502, 366, 409]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-04-align-guide-install-docs-and-docket-example-yml-with-the-go-design.md'
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
| Spec | [2026-10-04-align-guide-install-docs-and-docket-example-yml-with-the-go-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-align-guide-install-docs-and-docket-example-yml-with-the-go-design.md) |
<!-- docket:artifacts:end -->

## Why

docket's human-facing documentation still describes how docket worked before the Go binary. Following it produces config the binary refuses and commands that don't exist. Copying `.docket.example.yml` verbatim as `.docket.yml` blocks every repository write today, because it spells out five `skills:` values and any explicit `skills:` value blocks writes.

A read-only audit at grooming (2026-10-04) found the drift far wider than this stub's original list. It spans every area: the README, the guide, install, concepts, reference (including the glossary and the harness runbooks), and the example config.

- **Whole pages or sections teach removed or refused features:**
  - delegation across harnesses
  - workflow-skill rebinding
  - single-branch mode
  - copying archived records to main
  - dummy mode
  - auto-capture
  - per-repository agent files
  - the glossary's obsolete-terms section
- **Nonexistent things are named throughout:** `docket status` sweeping and reclaiming, stale-claim and stalled-dependency checks, `docket board-refresh`, `sync-agents.sh`, and more.
- **Change numbers are cited everywhere**, and nothing guards the docs, so the drift accumulated silently.
- **"The daily loop" is misleading.** Nothing is daily, and "loop" means five different things across the pages.

The skills and agent files have the same problem, and fixing them changes what agents actually do. That work was split into #502, which builds after this change.

## What changes

Bring every living human-facing doc in line with the binary as it is today. The linked spec carries the rules, the verified facts, and a page-by-page worklist. The scope is `README.md`, `docs/README.md`, `docs/guide/`, `docs/install/`, `docs/concepts/`, `docs/reference/` (glossary and harness pages included), and the acceptance checklist in `docs/release/`.

- **Rules:** describe only current behaviour, with the code as the oracle.
  - Removed or refused settings, features, commands, and old names are deleted outright. There are no tombstones and no "not supported" notes.
  - No page cites an individual change or PR.
  - ADR citations stay where the ADR's decision is still current; concept-page "Decided in" bullets for removed behaviour go.
  - Point-in-time records are untouched.
- **Pages deleted:**
  - `docs/install/delegating-across-harnesses.md`
  - `docs/install/workflow-roles.md` (its default-roles table moves to the skills-and-agents reference)
  - the Codex validation runbook and its nested-launch fixture
  - the stale Cursor permission and sandbox example files
- **Pages rewritten** around today's behaviour: keeping the backlog honest, where the metadata lives, config layers, models and effort, the per-harness install pages, the finalize and run-tracker concepts, learnings, the config-key and outcomes references, and the glossary.
- **Terminology:**
  - "The daily loop" becomes **"the five steps"**: the page is renamed `five-steps.md`, titled "Using docket: the five steps".
  - "Loop" is reserved for `/loop`. The builder is "implement-next" or "a build run", "fix loop" becomes "fix pass", and "build loop" becomes "builds".
- **`.docket.example.yml`** lists only working keys and is safe to copy verbatim. Its test flips: every supported key must appear, no unsupported key may appear (derived from the schema), and a verbatim copy passes the write check.
- **Other cleanup:**
  - Fix the `agents/harness-defaults.yml` comments, cutting a new versioned fixture to match.
  - Delete the dead `scripts/runners/` files.
  - Correct `docket run`'s "(read-only)" help text.
- **A new repo test** over the living doc roots fails on change or PR citations, and on any config key the schema marks obsolete, inert, or refused. The key list is read from the schema, and the test is mutation-tested.
- Every test that pins wording on these files is updated in the same commit as the text it guards.

## Out of scope

- The skills, their references and templates, the agent wrappers, and `AGENTS.md` — that is #502, which depends on this change.
- Changing what the binary does, including removing the obsolete, inert, or refused rows from the config schema. This change aligns the prose to the code, not the reverse.
- Point-in-time records: archived changes, results, ADRs, specs and plans, `docs/comparison/`, `docs/codex/fixtures/`.
- Rewriting the Codex validation runbook. It is deleted; a fresh one is written against the real commands when Codex work resumes.
- Catching refused *values* of supported keys (for example `finalize.gate: ci`) in the new guard — left to review.
- A guard that keeps doc command snippets in sync with the capability catalog.
