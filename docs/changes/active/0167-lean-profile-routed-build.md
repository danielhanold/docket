---
id: 167
slug: lean-profile-routed-build
title: Lean profile-routed build — fresh task workers without review loops
status: proposed
priority: high
type: feat
created: 2026-07-30
updated: 2026-07-30
depends_on: []
related: [42, 44, 135, 137]
discovered_from: []
adrs: [23]
spec: docs/superpowers/specs/2026-07-30-lean-profile-routed-build-design.md
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
| Spec | [2026-07-30-lean-profile-routed-build-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-30-lean-profile-routed-build-design.md) |
| ADRs | [ADR-0023](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0023-configurable-sdd-build-model.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` currently wraps Superpowers SDD, whose per-task implementer/reviewer pairs,
fix/re-review rounds, and whole-branch review duplicate Docket's own review boundary and dominate
long-run token use. Fresh per-task implementers and focused tests remain valuable; the repeated
review topology and implicit effort inheritance do not.

## What changes

- Add a Docket-owned build controller and compact shared task-worker skill.
- Route each plan task to a Claude Code `economy`, `standard`, or `premium` model/effort profile.
- Preserve focused TDD and one task commit while removing per-task review agents.
- Run the full suite once after the task sequence using `finalize.test_command` or the existing
  detection fallback, with one bounded integration-repair path.
- Make compact resume checkpointing global-able and default-off.
- Dogfood the new build through this repository's `skills.build` setting without changing the
  shipped cross-harness default.

## Out of scope

- Cursor and Codex profile dispatch.
- Replacing the remaining independent whole-branch `skills.review` role.
- Hard subagent turn caps.
