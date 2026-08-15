---
id: 308
slug: git-adapter-and-authoritative-object-source
title: 'Git adapter and authoritative object source'
status: done
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-15
depends_on: [304]
stacked_on:
related: []
discovered_from: [303]
adrs: [1, 34]
spec: docs/superpowers/specs/2026-08-13-git-adapter-and-authoritative-object-source-design.md
plan: docs/superpowers/plans/2026-08-15-git-adapter-and-authoritative-object-source.md
results: docs/results/2026-08-15-git-adapter-and-authoritative-object-source-results.md
trivial: false
auto_groomable:
branch: feat/git-adapter-and-authoritative-object-source
claimed_at: 
pr: 210
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-13-git-adapter-and-authoritative-object-source-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-git-adapter-and-authoritative-object-source-design.md) |
| Plan | [2026-08-15-git-adapter-and-authoritative-object-source.md](https://github.com/danielhanold/docket/blob/feat/git-adapter-and-authoritative-object-source/docs/superpowers/plans/2026-08-15-git-adapter-and-authoritative-object-source.md) |
| Results | [2026-08-15-git-adapter-and-authoritative-object-source-results.md](https://github.com/danielhanold/docket/blob/feat/git-adapter-and-authoritative-object-source/docs/results/2026-08-15-git-adapter-and-authoritative-object-source-results.md) |
| PR | 210 |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0034](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0034-repo-root-anchored-to-main-worktree.md) |
<!-- docket:artifacts:end -->

## Why

Authoritative reads and Git identities must be available without modifying the user's checkout or
letting Git command details leak into domain code.

## What changes

Add a typed `internal/gitcli` adapter that resolves the primary repository from any linked
worktree, executes Git directly under a controlled non-interactive environment, discovers remote
refs, fetches an exact authoritative branch revision, and exposes an immutable revision-bound
object source. Single and batch reads return exact blob bytes, Git modes, and opaque blob IDs while
leaving the invocation checkout untouched. Real temporary-repository tests cover both metadata
topologies and hostile path, environment, failure, and concurrency cases.

## Out of scope

Configuration resolution or metadata-mode selection; Markdown parsing or patching; domain and
repository snapshot assembly; metadata transaction worktrees, commits, leases, pushes, or retries;
status and health presentation; planning mutations; feature workspaces; GitHub and pull requests;
process supervision; agent workflows; finalize and recovery; installation, release, self-hosting,
and cutover behavior owned by changes 0305–0307 and 0309–0318.

## Design decisions

The approved focused design is in the linked spec. Discovery canonicalizes the primary worktree and
Git common directory without treating a linked checkout as the repository root. A targeted fetch
updates only Git's object/ref state, resolves one exact commit, and opens a source that never moves;
a later refresh creates a new source. Tree listings and multi-blob reads use NUL-safe, batch-oriented
plumbing, return blob IDs as entity versions, and never infer configuration or interpret documents.

Git execution is private to the adapter: consumers receive typed operations rather than an
arbitrary command runner. Commands use argument arrays, preserve ordinary authentication support,
remove repository/config redirection from the inherited environment, disable prompting, separate
data from diagnostics, and enforce cancellation and bounded timeouts. Failures have stable kinds
without parsing human stderr. A missing optional path is read data, while an unavailable executable,
repository, remote, ref, or malformed plumbing response is a typed failure.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- 2026-08-15 — Reconciled against origin/main. Changes 0304–0307 have all merged: `internal/app`,
  `internal/config`, `internal/document`, `internal/domain`, `internal/repository` exist; no
  `internal/gitcli` yet, so this change's scope is intact and unduplicated. Updated one stale spec
  sentence that still called 0307's domain work "pending". ADR-0092/0093 (from 0307) concern
  stacked-base semantics and repository-reference severity — no bearing on this adapter. No scope
  change; no follow-up work surfaced (auto-capture: nothing minted).
