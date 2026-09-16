---
id: 432
slug: 'complete-native-codex-runner'
title: 'Complete native Codex runner'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-16'
updated: '2026-09-16'
depends_on: []
stacked_on: 431
related: [425, 412]
discovered_from: [431]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

Native Codex docket-implement-next still cannot complete a verified workflow despite the implemented 425 dispatch repairs and repeated 431 acceptance runs. The latest run passed the scoped build and published results but halted at final-certification continuation because the returned handoff lacked the required single-use token and the continuation had no parent capability. Completion of Codex compatibility is a bounded goal distinct from shared machinery simplification and the supervisor proposed in 412.

## What changes

Finish native Codex dispatch, actual child completion observation, complete receipt transport, supported continuation, exact-HEAD certification, native review, results/evidence verification and PR completion under the existing Docket contract. Reproduce every observed failure at its actual CLI/native boundary and rehearse the complete remaining workflow before another acceptance attempt. Preserve 431 as the acceptance case, including its completed plan, worker, published results and counters.

Branch provenance (user-approved): fix/complete-native-codex-runner was branched from change 431's clean local and published remote tip ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8 on 2026-09-16, not from main or merely the 425 repair branch. Source branch: chore/native-codex-acceptance-for-active-worker-validation. Both tips were verified equal before creation. The starting commit includes repair 95660e8fee6e08ba1f439a0283dba4f0364e7d0a, plan 95842609f2a0b1c9c5e8646e8cba8d8a105babd7, worker bbe70004a97f716dfa197d497985ef8930433169, and 431's published results. The branch was pre-created at the human's explicit request; preserve it when establishing future claim/workspace ownership, never replace it from main. Its intended PR base is 431's branch while that parent remains unmerged.

Track deferred shared simplification in docs/changes/research/shared-orchestration-simplification.md, and supervisor findings in docs/changes/research/0412-supervisor-findings.md linked to existing change 412. These are discovery notes, not prerequisites or implementation authorization.

## Out of scope

No new acceptance fixture/change, no restart or deletion of completed 431 implementation, no reset of budgets or ownership records, no automatic PR merge, no shared orchestration redesign, and no implementation of 412. No weakening of validation or routing native roles through agent.enter, generic agents, shell runners or another harness. Fix a shared defect only when necessary to satisfy the existing Codex contract, with cross-harness regression coverage; surface contract changes separately.
