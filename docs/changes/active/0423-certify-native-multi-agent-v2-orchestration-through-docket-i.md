---
id: 423
slug: 'certify-native-multi-agent-v2-orchestration-through-docket-i'
title: 'Certify native Multi-Agent V2 orchestration through Docket ImplementNext'
status: 'proposed'
priority: 'critical'
type: 'chore'
created: '2026-09-11'
updated: '2026-09-11'
depends_on: []
stacked_on:
related: [323, 384, 393, 412]
discovered_from: [323, 412]
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

Docket adopted Codex app-server root entry after real ImplementNext runs could not launch docket-plan-writer as a nested child. Subsequent controlled tests found a more specific cause: the affected coordinator was pinned to a Multi-Agent V1 model, while an otherwise equivalent Multi-Agent V2 model received collaboration tools and successfully launched the same registered child. Before Docket reverses a large body of launch architecture, it needs durable, production-shaped evidence that the distinction is model capability rather than prompt wording, reasoning effort, role complexity, or an artifact of the earlier minimal probes.

## What changes

Create an isolated disposable Docket repository with one real build-ready candidate and run the actual installed docket-implement-next role through ordinary native named-agent dispatch on a model whose live Codex catalog reports Multi-Agent V2. Prove from machine evidence—not coordinator prose—that ImplementNext launches the actual docket-plan-writer, receives a valid committed plan, attaches and consumes that plan, then launches and receives one real feature-worktree child before stopping at a deliberate bounded checkpoint prior to review or PR creation. Run an otherwise equivalent Multi-Agent V1 negative control and prove that the coordinator lacks callable collaboration controls. Capture Codex CLI version, model identifiers, reported multi-agent versions, session lineage and depths, tool calls and receipts, Docket artifacts, Git state, and the absence of root-coordinator agent.enter on the positive path. Make the fixture repeatable without touching the production backlog or remote.

## Out of scope

Changing production dispatch routing, changing shipped model pins, adding the capability registry or repository diagnostic, removing or deprecating agent.enter, completing the disposable candidate through review or an open PR, weakening feature-worktree entry guarantees, or redesigning the run gate. This change establishes evidence and a reusable certification boundary only.
