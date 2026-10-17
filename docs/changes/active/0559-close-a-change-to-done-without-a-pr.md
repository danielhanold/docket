---
id: 559
slug: 'close-a-change-to-done-without-a-pr'
title: 'Close a change to done without a PR'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [547]
discovered_from: [547]
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

Some changes deliver no code. A measurement, research, or attended-check change produces a results file and nothing else — #547 is the first: it measured whether the 1h cache TTL pin cut implement-next's cost, attached a results file confirming it, and has nothing to merge. Today a change reaches a final status only two ways: finalize after a merged PR (done), or change.kill (killed, which reads as abandoned). A completed measurement therefore either sits on the board as proposed forever or gets recorded as abandoned, and neither is true.

## What changes

A typed operation that moves a change with an attached results file to done without a PR, through the same archive transaction finalize uses (archive into archive/ with the date prefix, re-render the Artifacts block and backlinks, re-render the board, one metadata commit). It should refuse when the change has a feature branch, an open PR, or no results file, so code-bearing work can't bypass the merge path. Grooming decides the name, which source statuses it accepts (proposed only, or also in-progress), how the board and history show a PR-less done, and whether the skills (docket-status, docket-finalize-change, the results template) mention it. First user: #547.

## Out of scope

Changing how PR-bearing changes close out. Making implement-next pick up or build measurement changes. Retiring change.kill or changing what killed means.
