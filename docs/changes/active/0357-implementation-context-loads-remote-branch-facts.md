---
id: 357
slug: implementation-context-loads-remote-branch-facts
title: Implementation context must load remote branch facts before judging stack base
status: proposed
priority: high
type: fix
created: 2026-08-26
updated: 2026-08-26
depends_on: []
stacked_on:
related: [356, 298]
discovered_from: []
adrs: [92]
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md) |
<!-- docket:artifacts:end -->

## Why

`docket context implementation` is the claim gate for `docket-implement-next`. `ContextImplementation` (`internal/app/implementation_context.go`) evaluates readiness with `domain.NewBranchFacts(nil)` — an empty remote-branch set. The comment says remotes are deliberately not re-read here, and that the facts-backed resolution happens inside the claim transaction.

A live stack parent (in-progress, `branch:` recorded, remote ref present) only resolves under domain rule 4 when that branch is **in facts**. With empty facts, `HasBranch` is always false, so the walk returns `branch-absent` and the child is `not-ready-stack-base-unresolved`. Claim never runs.

Observed live 2026-08-26 in scp-qarch-deploy: child 0003 stacked on stack-root 0004 (`feature/eks-consumers` on origin). `docket-status` and `stack-base.sh` (both facts-backed) reported ready / resolved; `docket context implementation --id 3` refused. Status, workspace-prepare, and claim already call `reader.BranchFacts`. Only this pre-claim bundle skips it. No parent-status edit can both pass this gate and keep the child's base on the stack root: `done` or a fake `stacked-merged` would resolve to the integration branch.

## What changes

In `ContextImplementation`, load remote branch facts the same way `Status` and workspace-prepare already do (`deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))`) and pass that set into `selectContextChange` / `EvaluateReadiness` / `ResolveEffectiveBase`. Drop the empty-facts shortcut and the comment that treats an unresolved base as safer than a fabricated one — fabricating nothing while refusing to look is what blocks every stacked child.

Pin with a test: a proposed child stacked on an in-progress parent whose `branch:` is in the fake reader's remote set must return a claimable bundle whose `EffectiveBase` is that parent branch, not `not-ready-stack-base-unresolved`. A parent whose branch is absent from facts still refuses.

## Out of scope

- Cursor wrapper `name:` quoting (change 0356).
- `aborted-run` false positives on long-lived stack roots.
- Other `NewBranchFacts(nil)` sites (reclaim, finalize cleanup) unless they also gate stacked claim.
- Changing stack-resolution rules or `stack-base.sh`.
