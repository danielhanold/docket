---
id: 478
slug: 'gofmt-internal-githubcli-comment-integration-test-go'
title: 'gofmt internal/githubcli/comment_integration_test.go'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: []
stacked_on:
related: []
discovered_from: [471]
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

internal/githubcli/comment_integration_test.go already failed `gofmt` before change 0471 and was deliberately left alone there so that change's diff stayed purely about names. Nothing else tracks it, so it would otherwise be forgotten.

## What changes

Run `gofmt -w` on internal/githubcli/comment_integration_test.go and confirm the whole suite still passes. Formatting only; no behavior change.

## Out of scope

Any other file or any non-formatting edit; changing how `gofmt` is enforced in the suite.
