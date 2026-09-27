---
id: 455
slug: 'document-finalize-s-record-invalid-reason-in-the-docket-fina'
title: 'Document finalize''s record-invalid reason in the docket-finalize-change skill'
status: 'proposed'
priority: 'medium'
type: 'docs'
created: '2026-09-24'
updated: '2026-09-27'
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

Change 0449 made two GitHub-effect operations validate the named change first and refuse with a new `record-invalid` reason: `finalize.merge` (`ReasonMergeRecordInvalid`, `internal/app/finalize_merge.go`) and `pr.publish` (`ReasonPRRecordInvalid`, `internal/app/pr_publish.go`). Neither skill that drives them documents the reason. `docket-finalize-change` step 8 lists the merge's refusal tokens without it, and `docket-implement-next`'s "Publish the PR" paragraph never mentions it, so an operator or agent that hits it has no documented remedy.

The check covers more than the change's own record. Per ADR-0127 it also covers records the change structurally requires: its `depends_on` targets and stack ancestors. Associative links (`related`, `discovered_from`, `adrs`) are not followed. The refusal's `findings` name each bad record's code and path. One case is confusing on its own: if the PR was already merged outside docket and a record in that scope has an error, the merge step now reports `record-invalid` where it used to report `already-merged`.

Trivial: this is prose describing behavior that already shipped, plus one `gofmt` fix. There is no design question.

## What changes

- **`skills/docket-finalize-change/SKILL.md`, step 8 (merge):** add `record-invalid` to the merge's refusal tokens. It returns `blocked` before any GitHub call, and the run halts. Remedy: repair the records the result's `findings` name (the change itself, a `depends_on` target, or a stack ancestor; never a merely related record), then re-run finalize. Add one sentence on the merged-outside-docket case: it now reports `record-invalid` instead of `already-merged`. That is expected, because closeout would refuse the same defect anyway.
- **`skills/docket-implement-next/SKILL.md`, "Publish the PR":** add one clause saying `pr.publish` refuses `record-invalid` (`invalid-state`) before any GitHub call, with the same scope and remedy.
- **`internal/githubcli/comment_integration_test.go`:** run `gofmt -w` (`gofmt -l` flags it). The drift predates 0449; it is bundled here only so a trivial fix has a tracked home.

No new test or prose guard: no repoguard currently pins skill reason tokens, and adding one is out of proportion (YAGNI).

## Out of scope

Changing either operation's behavior, or the precedence between `record-invalid` and `already-merged`. Adding a real-gate end-to-end finalize test with a broken record, which 0449 accepted as a known limit. Adding a guard that pins skill-prose reason tokens.
