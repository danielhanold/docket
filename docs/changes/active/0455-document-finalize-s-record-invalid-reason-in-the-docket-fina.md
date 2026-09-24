---
id: 455
slug: 'document-finalize-s-record-invalid-reason-in-the-docket-fina'
title: 'Document finalize''s record-invalid reason in the docket-finalize-change skill'
status: 'proposed'
priority: 'medium'
type: 'docs'
created: '2026-09-24'
updated: '2026-09-24'
depends_on: []
stacked_on:
related: [449]
discovered_from: [449]
adrs: [127]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0127](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0127-scoped-metadata-validation-for-named-operations.md) |
<!-- docket:artifacts:end -->

## Why

Change 0449 made finalize's GitHub merge and PR publication check the named change's own record first and refuse with a new `record-invalid` reason. The `docket-finalize-change` skill's list of reasons doesn't mention it, so an operator or agent that hits it has no documented remedy. One case is confusing on its own: if the change's PR was already merged outside docket and its record has an error, the merge step now reports `record-invalid` where it used to report `already-merged`.

## What changes

Add `record-invalid` to the `docket-finalize-change` skill's reason prose, with its remedy: repair the change's own record, then re-run finalize. Mention the merged-outside-docket case explicitly, and explain that closeout would refuse the same defect anyway, so the earlier report is expected behavior. Also fix the gofmt drift in `internal/githubcli/comment_integration_test.go` (`gofmt -l` flags it). The drift predates 0449; it goes here only so a trivial fix has a tracked home.

## Out of scope

Changing finalize's behavior or the precedence between `record-invalid` and `already-merged`. Adding a real-gate end-to-end finalize test with a broken unrelated record, which 0449 accepted as a known limit.
