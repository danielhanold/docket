---
id: 34
slug: consuming-repo-script-resolution
title: Helper scripts unreachable in consuming repos — skills call repo-relative `scripts/…` that exists only in the docket source repo
status: proposed
priority: high
created: 2026-06-20
updated: 2026-06-21
depends_on: []
related: []
adrs: [12]
spec: docs/superpowers/specs/2026-06-21-consuming-repo-script-resolution-design.md
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Why

Every docket skill invokes its deterministic helper scripts through a **bare,
CWD-relative path** (`eval "$(scripts/docket-config.sh --export)"`,
`scripts/render-board.sh …`, `scripts/archive-change.sh …`, etc.). Those scripts live
**only in the docket source repo** (`~/dev/docket/scripts/`); the skills reach consuming
repos via symlink (`link-skills.sh`), but **nothing makes `scripts/` reachable there** —
`install.sh` links skills + agents only, and `migrate-to-docket.sh` never vendors the
scripts in. So in a consuming repo (`scripts/` is the consuming project's own dir), every
deterministic primitive is unreachable: config + bootstrap resolution, board render,
terminal archive/publish, ADR index, the GitHub mirror, the health/board checks.

The skill's only recourse is to **hand-work each operation from the convention prose**,
losing determinism, the fail-closed config guard, idempotency, and the scripts' own
validation — and it fails **silently** (a bare `no such file or directory` reads as a
glitch, not a structural gap). **Observed live** during markhaus change #43 (migrated
2026-06-04): the whole build ran in manual-fallback mode. This is a docket setup/contract
defect surfaced in every consuming repo, not a consuming-repo problem.

## What changes

Give the skills one reliable **absolute** path to the docket clone's `scripts/`, via an
env var, and make its absence fail loud:

- Introduce **`DOCKET_SCRIPTS`** (absolute path to the docket clone's `scripts/`). Skills
  resolve every call as `"${DOCKET_SCRIPTS:?run docket/install.sh}/<name>.sh"` — the `:?`
  makes a missing install fail loud with the remedy.
- **`install.sh` injects it** (it already holds the absolute path): primary = a
  **shell-profile `export`** (re-sourced on every Bash call, so it reaches the
  subagents docket dispatches), reinforcement = user-level `~/.claude/settings.json`
  `env`. Points at the **live clone** the skill symlinks already use → **zero drift**.
- Re-running `install.sh` **back-fills** already-migrated repos (markhaus included).

Full design — injection mechanics, the multi-shell profile write (zsh/bash `export`, fish
`set -gx`), the verification, the `DOCKET_` namespacing constraint, and the alternatives
weighed (copy-into-`.claude` rejected for drift; PATH-CLI / realpath / per-repo-shim
demoted) — is in the linked **spec**.

## Out of scope

- Copy/symlink vendoring of the scripts into the consuming repo (rejected — drift).
- The heavier resolutions (PATH CLI dispatcher, realpath-from-symlink, per-repo shim).
- Rewriting the scripts' internal logic; retiring the convention's manual-prose fallback
  (stays a true last resort); tightening the non-namespaced `GIT`/`REPO` mock seams;
  Windows profile injection.

## Open questions

Design is settled (see spec); these are build-time choices:

- Shell support floor (zsh + bash + fish?) and write strategy (all-present vs detect
  `$SHELL`; prefer an always-sourced file like zsh `~/.zshenv`).
- Whether each present harness also gets its settings-`env` equivalent written.
- Retire the manual-prose fallback, or keep it behind an explicit override.
- A CI drift-guard (mirror `sync-agents.sh --check`) asserting a consuming repo can
  resolve `docket-config.sh` via `DOCKET_SCRIPTS`.
