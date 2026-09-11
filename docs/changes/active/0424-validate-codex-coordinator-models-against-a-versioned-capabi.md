---
id: 424
slug: 'validate-codex-coordinator-models-against-a-versioned-capabi'
title: 'Validate Codex coordinator models against a versioned capability registry'
status: 'proposed'
priority: 'critical'
type: 'feat'
created: '2026-09-11'
updated: '2026-09-11'
depends_on: [423]
stacked_on:
related: [384, 393, 412]
discovered_from: [423]
adrs: [114]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0114](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0114-anchor-codex-feature-scoped-role-entry-to-the-owning-worktre.md) |
<!-- docket:artifacts:end -->

## Why

Docket currently treats a role's model and launch posture as independent configuration, but Codex exposes collaboration tools to nested agents according to the selected model's multi-agent capability. That allowed docket-implement-next to be pinned to a Multi-Agent V1 leaf model even though its contract requires it to dispatch plan, build, and review children. The resulting failure was misdiagnosed as a general Codex nesting limitation and drove a much larger launch workaround. Docket needs a deterministic policy that makes known-incompatible coordinator assignments visible before they are shipped or installed without adding a live catalog probe to every workflow run.

## What changes

Add a typed minimum multi-agent capability to the Docket agent inventory and derive the set of dispatch-owning roles from that metadata rather than maintaining a hand-written role list or scanning prose for spawn-agent spellings. Add a bundled, versioned Codex model-capability registry recording exact model identifiers, multi-agent versions, and observation provenance. Make repository guards and Codex configuration/install generation reject a known Multi-Agent V1 model for a coordinator and accept a known V2-or-newer model. Add a dedicated read-only docket repository diagnostic that validates configured Codex roles from the bundled registry without a live call by default, plus an explicit live audit/refresh mode that compares against the installed Codex CLI's model catalog and reports additions, removals, and changed capabilities. Treat an unknown coordinator model as a warning and support an exact-model, machine-local V2 assertion for newly released models; the assertion must identify its provenance, become stale across a relevant CLI-version change, and must not override a model already known to be V1. A live contradiction is an error rather than a silent override. Document the V1 leaf/V2 coordinator rule, deterministic versus live authority, refresh procedure, warning and override behavior, and failure remedies in the README and Codex installation documentation.

## Out of scope

Querying Codex on every agent dispatch, silently rewriting model pins or the bundled registry, allowing an operator assertion to contradict a known V1 entry, treating family names such as Luna or Terra as the capability contract, restoring native coordinator routing, changing feature-worktree launch semantics, or removing agent.enter. This change provides policy, validation, diagnostics, fixtures, and documentation; production routing changes remain dependent follow-up work.
