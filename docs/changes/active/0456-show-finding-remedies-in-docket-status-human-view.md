---
id: 456
slug: 'show-finding-remedies-in-docket-status-human-view'
title: 'Show finding remedies in docket status human view'
status: 'in-progress'
priority: 'medium'
type: 'chore'
created: '2026-09-25'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [454]
discovered_from: [454]
adrs: []
spec: 'docs/superpowers/specs/2026-09-27-show-finding-remedies-in-docket-status-human-view-design.md'
plan: 'docs/superpowers/plans/2026-09-27-show-finding-remedies-in-docket-status-human-view.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/show-finding-remedies-in-docket-status-human-view'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-27T08:33:04Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-27-show-finding-remedies-in-docket-status-human-view-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-27-show-finding-remedies-in-docket-status-human-view-design.md) |
| Plan | [2026-09-27-show-finding-remedies-in-docket-status-human-view.md](https://github.com/danielhanold/docket/blob/chore/show-finding-remedies-in-docket-status-human-view/docs/superpowers/plans/2026-09-27-show-finding-remedies-in-docket-status-human-view.md) |
<!-- docket:artifacts:end -->

## Why

`docket status`'s human view hides part of the health surface its `--json` output carries. Findings of severity `notice` (today the configuration `deferred-setting`/`inert-setting` diagnostics) are never shown or counted, and each finding's remedy is dropped. Change 0454 added a `branch-malformed` finding whose remedy (a filled-in `repair-identity` command) is the actionable part, yet a human running plain `docket status` never sees it. Every sibling human renderer (`repository prepare`, `repository check`, `diagnostic config`) already prints remedies by default; status is the outlier.

## What changes

`docket status`'s human view shows every finding severity, grouped under a heading per non-empty severity in fixed order (errors, warnings, notices), and prints each finding's remedy on an indented continuation line. The health line counts all three severities; notices never make a repository unhealthy. Always on, no flag. Confined to the status human renderer and its golden tests.

## Out of scope

Changing the JSON output (including a notice summary counter), rendering `related` in the human view, the set of findings or their remedy text, other commands' renderers, and any flag to toggle the new output.

## Reconcile log

### 2026-09-27

2026-09-27 — Reconciled against main cdb425c0: internal/app/status_human.go still counts only error/warning (countFindings) and writeFinding still drops Remedy; last touched by 0363, no intervening change covers this. Related 0454 is done. Scope unchanged.
