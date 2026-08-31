---
id: 206
slug: delegated-runner-runs-are-anchored-at-the-main-worktree-not
title: Delegated runner runs are anchored at the main worktree, not the feature worktree
status: done
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: [205]
related: [79, 192]
discovered_from: [205]
adrs: [34, 68]
spec: docs/superpowers/specs/2026-08-05-delegated-run-worktree-anchor-design.md
plan: docs/superpowers/plans/2026-08-05-delegated-run-worktree-anchor-plan.md
results: docs/results/2026-08-05-delegated-runner-runs-are-anchored-at-the-main-worktree-not-results.md
trivial: false
auto_groomable:
branch: feat/delegated-runner-runs-are-anchored-at-the-main-worktree-not
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/157
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-05-delegated-run-worktree-anchor-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-05-delegated-run-worktree-anchor-design.md) |
| Plan | [2026-08-05-delegated-run-worktree-anchor-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-05-delegated-run-worktree-anchor-plan.md) |
| Results | [2026-08-05-delegated-runner-runs-are-anchored-at-the-main-worktree-not-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-05-delegated-runner-runs-are-anchored-at-the-main-worktree-not-results.md) |
| ADRs | [ADR-0034](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0034-repo-root-anchored-to-main-worktree.md), [ADR-0068](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0068-delegated-run-anchor-is-an-explicit-argument.md) |
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

Two facts sharpen the failure mode. Feature worktrees live at `<repo>/.worktrees/<slug>` — *inside*
the main worktree — so codex's `workspace-write` sandbox already permits writes to the feature tree;
this is not a permission failure. The exposure is the starting cwd and the checked-out branch: a
worker that does not faithfully follow the prompt's instruction commits code onto the integration
branch in the shared primary checkout, unattended.

## What changes

A delegated run's anchor becomes an **explicit argument whose default is the main worktree**; the
delegated agent's scope decides which. ADR-0034 stands unamended — nothing resolves an anchor from
the caller's CWD, and the only way off the main worktree is an argument someone deliberately wrote.

- **`runner-dispatch.sh`** gains an optional `--worktree <path>`, resolved through
  `docket_anchor_path` so a relative value joins to the main worktree and stays cwd-independent.
  Three loud gates: `--worktree` is required for `build-*` agents; the resolved anchor must be a
  directory; it must belong to this repo's worktree set. Absent the flag, behavior is
  byte-identical to today.
- **The three adapters are unchanged** — only their contracts' env tables, from "main-worktree
  path" to "run anchor". The facade owning the anchor is what makes this free.
- **`sync-agents.sh`'s `emit_shim`** bakes the flag into `build-*` shims as a required slot, with
  an abort-and-report rule when the caller named no worktree. Other shims are untouched.
  `docket-build`'s dispatch section notes the flag as the channel.
- **A new ADR** records the framework rule (`relates_to: [34]`), plus a dated `## Update` on
  ADR-0034 pointing at it; both ids ride this change's `adrs:`.

Design, rejected alternatives, and the test matrix are in the spec.

## Out of scope

- The opencode adapter's own flag mapping and permission gate (shipped and settled in 0205).
- Delegating orchestrator agents — 0205's "delegate leaves, not orchestrators" rule is unaffected.
- `ensure-claude-settings.sh`'s remaining `--show-toplevel` use (ADR-0034's known residual).

## Reconcile log

### 2026-08-05 — build-time reconcile

Re-read against `origin/main`, the spec, `related: [79, 192]`, ADR-0034, and the recently-archived
0205. **No drift — scope stands unchanged.** Every premise the spec rests on verified against
current code:

- `depends_on: [205]` is satisfied — `scripts/runners/opencode.{sh,md}` are on `origin/main`, so
  the contract edit this change makes to `opencode.md` has a file to land in.
- `scripts/runner-dispatch.sh` still anchors with `REPO_ROOT="$(docket_main_worktree)"` and still
  parses exactly the four flags (`--runner/--agent/--model/--effort`), so `--worktree` is a clean
  addition to an unchanged parse loop.
- `docket_anchor_path` exists in `scripts/lib/docket-root.sh` with the documented signature and the
  absolute-passthrough / `"."`-to-root / relative-join behavior the design routes through.
- All three adapters (`codex.sh`, `cursor.sh`, `opencode.sh`) read `DOCKET_REPO_ROOT` verbatim into
  their own directory flag, so they stay code-unchanged; only their contracts' env rows move.
- `emit_shim` still receives the agent name as `$5`, so the `build-*` slot can be baked per-agent.

One scope refinement (a clarification, not a change of intent): the adapter-contract edits are
slightly wider than "one env-table row each" — `opencode.md`'s Purpose prose also states the anchor
comes from `docket_main_worktree()`, and `cursor.sh`/`codex.sh` carry the same claim in a header
comment. Those restatements are corrected alongside their tables, so no surface is left asserting
the anchor is always the primary checkout.

No auto-capture candidates surfaced — nothing adjacent rose above the materiality bar.
