---
id: 44
slug: configurable-build-model
title: Configurable TDD build model for docket-implement-next
status: proposed
priority: low
created: 2026-07-07
updated: 2026-07-07
depends_on: [43]
related: [16]
adrs: []
spec: docs/superpowers/specs/2026-07-07-configurable-build-model-design.md
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
| Spec | [2026-07-07-configurable-build-model-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-07-configurable-build-model-design.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` builds each change through `superpowers:subagent-driven-development` (SDD),
which dispatches an implementer per plan task, a task-reviewer after each, fix subagents, and a
final whole-branch code-reviewer — where the vast majority of a build's tokens are spent. SDD
picks each dispatch's model by **controller judgment** (a prose "Model Selection" heuristic); there
is **no config knob**. So a repo cannot express a policy like "build implementers on Haiku 4.5,
reviewers on Sonnet 5" — the single biggest cost lever in the system is unconfigurable. #0016
explicitly scoped this out; this change closes it, reusing #0043's tier vocabulary.

## What changes

Add a **`build:` config surface** with two per-role tiers, resolved through #0043's tier map, plus
a behavioral rule in `docket-implement-next` (full detail in the linked spec):

- **`build.implementer`** governs the per-task implementer **and** fix subagents; **`build.reviewer`**
  governs the task-reviewer **and** the final code-reviewer. Values are tier names (explicit model
  IDs also accepted).
- **`docket-implement-next`** resolves these at plan-execution time and fills SDD's already-required
  `model:` field from them; **unset → SDD's own Model Selection** (purely additive, backward-
  compatible). No new script, no fork of SDD.

A set role is a blunt, deliberate override of SDD's per-complexity adaptivity — trading it for a
predictable cost/quality policy. Likely warrants a small ADR — decided at build.

## Out of scope

- Redesigning or forking SDD — this only supplies the `model:` value SDD already requires.
- Per-task / per-complexity build-model config (mechanical/integration/architecture buckets) — a
  possible future refinement.
- The reconcile/plan/escalation model of implement-next itself — that stays the `implement-next`
  wrapper's tier (#0042/#0043); `build:` governs only the SDD sub-dispatches.

## Open questions

- Config placement — top-level `build:` vs nested under `agents:` (lean top-level).
- Whether the final code-reviewer folds into `build.reviewer` (this design) or gets its own role.
- Exact SDD override point — confirm against the SDD version in use at build.

## Reconcile log
