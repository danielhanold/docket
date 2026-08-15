---
id: 324
slug: model-pinned-plan-writer-agent
title: 'Extract plan writing into a model-pinned internal agent'
status: in-progress
priority: critical
type: feat
created: 2026-08-15
updated: 2026-08-15
depends_on: []
stacked_on:
related: [16, 17, 49, 96, 311, 315]
discovered_from: []
adrs: [8, 15, 16, 18, 44, 59, 64, 83]
spec: docs/superpowers/specs/2026-08-15-model-pinned-plan-writer-agent-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/model-pinned-plan-writer-agent
claimed_at: 2026-08-15T15:51:13Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-15-model-pinned-plan-writer-agent-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-15-model-pinned-plan-writer-agent-design.md) |
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md), [ADR-0018](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0018-pluggable-skills-passthrough-degrade.md), [ADR-0044](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0064](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md), [ADR-0083](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0083-agent-worktree-scope-is-a-declared-frontmatter-fact.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` currently authors the implementation plan at the orchestrator's own model
and effort. Lowering that pin to price routine orchestration more economically therefore lowers the
quality ceiling of the plan that guides every downstream build task.

Planning needs an independent model-and-effort boundary so the orchestrator can be tuned for its
coordination workload without coupling that choice to the judgment-heavy plan artifact.

## What changes

Add an internal, feature-worktree-scoped `docket-plan-writer` agent with independent shipped and
user-overridable model/effort settings. Keep planning inside `docket-implement-next` Step 4: the
parent prepares the worktree and context, dispatches the planner in the foreground, verifies its
committed plan artifact, and records the returned repo-relative path in `plan:`.

The plan writer continues to honor the resolved `skills.plan` binding, including custom plan
locations and the existing missing-skill fallback. Its plan commit persists the path for recovery;
the return is a non-terminal sub-step receipt, and the parent must attach the plan and continue into
the build. Update the agent/config documentation, dispatch and generator guards, resume contract,
and the Go embedded asset snapshot. Land this change before change 0315 so the Go migration's
claim-to-implemented workflow reconciles against the settled planning boundary.

## Out of scope

- Adding a public plan-writer skill or another workflow-role configuration key.
- Changing the default `skills.plan` binding, the build/review roles, or Step 4's lifecycle
  postcondition and top-level numbering.
- Moving plan judgment or harness-native agent dispatch into the Go engine.
- Changing change 0315's dependency graph or implementing any other Go-migration slice.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
