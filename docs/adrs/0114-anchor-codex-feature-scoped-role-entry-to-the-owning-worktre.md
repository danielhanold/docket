---
id: 114
slug: 'anchor-codex-feature-scoped-role-entry-to-the-owning-worktre'
title: 'Anchor Codex feature-scoped role entry to the owning worktree'
status: 'Accepted'
date: '2026-09-08'
supersedes: [103]
reverses: []
relates_to: [83, 103]
change: 393
---

## Context

ADR-0103 correctly selected foreground app-server root-thread entry for parent-facing root coordinators, but its statement that every ordinary role uses native child dispatch assumed that native child dispatch preserved the owning workflow's worktree. Finalizing change 0349 through change 0393 disproved that assumption: the coordinator ran in change 0393's worktree while the owned rebase conflict lived in change 0349's worktree, so the resolver inspected an empty index rather than the live unmerged entries. The worktree is a capability boundary, not incidental launch context. ADR-0083's agent-layer separation keeps the workflow edge, role behavior, and harness launch mechanics distinct; this decision changes only the Codex harness's entry route for a role whose typed scope is feature.

## Decision

Keep ADR-0103's root-coordinator decision: parent-facing root coordinators enter through foreground app-server `agent.enter` at the caller's absolute cwd. Add required typed `worktree-scope: feature|metadata` to the shared agent inventory and role contract, and route ordinary roles by that scope: (1) metadata-scoped ordinary children use native named-agent dispatch; (2) feature-scoped ordinary children use foreground app-server `agent.enter --worktree <absolute canonical feature-worktree root>` after verifying that the target is an existing registered non-primary worktree of the same repository; and (3) root coordinators use caller-cwd entry. The owning workflow supplies the feature root after it resolves typed context or workspace state; the child request remains unchanged, except that feature-role payloads name the same absolute root and the role halts before reads or writes if its actual cwd does not equal it. Change id, caller cwd, role-name pattern, request prose, `codex exec`, runner resurrection, and parent relay are not authority for selecting or reconstructing that root: each either lacks the verified worktree identity or changes the selected native launch contract. The installed role contract remains the single source for instructions, model, effort, skills, and foreground completion.

## Consequences

The root-coordinator route remains intact and metadata children retain their native dispatch contract. Every feature-scoped ordinary role, including plan writer, build profiles, review rungs, rebase resolver, and integration repair, receives the verified worktree as its actual app-server cwd. A missing, foreign, primary, nonexistent, nested, or mismatched worktree fails visibly before Git reads or writes, preventing a resolver from silently inspecting its coordinator's tree. Generated Codex dispatch policy and startup guards must derive scope from inventory rather than role-name lists, and tests must cover the three-way matrix, invalid worktree rejection, exact thread cwd, and a real cross-worktree conflict where the resolver observes the selected worktree's unmerged index. This is a narrow reuse of app-server entry; it does not introduce a generic runner, relay protocol, broad delegation grant, or change identity flag.

## Alternatives considered

Keep native child dispatch for all ordinary roles: rejected because it has no working-directory channel and reproduced the finalize conflict failure. Infer a worktree from change id, caller cwd, role-name pattern, or prose: rejected because those are hints, not verified runtime authority, and can identify a different change's tree. Add a change id or worktree inference flag to `agent.enter`: rejected because workflow identity remains in the payload and the operation must receive explicit verified scope. Use `codex exec`, resurrect another runner, or substitute another harness: rejected because each creates a different session and does not preserve the selected app-server role contract. Add a typed parent relay: rejected because it inserts the parent into child edges and creates continuation state instead of restoring the direct feature-child contract.
