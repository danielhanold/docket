---
id: 135
slug: cursor-agent-wrapper-contract
title: "Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort"
status: proposed
priority: high
type: fix
created: 2026-07-23
updated: 2026-07-23
depends_on: []
related: [16, 44, 45, 46, 48, 49, 66, 113]
discovered_from: []
adrs: [8, 15, 24]
spec:
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
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md) |
<!-- docket:artifacts:end -->

## Why

A live `docket-implement-next` run under Cursor exposed that docket's generated Cursor wrappers do
not match Cursor's documented subagent contract. The wrapper advertised
`superpowers:subagent-driven-development` as the resolved build skill, but the child had no
documented Skill tool with which to invoke it. Plan, build, review, and finish consequently degraded
to their inline `auto` fallbacks. The run still produced a plausible PR, but it did not execute the
SDD workflow's fresh per-task implementers, TDD discipline, or per-task review gates.

This is the Cursor-specific instance of the defect already recorded by the
`skill-fallback-degrades-discipline` learning from change 0066: successful artifacts can conceal
that the configured workflow discipline was unreachable in the harness that actually ran the
build.

The same investigation found a second contract mismatch in model configuration. Cursor's current
[Subagents documentation](https://cursor.com/docs/subagents) documents these custom-agent
frontmatter fields: `name`, `description`, `model`, `readonly`, and `is_background`. Reasoning
effort is a model parameter encoded in the model value, for example
`model: claude-opus-4-8[effort=high]`. Docket instead generates a Claude-shaped wrapper for Cursor:

```yaml
model: claude-opus-4-8
effort: xhigh
skills: [docket-implement-next, docket-convention]
```

Neither the standalone `effort:` field nor `skills:` preload is part of Cursor's documented
frontmatter. In `sync-agents.sh`, `emit_for_harness()` routes every non-Codex harness through the
same generic Markdown emitter, so Cursor inherits Claude Code semantics. Existing Cursor generation
tests preserve that shape rather than checking Cursor's actual contract. Model pinning, reasoning
effort, and skill availability can therefore all differ from what docket reports.

Cursor does document nested subagent launches through the Task tool (within its nesting limit and
subject to mode, hook, and tool policy). That may provide a native execution route for SDD's
implementer/reviewer tree, but docket has not designed, generated, or runtime-verified that route.

## What changes

Make generated Cursor agents conform to Cursor's current subagent contract and prove that the
workflow capabilities docket advertises are reachable:

- Add a Cursor-specific wrapper emitter instead of passing Cursor through the generic Claude-style
  Markdown emitter.
- Translate the resolved model and effort into Cursor's documented model-parameter syntax, preserving
  valid model options and defining how `inherit`, `auto`, unavailable models, and unsupported effort
  values behave.
- Stop emitting unsupported Cursor frontmatter fields. Replace `skills:` preload with a
  Cursor-compatible mechanism that gives each docket wrapper the required docket skill and
  convention instructions.
- Provide a real Cursor execution path for configured workflow roles, especially
  `superpowers:subagent-driven-development`, including its nested implementer and reviewer Task
  dispatches where the harness permits them.
- Define an honest failure posture when a configured discipline cannot run. An autonomous build
  must not silently substitute inline implementation while presenting SDD/TDD/review as having run;
  either the configured fallback must be explicit and auditable or the run must halt.
- Update config and agent-layer documentation so Claude Code, Cursor, and Codex semantics are
  described separately where they differ.
- Replace tests that assert byte-identical Claude/Cursor wrappers or standalone Cursor `effort:`
  fields with harness-contract tests. Add a Cursor runtime smoke test that verifies the effective
  model/effort, required instructions, nested Task access, and actual SDD workflow reachability
  rather than only checking generated text.

## Out of scope

- Changing the Superpowers SDD or TDD skills themselves.
- Retrofitting or reopening the consuming `cet-devops` change 6 implementation or its PR.
- Completing change 0044's broader configurable per-role build-model design, except for keeping its
  eventual configuration compatible with the corrected Cursor emitter.
- General redesign of Claude Code or Codex wrapper generation beyond changes needed to preserve
  their existing behavior while splitting out Cursor semantics.

## Open questions

- What is the supported Cursor replacement for startup `skills:` injection: generated prompt
  inclusion, explicit linked-skill instructions, direct skill invocation from the wrapper, or
  another documented mechanism?
- Can Cursor's nested Task support execute SDD faithfully from a docket wrapper in every supported
  mode, and how should a tool-policy denial surface?
- Which model IDs and effort values should docket validate or normalize per Cursor model, versus
  passing through and relying on Cursor's documented compatible-model fallback?
- How can an automated smoke test prove the effective model and effort rather than merely proving
  that the generated `model:` string has the expected syntax?
- Should unavailable configured workflow skills always produce the `halted` disposition for
  autonomous builds, or can an explicitly configured `auto` value remain the sole authorization for
  inline fallback?
- Does this correction supersede any Cursor-specific portions of ADR-0008 or ADR-0015, or only
  refine their harness mappings?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
