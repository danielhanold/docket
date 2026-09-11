---
id: 425
slug: 'restore-native-codex-dispatch-for-multi-agent-v2-docket-coor'
title: 'Restore native Codex dispatch for Multi-Agent V2 Docket coordinators'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-11'
updated: '2026-09-11'
depends_on: [423, 424]
stacked_on:
related: [393, 407, 412]
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

Docket's current Codex routing enters every root-coordinator role through docket agent enter because earlier failures appeared to show that registered nested agents could not coordinate. The production-shaped certification and capability policy should establish the narrower rule: a nested agent can coordinate when its selected model supports Multi-Agent V2, while V1 models are intentional leaves. Once that premise is proven and invalid pins are guarded, app-server root promotion should no longer be the default launch topology for Docket coordinators.

## What changes

Change the Codex adapter and generated repository dispatch contract so a coordinator whose role metadata requires Multi-Agent V2 is launched through ordinary native named-agent dispatch, with shipped model assignments satisfying the capability registry. Preserve the user's request, gate dispatch-context token, resume and continuation identities, model and effort pins, skill preload, role receipt, and foreground parent/child lifecycle. Keep the implement-next run-gate facade: the parent's keyed verdict, attribution, continuation, retry, and halt rules remain independent of the launch adapter. Preserve foreground agent.enter --worktree for feature-scoped children until native Codex dispatch can carry and verify the owning worktree. Remove automatic root-coordinator agent.enter selection and forbid a post-failure fallback that could duplicate a child after claim or mutation. Update generated AGENTS.md prose, typed launch routing, golden Codex definitions, repository guards, installation validation, and live integration coverage. Mutation-test the native route and the separation between coordinator launch, feature-worktree launch, and gate ownership. Record a successor ADR that reverses the root-coordinator portion of ADR-0114 while retaining its feature-worktree safety decision.

## Out of scope

Removing the agent.enter operation, changing the feature-worktree entry route, deleting or weakening the run gate, probing the live model catalog during each dispatch, changing Docket's workflow topology or Tier-C halt policy, completing change 0412's autonomous gate supervisor, or using a V1 model as a coordinator through an automatic compatibility fallback.
