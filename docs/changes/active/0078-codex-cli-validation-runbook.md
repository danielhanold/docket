---
id: 78
slug: codex-cli-validation-runbook
title: Codex CLI live-validation runbook — prove docket works end-to-end under Codex
status: implemented
priority: high
created: 2026-07-15
updated: 2026-07-16
depends_on: [77]
related: [45]
adrs: []
spec: docs/superpowers/specs/2026-07-15-codex-cli-validation-runbook-design.md
plan: docs/superpowers/plans/2026-07-16-codex-cli-validation-runbook.md
results: docs/results/2026-07-16-codex-cli-validation-runbook-results.md
trivial: false
auto_groomable:
branch: feat/codex-cli-validation-runbook
pr: https://github.com/danielhanold/docket/pull/89
blocked_by:
reconciled: true
type: chore
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-15-codex-cli-validation-runbook-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-15-codex-cli-validation-runbook-design.md) |
| Plan | [2026-07-16-codex-cli-validation-runbook.md](https://github.com/danielhanold/docket/blob/feat/codex-cli-validation-runbook/docs/superpowers/plans/2026-07-16-codex-cli-validation-runbook.md) |
| Results | [2026-07-16-codex-cli-validation-runbook-results.md](https://github.com/danielhanold/docket/blob/feat/codex-cli-validation-runbook/docs/results/2026-07-16-codex-cli-validation-runbook-results.md) |
| PR | [#89](https://github.com/danielhanold/docket/pull/89) |
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
- Change 0079's runner delegation (a Claude *parent* offloading onto a `codex exec` *child*)
  — the opposite direction from the native-Codex execution this runbook validates.
- Implementing user-level `~/.codex/AGENTS.md` dispatch — Phase 4 only produces the evidence
  for ADR-0036's deferred decision; acting on it is a follow-up.
- Other unvalidated harness tokens (kiro, windsurf) and autonomous-loop soak testing.

## Open questions

- None beyond the spec's noted doc-drift risk (re-check live Codex docs when finalizing
  exact commands).

## Reconcile log

### 2026-07-16 — reconciled at claim (build time)

Design holds; no re-brainstorm needed. Dependency 0077 is `done` (PR #85), so the generated
Codex artifacts the runbook targets now exist as a *capability*. Folded current reality into
the spec as a "Reconcile update" section:

- **Path corrections** — `sync-agents.sh`/`link-skills.sh` are repo-root, not `scripts/`; the
  new `docs/codex/setup.md` (on `main`) is the static setup doc this runbook is the
  live-execution counterpart to (extend, don't duplicate).
- **Opt-in first** — this repo currently opts out (`agent_harnesses` commented out, no
  `.docket.local.yml`), so no `.codex/agents/*.toml` or `AGENTS.md` are on disk; Phase 1 must
  opt in then `sync-agents.sh`, and assert artifacts directly (`--check` is a vacuous exit-0
  while opted out).
- **Scope sharpened against 0079** — 0079 (runner delegation, merged) is the *opposite*
  direction (Claude parent → `codex exec` child); this runbook validates the *native* path
  where Codex runs skills whose bash reaches scripts through the `docket.sh` facade (0068)
  under Codex's own sandbox. Recorded as out of scope.
- **Phase 4 feeds ADR-0036** — that ADR deferred the user-level `~/.codex/AGENTS.md` dispatch
  decision to this validation; Phase 4 must record definitive evidence (automatic / prompted /
  refused, and whether user-level dispatch is needed). Acting on it stays a follow-up.
- **Phase 5 needs real Codex slugs** — built-ins carry Claude model IDs (sync-agents warns);
  proving a honored pin requires `agents.codex.<agent>.model` set to a real slug (e.g.
  `gpt-5.1-codex`, via `codex debug models`) with `effort:` emitted verbatim as
  `model_reasoning_effort`.
- **Restart note** — Codex registers agents at process start; Phases 3–5 restart the session
  after each `sync-agents.sh`.
