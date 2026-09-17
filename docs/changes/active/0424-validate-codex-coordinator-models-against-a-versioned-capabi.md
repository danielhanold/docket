---
id: 424
slug: 'validate-codex-coordinator-models-against-a-versioned-capabi'
title: 'Validate Codex coordinator models against a versioned capability registry'
status: 'in-progress'
priority: 'critical'
type: 'feat'
created: '2026-09-11'
updated: '2026-09-17'
depends_on: [423]
stacked_on: 425
related: [384, 393, 412]
discovered_from: [423]
adrs: [114]
spec: 'docs/superpowers/specs/2026-09-14-validate-codex-coordinator-models-against-a-versioned-capabi-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/validate-codex-coordinator-models-against-a-versioned-capabi'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-17T19:14:58Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-14-validate-codex-coordinator-models-against-a-versioned-capabi-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-14-validate-codex-coordinator-models-against-a-versioned-capabi-design.md) |
| ADRs | [ADR-0114](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0114-anchor-codex-feature-scoped-role-entry-to-the-owning-worktre.md) |
<!-- docket:artifacts:end -->

## Why

The original Codex nesting failure came from assigning a coordinator a model that could not perform its required native child dispatch. Completed POC 423 established the positive path with verified model assignments. The user chose to restore production native dispatch in 425 first and manually configure model/effort for dispatch-owning roles in the interim. Automated model validation is follow-up protection against configuration regressions, not a prerequisite for that working route.

## What changes

Add automated model-capability policy around 425's existing native Codex route, with production integration after 425 and a separately approved pre-merge dogfood option below. Add a typed minimum multi-agent capability to the Docket agent inventory and derive dispatch-owning roles from that metadata. Add a bundled, versioned registry of exact model identifiers, multi-agent versions and observation provenance. Installation, configuration diagnostics, and developer fixture generation reject known V1 models for dispatch-owning roles and accept known V2-or-newer assignments; this introduces no agent-entry or dispatch-time enforcement. Add a deterministic read-only configuration diagnostic and an explicit live catalog audit/refresh that reports additions, removals and changed capabilities. Unknown coordinator models warn. Exact-model repository-local V2 assertions carry provenance, become stale on relevant CLI changes and cannot override known V1 capability; they inform repository-context diagnostics and fixtures, not machine-wide installation. A live contradiction is an error in that audit report, not a persistent invalidation. Preserve operator-selected model/effort values and do not silently rewrite pins or the registry.

Delivery order is 423 → 425 → 424. Until this change is delivered, the operator uses existing configuration layers to select and regenerate suitable exact model/effort assignments, including the foreground parent and every role that dispatches through its active skills. Leaf workers need not be V2 merely because they are children. This interim practice is documented by 425; it does not need a registry implementation or a new override system. The existing design has been reconciled against 425 candidate `80e9d2f805febe7bc9907fcb56588a57160d4d40`. ADR-0119 supersedes the historical ADR-0114 reference for native dispatch and feature binding. Keep this proposal unstacked and do not write its implementation plan until the separately approved launch boundary and fresh candidate validation.

Document the V1 leaf/V2 coordinator distinction, deterministic versus live authority, registry refresh procedure, warnings, local assertions and remedies in the README and Codex installation documentation.

The user also approved preparing a pre-merge dogfood build on 425's tested PR branch. At that separately authorized launch boundary, confirm the reconciled design and fresh candidate validation, replace the dependency on 425 with `stacked_on: 425` while retaining `depends_on: [423]`, and validate the effective base. Keep the current dependencies until then. The dogfood uses a separate 424 worktree/PR and does not merge 424 into 425; production integration remains 425 first, then 424. See 425's specification for provenance, isolation, failure and finalization requirements.

## Out of scope

Restoring native dispatch or implementing feature-worktree binding (425), blocking 425 on this registry, querying Codex on every dispatch, adding model-policy enforcement to `RoleContractFor` or `agent.enter`, silently rewriting pins/registry, allowing assertions to contradict known V1 entries, inferring capability from model families or effort levels, changing coordinator routing or gate authority/ownership/continuation/cancellation/retry behavior, shared orchestration simplification from 432, and legacy agent.enter retirement (426). This change remains proposed with an approved reconciled design; implementation planning and launch await separate approval and fresh validation against the selected 425 candidate.

## Reconcile log

### 2026-09-17 — approved stacked dogfood preparation

The human approved replacing the completed-change dependency on 425 with `stacked_on: 425`, retaining `depends_on: [423]`. This supersedes the earlier keep-unstacked preparation boundary above. Candidate 425 is pinned at `70de2fc251ea9720a84bfe3a02c0f79bf15ed21e`; its only delta from the reconciled design baseline is test-comment formatting. CI run 35258671220 succeeded. The human reported fresh parent and real native review-child candidate provenance verification; that contract-required review abort was a loading check, not an implementation acceptance run. The local serial timing finding (96s against 90s) remains recorded, not waived by green CI.

Preparation does not claim this change or create its implementation plan/worktree. The human will launch native ImplementNext for explicit change 424 in a separate candidate-loaded session, using its own worktree and reviewed PR against 425's branch. Stop at the open reviewed PR; do not merge 424 into 425 or main. Preserve all gate, ownership, continuation, cancellation and budget rules. No global stable installation replacement is authorized.

### 2026-09-17

Human-approved design reconciliation against 425 candidate `80e9d2f805febe7bc9907fcb56588a57160d4d40`, following consultant critique and explicit approval of the installation-only enforcement boundary and repository-context assertion scope. Updated the existing specification in place, retaining its core registry and inventory design. Clarified live unknown values versus strict bundled schema, report-local audit contradictions, typed registry packaging, preservation of published installation state with existing recovery semantics, assumptions and focused coverage. Current source verification and independent review are distinct from historical 431 native success with human interventions; exact-candidate native validation remains outstanding before launch. The ADR-0114 relation is retained as historical context; ADR-0119 is the governing successor cited in the design. Status, dependencies, stack, lifecycle fields and generated blocks remain unchanged. No implementation plan, worktree, PR, runtime replacement or dogfood dispatch is authorized by this amendment.
