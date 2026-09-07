---
id: 327
slug: stack-closeout-must-prove-integration-reachability
title: 'Stacked-merged close-out can stamp `done` after a stale-worktree rebase clobbers the child — prove reachability in git, not metadata'
status: 'in-progress'
priority: high
type: fix
created: 2026-08-18
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [298, 316, 336, 369, 370]
discovered_from: []
adrs: [92]
spec: 'docs/superpowers/specs/2026-09-07-stack-closeout-must-prove-integration-reachability-design.md'
plan: 'docs/superpowers/plans/2026-09-07-stack-closeout-must-prove-integration-reachability.md'
results:
trivial: false
auto_groomable:
branch: 'fix/stack-closeout-must-prove-integration-reachability'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-07T15:08:29Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-stack-closeout-must-prove-integration-reachability-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-stack-closeout-must-prove-integration-reachability-design.md) |
| Plan | [2026-09-07-stack-closeout-must-prove-integration-reachability.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-07-stack-closeout-must-prove-integration-reachability.md) |
| ADRs | [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md) |
<!-- docket:artifacts:end -->

## Why

A stack can still be archived as done without proving that its child changes reached integration. The original 2026-08-18 incident in cet-terraform involved a stale parent worktree overwriting a child merge. The Go port has since added fresh feature-head checks and explicit rewrite leases, closing that particular route. The remaining closeout gap is independent: it verifies the root merge in Git but accepts descendant delivery from statuses and merged PR destinations alone.

The 2026-09-07 necessity check confirmed that the existing positive root-carry test archives a child with a fabricated, nonexistent merge-result ID. This change remains necessary to prevent false delivery claims and to stop publishing a rewrite that has lost carried work.

Docket now prefers rebase merges, so requiring the original child commit to remain an ancestor would reject legitimate delivery. The fix must prove preserved work after rebase/squash while refusing cases it cannot verify.

2026-09-07 — Still necessary; narrowed to the Go preservation gap after inspection of origin/main at 0d1a7e1b36af36c1f805952cd3836e84dbad5b6f. Changes 0298, 0316, 0336, 0369, and 0370 are done. No prerequisites remain. The targeted existing root-carry integration test passed with an absent child merge-result object, confirming that the current path performs no descendant Git proof. The user selected support for legitimate rebase/squash with verified preservation. Detailed proof semantics, conservative refusals, and acceptance coverage are in the linked spec.

## What changes

Add a shared Go preservation check for carried stacked changes. Accept original merge-result ancestry or an exact comparison of the child's changed Git entries; preserve the existing rebase, merge-commit, and squash policy.

Apply the check before and after a parent rebase, before publishing or merging its exact head, and when recording stacked or root closeout. A missing or unproven child prevents the whole root archive and keeps recovery resources intact. Ambiguous overlapping edits are reported as preservation unproven, with no metadata-only override.

Replace the fabricated-child positive fixture with a real stack, add negative and rewritten-history coverage, and mutation-test each enforcement boundary. Update maintained stack/finalize documentation to distinguish PR relationships from Git proof.

## Out of scope

- Restoring the historical cet-terraform incident or auditing/reopening already archived done records.
- Rebuilding the stale-workspace and explicit-lease protections already present in Go.
- Restoring Bash runtime paths, changing the stacking model, or changing merge-method preference.
- Automatic workspace resets, semantic equivalence, new configuration/override knobs, or shared proof-receipt infrastructure.
- Broad branch/workspace cleanup redesign; the existing conservative retention of carried-child resources remains a separate limitation.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-09-07

2026-09-07 — Reconciled against origin/main at 0d1a7e1b (the exact commit the spec's 2026-09-07 necessity assessment was groomed against; unchanged since). Confirmed the referenced Go symbols still exist and match the spec: FinalizeRebase and receipt/lease handling in internal/app/finalize_rebase.go; closeoutIntegrationDestination and DeriveRootCloseoutSet/proveCarry/probeDescendantFacts in internal/app/finalize_closeout.go and internal/domain/stackcloseout.go; the positive root-carry fixture with a fabricated child merge-result in internal/app/finalize_closeout_integration_test.go. Scope, out-of-scope, and relations (related [298,316,336,369,370] all done, adrs [92], depends_on []) hold as groomed — no adjustments needed. Proceeding to plan and build the shared Git preservation proof and its enforcement at rebase, publish, merge, and stacked/root closeout.
