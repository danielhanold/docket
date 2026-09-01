---
id: 394
slug: 'give-docket-skills-an-authoritative-compact-cli-capability-c'
title: 'Give Docket skills an authoritative compact CLI capability catalog'
status: 'in-progress'
priority: 'high'
type: 'feat'
created: '2026-09-01'
updated: '2026-09-01'
depends_on: []
stacked_on:
related: [360, 369, 370, 371, 377]
discovered_from: [360]
adrs: [3, 20, 36]
spec: 'docs/superpowers/specs/2026-09-01-give-docket-skills-an-authoritative-compact-cli-capability-c-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/give-docket-skills-an-authoritative-compact-cli-capability-c'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-01T23:42:59Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-01-give-docket-skills-an-authoritative-compact-cli-capability-c-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-01-give-docket-skills-an-authoritative-compact-cli-capability-c-design.md) |
| ADRs | [ADR-0003](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0003-convention-reference-loading.md), [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md) |
<!-- docket:artifacts:end -->

## Why

Change 0360 records CLI and skill drift as one contributor to implement-next coordination cost, but the underlying problem is broader and harness-independent: Docket workflows have no authoritative machine-readable view of the verbs exposed by the running Go binary. Cursor makes the gap visible by trying commands and inspecting --help; other harnesses may only hide the same probing. Hand-maintained command spellings in skills can drift from the binary, and discovery by trial is especially unsafe when a well-formed probe can mutate metadata or external systems.

## What changes

Add one repository-independent, read-only capability bootstrap that returns the running binary's complete executable leaf-command catalog in compact protocol-v1 JSON. Derive command paths and invocation signatures from the live Cobra tree, require a closed effect classification for every leaf, keep the current catalog within a measured byte budget, and migrate maintained Docket workflows to fetch the catalog once before repository preparation and resolve executable spellings from it. Add correspondence, mutation, payload-budget, generated-asset, and cross-harness acceptance coverage so no maintained workflow falls back to --help, guessed verbs, or command probes.

## Out of scope

An MCP server or adapter; making MCP Docket's primary interface; complete request/result JSON-schema discovery owned by change 0360; workflow-policy redesign; new lifecycle operations unrelated to capability discovery; rewriting historical changes, specs, plans, results, or Accepted ADR prose.

## Reconcile log

### 2026-09-01

### 2026-09-01 — reconcile (implement-next)

Assessed the design against current `main`/`docket` reality; the proposal and spec hold unchanged, no scope adjustment needed.

- Related predecessors 0369, 0370, 0371, 0377 are all `done` (archived): the frozen Bash facade is deleted, maintained consumers already run against the native `docket` Go binary, and generated agent invocation is on native host dispatch. This confirms the premise that the running Cobra tree is now the sole owner of which verbs exist, while installed skills still carry hand-authored spellings — exactly the gap this change closes.
- Related change 0360 remains `proposed` and keeps its distinct scope: complete request/result JSON-schema discovery. This change stays strictly the verb/effect *invocation* catalog and does not touch schema discovery (acceptance criterion 9).
- Verified `docket capabilities` does not yet exist (`unknown command "capabilities"`), so the new public leaf is genuinely net-new. The production Cobra root is assembled in `internal/cli/root.go` (`root.AddCommand(...)` at the tail of `run`), with per-group builders (`newChangeCommand`, `newContextCommand`, etc.) in `internal/cli/*.go`; effect + stable-operation-id metadata will be co-located on leaf registration there (typed annotations / equivalent structure the walker consumes), never a second command-name map.
- Cited ADRs 0003 (convention reference-loading), 0020 (generated agent artifacts machine-local), 0036 (Codex AGENTS.md dispatch block committed, machine-neutral) remain Accepted and relevant to the workflow-consumption + generated-asset regeneration parts.

No obsolescence, no fundamental invalidation. Proceeding to plan and build.

Follow-up noted for deliberate human capture (not minted here): the docket-implement-next / docket-convention skill prose still describes some transactions with `--id`/`--version` flags (e.g. `docket change reconcile --id <id> --version <v>`) while the shipped binary takes `--input <request-file>` with id/version inside the JSON. That instruction-vs-binary drift is the very class this change's Step-0 migration targets; ensuring the migrated skill prose matches the real invocation shapes is in-scope for the skill-migration work here.
