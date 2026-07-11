---
id: 25
slug: docket-worktrees-disable-git-hooks
title: docket bookkeeping commits skip shared git hooks via worktree-scoped core.hooksPath
status: Accepted
date: 2026-07-11
supersedes: []
reverses: []
relates_to: [1]
change: 63
---

## Context

Git hooks are shared across every worktree via the common git dir. docket makes many
machine-generated bookkeeping commits: into the `.docket` metadata worktree parked on
the orphan `docket` branch (ADR-0001's metadata-branch model), plus doc-management
commits onto the integration branch through transient worktrees (migrate's prune,
terminal-publish's publish).

In a repo that uses a git-hook framework (pre-commit.com, husky, lefthook) with a
`pre-commit` hook, those commits hard-fail. The orphan `docket` branch has no
`.pre-commit-config.yaml`, so pre-commit exits 1 ("No .pre-commit-config.yaml file
was found"); and the integration-branch doc commits would run the team's hooks against
commits those hooks were never meant to guard. This was observed in practice (Cursor
in a pre-commit repo): a metadata commit was blocked and recovered only by an
improvised per-commit env-var workaround — fragile and non-deterministic. docket had
no systematic handling of hooks anywhere (no `--no-verify`, no `core.hooksPath`, no
hook logic at all).

## Decision

Disable git hooks on every docket-**owned** worktree by construction, at
worktree create/ensure time, via a single idempotent helper
`scripts/disable-worktree-hooks.sh`. The helper enables git's local
`extensions.worktreeConfig` and sets a worktree-scoped `core.hooksPath` pointing at
an empty, docket-owned directory (`<git-common-dir>/docket/empty-hooks`, an absolute
path to a real empty dir). Every commit into that worktree then finds no hooks and
proceeds. This is enforced *by construction* — there is no per-commit flag to forget —
and it is *framework-agnostic*: it disables the hook mechanism, not one framework's
config.

**Scope is metadata bookkeeping only.** The call sites are the persistent `.docket`
metadata worktree (docket-status ensure plus the convention's Step-0 preamble),
migrate-to-docket's transient seed/prune worktrees, and terminal-publish's transient
publish worktree. Feature-branch **code** commits keep running the team's hooks —
feature worktrees are never passed to the helper, because real code headed to a PR
must still pass the team's gates.

**Alternatives rejected:** per-commit `--no-verify` (forgettable — the exact failure
mode observed); framework-specific env vars such as `PRE_COMMIT_ALLOW_NO_CONFIG` (not
framework-agnostic); a configurable `.docket.yml` hooks policy (YAGNI — bookkeeping
never wants the team's hooks, code always does).

**Safety.** Enabling `extensions.worktreeConfig` makes `core.worktree`/`core.bare`
read per-worktree, so a pre-existing value in the *common* config would stop applying
to linked worktrees. The helper enables the extension first (git requires it before
any `--worktree` write), relocates a genuinely-needing-relocation common value
(`core.worktree`, or `core.bare=true`) to the main worktree's per-worktree config,
and rolls back the enable (fail-closed, exit 1) if that relocation cannot be done
safely. The ubiquitous default `core.bare=false` that git writes into every repo is
deliberately left in place — it is harmless, and relocating it universally would add
stderr noise and a concurrent-loop race.

The change is **local-only** (never the remote, teammates' clones, or the committed
`.docket.yml`) and idempotent/self-healing: a repeat call is a clean no-op, so
existing installs are fixed on their next docket run.

## Consequences

**Enables:** docket coexists with any git-hook framework out of the box, harness-
agnostically (Cursor / Claude Code / Codex), with nothing to configure.

**Costs / gives up:** docket now enables `extensions.worktreeConfig` — a local,
idempotent `.git/config` change requiring git ≥ 2.20 — in every docket repo, and owns
an empty hooks directory under `.git/`. Feature-branch code deliberately still runs
the team's hooks (skipping them is an explicit non-goal).

**Establishes an invariant future code must respect:** docket-**owned** worktrees have
hooks disabled. A new site that creates a docket-owned worktree must call
`disable-worktree-hooks.sh` after `git worktree add`, and no feature/code worktree may
ever be passed to it.
