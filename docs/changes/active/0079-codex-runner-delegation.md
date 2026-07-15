---
id: 79
slug: codex-runner-delegation
title: Delegate docket agent runs to OpenAI Codex via an explicit runner field
status: proposed
priority: medium
created: 2026-07-15
updated: 2026-07-15
depends_on: []
related: [16, 44, 45, 46, 61, 62, 77, 78]
adrs: [15]
spec: docs/superpowers/specs/2026-07-15-codex-runner-delegation-design.md
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
| Spec | [2026-07-15-codex-runner-delegation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-15-codex-runner-delegation-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md) |
<!-- docket:artifacts:end -->

## Why

Under the Claude Code harness every docket agent dispatch runs as a Claude Code subagent — billed
to the Claude subscription, limited to Claude models. Daniel also holds an OpenAI Codex
subscription with its own capacity, and Codex CLI is a working agentic harness (superpowers
installed, native multi-agent support). There is currently no way to say "run this docket agent on
Codex," so that capacity and model diversity are unusable for backlog work. OpenAI's own
`codex-plugin-cc` doesn't close the gap: it is a human-facing slash-command layer over
background jobs, not a skill-aware dispatch bridge (researched during brainstorm; see spec).

## What changes

Whole-run delegation of autonomous docket agents to `codex exec`, activated by explicit config
(full design in the linked spec):

- **`runner: codex`** — a new optional per-agent key in the `agents:` block (claude harness only;
  default `native`). `model:` stays ADR-0015 opaque passthrough, handed verbatim to
  `codex exec --model`; `effort:` maps to Codex reasoning effort. New optional `runner_codex:`
  block for sandbox posture (default workspace-write + network, approvals never).
- **Shim wrappers** — `sync-agents.sh` generates the normal wrapper file whose body is a shim:
  one foreground call to a new deterministic `codex-dispatch.sh` (preflight, prompt assembly,
  flags, foreground `codex exec`, final-message relay). All invocation paths (fork, `@`-dispatch,
  composition) inherit delegation unchanged.
- **Skill availability** — already in place: `link-skills.sh` links docket skills into
  `~/.codex/skills` (per #0077); this change only verifies the dispatch prompt can invoke them
  by name.
- A delegated orchestrator's sub-dispatches run Codex-natively (`spawn_agent`, via superpowers'
  Codex adaptation); only autonomous wrappers are delegatable — interactive skills stay inline.

## Out of scope

- Mixed topology (Claude Code orchestrator routing individual SDD build leaves to `codex exec`) —
  possible follow-up change.
- Runners other than Codex; `runner:` under non-claude harness keys (warned-and-ignored).
- Automating Codex install/auth/superpowers setup — documented prerequisites.
- Carrying per-child model pins into Codex-side sub-dispatches (accepted limitation).

## Open questions

- Exact `codex exec` final-message capture flag on the installed version.
- Whether #0077's TOML agents (`.codex/agents/docket-*.toml`) let a delegated orchestrator's
  Codex-side children resolve model pins, softening the accepted pin-loss limitation (verify at
  build).
- Whether delegating `docket-finalize-change` to Codex sidesteps the merge-without-review
  classifier — interacts with #0062; policy question, must not become a silent bypass.

## Reconcile log
