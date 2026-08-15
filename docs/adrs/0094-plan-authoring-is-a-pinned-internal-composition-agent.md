---
id: 94
slug: plan-authoring-is-a-pinned-internal-composition-agent
title: "Plan authoring is a pinned internal composition agent owning one git-verifiable artifact"
status: Accepted
date: 2026-08-15
supersedes: []
reverses: []
relates_to: [8, 18, 44, 59, 64, 83]
change: 324
---

## Context

`docket-implement-next` authored the Step 4 plan inline, at the orchestrator's own model and
effort. That couples two things which want independent tuning: the cost of routine orchestration
(claiming, reconciling, dispatching, bookkeeping) and the quality ceiling of the plan, which is the
single artifact every downstream build task reads. Lowering the orchestrator's model to price the
coordination economically would silently lower the plan's quality with it; raising the plan's
quality would mean paying that rate for all the coordination too. Planning needs its own
model-and-effort boundary.

The obvious extraction — a plan-writing subagent — carries a real hazard: docket's wrapper-bearing
worker agents deliberately perform **no** docket metadata operations, and an agent that both
authors the plan and writes `plan:` into the change manifest would become a second metadata writer,
splitting ownership of the manifest across two contexts.

## Decision

Step 4's plan authoring is extracted into a pinned **internal composition agent**,
`docket-plan-writer` (worktree-scope: feature; no preloaded skill, no injected convention), rather
than being authored inline at the orchestrator's model and effort.

The rule:

- **The agent owns exactly one git-verifiable artifact** — the plan file, committed on the feature
  branch with its `docket:backlink` block and an exact `Docket-Plan-Path: <repo-relative-path>` git
  trailer on the commit.
- **It returns only `PLAN_PATH=<repo-relative-path>`.** That return is a *sub-step receipt*, not a
  terminal disposition — it reports where the artifact landed, never that a run completed.
- **The parent retains orchestration and metadata attachment.** The implementer verifies **from
  git** before writing `plan:` into the manifest: containment in the feature worktree, a
  single-artifact delta since the pre-dispatch HEAD, exactly one `Docket-Plan-Path:` trailer whose
  value equals the return, and a valid backlink. There is **deliberately no directory allowlist** —
  containment plus single-artifact delta is the constraint; an allowlist would encode repo layout
  into the gate.
- **The path trailer is the resumability mechanism.** If the parent stops after the child returns
  but before the manifest write, the trailer makes the committed plan recoverable from git alone,
  closing the return-to-field gap.
- **Dispatch unavailability is Tier C (authorized-or-halt).** An explicitly configured
  `skills.plan: auto` is the human's authorization for the parent to fall back to authoring the
  plan inline; any other resolved value that cannot dispatch is abort-and-report. There is **never**
  a silent inline fallback.

## Consequences

- Orchestration model/effort and plan-quality model/effort tune independently: the orchestrator can
  be priced for coordination without dragging the plan's ceiling down with it.
- Adds the 17th shipped agent, registered end-to-end — including the Go built-in agent registry and
  a frozen v0.9.3 parity fixture — so the wrapper generation path is exercised for it like every
  other shipped agent.
- The plan writer performs **no** docket metadata operations, preserving the wrapper-cardinality
  boundary: the manifest keeps a single writer, the implementer.
- The verification gate is git-derived rather than trust-derived, so a misbehaving or truncated
  child cannot cause a wrong `plan:` to be recorded; the cost is that the parent must snapshot HEAD
  before dispatching and read the commit trailer after.
- One more dispatch on the critical path of every implement run, and one more place a dispatch
  outage can halt it — bounded by the Tier C posture rather than by silent degradation.
