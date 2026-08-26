---
id: 353
slug: 'dispatched-docket-implement-next-subagent-cannot-reach-agent'
title: 'Dispatched docket-implement-next subagent cannot reach agent-only workers, halting every non-trivial change at Step 4'
status: 'killed'
priority: 'critical'
type: 'fix'
created: '2026-08-26'
updated: '2026-08-26'
depends_on: []
stacked_on:
related: [334]
discovered_from: [351]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

When `docket-implement-next` is dispatched as a named subagent — the path the CLAUDE.md "Docket agents — dispatch, don't run inline" rule mandates via the harness's native named-agent dispatch — the resulting subagent's runtime tool surface does NOT include the Agent/Task spawn tool. Claude Code disables nested subagent spawning even though `.claude/agents/docket-implement-next.md` declares no tool restriction ("All tools"). The child can still fork skill-backed peers through the Skill tool (verified: `docket-status` forked fine in Step 0), but it has no mechanism to dispatch an agent-only worker.

`docket-plan-writer`, the four `docket-build-*` profile workers, and the three `docket-review-*` rung workers wrap NO skill by design (convention *Composition*: five wrappers wrap no skill; plan-writer is a runtime passthrough to whatever `skills.plan` names, so it cannot be a static skill). They are therefore reachable ONLY via native Agent/Task dispatch. A dispatched implement-next child reaches Step 4, tries `Skill(docket-plan-writer)` → "Unknown skill", searches the deferred/lazily-loaded tool surface and finds no spawn tool, and — per the convention's Dispatch-capability resolution (change 0137) — correctly declares dispatch unavailable and halts Tier C (authorized-or-halt), because the resolved `skills.plan` is not `auto`. The child followed its contract exactly; the failure is structural, not a child bug.

Consequence: EVERY non-trivial change halts at Step 4 when implement-next is run as a dispatched subagent. Only `skills.plan: auto` (inline authoring, which the user may not want) or running at the top level sidesteps it. The autonomous loop escapes the bug solely because `/loop docket-implement-next` executes the skill at the top-level main loop, where the Agent tool is present — so the whole agent layer (change 0016) silently depends on the runner context having nested-dispatch capability, an assumption that is false for a named-subagent dispatch on this harness.

Observed live on change 0351 (2026-08-26): claimed, reconciled, feature workspace prepared clean, then halted at Step 4 with nothing built and no PR. This collision between the "dispatch, don't run inline" guidance and this harness's no-nested-spawn behavior appears brand new relative to the last five changes, which drained via `/loop` (top level) or the fork path.

## What changes

Design decision needed (needs brainstorm). Candidate directions:

- Define the canonical run context for `docket-implement-next` (and any docket skill that dispatches agent-only workers) on harnesses that disable nested subagent spawning: require it to run at the top-level main loop (e.g. via `/loop`), and carve implement-next out of the "dispatch, don't run inline" rule so a human/orchestrator does not route it into a broken nested-subagent context.
- OR provide a subagent-reachable dispatch mechanism for agent-only workers — e.g. a runner facade the child can shell to (`docket.sh`-style) that spawns the pinned worker out-of-band — so the convention's Dispatch-capability resolution actually resolves inside a subagent.
- OR give the agent-only workers (plan-writer, the build profiles, the review rungs) forkable skill wrappers so a subagent's Skill tool can reach them — explicitly weighed against plan-writer's runtime-passthrough design (it wraps no skill on purpose because `skills.plan` may name any installed skill).
- Improve the Step-4 halt diagnostic to name this specific cause and point the operator at the working invocation (top-level `/loop`), instead of a generic Tier-C abort.
- Add a guard/health check that detects when implement-next is executing without nested-dispatch capability and aborts BEFORE claiming, so a run never reaches Step 4 only to halt.

## Out of scope

- The duplicate `## Run halted` heading defect in the halt-report authoring path (observed on 0351: the child's request body repeats the `## Run halted` H2 that the `docket change halt` transaction already wraps, which additionally wedges `docket change resume-halted` via `ApplySectionEdits`' duplicate-owned-heading guard). Real and adjacent, but a DISTINCT bug — not folded into this change; capture it separately.
- Actually changing plan-writer's runtime-passthrough design is a solution candidate to evaluate during brainstorm, not a committed outcome here.
- The run-gate attribution machinery (`gate-before`/`gate-verdict`) behaved correctly on 0351 and is not in scope.

## Why killed

Misdiagnosis. This stub claimed docket structurally cannot dispatch its agent-only workers (plan-writer/build/review), halting every non-trivial change. That is false for the real invocation path: docket's autonomous skills carry `context: fork` + `agent:` frontmatter, so the intended invocation (a slash command / the Skill tool, and the autonomous loop) FORKS the session and retains the Agent/Task tool, which dispatches the workers fine (a prior `/docket-implement-next` run built change 251 end-to-end). The 0351 halt was caused by invoking docket-implement-next through the raw Agent/Task tool as a named subagent (an operator tool-choice error), which spawns a tool-stripped subagent. The one genuinely real defect uncovered on 0351 — the duplicate `## Run halted` heading in the halt-report authoring path — is re-captured as a focused stub. Superseded by that stub; see it and change 0351.
