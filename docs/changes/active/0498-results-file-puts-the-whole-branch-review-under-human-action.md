---
id: 498
slug: 'results-file-puts-the-whole-branch-review-under-human-action'
title: 'Results file puts the whole-branch review under Human actions and testing'
status: 'in-progress'
priority: 'low'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [410, 440]
discovered_from: [494]
adrs: []
spec: 'docs/superpowers/specs/2026-10-04-results-file-puts-the-whole-branch-review-under-human-action-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/results-file-puts-the-whole-branch-review-under-human-action'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-04T06:09:12Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-results-file-puts-the-whole-branch-review-under-human-action-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-results-file-puts-the-whole-branch-review-under-human-action-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0494's results file put its `### Whole-branch review` subsection inside `## Human actions and testing`. That section is for things a human should do or check, so filing review outcomes there made it look heavier than it was and hid the review where a reader won't look for it.

The cause is a guidance gap, not a one-off: implement-next's Step 6.5 and `references/fix-loop.md` tell the run to write review findings into the results file but never name a section, and the template only says where fixed findings should *not* go. Runs improvise in both directions — 0494 used Human actions; three other recent results files filed "Fixed after review" entries under Known issues. Meanwhile the PR body already carries the full review disposition table, so the full per-finding list in the final results file is a second copy of code-level detail.

## What changes

Make the PR body the only home for the full review table, and give the final results file a one-line review summary:

- **Verification performed** gets one line saying which review ran and how its findings ended (e.g. "5 findings, all fixed in-branch; full table in the PR body").
- **Known issues and follow-ups** gets an entry for every finding left unfixed or reported as follow-up work, plus any fixed finding that still leaves a real risk (the existing rule).
- During the build, the full findings may still sit in the results file so they survive a halt before the PR exists; the final write condenses them to the summary line.
- **Human actions and testing** is stated to hold only what a human should do or check, never a record of what the run already checked.

The wording lands in the results template, Step 6.5, and `fix-loop.md` (plus the regenerated embedded copies), pinned by a mutation-tested prose-contract row.

## Out of scope

- Any validator, health check, or merge-boundary refusal on which section holds what — guidance only, no new blocking gate.
- Editing 0494's or any other merged results file — merged plans and results are frozen build records.
- Adding, removing, or reordering results sections, or changing the PR-body disposition table.
- An ADR — this is a placement rule inside the existing 0410/0440 results design.

## Reconcile log

### 2026-10-04

Reconciled against main 20bc0a36a. The anchors the spec cites are unchanged: fix-loop.md still carries the retired sentence (results-checkpoint linkage), results-template.md still has the `## Verification performed` guidance, and the `change_0440_*` prose-contract rows remain the house pattern. Related 0410/0440 are done; no recent change (0494-0496) touched these files' placement guidance. Scope unchanged; no ADR.
