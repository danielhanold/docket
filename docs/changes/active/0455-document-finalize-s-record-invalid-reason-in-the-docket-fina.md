---
id: 455
slug: 'document-finalize-s-record-invalid-reason-in-the-docket-fina'
title: 'Document finalize''s record-invalid reason in the docket-finalize-change skill'
status: 'in-progress'
priority: 'medium'
type: 'docs'
created: '2026-09-24'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [449]
discovered_from: [449]
adrs: [127]
spec: 'docs/superpowers/specs/2026-09-27-document-finalize-s-record-invalid-reason-in-the-docket-fina-design.md'
plan: 'docs/superpowers/plans/2026-09-27-document-finalize-s-record-invalid-reason-in-the-docket-fina.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'docs/document-finalize-s-record-invalid-reason-in-the-docket-fina'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-27T10:00:41Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-27-document-finalize-s-record-invalid-reason-in-the-docket-fina-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-27-document-finalize-s-record-invalid-reason-in-the-docket-fina-design.md) |
| Plan | [2026-09-27-document-finalize-s-record-invalid-reason-in-the-docket-fina.md](https://github.com/danielhanold/docket/blob/docs/document-finalize-s-record-invalid-reason-in-the-docket-fina/docs/superpowers/plans/2026-09-27-document-finalize-s-record-invalid-reason-in-the-docket-fina.md) |
| ADRs | [ADR-0127](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0127-scoped-metadata-validation-for-named-operations.md) |
<!-- docket:artifacts:end -->

## Why

Change 0449 made two GitHub-effect operations validate the named change first and refuse with a new `record-invalid` reason: `finalize.merge` (driven by `docket-finalize-change`) and `pr.publish` (driven by `docket-implement-next`). Neither skill documents the reason, so an operator or agent that hits it has no documented remedy.

The check covers the change and the records it structurally requires (`depends_on` targets, stack ancestors). It does not cover merely related ones, and the refusal's `findings` name the bad records. One case is confusing on its own: a PR already merged outside docket, with a defective record in scope, now reports `record-invalid` where it used to report `already-merged`.

## What changes

- Document `record-invalid` in `docket-finalize-change` (merge step) and `docket-implement-next` (PR publish), with its structural scope and remedy: repair the records the refusal's `findings` name, then re-run. Explain the merged-outside-docket case as expected behavior.
- Fix the unrelated `gofmt` drift in `internal/githubcli/comment_integration_test.go`, bundled here only as a tracked home.

Detailed design is in the linked spec.

## Out of scope

Changing either operation's behavior or the precedence between `record-invalid` and `already-merged`. A real-gate end-to-end finalize test with a broken record (0449 accepted this as a known limit). A guard pinning skill-prose reason tokens.

## Reconcile log

### 2026-09-27

2026-09-27 — Reconciled against main 4d782afba. Scope unchanged: record-invalid is still undocumented in both skills (grep finds no hit under skills/), ReasonMergeRecordInvalid / ReasonPRRecordInvalid still exist as specified, and gofmt -l still flags internal/githubcli/comment_integration_test.go. No new constraints from recent archived changes.
