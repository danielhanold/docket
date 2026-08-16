---
id: 313
slug: workspaces-github-pr-adapter-and-build-evidence
title: 'Workspaces, GitHub PR adapter, and build evidence'
status: in-progress
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-16
claimed_at: 2026-08-16T16:48:46Z
depends_on: [308, 309]
stacked_on:
related: [170, 206, 208]
discovered_from: [303]
adrs: [34, 66, 83, 92]
spec: docs/superpowers/specs/2026-08-16-workspaces-github-pr-adapter-and-build-evidence-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/workspaces-github-pr-adapter-and-build-evidence
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-16-workspaces-github-pr-adapter-and-build-evidence-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-16-workspaces-github-pr-adapter-and-build-evidence-design.md) |
| ADRs | [ADR-0034](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0034-repo-root-anchored-to-main-worktree.md), [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md), [ADR-0083](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0083-agent-worktree-scope-is-a-declared-frontmatter-fact.md), [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md) |
<!-- docket:artifacts:end -->

## Why

Feature-branch and pull-request effects need typed, ownership-safe, idempotent mechanics before an
agent workflow can compose them reliably.

## What changes

Add named feature-branch operations to the typed Git adapter, a manifest-backed service for
preparing, inspecting, publishing, and cleanly removing one owned `.worktrees/<slug>` workspace, a
typed `gh` adapter that creates/adopts/versioned-updates one exact pull request, and a strict
build-evidence codec and exact-head verifier. Every external write follows probe, act, verify so a
retry after response loss adopts the effect instead of duplicating it.

## Out of scope

Configuration, document/domain/transaction behavior owned by changes 0305–0312; agent dispatch,
claim-to-implemented orchestration, and metadata transitions; local process supervision and gate
execution; rebase, force-push, merge/finalize, terminal cleanup policy, release, and cutover.

## Design decisions

The linked focused spec consumes `domain.ResolveEffectiveBase` rather than duplicating stack
policy, keeps long-lived feature workspaces separate from detached metadata transactions, requires
manifest plus live Git identity before any workspace cleanup, publishes feature refs without
forcing divergence, scopes every post-discovery `gh` call to an explicit repository, and recovers
ambiguous branch/PR responses by verifying the authoritative external postcondition. GitHub tests
use a protocol-faithful executable fake; live PR acceptance remains change 0317's release gate.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
