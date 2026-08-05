---
id: 206
slug: delegated-runner-runs-are-anchored-at-the-main-worktree-not
title: Delegated runner runs are anchored at the main worktree, not the feature worktree
status: proposed
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: []
discovered_from: [205]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0205 shipped the `opencode` runner adapter and, with it, the first documented recipe that
delegates the four `docket-build-*` profile workers to a child harness. Its whole-branch review
surfaced a framework-level mismatch that predates the adapter but only becomes reachable now.

`runner-dispatch.sh` anchors every delegated run with
`export DOCKET_REPO_ROOT="$(docket_main_worktree)"` — deliberately cwd-independent (ADR-0034), so
it returns the repo's PRIMARY checkout even when the caller stands in `.worktrees/<slug>`. Each
adapter then hands that path to the child (`codex exec -C`, `opencode run --dir`).

That anchor is correct for the agents delegation has shipped for until now — `docket-status` and
`docket-adr` are metadata-scoped and belong in the main tree. It is wrong for a build worker: the
`docket-build-task` contract requires the worker to stay **inside the feature worktree, on its
branch**. A delegated `build-economy` therefore starts in the main tree on the integration branch,
holding whatever permission grant its runner was configured with (`--auto` under opencode), and the
only thing pointing it at the correct tree is prose in the relayed prompt.

## What changes

Decide and implement the anchoring rule for delegated runs. Options weighed in the review:

- Have the facade resolve the caller's cwd when it is inside the repo, and pass that instead of the
  main worktree — preserving `docket_main_worktree()` as the fallback.
- Add an explicit worktree argument to the dispatch contract, set by the shim for worker agents.
- Keep the anchor and make the constraint explicit: forbid delegating `build-*` agents, or document
  the limitation loudly in each adapter contract.

Whichever is chosen, it is a framework decision affecting all three adapters, not an opencode
detail, and it likely touches ADR-0034's reasoning.

## Out of scope

- The opencode adapter's own flag mapping and permission gate (shipped and settled in 0205).
