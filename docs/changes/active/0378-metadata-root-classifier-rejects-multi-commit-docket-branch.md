---
id: 378
slug: 'metadata-root-classifier-rejects-multi-commit-docket-branch'
title: 'Shared metadata-root classifier misreads any multi-commit docket branch as foreign'
status: 'in-progress'
priority: high
type: fix
created: '2026-08-30'
updated: '2026-08-30'
depends_on: []
stacked_on:
related: [352, 363, 370, 371, 372, 377]
discovered_from: [377]
adrs: [1, 99]
spec: 'docs/superpowers/specs/2026-08-30-metadata-root-classifier-rejects-multi-commit-docket-branch-design.md'
plan:
results:
trivial: false
auto_groomable:
branch: 'fix/metadata-root-classifier-rejects-multi-commit-docket-branch'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-30T19:35:28Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-30-metadata-root-classifier-rejects-multi-commit-docket-branch-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-30-metadata-root-classifier-rejects-multi-commit-docket-branch-design.md) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

The native repository-setup services confuse the original metadata seed with the latest commit.
Their ownership checks accept only a one-commit branch, so normal docket history is reported as
foreign. This defect already exists on main and was reproduced with `docket repository check`
against this repository; it now blocks change 0377's repository preparation work.

The fix must recognize both native and older receiptless seeds without admitting unrelated
branches. This repository's receiptless root exactly matches the metadata copied from a historical
integration snapshot, demonstrating why requiring a native receipt alone would not fix the live
case. Recognizing a valid history must also remain separate from permission to replace its seed:
migration recovery must never discard commits written after that seed.

## What changes

- Verify the sole orphan root using a valid native receipt and its tree, or exact legacy seed
  equivalence, including historical source snapshots for receiptless migrations.
- Consolidate ownership verification across check, init, and migrate, with an explicit unknown
  outcome for unavailable evidence and the existing foreign-branch refusals intact.
- Preserve operation-specific create-only, lease, and recovery checks; refuse partial migration
  steps that could replace a seed after later commits or prune integration unsafely.
- Add behavioral coverage for native and legacy histories, false ownership proofs, failed probes,
  and concurrent advances; verify the real repository no longer reports false foreign ownership.
- Deliver as a standalone predecessor of 0377, whose eventual authorized continuation must reuse
  the shared verifier for repository preparation.

## Out of scope

- Implementing or resuming 0377 or 0370, changing their branches, or deleting the frozen facade.
- Metadata topology, receipt-format, or trust-management redesign; main-mode restoration;
  history rewriting or hard-coded seed allowlists.
- Bare single-root acceptance, loss of later metadata commits, or weakening operation-specific
  authorization, synchronization, and publication guards.
- Repairing unrelated live repository health or corpus findings.

## Design decisions

Approved with the human on 2026-08-30: shared root verification, native receipt-and-tree proof,
exact legacy equivalence against historical source content, and a standalone predecessor for 0377.
The linked spec defines the operation-specific safety boundaries. There are no outstanding design
questions or dependencies; grooming does not authorize implementation or continuation of 0377.

## Reconcile log

### 2026-08-30

Reconciled against current `main` and `docket` state. The design still holds without scope change:

- Confirmed the three faulty root-equals-tip ownership predicates named in the spec still exist in the app layer: `augmentCheckFacts`, `expectedInitShape`, and `metadataRootParentless` all compare the sole reachable metadata root against the current metadata tip, so any docket branch with more than one commit is misclassified `metadata-root-foreign`. The `reposetup.RootCommits`/`RootParentless` contracts already describe receipt/tree and legacy-equivalence proofs the implementation does not yet fulfill.
- Confirmed the surrounding-work assumptions: changes 0352, 0363, 0371, 0372 are `done`; 0377 is `in-progress` and halted after its first two build tasks (its prepare implementation copied the faulty predicate); 0370 remains halted behind 0377. ADR-0001 (orphan metadata branch) and ADR-0099 (one metadata topology) are unchanged.
- Scope, out-of-scope, and relations remain accurate: `depends_on` empty (no stack parent), `related: [352, 363, 370, 371, 372, 377]`, `adrs: [1, 99]`, `discovered_from: [377]`. No relation edits required.
- This change stays a standalone, independently-mergeable predecessor of 0377. Grooming already added 0378 to 0377's dependency set and this reconcile deliberately does not touch 0377's branch, halt record, or frozen build plan.
- Build with the currently functioning workflow; do not require 0377's unfinished `repository prepare` command. Stop at an open PR.
