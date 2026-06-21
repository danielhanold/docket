# Consuming-repo script resolution — reach docket's helper scripts via `DOCKET_SCRIPTS`

**Change:** #34
**Status:** design (brainstorm output; stops at spec per `docket-groom-next`)
**Date:** 2026-06-21

## Problem

Every docket skill invokes its deterministic helper scripts through a **bare,
CWD-relative path** — `eval "$(scripts/docket-config.sh --export)"`,
`scripts/render-board.sh …`, `scripts/archive-change.sh …`,
`scripts/terminal-publish.sh …`, `scripts/render-adr-index.sh …`,
`scripts/github-mirror.sh …`, `scripts/board-checks.sh …`, `scripts/adr-checks.sh …`.
Those scripts live **only in the docket source repo** (`~/dev/docket/scripts/`).

A skill runs with its CWD set to the **consuming** repo (e.g. `~/dev/markhaus`), so
`scripts/<name>.sh` resolves against *that* repo's `scripts/` — which holds the
consuming project's own scripts, not docket's. The skills reach consuming repos via
**symlink** (`link-skills.sh` → each harness's `~/.claude/skills/`), but **nothing
makes `scripts/` reachable the same way**: `install.sh` runs only `link-skills.sh`
(skills) + `sync-agents.sh` (agent wrappers); `link-skills.sh` symlinks skill dirs
only; `migrate-to-docket.sh` resolves its *own* helper as
`$MIGRATE_DIR/scripts/ensure-claude-settings.sh` and never vendors the rest. There is
a resolution **asymmetry**: migrate resolves scripts relative to the docket repo
(correct), the skills relative to the consuming-repo CWD (broken).

Consequence (observed live in markhaus change #43, migrated 2026-06-04): every
deterministic primitive is unreachable in a consuming repo — config + bootstrap
resolution (incl. the fail-closed guard and the `DOCKET`/`LIVE` 2×2), board render,
terminal archive/publish, ADR index, the GitHub mirror, the health/board checks. The
agent's only recourse is to **hand-work each operation from the convention prose**,
which loses determinism, the fail-closed behaviour, idempotency, and the scripts'
validation — and fails **silently** (a bare `no such file or directory` reads as a
glitch, not a structural gap). This is a docket setup/contract defect surfaced in
every consuming repo, not a consuming-repo problem.

## Goal

Give the skills one reliable, **absolute** path to the docket clone's `scripts/`,
resolved through an env var, with **no per-repo install, no drift, and a loud failure**
when it is missing.

## Decisions (locked in brainstorm)

1. **Env var, not vendoring.** Introduce `DOCKET_SCRIPTS` = the absolute path to the
   docket clone's `scripts/`. Skills resolve every call as
   `"${DOCKET_SCRIPTS:?run docket/install.sh}/<name>.sh"`. **Rejected:**
   copying/symlinking the scripts into the consuming repo (e.g.
   `<repo>/.claude/docket/scripts/`) — a copy **drifts** (the scripts are developed in
   lockstep with the live-symlinked skills; a copied script lags a skill edit), a
   symlink doesn't survive a fresh clone; copying only buys hermetic version-pinning,
   which is not wanted.
2. **Points at the live clone → zero drift.** `DOCKET_SCRIPTS` names the same clone the
   skill symlinks already point into, so skills (live) and scripts (live) stay
   version-matched automatically. This is the property that makes the env var strictly
   better than any copy.
3. **Injection = shell-profile `export` (primary) + settings `env` (reinforcement).**
   The harness re-initialises every Bash-tool shell from the user profile on **each**
   call, so a profile `export` is re-sourced by the main session *and* by dispatched
   **subagent** Bash shells — the case that matters for docket's subagent-heavy
   autonomous skills. This works by **per-call re-sourcing, not OS process-tree
   inheritance** (verified — see below). The user-level `~/.claude/settings.json` `env`
   block is reinforcement for the main session / sessions not launched from a
   profile-sourcing shell.
4. **`install.sh` writes both.** It already computes the docket clone's absolute path
   (`SCRIPT_DIR`), so it holds the literal value the injector needs. The settings-`env`
   write reuses the `ensure-claude-settings.sh` idempotent-`jq` precedent (change 0027).
5. **Fail loud, not silent.** The `${DOCKET_SCRIPTS:?…}` form turns a missing/incomplete
   install into a clear, actionable error at the first script call — folding the former
   standalone "guardrail" option in for free. Each skill's Step 0 surfaces it.
6. **`DOCKET_` namespacing.** `DOCKET_SCRIPTS` joins the scripts' existing `DOCKET_MODE`
   / `DOCKET_INTEGRATION_BRANCH` / `DOCKET_HARNESS_ROOT` seams. **Constraint:** every env
   var docket introduces is `DOCKET_`-namespaced, to avoid collisions in the user's
   shared shell environment.
7. **Scripts stay shell-agnostic.** The helpers keep their `#!/usr/bin/env bash` shebang
   and run via it regardless of the user's login shell — no per-shell port. Only the
   profile-`export` *write* is shell-specific (decision 3 / the injection section).
8. **Back-fill is free for the env route.** Re-running `install.sh` repairs
   already-migrated repos (markhaus today has the scripts absent) — the user-level
   injection is repo-agnostic, so one re-run covers every repo and worktree.

## The env var

```
DOCKET_SCRIPTS = <docket clone>/scripts        # absolute, e.g. /Users/me/dev/docket/scripts
```

Skill call-site shape (uniform, ~one token per call):

```
eval "$("${DOCKET_SCRIPTS:?run docket/install.sh}"/docket-config.sh --export)"
"${DOCKET_SCRIPTS:?run docket/install.sh}"/render-board.sh --changes-dir …
```

## Injection — where `install.sh` writes it

The scripts are shell-agnostic; the **profile write is shell-specific** and must match
the shell the Bash tool actually launches (it runs the user's `$SHELL` — `/bin/zsh` in
the markhaus session). `install.sh` detects the user's shell(s) and writes the right rc
with the right syntax:

| Shell | File | Syntax |
|---|---|---|
| zsh  | `~/.zshenv` (always sourced) or `~/.zshrc` | `export DOCKET_SCRIPTS="…"` |
| bash | `~/.bashrc` / `~/.bash_profile` | `export DOCKET_SCRIPTS="…"` |
| fish | `~/.config/fish/config.fish` | `set -gx DOCKET_SCRIPTS "…"` |

Plus the shell-agnostic reinforcement: `env.DOCKET_SCRIPTS` in user-level
`~/.claude/settings.json` (idempotent `jq` write, per the `ensure-claude-settings.sh`
pattern). Both writes are idempotent and marked so re-running `install.sh` is a no-op
when already present. Prefer an always-sourced file where one exists (zsh `~/.zshenv`).

## Verification (done during brainstorm)

- ✅ **`settings.json` `env` injects into the Bash tool.** Claude Code docs
  ([env-vars](https://code.claude.com/docs/en/env-vars.md)): *"Every command executed
  via the Bash tool … can read these variables … injected … at startup."* Read at
  session start; values are literal strings (no `${…}`/`~` expansion) — fine, since
  `install.sh` writes a resolved absolute path.
- ✅ **Profile `export` reaches subagents.** Empirically: each Bash call (main session
  AND a dispatched subagent) runs `/bin/zsh` and **sources `~/.zshrc`** — a dispatched
  subagent saw the profile-only vars `ZSH`, `TF_PLUGIN_CACHE_DIR`, `OPENTUI_FORCE_WCWIDTH`.
- ⚠️ **Process-env inheritance is NOT reliable for subagents.** A Claude-injected process
  var (`CLAUDE_EFFORT`) was set in the main session but **UNSET** in the subagent (while
  `AI_AGENT` was present in both) — so the profile reaches subagents via **per-call
  re-sourcing**, not inheritance. This is why the profile `export` is the primary path,
  not settings `env`.
- Minor: a known Windows bug (gh #20112) where settings `env` isn't injected into Bash —
  docket is bash/macOS-Linux, low concern.

## Touch-points

- **`install.sh`** — write `DOCKET_SCRIPTS` to the detected shell profile(s) + user-level
  `~/.claude/settings.json` `env`; idempotent; this is the back-fill path too.
- **Every skill body** — replace bare `scripts/<name>.sh` with
  `"${DOCKET_SCRIPTS:?run docket/install.sh}"/<name>.sh` uniformly (`docket-new-change`,
  `docket-groom-next`, `docket-auto-groom`, `docket-implement-next`, `docket-status`,
  `docket-finalize-change`, `docket-adr`, and the convention's bootstrap-probe references).
  Step 0 inherits the fail-loud check via `:?`.
- **`migrate-to-docket.sh`** — ensure `install.sh` has run (or point at it) so a freshly
  migrated repo is immediately script-reachable, not just settings-granted.
- **Convention** — document `DOCKET_SCRIPTS` resolution and the `DOCKET_` namespacing
  constraint where the script family is described.
- **Tests** — assert a consuming-repo shell with `DOCKET_SCRIPTS` set resolves
  `docket-config.sh`; assert the multi-shell profile-write syntax (zsh/bash `export`,
  fish `set -gx`); assert idempotent re-runs; a `sync-*`-style check that every skill body
  references `${DOCKET_SCRIPTS` rather than a bare `scripts/`.

## Out of scope

- Copy/symlink-into-repo vendoring (rejected — drift).
- The heavier resolutions considered and demoted: a `docket` CLI dispatcher on `PATH`,
  resolving the scripts dir from the skill symlink's own realpath at Step 0, a per-repo
  `link-scripts.sh` shim — all more moving parts than the env var for the same reach.
- Rewriting the scripts' internal logic — they work; the defect is *reachability*.
- Retiring the convention's "prose is the contract" fallback documentation — it stays a
  true last resort (but is no longer the de-facto path).
- Tightening the scripts' non-namespaced `GIT` / `REPO` mock seams — minor adjacent debt,
  separate change.
- Windows profile injection (gh #20112).

## Open questions (build-time)

- **Shell support floor + write strategy:** zsh + bash + fish as the floor? Write to all
  present shells' rc, or detect `$SHELL` and write one? Prefer an always-sourced file
  (zsh `~/.zshenv`)?
- **Per-harness settings `env`:** docket targets `.claude`/`.codex`/`.cursor`/… — does
  each present harness get its settings-`env` equivalent written (mirroring how
  `link-skills.sh` links into each present harness)?
- **Retire the manual-prose fallback** once scripts are reachable + fail loud, or keep it
  behind an explicit override?
- **CI drift-guard:** mirror `sync-agents.sh --check` with a check that a consuming repo
  can actually resolve `docket-config.sh` via `DOCKET_SCRIPTS` — catch this whole class
  early.

## ADRs

Cites **ADR-0012** (the `docket-status` script-vs-model boundary): the scripts are the
deterministic layer the model defers to, and this change restores that layer's
reachability from consuming repos. Likely **produces a new ADR** recording the
consuming-repo script-resolution contract (`DOCKET_SCRIPTS`, profile-`export`-primary +
settings-`env`-reinforcement, fail-loud) — assigned at build time by `docket-adr`.
