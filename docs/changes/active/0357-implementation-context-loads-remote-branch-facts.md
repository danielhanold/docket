---
id: 357
slug: implementation-context-loads-remote-branch-facts
title: Implementation context must load remote branch facts before judging stack base
status: 'in-progress'
priority: high
type: fix
created: 2026-08-26
updated: '2026-08-26'
depends_on: []
stacked_on:
related: [298, 316, 327, 347, 356]
discovered_from: []
adrs: [92]
spec: docs/superpowers/specs/2026-08-26-implementation-context-remote-branch-facts-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'fix/implementation-context-loads-remote-branch-facts'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-26T22:12:11Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-26-implementation-context-remote-branch-facts-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-26-implementation-context-remote-branch-facts-design.md) |
| ADRs | [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md) |
<!-- docket:artifacts:end -->

## Why

`docket context implementation` is the claim gate for `docket-implement-next`. `ContextImplementation` (`internal/app/implementation_context.go`) evaluates readiness with `domain.NewBranchFacts(nil)` — an empty remote-branch set. The comment says remotes are deliberately not re-read here, and that the facts-backed resolution happens inside the claim transaction.

A live stack parent (in-progress, `branch:` recorded, remote ref present) only resolves under domain rule 4 when that branch is **in facts**. With empty facts, `HasBranch` is always false, so the walk returns `branch-absent` and the child is `not-ready-stack-base-unresolved`. Claim never runs.

Observed live 2026-08-26 in scp-qarch-deploy: child 0003 stacked on stack-root 0004 (`feature/eks-consumers` on origin). `docket-status` and `stack-base.sh` (both facts-backed) reported ready / resolved; `docket context implementation --id 3` refused. Status, workspace-prepare, and claim already call `reader.BranchFacts`. Only this pre-claim bundle skips it. No parent-status edit can both pass this gate and keep the child's base on the stack root: `done` or a fake `stacked-merged` would resolve to the integration branch.

## What changes

Load remote branch facts in `ContextImplementation` from the same pinned snapshot and through the same reader seam used by status, claim, and workspace preparation. Use that one fact set for automatic selection, explicit-id eligibility, readiness, and effective-base reporting. A facts read that fails stops with the existing typed reader failure; a genuinely absent parent branch remains unresolved.

Cover the orchestration at the fake-reader seam and with a real-Git workflow regression. The real path must show that a proposed child stacked on a live parent with a pushed recorded branch passes implementation context, can be claimed, and prepares its workspace from the parent branch rather than the integration branch. Full design: see the linked spec.

## Out of scope

- Cursor wrapper `name:` quoting (change 0356).
- `aborted-run` false positives on long-lived stack roots.
- Other `NewBranchFacts(nil)` sites (reclaim, finalize cleanup) unless they also gate stacked claim.
- Changing stack-resolution rules or `stack-base.sh`.

## Open questions

None — settled in the 2026-08-26 grooming session. The regression boundary includes both focused unit coverage and a real-Git workflow proof.

## Reconcile log

### 2026-08-26

2026-08-26 — Reconciled against current `main`. `internal/app/implementation_context.go` still builds an empty fact set at the `facts := domain.NewBranchFacts(nil)` seam (with the stale comment) and threads it through `selectContextChange`, `EvaluateReadiness`, and `ResolveEffectiveBase` — exactly the defect the spec describes; no upstream change has fixed or moved it. The reader seam is unchanged: `deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))` is the established call shape (change_claim.go, workspace_ops.go, status.go, finalize_*.go), and `classifyStatusError` is the existing typed-failure path. Cited ADR-0092 remains Accepted and its rules are untouched by this change. Related changes 298/316/327/347/356 remain out of scope per the spec. Design holds as written; no scope adjustment. Proceeding to plan and build.
