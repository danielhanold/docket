---
id: 460
slug: 'artifact-backlink-refuses-an-absolute-change-path-with-unkno'
title: 'artifact.backlink refuses an absolute --change path with unknown-change'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-25'
updated: '2026-09-25'
depends_on: []
stacked_on:
related: []
discovered_from: [458]
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

During change 0458's implement-next run, `docket artifact backlink --change <absolute path>` returned `unknown-change`. It only succeeded with the repo-relative path, which cost the run an extra results commit. Agents naturally pass absolute paths, so the error is surprising and the message doesn't say the path form is the issue.

## What changes

Make artifact.backlink resolve an absolute --change path that points inside the repository (or its .docket metadata worktree) to the same change as the repo-relative form. If a path can't be resolved, reject it with an error that names the path problem rather than `unknown-change`. Check the --artifact flag for the same behavior. Add tests for the absolute and relative forms.

## Out of scope

Path handling in other operations beyond a check of --artifact on the same command; the gate-drive scope-closed issue found in the same run (tracked separately).
