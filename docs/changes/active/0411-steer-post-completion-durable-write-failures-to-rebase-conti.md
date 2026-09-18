---
id: 411
slug: 'steer-post-completion-durable-write-failures-to-rebase-conti'
title: 'Steer post-completion durable-write failures to rebase-continue, not abort'
status: 'in-progress'
priority: 'low'
type: 'docs'
created: '2026-09-08'
updated: '2026-09-18'
depends_on: []
stacked_on:
related: [349, 396, 408, 413]
discovered_from: [349]
adrs: [105, 113]
spec: 'docs/superpowers/specs/2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti-design.md'
plan: 'docs/superpowers/plans/2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti.md'
results: 'docs/results/2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'docs/steer-post-completion-durable-write-failures-to-rebase-conti'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-18T15:15:51Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti-design.md) |
| Plan | [2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti.md](https://github.com/danielhanold/docket/blob/docs/steer-post-completion-durable-write-failures-to-rebase-conti/docs/superpowers/plans/2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti.md) |
| Results | [2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti-results.md](https://github.com/danielhanold/docket/blob/docs/steer-post-completion-durable-write-failures-to-rebase-conti/docs/results/2026-09-18-steer-post-completion-durable-write-failures-to-rebase-conti-results.md) |
| ADRs | [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md), [ADR-0113](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0113-resolver-dispatches-are-admitted-by-durable-pre-dispatch-res.md) |
<!-- docket:artifacts:end -->

## Why

A deep-review finding on change 0349 (PR #288) identified a narrow recovery hazard: an owned resolver continuation can complete or advance to another conflict, then fail to persist its reservation reconciliation. The existing continuation operation can recover that state without replaying Git or spending another resolver attempt, but its diagnostics and the finalize abort-flow guidance do not make that remedy discoverable. Aborting a completed rebase restores the original feature head and discards the local rewrite; completion of the rebase is not completion of the PR merge.

Current-source inspection locates the actionable write failures in the continuation and started-continuation reconciliation paths in internal/app/finalize_rebase.go. The original reference to internal/app/finalize_reserve.go is contextual: reservation admission occurs before dispatch and cannot establish that a rebase completed.

## What changes

- Make the three reservation-reconciliation write-failure messages identify the failed durable write and direct the operator to finalize.rebase-continue with the same change, owned attempt, and original resolved report/reservation. Distinguish whole-rebase completion from advancement to another conflict.
- Explain the recovery exception beside the resolver/abort guidance in docket-finalize-change and its gate-failure reference. Preserve the workspace on persistent write failure; keep the separate finalize.rebase route for a running suite and the existing verified abort routes for their actual failure cases.
- Keep the guidance harness-agnostic and verify the narrow boundary with injected write failures, unchanged reservation accounting, and no repeated Git continuation. Regenerate the embedded skill assets from maintained source.

## Out of scope

No changes to rebase-continue or rebase-abort state transitions, reservation admission or budget policy, receipt/protocol schemas, gate/evidence or merge policy, or generated-bundle conflict handling. No blanket retry of receipt-write-failed, automatic recovery loop, direct receipt repair, new resolver dispatch, or generic human-output renderer. Preserve the existing architecture decisions and historical artifacts; grooming stops at this linked specification.

## Reconcile log

### 2026-09-18

2026-09-18 — Reconciled against current main (3ccf9fac). Spec grounding holds unchanged: the three reservation-reconciliation write-failure sites are present in internal/app/finalize_rebase.go — the post-StageAndContinueRebase reconcile write (finalizeRebaseContinueBudgeted), and the advanced-conflict and completed-rebase branches of finalizeRebaseReconcileStarted, which currently return werr.Error() alone. Related changes 349/396/408/413 are all done; ADR-0105 and ADR-0113 already cover the decisions. Base resolves to main; no prerequisite or stacked branch. Scope, relations, and acceptance criteria unchanged; proceeding to plan and build.
