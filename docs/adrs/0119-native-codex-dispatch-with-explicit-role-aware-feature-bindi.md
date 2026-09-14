---
id: 119
slug: 'native-codex-dispatch-with-explicit-role-aware-feature-bindi'
title: 'Native Codex dispatch with explicit, role-aware feature binding'
status: 'Accepted'
date: '2026-09-14'
supersedes: [114]
reverses: []
relates_to: [103, 83]
change: 425
---

## Context

ADR-0114 addressed a real cross-worktree resolver failure by selecting app-server root entry and startup-cwd equality. Accepted change-423 evidence now proves a continuous native coordinator, planner, and worker flow in which children inherit primary startup and bind to an explicitly validated fresh feature. The earlier failures involved incomplete inputs, scope identity, lost or misparsed replies, and hidden templates; they did not establish that native dispatch was unavailable. Production must validate those boundaries and test generated assets.

## Decision

Codex uses registered native named-agent dispatch for every role. Typed launch and scope metadata remains shared, but Codex does not translate it into automatic agent.enter. A pinned role assignment is validated against registered repository and workspace identity before feature reads or writes; primary startup is permitted and later feature operations target explicit absolute paths. Planner resources and the results template are explicit, while immutable assignments remain separate from live worker authority. A small read-only Go validation boundary reuses existing workspace and gate backends and public gate response shapes. It does not read private gate records through a second transport or launch agents. Existing gate ownership, continuation, and cancellation remain authoritative. Other harness behavior and model policy are unchanged.

## Consequences

Correct feature work no longer depends on an unsupported native cwd option. Integrity checks and explicit path discipline do not provide a sandbox, so telemetry limits remain visible. Operator-selected exact model and effort pins remain a launch prerequisite until change 424. Candidate production assets and native review require fresh acceptance; change 423 remains design evidence. Legacy runner retirement remains change 426. Missing resources, tools, identity, or ownership halt instead of selecting another runtime.

## Alternatives considered

Keep root entry, which conflicts with restored native coordination; assume startup equals the target, which fails observed host behavior; trust instructions without checked assignments, repeating cross-tree and incomplete-input failures; ship the proof-of-concept private-record and scripting glue, duplicating backend authority and dependencies; add a model registry now, which belongs to change 424; or build a sandbox or tracer beyond the required observation contract.
