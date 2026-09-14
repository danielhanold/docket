---
id: 424
slug: 'validate-codex-coordinator-models-against-a-versioned-capabi'
title: 'Validate Codex coordinator models against a versioned capability registry'
status: 'proposed'
priority: 'critical'
type: 'feat'
created: '2026-09-11'
updated: '2026-09-14'
depends_on: [423, 425]
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

The original Codex nesting failure came from assigning a coordinator a model that could not perform its required native child dispatch. Completed POC 423 established the positive path with verified model assignments. The user chose to restore production native dispatch in 425 first and manually configure model/effort for dispatch-owning roles in the interim. Automated model validation is follow-up protection against configuration regressions, not a prerequisite for that working route.

## What changes

After 425 lands, add automated model-capability policy to its existing native Codex route. Add a typed minimum multi-agent capability to the Docket agent inventory and derive dispatch-owning roles from that metadata. Add a bundled, versioned registry of exact model identifiers, multi-agent versions and observation provenance. Repository guards and Codex configuration/install generation reject known V1 models for dispatch-owning roles and accept known V2-or-newer assignments. Add a deterministic read-only configuration diagnostic and an explicit live catalog audit/refresh that reports additions, removals and changed capabilities. Unknown coordinator models warn; an exact-model machine-local V2 assertion carries provenance, becomes stale on relevant CLI changes and cannot override known V1 capability. A live contradiction is an error. Preserve operator-selected model/effort values and do not silently rewrite pins or the registry.

Delivery order is 423 → 425 → 424. Until this change is delivered, the operator uses existing configuration layers to select and regenerate suitable exact model/effort assignments, including the foreground parent and every role that dispatches through its active skills. Leaf workers need not be V2 merely because they are children. This interim practice is documented by 425; it does not need a registry implementation or a new override system. Groom this proposal against 425's delivered role/configuration surfaces before writing its implementation plan.

Document the V1 leaf/V2 coordinator distinction, deterministic versus live authority, registry refresh procedure, warnings, local assertions and remedies in the README and Codex installation documentation.

The user also approved preparing a pre-merge dogfood build on 425's tested PR branch. At that later launch boundary, first groom this change, replace its dependency on completed 425 with `stacked_on: 425`, and validate the effective base. Keep the current dependency until then. The dogfood uses a separate 424 worktree/PR and does not merge 424 into 425; production integration remains 425 first, then 424. See 425's specification for provenance, isolation, failure and finalization requirements.

## Out of scope

Restoring native dispatch or implementing feature-worktree binding (425), blocking 425 on this registry, querying Codex on every dispatch, silently rewriting pins/registry, allowing assertions to contradict known V1 entries, inferring capability from model families or effort levels, changing the established native routing contract, and legacy agent.enter retirement (426). This change remains proposed and needs its own design brainstorm.
