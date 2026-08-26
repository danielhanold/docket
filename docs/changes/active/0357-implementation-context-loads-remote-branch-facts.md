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
plan: 'docs/superpowers/plans/2026-08-26-implementation-context-remote-branch-facts.md'
results:
trivial: false
auto_groomable:
branch: 'fix/implementation-context-loads-remote-branch-facts'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-26T22:41:25Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-26-implementation-context-remote-branch-facts-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-26-implementation-context-remote-branch-facts-design.md) |
| Plan | [2026-08-26-implementation-context-remote-branch-facts.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-26-implementation-context-remote-branch-facts.md) |
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

## Run halted

### 2026-08-26

Halted 2026-08-26 (UTC) at the build gate: the full suite is reproducibly RED and no fix exists within change 357's scope. A human decision is required.

## What was built (correct, do not discard)

- Production fix landed at commit `33f5db87`: `internal/app/implementation_context.go` now loads remote branch facts via `deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))` from the pinned snapshot and threads one fact set through `selectContextChange`, `EvaluateReadiness`, and `ResolveEffectiveBase`; a facts-read error classifies through `classifyStatusError` and returns no bundle. This is exactly the spec's design and the whole point of change 357.
- Focused fake-reader tests landed in the same commit (`internal/app/implementation_context_test.go`): stacked-live-parent apply for both request shapes, absent-branch refusal, and facts-error typed failure. All pass.
- Real-Git regression landed at commit `5becf53b` (`internal/app/claim_workflow_git_test.go`): `TestStackedContextClaimWorkspaceFromParentBranch` proves the pre-claim gate applies, resolves the parent branch, claims, and prepares the workspace at the parent tip — in both metadata modes. It passes, and was mutation-verified (reverting the `BranchFacts` read reddens it at the pre-claim gate).

Reconcile (`reconciled: true`) and the plan are attached. The change's own behavior is complete and correct.

## Why the run halted

The build gate runs the whole suite (`scripts/run-tests.sh`). It is reproducibly RED, on two independent full-suite runs and one isolated run:

- Both full-suite runs failed with `panic: test timed out after 10m0s` in the Go package `internal/app` (`FAIL github.com/danielhanold/docket/internal/app` at ~506-600s), tripping `tests/test_go_toolchain.sh` (`go test ./...`) and, on the second run, also `tests/test_go_race.sh` (`go test -race ./...`).
- A premium integration-repair worker measured the package in isolation: `go test -race -count=1 ./internal/app/` -> `FAIL ... 600.315s` (643.76s wall). The package exceeds Go's default 600s per-package test timeout **even when run alone**, not merely under full-suite parallel load.

Root cause is systemic, not 357-scoped: `internal/app` is a single Go package whose real-Git integration suite is at/over Go's default 10-minute per-package timeout on this machine. The repo already documents this and assigns its durable fix — a test-level partition of `internal/app` — to follow-up **change 0333** (see `tests/test_go_race.sh` header, `tests/test_runtime_budgets.sh` RELIEF COUNTER A, `tests/runtime-budgets.tsv`). Change 357's spec-mandated `TestStackedContextClaimWorkspaceFromParentBranch` (~14-17s, both metadata modes) is a legitimately-sized addition that tips an already-near/over-budget package past the timeout.

## Why no in-scope fix exists

Every real remedy is out of change 357's scope or forbidden by the build contract:

- **Partition `internal/app`** into multiple packages/test binaries — this is change 0333's charter, a cross-cutting refactor beyond change 357.
- **Raise the `go test` timeout or cap GOMAXPROCS** in `tests/test_go_toolchain.sh` / `tests/test_go_race.sh` — suite-runner infrastructure (change 0304/0333 territory), out of change 357's scope.
- **Drop a metadata mode or weaken coverage** in 357's own regression — forbidden: the spec explicitly requires both metadata modes plus the focused and real-Git coverage; and the build contract forbids weakening tests.

The only coverage-preserving in-scope trim the repair worker found — eliminating one redundant `ContextImplementation` "claim-read" call in `claim_workflow_git_test.go` — saves ~2s, which does not bring the package to comfortable margin under the 600s timeout. Even fully removing 357's ~15s test would leave the package at ~585s, still razor-thin. The repair worker returned BLOCKED (not NEEDS_ESCALATION): a stronger model faces the identical structural wall.

## Recommended human decision (one of)

1. Sequence change **0333** (partition `internal/app`) ahead of 357, then resume 357's build gate; or
2. Grant an explicit out-of-scope allowance to adjust the per-package `go test` timeout and/or parallelism in the suite runner (0304/0333 territory), then resume; or
3. Accept single-mode coverage for 357's real-Git regression (a spec amendment), then resume.

None of these is decidable within change 357's autonomous build. The branch (`fix/implementation-context-loads-remote-branch-facts`, head `5becf53b`), its claim/lease, and the workspace are left intact for a resumed run.
