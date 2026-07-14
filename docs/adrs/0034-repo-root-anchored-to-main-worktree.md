---
id: 34
slug: repo-root-anchored-to-main-worktree
title: docket scripts anchor the repo root to the main worktree, never the caller's CWD
status: Accepted
date: 2026-07-14
supersedes: []
reverses: []
relates_to: []
change: 75
---

## Context

docket runs a three-worktree layout — the main checkout, the persistent `.docket/`
metadata worktree, and ephemeral `.worktrees/<slug>` feature worktrees — and agents, hooks,
and humans all `cd` freely between them. Historically the scripts derived the repo root from
wherever the caller was standing: `docket-config.sh` defaulted `REPO_DIR="."` and
absolutized it with `cd "$REPO_DIR" && pwd -P`; `cleanup-feature-branch.sh` and others used
`git rev-parse --show-toplevel`. Both return the LINKED worktree the caller happens to be in,
not the repo's primary checkout. A stray `cd` was therefore enough to misdirect a script, and
it did — in three verified ways: cleanup deleting the remote branch then failing (partial data
loss), preflight minting a nested `<repo>/.docket/.docket`, and cleanup deleting the agent's
own CWD. Two scripts in the repo (`sync-integration-branch.sh`, `disable-worktree-hooks.sh`)
had already independently solved this with the main-worktree idiom.

## Decision

Every docket script resolves the repo root as the MAIN worktree of the repo containing CWD,
using `git worktree list --porcelain | sed -n '1s/^worktree //p'` (git lists the main worktree
first, and the list is reachable from every worktree in the set). This is centralized in one
shared sourced helper, `scripts/lib/docket-root.sh` (`docket_main_worktree`,
`docket_anchor_path`, `docket_metadata_worktree`), which the resolver, preflight, cleanup,
docket-status, and render-change-links all adopt.

The rule and its three prohibitions a future contributor must know:

1. Resolve the root from the main worktree — NEVER `git rev-parse --show-toplevel` (returns
   the linked worktree from `.docket/` or a feature worktree).
2. NEVER derive the root as `dirname $METADATA_WORKTREE` — in main-mode `METADATA_WORKTREE`
   IS the repo root, so `dirname` yields the repo's PARENT. Skills read the `REPO_ROOT`
   literal from the `docket.sh preflight` block instead.
3. `REPO_ROOT` is emitted by `docket-config.sh` in the PLAIN export format ONLY — the shell
   format omits it because `ensure-claude-settings.sh` defines its own `REPO_ROOT` and `eval`s
   the shell export, and a shell-format `REPO_ROOT` would silently capture that variable.

Corollary (a distinct, easily-reintroduced gotcha worth stating): a pathspec passed to
`git -C "$mw" … -- <pathspec>` must be worktree-root-relative or absolute, NEVER
`$mw`-prefixed-relative — an `$mw`-prefixed pathspec under a `git -C "$mw"` already rooted at
`$mw` matches nothing. That exact bug had left docket-status.sh's artifacts-refresh block
silently dead.

Resolution stays a SOFT fallback when CWD is not in a git repo (empty resolution → prior
behavior), never a new hard error. Deliberately untouched: `archive-change.sh`'s
`git -C "$CHANGES_DIR" rev-parse --show-toplevel` (correctly resolves the passed dir's
worktree) and the `docket.sh` facade (must not `cd`, or it would re-resolve caller-relative
path arguments).

## Consequences

Enables: a docket script produces identical results from any CWD (verified byte-identical
`REPO_ROOT`/`METADATA_WORKTREE` from the main tree, `.docket/`, a feature worktree, and a
subdirectory), which is what makes preflight idempotent w.r.t. CWD and lets finalize run its
close-out from a durable root.

Costs / gives up: one deliberate behavior change — a resolver invoked from `<repo>/sub/` now
reads `<repo>/.docket.local.yml` and targets `<repo>/.docket` (previously `<sub>/…`), and
`--bootstrap` seeds `<repo>/.gitignore`. The anchor touches every facade-reached script (blast
radius), mitigated by the fact that the main worktree is the root each already intended.

One known out-of-scope residual: `ensure-claude-settings.sh` still uses `--show-toplevel` (a
session-time install helper, not facade-reached) — a reasonable future follow-up.
