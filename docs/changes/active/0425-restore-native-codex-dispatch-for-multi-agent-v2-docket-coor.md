---
id: 425
slug: 'restore-native-codex-dispatch-for-multi-agent-v2-docket-coor'
title: 'Restore native Codex dispatch for Multi-Agent V2 Docket coordinators'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-11'
updated: '2026-09-14'
depends_on: [423, 424]
stacked_on:
related: [393, 407, 412, 426]
discovered_from: [423]
adrs: [114]
spec: 'docs/superpowers/specs/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-design.md'
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
| Spec | [2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-design.md) |
| ADRs | [ADR-0114](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0114-anchor-codex-feature-scoped-role-entry-to-the-owning-worktre.md) |
<!-- docket:artifacts:end -->

## Why

Docket adopted agent.enter after nested coordination and feature-placement failures were generalized into Codex limitations. The POC narrowed both: native coordination works with capable model assignments, and the accepted option2 validates an assigned feature worktree before explicitly targeting feature work. The successful focused worker evidence supports this path; the final continuous certification remains a dependency. Production should retain gate, handoff and worktree checks while using native dispatch.

## What changes

Make native named-agent dispatch the Codex route for capable coordinators and feature planner, build and review children. Consume424 capability policy. Replace startup-cwd equality with verified explicit feature targeting; preserve credential-free fixed assignment inputs, complete real planner skill payloads, private dynamic child capabilities, outer gate context and any supplied epoch, independent catalogs, handoff/claim/acknowledgement, exact-commit gates and results checkpoints. Add ownership-aware diagnostics for correctly feature-only attached plans. Disclose mechanics through Codex-specific adapter and skill/agent references; preserve common behavior and other harnesses. Record a successor ADR to114;426 owns broader legacy retirement.

## Out of scope

Native startup-directory support, obscure host workarounds, automatic or fallback agent.enter, custom runners/relays/root relocation/generic substitutes, changing other harnesses or gate ownership/model capability authority, per-dispatch model probing, implementing426 legacy removal, hard-isolation or parallel-certification claims. Production425 remains dependent on accepted423 evidence and424.
