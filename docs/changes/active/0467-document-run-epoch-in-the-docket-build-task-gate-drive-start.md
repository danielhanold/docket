---
id: 467
slug: 'document-run-epoch-in-the-docket-build-task-gate-drive-start'
title: 'Scoped gate starts inherit the run epoch; thread it through the build chain'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-28'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [461, 463]
discovered_from: [461]
adrs: [111]
spec: 'docs/superpowers/specs/2026-09-28-document-run-epoch-in-the-docket-build-task-gate-drive-start-design.md'
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
| Spec | [2026-09-28-document-run-epoch-in-the-docket-build-task-gate-drive-start-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-28-document-run-epoch-in-the-docket-build-task-gate-drive-start-design.md) |
| ADRs | [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md) |
<!-- docket:artifacts:end -->

## Why

During change 0461's implement-next run, a build-task worker's `gate.drive.start` was refused `stale-run-epoch` until the worker improvised `--run-epoch`. Grooming traced a wider gap: the run epoch the parent's arm prints never reaches the child at all — the managed run-gate block copies only the dispatch context into the prompt, and `docket-implement-next`, `docket-build`, and `docket-build-task` never mention the epoch (no `--run-epoch` on `prepare-scope`, the scope bundle, or the build-owner starts). Separately, the driver documents that a scope's pinned epoch travels onto each scoped start, but the scoped start actually uses only the caller-presented value, so a worker that omits it is refused against an epoch-owned worktree.

## What changes

- **Driver:** a scoped `gate.drive.start` inherits the run epoch its scope pinned at `prepare-scope`; a presented epoch that differs from a pinned one refuses `scope-identity-mismatch`; scopes with no epoch behave as today. Workers never handle the epoch.
- **Prose:** the managed run-gate block copies the epoch into the implement-next dispatch prompt; `docket-implement-next` and `docket-build` pass `--run-epoch` to every `gate.drive.prepare-scope` and every build-owner `gate.drive.start`; `docket-build-task` states the epoch rides on the scope. Regenerate the managed block and embedded copies.
- **Guard:** a repoguard prose-contract test that those `prepare-scope` and build-owner start sites carry `--run-epoch`, mutation-tested.

## Out of scope

Changing the run-epoch fence, `stale-run-epoch` semantics, epoch settlement, or scope-less start behaviour. Deriving the epoch from the dispatch-context token (deferred; see spec). The foreign-incumbent epoch named in 0461's refusal is the expected consequence of an empty presented epoch, not a separate defect.
