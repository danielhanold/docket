---
id: 79
slug: codex-runner-delegation
title: Cross-harness runner delegation framework (first runner — OpenAI Codex)
status: done
priority: medium
created: 2026-07-15
updated: 2026-07-16
depends_on: []
related: [16, 44, 45, 46, 61, 62, 77, 78]
adrs: [15, 37, 38]
spec: docs/superpowers/specs/2026-07-15-codex-runner-delegation-design.md
plan: docs/superpowers/plans/2026-07-15-codex-runner-delegation.md
results: docs/results/2026-07-15-codex-runner-delegation-results.md
trivial: false
auto_groomable:
branch: feat/codex-runner-delegation
pr: https://github.com/danielhanold/docket/pull/86
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-15-codex-runner-delegation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-15-codex-runner-delegation-design.md) |
| Plan | [2026-07-15-codex-runner-delegation.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-15-codex-runner-delegation.md) |
| Results | [2026-07-15-codex-runner-delegation-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-15-codex-runner-delegation-results.md) |
| PR | [#86](https://github.com/danielhanold/docket/pull/86) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0037](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0037-runner-delegation-explicit-runner-field.md), [ADR-0038](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0038-runner-shim-wrapper-single-dispatch-chokepoint.md) |
<!-- docket:artifacts:end -->

## Why

A docket agent dispatch always runs on the harness hosting the session — under Claude Code, as a
Claude Code subagent, billed to the Claude subscription and limited to Claude models. Daniel also
holds an OpenAI Codex subscription with its own capacity, and Codex CLI is a working agentic
harness (superpowers installed, native multi-agent support). The general gap: a **parent harness**
cannot delegate an agent's whole run to a **child harness** with its own subscription, models, and
skills. Codex-from-Claude-Code is the motivating pair, but the mechanism should be a framework
that admits future pairs (other children like `gemini-cli`, other parents like Cursor) without
redesign. OpenAI's own `codex-plugin-cc` doesn't close the gap: it is a human-facing slash-command
layer over background jobs, not a skill-aware dispatch bridge (researched during brainstorm; see
spec).

## What changes

A **cross-harness runner delegation framework** — whole-run delegation of autonomous docket agents
to a child harness's non-interactive exec primitive, activated by explicit config — shipping with
**one implemented pair: parent `claude`, child `codex`** (full design in the linked spec):

- **`runner: <name>`** — a new optional per-agent key in the `agents:` block naming a registered
  runner (this change registers `codex`; default native). The harness key it sits under is the
  parent; only the `claude` parent is implemented here. `model:` stays ADR-0015 opaque
  passthrough; `effort:` maps through the adapter. A per-runner **`runners.<name>:`** config block
  holds child-specific knobs (for codex: sandbox posture, default workspace-write + network,
  approvals never).
- **Shim wrappers via a runner registry** — `sync-agents.sh` generates the normal wrapper file
  whose body is a runner-parameterized shim: one foreground call to the dispatch facade. All
  invocation paths (fork, `@`-dispatch, composition) inherit delegation unchanged.
- **Dispatch facade + per-runner adapters** — a runner-neutral `runner-dispatch.sh` hands off to
  `scripts/runners/<name>.sh`; each adapter owns preflight, prompt assembly, flag mapping, and
  foreground execution with final-message relay. `runners/codex.sh` (wrapping `codex exec`) is the
  first adapter.
- **Skill availability** — already in place for codex: `link-skills.sh` links docket skills into
  `~/.codex/skills` (per #0077); this change only verifies the dispatch prompt can invoke them by
  name. Future adapters document their own story.
- A delegated orchestrator's sub-dispatches run child-natively (for codex: `spawn_agent`, via
  superpowers' Codex adaptation); only autonomous wrappers are delegatable — interactive skills
  stay inline (framework rule).

## Out of scope

- Mixed topology (parent-hosted orchestrator routing individual SDD build leaves to a child
  harness) — folded into #0044's redesign (`build.<role>.runner: codex`).
- Additional runner adapters (`gemini-cli`, …) and additional parents (Cursor, …) — the seams
  ship; only claude→codex is implemented and verified. `runner:` under non-claude harness keys is
  warned-and-ignored (reserved, not an error).
- Automating Codex install/auth/superpowers setup — documented prerequisites.
- Carrying per-child model pins into child-harness sub-dispatches (accepted limitation).

## Open questions

- ~~Exact `codex exec` final-message capture flag on the installed version.~~ RESOLVED at
  reconcile: codex-cli 0.144.4 has `--output-last-message <FILE>` (plus `-m/--model`,
  `-s/--sandbox`, `-C/--cd`, `-c key=value` for `model_reasoning_effort` and
  `sandbox_workspace_write.network_access`).
- Whether #0077's TOML agents (`.codex/agents/docket-*.toml`) let a delegated orchestrator's
  Codex-side children resolve model pins, softening the accepted pin-loss limitation —
  unverifiable until #0077 merges; the accepted limitation stands.
- Whether delegating `docket-finalize-change` to Codex sidesteps the merge-without-review
  classifier — interacts with #0062; policy question, must not become a silent bypass.

## Reconcile log

- 2026-07-15 — Reconciled against current code + related records before planning. Findings:
  - **Design valid, no scope drift.** `origin/main` tip is 3dd283f (0075 merged): `docket.sh`
    facade, `REPO_ROOT` export, and `link-skills.sh` (repo root) all match the spec's assumed
    starting state. Skill availability on codex verified on this machine: `~/.codex/skills`
    carries the docket-* links.
  - **#0077 is concurrently `in-progress`** — its feature branch is fully built locally
    (results receipt at tip) but unpushed, no PR. Its spec guarantees a byte-identical
    non-codex (markdown) emitter path, so this change's claude-wrapper shim work overlaps it
    only structurally in `sync-agents.sh` (both add registry scaffolding + tests). The spec's
    "parallel in shape to #0077's emitter registry" is descriptive, not a dependency —
    `depends_on` stays `[]`; expect a mechanical rebase in whichever change merges second.
  - **Open question 1 resolved** (see above): `--output-last-message` pinned on the installed
    codex-cli 0.144.4.
  - **Prerequisite gap on this machine:** `[features] multi_agent` is NOT set in
    `~/.codex/config.toml`. It gates only a delegated orchestrator's child-native fan-out —
    not the build's single-agent live smoke dispatch (status) — so the build proceeds;
    documented as a README prerequisite and flagged to the human.
  - **0075 constraint folded in:** `runner-dispatch.sh` and the codex adapter must anchor to
    the durable `REPO_ROOT` (ADR-0034 cwd-independence), passing it as the child's working
    directory (`codex exec -C`).
