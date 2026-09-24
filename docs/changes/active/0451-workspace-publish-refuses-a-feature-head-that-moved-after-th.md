---
id: 451
slug: 'workspace-publish-refuses-a-feature-head-that-moved-after-th'
title: 'Workspace publish refuses a feature head that moved after the app-level check'
status: 'in-progress'
priority: 'low'
type: 'chore'
created: '2026-09-24'
updated: '2026-09-24'
depends_on: [444]
stacked_on:
related: [444]
discovered_from: [444]
adrs: []
spec:
plan:
results:
trivial: true
auto_groomable:
branch_prefix:
branch: 'chore/workspace-publish-refuses-a-feature-head-that-moved-after-th'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-24T06:22:25Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`WorkspacePublish` (internal/app/workspace_ops.go) checks that the worktree head equals the caller's expected head. It then admits the run-epoch mutation and calls `workspace.Service.PublishHead`, which re-reads the local head under its own lock and pushes whatever it finds. If a commit lands in between, the newer head is pushed. The window is milliseconds, and only a writer outside the run (a person, or an editor or agent inside `.worktrees/`) can hit it. The push is fast-forward-only with a lease, so nothing on the remote is overwritten. Change 0444 already logs such a push as unverified so it can never settle an uncertain journal entry, and `run.verify` reports `evidence-unverified` at the new head. The PR path already refuses a moved head through its expected-head check, so the two publish paths should behave the same. Found in change 0444's review (results file, Known issues).

**Trivial:** the fix adds one request field, one comparison under the lock that already exists, and one test. It follows the moved-head refusal the PR path already has, with no design choices open, so no spec is needed.

## What changes

- Add an expected-head field to `workspace.PublishRequest` (internal/workspace/publish.go).
- In `PublishHead`, after the reinspect under the operation lock, refuse with a `failed` or `contended` disposition and push nothing when the re-read local head differs from the expected head. Use whichever disposition the PR path's moved-head refusal maps to.
- `WorkspacePublish` passes `req.Head` as the expected head.
- Add a unit test: move the local head between the app-level check and `PublishHead`, and assert nothing is pushed and the result is refused.

## Out of scope

Changing the gate or journal accounting (change 0444 owns it). Other publish or push paths. Changing the remote lease or fast-forward rules.

## Reconcile log

### 2026-09-24

2026-09-24 — Re-read against main c67d07ad (0444 merged). WorkspacePublish still checks the head via Inspect and then calls PublishHead, which re-reads the local head under its lock with no expected-head comparison; the 0444 unverified-journal guard is present. Scope unchanged: add an expected head to PublishRequest, refuse under the lock on mismatch, pass req.Head, add a unit test.
