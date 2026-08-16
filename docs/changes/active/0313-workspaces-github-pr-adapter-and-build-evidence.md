---
id: 313
slug: workspaces-github-pr-adapter-and-build-evidence
title: 'Workspaces, GitHub PR adapter, and build evidence'
status: in-progress
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-16
claimed_at: 2026-08-16T17:02:05Z
depends_on: [308, 309]
stacked_on:
related: [170, 206, 208]
discovered_from: [303]
adrs: [34, 66, 83, 92]
spec: docs/superpowers/specs/2026-08-16-workspaces-github-pr-adapter-and-build-evidence-design.md
plan: docs/superpowers/plans/2026-08-16-workspaces-github-pr-adapter-and-build-evidence.md
results:
trivial: false
auto_groomable:
branch: feat/workspaces-github-pr-adapter-and-build-evidence
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-16-workspaces-github-pr-adapter-and-build-evidence-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-16-workspaces-github-pr-adapter-and-build-evidence-design.md) |
| Plan | [2026-08-16-workspaces-github-pr-adapter-and-build-evidence.md](https://github.com/danielhanold/docket/blob/feat/workspaces-github-pr-adapter-and-build-evidence/docs/superpowers/plans/2026-08-16-workspaces-github-pr-adapter-and-build-evidence.md) |
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

### 2026-08-16 — reconcile before build

Verified the spec's landed-foundation assumptions against current `main`. All hold:

- `internal/gitcli` (change 0308) exists with `Client`, `AddDetachedWorktree`, `RemoveWorktree`,
  `ListWorktrees`, `PushLease`/`IsAncestor`, `FetchBranch`/`ResolveRef`/`RemoteDefaultBranch`,
  `WithExecutable` injection, and typed values (`ObjectID`, `RefName`, `RemoteName`, `Repository`).
- `domain.ResolveEffectiveBase(Snapshot, Change, BranchFacts) EffectiveBase` exists in
  `internal/domain/stack.go` with the tagged `EffectiveBaseKind` set (`BaseResolved`, …); ADR-0092
  done-vs-stacked-merged rule implemented there. The spec consumes it — no drift.
- `internal/repository/transaction` (change 0309) exists (`Engine`, `NewEngine`, `Execute`, typed
  `Result`/`Disposition`/`Stage`/`Kind`/`Failure`); left untouched by this change as specified.
- `internal/document` whole-population marker validation + loss-preserving patch API present for
  `evidence` to reuse.
- Target packages `internal/workspace`, `internal/githubcli`, `internal/evidence` are all absent —
  net-new as the spec states.

Two planning watch-outs (already inside this change's scope, folded into planning — not scope
changes): (1) the git adapter's only worktree primitive is `AddDetachedWorktree` (detached HEAD); the
named branch-attached worktree add + non-forcing removal + remote-ref probe are genuinely new
`gitcli` primitives to build, as §"Feature-branch publication" already enumerates. (2) No reusable
executable-fake harness exists — gitcli tests drive real temp Git repos with a `requireGit` skip; the
protocol-faithful fake `gh` executable and its witness tests are net-new work this change owns via
`Client.WithExecutable`-style injection, as §"Testing strategy" already requires.

No obsolescence, no fundamental invalidation, no scope reduction. Design carried forward unchanged.
Auto-capture: no independently-valuable beyond-branch work surfaced (both watch-outs are in-scope);
nothing minted.
