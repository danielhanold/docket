# Consuming-repo script resolution — reach docket's helper scripts via `DOCKET_SCRIPTS_DIR`

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

1. **Env var, not vendoring.** Introduce `DOCKET_SCRIPTS_DIR` = the absolute path to the
   docket clone's `scripts/`. Skills resolve every call as
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}/<name>.sh"`. **Rejected:**
   copying/symlinking the scripts into the consuming repo (e.g.
   `<repo>/.claude/docket/scripts/`) — a copy **drifts** (the scripts are developed in
   lockstep with the live-symlinked skills; a copied script lags a skill edit), a
   symlink doesn't survive a fresh clone; copying only buys hermetic version-pinning,
   which is not wanted.
2. **Points at the live clone → zero drift.** `DOCKET_SCRIPTS_DIR` names the same clone the
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
5. **Fail loud, not silent.** The `${DOCKET_SCRIPTS_DIR:?…}` form turns a missing/incomplete
   install into a clear, actionable error at the first script call — folding the former
   standalone "guardrail" option in for free. Each skill's Step 0 surfaces it.
6. **`DOCKET_` namespacing.** `DOCKET_SCRIPTS_DIR` joins the scripts' existing `DOCKET_MODE`
   / `DOCKET_INTEGRATION_BRANCH` / `DOCKET_HARNESS_ROOT` seams. **Constraint:** every env
   var docket introduces is `DOCKET_`-namespaced, to avoid collisions in the user's
   shared shell environment.
7. **Scripts stay shell-agnostic.** The helpers keep their `#!/usr/bin/env bash` shebang
   and run via it regardless of the user's login shell — no per-shell port. Only the
   profile-`export` *write* is shell-specific (decision 3 / the injection section).
8. **Back-fill is free for the env route.** Re-running `install.sh` repairs
   already-migrated repos (markhaus today has the scripts absent) — the user-level
   injection is repo-agnostic, so one re-run covers every repo and worktree.
9. **Shell floor = zsh + bash + fish, with a POSIX `export` fallback.** Grounded in a
   survey of how common tools do shell integration — Starship, zoxide, mise, rustup, and
   Homebrew **all** support bash + zsh + fish first-class; everything beyond is a long tail
   (nushell strongest at 4/5, PowerShell Windows-centric, elvish/xonsh/tcsh/cmd niche).
   So `install.sh` writes `export` for bash/zsh and `set -gx` for fish, and — mirroring
   Homebrew `shellenv` and rustup — **falls back to a POSIX `export`** for any other/unknown
   shell. It prefers an **always-sourced** file where one exists (zsh `~/.zshenv`, as rustup
   does). nushell / PowerShell are deferred (add on demand; PowerShell only if docket ever
   targets Windows). The shell-agnostic settings-`env` write backstops anything the profile
   write misses.
10. **Fail loud now; relocate the fallback prose in a follow-up.** This change makes the
    script call **fail loud** via `${DOCKET_SCRIPTS_DIR:?…}`, but it **leaves the existing
    per-skill manual-fallback prose untouched**. Slimming the skills by moving that prose
    out of each `SKILL.md` into an on-demand **sibling file** (progressive disclosure, like
    the convention's `github-board-mirror.md`) is a separate cross-cutting change —
    **follow-up #37** — so #34 stays a focused reachability fix and the two passes over
    every skill body don't collide. During the transition the retained prose and the new
    fail-loud form coexist harmlessly.
11. **CI drift-guard.** Add a check (mirroring `sync-agents.sh --check`) that fails when a
    consuming repo cannot resolve `docket-config.sh` via `DOCKET_SCRIPTS_DIR`, and when any
    skill body still references a bare `scripts/<name>.sh` instead of `${DOCKET_SCRIPTS_DIR`.
    Catches this whole class early.

## The env var

```
DOCKET_SCRIPTS_DIR = <docket clone>/scripts        # absolute, e.g. /Users/me/dev/docket/scripts
```

Skill call-site shape (uniform, ~one token per call):

```
eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-board.sh --changes-dir …
```

## Injection — where `install.sh` writes it

The scripts are shell-agnostic; the **profile write is shell-specific** and must match
the shell the Bash tool actually launches (it runs the user's `$SHELL` — `/bin/zsh` in
the markhaus session). `install.sh` detects the user's shell(s) and writes the right rc
with the right syntax:

| Shell | File | Syntax |
|---|---|---|
| zsh  | `~/.zshenv` (always sourced) or `~/.zshrc` | `export DOCKET_SCRIPTS_DIR="…"` |
| bash | `~/.bashrc` / `~/.bash_profile` | `export DOCKET_SCRIPTS_DIR="…"` |
| fish | `~/.config/fish/config.fish` | `set -gx DOCKET_SCRIPTS_DIR "…"` |
| *other / unknown* | the shell's profile if detectable | POSIX `export` (fallback) |

bash + zsh + fish is the **floor** because every surveyed tool (Starship, zoxide, mise,
rustup, Homebrew) supports exactly that set first-class. For any other shell, follow the
Homebrew-`shellenv` / rustup pattern and emit a **POSIX `export`** (covers sh/dash/ash/ksh
and most). Prefer an **always-sourced** file where one exists — zsh `~/.zshenv` (rustup
writes there), not just `~/.zshrc`. nushell / PowerShell are out of the floor (decision 9).

Plus the shell-agnostic reinforcement: `env.DOCKET_SCRIPTS_DIR` in user-level
`~/.claude/settings.json` (idempotent `jq` write, per the `ensure-claude-settings.sh`
pattern). Both writes are idempotent and marked so re-running `install.sh` is a no-op
when already present — and the settings-`env` write backstops any shell the profile
write misses.

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

- **`install.sh`** — write `DOCKET_SCRIPTS_DIR` to the detected shell profile(s) + user-level
  `~/.claude/settings.json` `env`; idempotent; this is the back-fill path too.
- **Every skill body** — replace bare `scripts/<name>.sh` with
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh` uniformly (`docket-new-change`,
  `docket-groom-next`, `docket-auto-groom`, `docket-implement-next`, `docket-status`,
  `docket-finalize-change`, `docket-adr`, and the convention's bootstrap-probe references).
  Step 0 inherits the fail-loud check via `:?`. The existing manual-fallback prose is **left
  in place** here — its relocation is follow-up #37 (decision 10).
- **`migrate-to-docket.sh`** — ensure `install.sh` has run (or point at it) so a freshly
  migrated repo is immediately script-reachable, not just settings-granted.
- **Convention** — document `DOCKET_SCRIPTS_DIR` resolution + the `DOCKET_` namespacing
  constraint where the script family is described.
- **CI** — the drift-guard from decision 11 (a `sync-agents.sh --check`-style gate): a
  consuming repo resolves `docket-config.sh` via `DOCKET_SCRIPTS_DIR`, and no skill body
  references a bare `scripts/<name>.sh`.
- **Tests** — assert a consuming-repo shell with `DOCKET_SCRIPTS_DIR` set resolves
  `docket-config.sh`; assert the multi-shell profile-write syntax (zsh/bash `export`,
  fish `set -gx`, POSIX-`export` fallback); assert idempotent re-runs.

## Out of scope

- Copy/symlink-into-repo vendoring (rejected — drift).
- The heavier resolutions considered and demoted: a `docket` CLI dispatcher on `PATH`,
  resolving the scripts dir from the skill symlink's own realpath at Step 0, a per-repo
  `link-scripts.sh` shim — all more moving parts than the env var for the same reach.
- Rewriting the scripts' internal logic — they work; the defect is *reachability*.
- **Relocating / removing the per-skill manual-fallback prose — deferred to follow-up #37**
  (progressive-disclosure sibling files). This change leaves that prose untouched.
- Tightening the scripts' non-namespaced `GIT` / `REPO` mock seams — minor adjacent debt,
  separate change.
- Windows profile injection (gh #20112).

## Open questions (build-time)

Shell floor (decision 9) and the CI drift-guard (decision 11) are now decided; the
manual-fallback relocation moved to follow-up #37 (decision 10). One genuine question
remains:

- **Per-harness settings `env`:** the shell-profile `export` (the primary) is
  harness-agnostic and already covers every harness. The open question is only about the
  *reinforcement* layer — docket's `link-skills.sh` installs into whichever of
  `.claude`/`.codex`/`.cursor`/`.agents`/`.kiro`/`.windsurf` are present, and each has its
  own (or no) settings-`env` mechanism. Build decides whether `install.sh` writes each
  present harness's equivalent (looping like `link-skills.sh`) or only Claude Code's, with
  the others relying on the profile `export` alone. Low-stakes because the profile `export`
  is the actual guarantee; the settings write is belt-and-suspenders.

## ADRs

Cites **ADR-0012** (the `docket-status` script-vs-model boundary): the scripts are the
deterministic layer the model defers to, and this change restores that layer's
reachability from consuming repos. Likely **produces a new ADR** recording the
consuming-repo script-resolution contract (`DOCKET_SCRIPTS_DIR`, profile-`export`-primary +
settings-`env`-reinforcement, fail-loud) — assigned at build time by `docket-adr`.

## Reconcile note (2026-06-21 — implement-next, pre-plan)

Verified against current `origin/main` (46c4520); every decision above holds unchanged. Three
build-reality refinements for the plan (full record in the change's `## Reconcile log`):

1. **Drift-guard = test-suite static audit, not GitHub Actions.** The repo has no
   `.github/workflows/` CI. The decision-11 "`sync-agents.sh --check`-style gate" lands in
   `tests/` — mirroring `tests/test_change_links_coverage.sh` (a repo-wide sentinel scan) and how
   `sync-agents.sh --check` is itself exercised by `tests/test_sync_agents.sh`. The test suite is
   the de-facto gate; a `--check`-style invocable is the right shape and is wired into it.
2. **Wiring-sentinel updates are in-scope (newly surfaced).** Rewriting the ~65 call sites removes
   the literal `scripts/<name>.sh` substring from each skill body, which breaks ~8 existing
   sentinel tests that grep for it: `test_change_links_coverage`, `test_docket_config`,
   `test_render_board`, `test_closeout`, `test_adr_checks`, `test_render_adr_index`,
   `test_board_checks`. Update each to the `DOCKET_SCRIPTS_DIR`-resolved form in lockstep. Leave
   doc-reference greps that intentionally point at the docket clone's own path (e.g.
   `test_ensure_claude_settings.sh`'s README check, which documents
   `bash /path/to/docket/scripts/ensure-claude-settings.sh`).
3. **Settings-`env` write target.** User-level `~/.claude/settings.json` `.env.DOCKET_SCRIPTS_DIR`,
   idempotent `jq` per the `ensure-claude-settings.sh` precedent — a *distinct* file from that
   script's per-repo `settings.local.json` permissions write. The open per-harness question stays
   low-stakes; default to Claude Code only (the profile `export` is the harness-agnostic guarantee).
