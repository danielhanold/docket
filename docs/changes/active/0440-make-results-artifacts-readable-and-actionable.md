---
id: 440
slug: 'make-results-artifacts-readable-and-actionable'
title: 'Make results artifacts readable and actionable'
status: 'in-progress'
priority: 'medium'
type: 'refactor'
created: '2026-09-21'
updated: '2026-09-21'
depends_on: []
stacked_on:
related: [1, 190, 330, 374, 410]
discovered_from: []
adrs: [102]
spec: 'docs/superpowers/specs/2026-09-21-make-results-artifacts-readable-and-actionable-design.md'
plan: 'docs/superpowers/plans/2026-09-21-make-results-artifacts-readable-and-actionable.md'
results: 'docs/results/2026-09-21-make-results-artifacts-readable-and-actionable-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/make-results-artifacts-readable-and-actionable'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-21T13:05:16Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-21-make-results-artifacts-readable-and-actionable-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-21-make-results-artifacts-readable-and-actionable-design.md) |
| Plan | [2026-09-21-make-results-artifacts-readable-and-actionable.md](https://github.com/danielhanold/docket/blob/refactor/make-results-artifacts-readable-and-actionable/docs/superpowers/plans/2026-09-21-make-results-artifacts-readable-and-actionable.md) |
| Results | [2026-09-21-make-results-artifacts-readable-and-actionable-results.md](https://github.com/danielhanold/docket/blob/refactor/make-results-artifacts-readable-and-actionable/docs/results/2026-09-21-make-results-artifacts-readable-and-actionable-results.md) |
| ADRs | [ADR-0102](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0102-build-and-finalize-own-independent-gate-and-test-command-con.md) |
<!-- docket:artifacts:end -->

## Why

Results artifacts currently emphasize Docket implementation details and provide too little practical help to a mid-level engineer reviewing a delivered change. Humans need a clear account of the behavior, required actions, useful functional checks, and unresolved problems.

## What changes

Adopt the approved human-readable results specification: a short action statement at the top; behavior-focused outcomes; Important and Optional human checks with complete setup, steps, expected results, and cleanup; concise verification; and one plain-language known-issues and follow-ups section. Allow optional walkthroughs of automated behavior and link deeper technical detail. Strengthen shared authoring guidance and structural validation while preserving existing artifact and evidence contracts.

## Out of scope

Implementation during this capture; retrospective rewriting of historical results; changes to checkpoint ownership, exact-head evidence, merge policy, or post-merge behavior; readability scoring; new review rounds, lifecycle states, or configuration; automatic follow-up creation.

## Reconcile log

### 2026-09-21

2026-09-21: Reconciled against current code. The approved spec still matches reality: the canonical results template (skills/docket-implement-next/results-template.md) carries the old section set (Outcome / Human testing / Verification performed / Findings and limitations / Follow-ups), the shared Go structural validator (internal/app/results_content.go, ValidateResultsContent) enforces the H1 title, no-placeholder, required substantive ## Outcome, and no empty/filler-section checks but has no action-statement check, and the embedded copy (internal/assets/embedded/tree/skills/docket-implement-next/results-template.md) is regenerated via cmd/genassets with a suite drift gate. Implementation surface: (1) rewrite the template to the new reading order (Human action statement -> Outcome -> Human actions and testing -> Verification performed -> Known issues and follow-ups); (2) update authoring/convention guidance in docket-implement-next Step 6.5 and docket-convention results guidance, plus any maintained references, and reconcile the prohibition on manually checking automated behavior; (3) extend ValidateResultsContent (ResultsPhaseFinal) with a substantive-action-statement check near the top while preserving Outcome/placeholder/empty/filler checks; (4) regenerate the embedded bundle. No scope, dependency, or relation change required; related [1,190,330,374,410] and adrs [102] remain correct.
