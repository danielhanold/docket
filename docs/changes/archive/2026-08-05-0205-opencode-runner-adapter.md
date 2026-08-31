---
id: 205
slug: opencode-runner-adapter
title: opencode runner adapter — delegate build workers to OpenRouter models
status: done
priority: medium
type: feat
created: 2026-08-04
updated: 2026-08-05
depends_on: []
related: [79, 192, 195, 78]
discovered_from: [192]
adrs: [15, 37, 38, 63, 67]
spec: docs/superpowers/specs/2026-08-04-opencode-runner-adapter-design.md
plan: docs/superpowers/plans/2026-08-05-opencode-runner-adapter.md
results: docs/results/2026-08-05-opencode-runner-adapter-results.md
trivial: false
auto_groomable: false
branch: feat/opencode-runner-adapter
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/156
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-04-opencode-runner-adapter-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-04-opencode-runner-adapter-design.md) |
| Plan | [2026-08-05-opencode-runner-adapter.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-05-opencode-runner-adapter.md) |
| Results | [2026-08-05-opencode-runner-adapter-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-05-opencode-runner-adapter-results.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0037](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0037-runner-delegation-explicit-runner-field.md), [ADR-0038](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0038-runner-shim-wrapper-single-dispatch-chokepoint.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0067](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0067-runner-bearing-agent-requires-a-user-configured-model.md) |
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
- **Codex work** — change 0078 (Codex CLI validation runbook) sits at `implemented` with PR #89
  open and is settled separately; this change touches no codex adapter behavior except the shared
  runner-wide required-model rule.
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

### 2026-08-05

Verified the whole design against current `main`. Every load-bearing premise still holds:

- `REGISTERED_RUNNERS="codex cursor"` (sync-agents.sh:87) — unchanged since 0192, so the opencode
  token is still net-new.
- `scripts/runners/` holds exactly `codex.{sh,md}` and `cursor.{sh,md}`; no opencode adapter exists.
- `runner-dispatch.sh` already resolves an arbitrary `runners.<name>:` block per-key across the
  config layers and exports each key as `DOCKET_RUNNER_CFG_<KEY>`, so the new
  `runners.opencode.permissions` knob needs **no** resolver plumbing — only adapter-side reading,
  exactly as `runners.codex.sandbox` does via `DOCKET_RUNNER_CFG_SANDBOX`.
- 0168's provenance rule is live at `emit_wrapper` (sync-agents.sh:849-852) as
  `RES_MODEL_FROM_USER`/`RES_EFFORT_FROM_USER` gates, which is precisely the site the runner-wide
  required-model error attaches to — an empty `flag_model` on a `runner:`-bearing claude agent.
- `docs/opencode/setup.md` exists (created by 0192) and is the right home for the config recipe.
- 0192's opencode harness support is merged and archived; change 0195 (default-table retune) is
  still `proposed`/needs-brainstorm, so option A's decoupling argument stands unchanged.

One correction: the change and spec both said change 0078 was "being deferred as built on outdated
logic." It is in fact `implemented` with PR #89 open. Corrected in both — it changes no scope, since
0078 was already out of scope either way.

No scope adjustments otherwise. Auto-capture: no follow-up work surfaced that is not already
tracked (the model-table retune is 0195; the live-verify items are this change's own open questions,
resolved at build time).
