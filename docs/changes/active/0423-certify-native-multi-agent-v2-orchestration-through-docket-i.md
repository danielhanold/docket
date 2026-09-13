---
id: 423
slug: 'certify-native-multi-agent-v2-orchestration-through-docket-i'
title: 'Certify native Multi-Agent V2 orchestration through Docket ImplementNext'
status: 'proposed'
priority: 'critical'
type: 'chore'
created: '2026-09-11'
updated: '2026-09-13'
depends_on: []
stacked_on:
related: [323, 384, 393, 412, 424, 425, 426]
discovered_from: [323, 412]
adrs: [59, 60, 94, 114]
spec: 'docs/superpowers/specs/2026-09-13-certify-native-multi-agent-v2-orchestration-through-docket-i-design.md'
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
| Spec | [2026-09-13-certify-native-multi-agent-v2-orchestration-through-docket-i-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-13-certify-native-multi-agent-v2-orchestration-through-docket-i-design.md) |
| ADRs | [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md), [ADR-0114](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0114-anchor-codex-feature-scoped-role-entry-to-the-owning-worktre.md) |
<!-- docket:artifacts:end -->

## Why

Docket adopted Codex app-server root entry after real ImplementNext runs could not launch docket-plan-writer as a nested child. Subsequent controlled tests found a more specific cause: the affected coordinator was pinned to a Multi-Agent V1 model, while an otherwise equivalent Multi-Agent V2 model received collaboration tools and successfully launched the same registered child. Before Docket reverses a large body of launch architecture, it needs durable, production-shaped evidence that the distinction is model capability rather than prompt wording, reasoning effort, role complexity, or an artifact of the earlier minimal probes.

## What changes

Create a repeatable, isolated disposable Docket repository with one real build-ready candidate and run the actual installed docket-implement-next role through native named-agent dispatch on a model whose live Codex catalog reports Multi-Agent V2. Prove that the actual docket-plan-writer and one actual build-profile worker are native nested children, that they operate in the correct registered feature worktree with the startup guard intact, and that ImplementNext verifies, attaches, and consumes the committed plan before consuming the worker's completed result. No invocation of agent.enter is permitted anywhere in the POC; separate root sessions, app-server role entry, substitute agents, and parent relays do not count.

Use documented fixture-local native-routing overrides while preserving the real role bodies, skills, receipts, and worktree checks. Run a matched Multi-Agent V1 control, capture authoritative model-capability and top-level tool evidence, and distinguish a confirmed negative from an environmental failure or a contradicted hypothesis. Capture complete native lineage and launch evidence, role/config provenance, committed artifacts, Git/worktree identities, terminal drive receipts, and the deliberate bounded halt before review or PR creation. Include deterministic evidence validation and mutation tests. If the host cannot satisfy native dispatch and the worktree boundary together, report certification incomplete without weakening either requirement. Keep all experimental mutations inside disposable repositories and local remotes.

## Out of scope

Changing production dispatch routing or shipped model pins; adding the capability registry or repository diagnostic owned by change 0424; removing or deprecating agent.enter in production; completing the disposable candidate through review or PR creation; weakening feature-worktree startup guarantees; redesigning the run gate or implementing change 0412's supervisor; modifying historical certification records or ADRs; and resuming or mutating production backlog work. This change delivers evidence and a reusable native-only certification boundary.
