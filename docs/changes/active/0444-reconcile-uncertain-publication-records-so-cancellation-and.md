---
id: 444
slug: 'reconcile-uncertain-publication-records-so-cancellation-and'
title: 'Reconcile uncertain publication records so cancellation and resume can finish'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-09-23'
updated: '2026-09-23'
depends_on: []
stacked_on:
related: [313, 375, 435, 437, 441]
discovered_from: []
adrs: [118, 124]
spec: 'docs/superpowers/specs/2026-09-23-reconcile-uncertain-publication-records-so-cancellation-and-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/reconcile-uncertain-publication-records-so-cancellation-and'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-23T06:54:57Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-23-reconcile-uncertain-publication-records-so-cancellation-and-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-23-reconcile-uncertain-publication-records-so-cancellation-and-design.md) |
| ADRs | [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0124](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md) |
<!-- docket:artifacts:end -->

## Why

A publication whose result cannot be observed leaves an uncertain run-epoch journal entry. A later successful retry appends a separate completed entry without settling the original. Cancellation then repeatedly reports mutation-pending even with no running processes, and resume remains blocked waiting for confirmed cancellation. Reported by a separate diagnostic session; the current source confirms the status-only accounting gap.

## What changes

Extend the existing run-epoch publication entries with the original effect identity, and during cancellation and successful closeout settle an uncertain entry only when a later completed entry in the same epoch carries an identical identity — a local journal match with no Git/GitHub calls. Preserve fencing and resume rules, update only the matched original entry, and leave unmatched, missing or ambiguous evidence pending. Cover identical successful retries, mismatched identities, interrupted persistence and legacy entries. The linked spec records the implementation trace, prior changes and ADRs, alternatives, and regression criteria.

## Out of scope

New cancellation/resume policy, force-clearing uncertain work, publication rollback, background services, new retry layers or persistent stores, generic effect recovery, unrelated workflow redesign, bulk legacy repair, and implementation during grooming. Live remote re-observation, settling uncertain entries without a completed identical retry, and remote-destination binding are also out of scope. Entries without original identity or proof that the publication invocation returned remain fail-closed.

## Reconcile log

### 2026-09-23

2026-09-23 — Reconciled against main 442770e1 (identical to the spec design baseline). All cited symbols (AdmittedMutation, admitWorkflowMutation, mutationJournalStatus, reconcileEpochTeardown, verifyTerminalEpochQuiescence, accountCompletionMutations, validateResumeQuiescence) exist as described; related 0313/0375/0435/0437/0441 are done; no newer archived change or ADR touches publication-journal reconciliation. Scope unchanged.
