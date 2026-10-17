---
id: 559
slug: 'close-a-change-to-done-without-a-pr'
title: 'Let a change deliver a results artifact instead of a PR'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [547, 560, 561]
discovered_from: [547]
adrs: [129, 141, 148]
spec: 'docs/superpowers/specs/2026-10-17-close-a-change-to-done-without-a-pr-design.md'
plan: 'docs/superpowers/plans/2026-10-17-close-a-change-to-done-without-a-pr.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/close-a-change-to-done-without-a-pr'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-17T18:05:30Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-17-close-a-change-to-done-without-a-pr-design.md](../../superpowers/specs/2026-10-17-close-a-change-to-done-without-a-pr-design.md) |
| Plan | [2026-10-17-close-a-change-to-done-without-a-pr.md](../../superpowers/plans/2026-10-17-close-a-change-to-done-without-a-pr.md) |
| ADRs | [ADR-0129](../../adrs/0129-collision-free-docket-vocabulary.md), [ADR-0141](../../adrs/0141-build-artifacts-plan-results-evidence-live-on-the-metadata-b.md), [ADR-0148](../../adrs/0148-every-keyed-run-done-verdict-drives-the-successful-run-close.md) |
<!-- docket:artifacts:end -->

## Why

Some changes deliver no code. Research, measurements, and preparation work for a larger epic end in findings, recorded in a results artifact, with nothing to merge; #547 is the current example. Today a change reaches a final status only through finalize after a merged PR (`done`) or through `change.kill` (`killed`, which reads as abandoned), so a finished research change either stays on the board forever or is recorded as abandoned: #124 and #545 were killed although they were finished, and #423 was hand-edited to `done`. The autonomous loop cannot run such a change either, because implement-next always ends in a PR.

## What changes

Every change gets a `deliverable`: `pr` (the default) or `results-artifact`, editable through grooming along with `type` and `branch_prefix`. A new `docket change complete` operation moves a build-ready or in-progress `results-artifact` change with a valid results artifact to `done` and archives it, with no PR. implement-next gains a results-artifact ending: it does the work in the feature worktree without committing, halts at any step that needs a human, attaches the results artifact, has it independently reviewed, fixes the findings, completes the change, and cleans up its worktree. `finalize cleanup`, the sweep, the Artifacts links, the archive table header, and `run verify` learn the new ending, and the docs, skills, and an ADR record that `done` no longer implies a merge. Every no-op replay result names what already happened (`already-completed`, and finalize's bare `already` values renamed the same way). The spec holds the design.

## Out of scope

How `pr` changes close, finalize, and kill, apart from renaming finalize's bare `already` results. Reclaim. Showing `deliverable` on the board. Keeping changes that need a human away from the autonomous loop, and letting a loop continue past a halt (#560). A docket-free Markdown export of artifacts (#561). Fixing archived #423, #124, or #545, and completing #547 itself.

## Reconcile log

### 2026-10-17

2026-10-17: Groomed the same day; spec traced against current main (417ffd446). Commits since grooming touch only suiterunner and install surfaces, none of the fields, operations, or skills this change edits. No `deliverable` code exists yet; capability catalog is 15862 bytes, so one new entry still fits under the 16 KB cap. Scope unchanged.
