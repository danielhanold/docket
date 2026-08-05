---
id: 205
slug: opencode-runner-adapter
title: opencode runner adapter — delegate build workers to OpenRouter models
status: proposed
priority: medium
type: feat
created: 2026-08-04
updated: 2026-08-04
depends_on: []
related: [79, 192, 195, 78]
discovered_from: [192]
adrs: [15, 37, 38, 63]
spec: docs/superpowers/specs/2026-08-04-opencode-runner-adapter-design.md
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-04-opencode-runner-adapter-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-04-opencode-runner-adapter-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0037](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0037-runner-delegation-explicit-runner-field.md), [ADR-0038](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0038-runner-shim-wrapper-single-dispatch-chokepoint.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md) |
<!-- docket:artifacts:end -->

## Why

Change 0192 made `opencode` a fully shipped docket harness — registration, native emitter, dispatch
wiring, and a sixteen-agent default block on OpenRouter model IDs — and explicitly scoped out the
one thing that would let an existing Claude Code session use it: *"A Claude-to-opencode whole-run
runner (`REGISTERED_RUNNERS` unchanged); possible follow-up."* This is that follow-up.

The motivation is cost asymmetry. Through OpenRouter, opencode reaches DeepSeek-tier models at
roughly 3¢ per task against Claude Opus at $2.40. Because docket's build and review roles are
already separate wrappered agents (ADR-0063), delegation can be selective: send the four build
profile workers to cheap models while review stays on the existing Claude subscription. Today
that is impossible from a Claude Code session — there is no `opencode` adapter, so `runner:
opencode` is a generation-time error.

An alternative was considered and rejected during brainstorm: a wrapper script calling the
OpenRouter API directly instead of going through a harness. The premise — that the harness stands
between docket and the cheap model — is false. OpenRouter is opencode's model backend; opencode is
the agent runtime, and docket already reaches OpenRouter through it. A runner delegates a whole
agentic run (plan, branch, edit, test, commit, PR); a raw API call returns one completion. Closing
that gap means rebuilding opencode in shell, and forces docket to own model semantics that
ADR-0015's opaque passthrough exists to avoid. Full reasoning in the spec.

## What changes

- **`scripts/runners/opencode.sh`** + its `opencode.md` contract, a ~100-line sibling of
  `codex.sh`, plus the `REGISTERED_RUNNERS` entry. Flag mapping to `opencode run`: `--model`
  verbatim, effort → `--variant` (which takes docket's `max` natively, so no mapping table),
  repo root → `--dir`.
- **A `runners.opencode.permissions` knob** — `ask` (default) | `auto-approve` — gating opencode's
  `--auto`. An enum, parallel to `runners.codex.sandbox`. Under `ask`, a delegated run fails loudly
  at preflight rather than hanging on an approval prompt nobody can answer; `auto-approve` is a
  deliberate, visible line the human writes.
- **A runner-wide required-model rule.** A `runner:`-bearing agent with no user-configured model
  becomes a loud generation-time error, replacing today's silent fall-through to the child's own
  default. Under OpenRouter that default is pay-per-token and of unknown identity — a mistake that
  surfaces on the bill, not in the run. Applied framework-wide rather than opencode-only.
  Behavior change for existing model-less codex/cursor configs; **needs an ADR**.
- **A config recipe in `docs/opencode/setup.md`** — the build-delegated / review-native block with
  verified model IDs, the "delegate leaves, not orchestrators" rule, and what `auto-approve`
  actually grants.

Model selection is **explicit user config**, not resolved from 0192's shipped opencode block. That
block answers "if opencode ran this whole project, what should each role cost?"; delegation asks
which rows should leave the Claude subscription. Cross-indexing the two would also make a native
retune silently change delegated builds. Rationale in the spec.

## Out of scope

- **Model retuning** — change 0195 owns the opencode default table and its open questions.
- **Sidecar cross-indexing**, and any change to 0168's provenance rule beyond the required-model
  error.
- **Codex work** — change 0078 is being deferred separately as built on outdated logic.
- **New restrictions on which agents are delegatable** — unchanged; this ships capability, and
  which agents are pinned is user config.
- **Parent harnesses other than `claude`** — `runner:` elsewhere stays reserved.

## Open questions

All are live-session verify items, not design gaps; enumerated with reasoning in the spec.

1. Confirm model IDs against `opencode models` — no in-repo test can detect a wrong ID.
2. Confirm `--variant` omission yields the provider default (0192 flagged its own equivalent case
   as unprobed).
3. Confirm `--auto` semantics and the deny-list interaction — both inferred from one line of help
   text.
4. Confirm the relay surface: `--format json` versus default formatted output.
5. Live-certify one delegated `build-economy` dispatch end to end before this is called done.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
