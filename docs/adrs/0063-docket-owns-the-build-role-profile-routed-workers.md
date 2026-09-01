---
id: 63
slug: docket-owns-the-build-role-profile-routed-workers
title: Docket owns the build role — profile-routed workers, model and effort on named agents
status: 'Superseded by ADR-0102'
date: 2026-07-30
supersedes: [23]
reverses: []
relates_to: [15, 16, 18, 59]
change: 167
---

## Context

ADR-0023 decided that docket would make build-phase model selection configurable by adding a
`build:` surface of per-role direct model IDs *inside* superpowers SDD's existing topology. SDD
dispatches a per-task implementer, a per-task reviewer, fix subagents, and a final whole-branch
code-reviewer; ADR-0023 aimed only at naming the models those roles use. Its implementing change
0044 was killed as superseded on 2026-07-30 without shipping — its PR #69 went stale behind the
0068/0072 facade rework and the later agent-layer changes.

Change 0167 re-examined the premise and found the knob was aimed at the wrong lever. For `T` plan
tasks and `R` failed task-review rounds, SDD's composition is roughly `2T + 2R + 2` nested agent
runs — and docket then invokes its own separately-configurable `skills.review` role for **another**
whole-branch review. The repeated review topology, not the per-task implementer, is what dominates
long-run token use, and no choice of model IDs reduces it. Fresh per-task implementers and focused
tests are the two mechanisms that buy the most confidence per token; the multiplicative task-review
loop and the duplicate branch review are the expensive parts.

Separately, SDD's prose "Model Selection" chooses models by controller judgment, and its
implementer/reviewer templates offer no corresponding per-dispatch **effort** choice — so reasoning
effort was inherited implicitly from the ambient session and was never expressible at all.

## Decision

Docket owns its build role rather than configuring someone else's topology.

1. **A docket-owned build role.** `docket-build` is a controller skill bound through the existing
   pluggable `skills.build` role; `docket-build-task` is a compact worker contract; and three named
   Claude agents — `docket-build-economy`, `docket-build-standard`, `docket-build-premium` —
   preload that worker and differ **only** in `model:` and `effort:`.

2. **Per-task profile routing.** Each plan task is routed to one profile, by an explicit
   `**Build profile:**` plan override or by an automatic rubric whose asymmetry is deliberate:
   `economy` must be **positively established** (fully specified, localized, pattern-following, no
   consequential risk); a named risk (auth/security boundaries, migrations or irreversible data
   changes, concurrency or locking, release infrastructure, unresolved architecture) selects
   `premium`; uncertainty defaults to `standard`. Each task may escalate **at most once** —
   economy→standard, standard→premium, premium→halt — and the retry consumes the task's whole
   allowance, so a task that began at economy halts rather than climbing to premium.

3. **Model and effort are properties of a named agent**, resolved through docket's existing
   generated-agent layer (overridable at the global, repo-committed, and repo-local layers) — not
   arguments a controller improvises per dispatch. That is what makes effort first-class, which is
   precisely what a direct-model-ID surface inside SDD could not deliver. The profile entries live
   under `agents.claude`, never `agents.default`, because Claude model IDs in the harness-neutral
   fallback would falsely present themselves as harness-portable (ADR-0015).

4. **No per-task review, one whole-branch review.** The build performs no per-task independent
   review and no final review of its own — the worker's self-review is part of implementation.
   Docket's single independent whole-branch review remains `docket-implement-next` Step 6's
   `skills.review` role.

5. **Tests.** Task workers run focused tests; the full suite runs **once** after all tasks, derived
   from `finalize.test_command` or finalize's existing auto-detection rather than a second,
   driftable test-command key. A red suite becomes one synthetic integration-repair task on
   standard→premium→halt, with no repair/review loop.

6. **Resume checkpointing** is a new `build.checkpoint` config leaf, global-able, default `false`.

7. **ADR-0023's `build:` per-role direct-model-ID surface is not built.** The configuration point is
   the agent layer's existing per-agent model/effort resolution instead, so no new config surface
   competes with it.

## Consequences

- A clean `T`-task build dispatches `T` workers plus docket's one whole-branch reviewer — `T+1`
  nested runs across build and review, against SDD's ~`2T+2` clean path.
- Cost becomes an explicit, per-task, configurable decision with a named reason, instead of an
  implicit inheritance. `premium` buys greater reasoning investment, **not** a stronger correctness
  guarantee: every profile carries identical testing and completion obligations.
- **What is given up: SDD's independent per-task reviewer.** Task-level defects must now be caught
  by the worker's own TDD and self-review, or by the single whole-branch review. This is a
  deliberate bet that focused tests plus one broad review catch more per token than a reviewer per
  task — and the whole-branch review's ability to see cross-task interaction defects (which
  per-task reviews structurally cannot) is what makes the bet reasonable.
- The Claude-first release ships profile agents with Claude model IDs only; Cursor and Codex profile
  mappings are deferred to changes 0168 and 0169, and the shipped cross-harness `skills.build`
  default stays `superpowers:subagent-driven-development`, so users who do nothing see no behavior
  change. Docket's own repo opts in, dogfooding it.
- Selecting `docket-build` without resolvable profile dispatch is **Tier C authorized-or-halt**
  (ADR-0059): only an explicit `skills.build: auto` authorizes inline execution.

This ADR **supersedes ADR-0023**.

## Update — 2026-08-02 (change 0193)

The Consequences above state that the shipped cross-harness `skills.build` default "stays
`superpowers:subagent-driven-development`, so users who do nothing see no behavior change". That
consequence no longer holds: change 0193 flipped the built-in default for `skills.build` to
`docket-build`, and removed this repo's own `skills:` pin so it dogfoods the shipped default rather
than an override.

The **Decision** is unchanged and unreversed — docket still owns the build role, the profile-routed
worker topology is untouched, and nothing about routing, escalation, or the single end-of-build
suite gate is affected. What changed is only the rollout posture: the conservative default was a
first-release hedge pending evidence, and `docket-build` had by then been exercised enough on this
repo to be trusted as the default. A user who does nothing now gets `docket-build`; the escape hatch
is unchanged — set `skills.build` explicitly (including to
`superpowers:subagent-driven-development`) at any config layer.

