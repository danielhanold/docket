---
id: 78
slug: codex-cli-validation-runbook
title: Codex CLI live-validation runbook — prove docket works end-to-end under Codex
status: proposed
priority: high
created: 2026-07-15
updated: 2026-07-15
depends_on: [77]
related: [45]
adrs: []
spec: docs/superpowers/specs/2026-07-15-codex-cli-validation-runbook-design.md
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
| Spec | [2026-07-15-codex-cli-validation-runbook-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-15-codex-cli-validation-runbook-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0077 makes docket emit what Codex documents it reads, but nobody has confirmed
the loop live: skills loading in Codex CLI, docket's bash scripts running under its
sandbox, the TOML agents being listed, skill→subagent dispatch via the AGENTS.md block,
the model/effort pin holding, and metadata writes landing on `origin/docket`. Both
Cursor rollout verifications surfaced real gaps; Codex needs the same evidence before
docket can claim first-class support for Claude, Cursor, AND Codex.

## What changes

A six-phase guided checklist Daniel executes interactively in Codex CLI against a
fixture repo — (1) setup + generated-artifact assertions, (2) skills load + script/
sandbox smoke test, (3) agents listed, (4) dispatch honored, (5) model/effort pin
honored, (6) end-to-end trivial `docket-new-change` with a must-land board pass. Exact
commands/prompts and expected observable outcomes per step; findings land in a results
doc; every gap becomes a follow-up stub. Passes when phases 1–3 and 6 are green and
4–5 have definitive observed answers.

## Out of scope

- Fixing what the runbook finds (follow-up changes).
- `codex exec` / CI automation of the runbook.
- Other unvalidated harness tokens (kiro, windsurf) and autonomous-loop soak testing.

## Open questions

- None beyond the spec's noted doc-drift risk (re-check live Codex docs when finalizing
  exact commands).

## Reconcile log
