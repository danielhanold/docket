---
id: 34
slug: consuming-repo-script-resolution
title: Helper scripts unreachable in consuming repos — skills call repo-relative `scripts/…` that exists only in the docket source repo
status: proposed
priority: high
created: 2026-06-20
updated: 2026-06-20
depends_on: []
related: []
adrs: []
spec:
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
CWD-relative path** — `eval "$(scripts/docket-config.sh --export)"`,
`scripts/render-board.sh …`, `scripts/archive-change.sh …`,
`scripts/terminal-publish.sh …`, `scripts/render-adr-index.sh …`,
`scripts/github-mirror.sh …`, `scripts/board-checks.sh …`, `scripts/adr-checks.sh …`.
Those scripts live **only in the docket source repo** (`~/dev/docket/scripts/`).

A skill runs with its CWD set to the **consuming** repo (e.g. `~/dev/markhaus`),
so `scripts/<name>.sh` resolves against *that* repo's `scripts/` directory — which
contains the consuming project's own scripts, not docket's. The skills themselves
reach consuming repos via **symlink** (`link-skills.sh` symlinks
`skills/<name>` into each harness's global `~/.claude/skills/`), but **nothing
makes the `scripts/` directory reachable the same way**:

- `install.sh` runs only `link-skills.sh` (skills) + `sync-agents.sh` (agent
  wrappers). Neither touches `scripts/`.
- `link-skills.sh` symlinks skill directories only.
- `migrate-to-docket.sh` resolves its *own* helper as `$MIGRATE_DIR/scripts/ensure-claude-settings.sh`
  (relative to the docket repo) and never copies/symlinks the rest of `scripts/`
  into the migrated repo.
- The README's only scripted-invocation guidance is the absolute
  `bash /path/to/docket/scripts/ensure-claude-settings.sh` — confirming the scripts
  are *not* expected to be on `PATH` or vendored into the consuming repo.

So there is a resolution **asymmetry**: `migrate-to-docket.sh` resolves scripts
relative to the docket repo (correct), while the **skills** resolve them relative
to the consuming repo's CWD (broken).

**Observed live**, during a `docket-implement-next` run for markhaus change #43
(markhaus was migrated to docket-mode 2026-06-04):

```
$ eval "$(scripts/docket-config.sh --export)"
(eval):1: no such file or directory: scripts/docket-config.sh
$ ls scripts/            # markhaus
install-canonical-app.sh   # ← only the project's own script; no docket scripts
```

Because the scripts are absent, **every deterministic primitive is unreachable**
in a consuming repo: config + bootstrap resolution (`docket-config.sh`, including
the fail-closed guard and the `DOCKET`/`LIVE` 2×2), board render
(`render-board.sh`), terminal archive/publish (`archive-change.sh`,
`terminal-publish.sh`), ADR index (`render-adr-index.sh`), the GitHub mirror
(`github-mirror.sh`), and the health/board checks. The agent's only recourse is
to **fall back to performing each operation by hand from the convention prose**
("the prose is the contract the script implements verbatim"). That fallback:

- loses determinism, the fail-closed config behaviour, the bootstrap 2×2 guard,
  idempotency, and the scripts' own validation (e.g. malformed-id checks);
- is slow and error-prone, and risks subtle divergence from the scripts' actual
  behaviour (the agent re-implements config resolution, board rendering, the
  terminal-publish path-copy, etc., from prose, by hand, every run);
- fails **silently** — a bare `no such file or directory` looks like a transient
  glitch, not a structural install gap, so it is easy to limp past without
  noticing the guarantees that were dropped.

This is a setup/path-resolution defect in docket itself (the install + skill
contract), surfaced in every consuming repo, not a markhaus problem.

## What changes

The whole problem reduces to one thing: **the skills need a single absolute path
to the docket clone's `scripts/`**, instead of the bare CWD-relative `scripts/…`.
So the fix is small — give them that path and make its absence fail loud. Two
simple mechanisms, both grounded in machinery docket already has:

### A. Env var pointing at the live scripts dir — **recommended**

- `install.sh` already computes the docket clone's absolute path
  (`SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`), so it already
  *holds* the value (a literal absolute path — exactly what the injector needs,
  since settings `env` values are literal strings with no expansion).
- **Injection point (refined by the verification below).** Write the var in **two
  places**, both of which `install.sh` can do: (1) a **shell-profile `export`** in
  the user's shell rc — the harness re-initialises every Bash-tool shell from the
  user profile on **each** call, so a profile export is re-sourced by the main
  session *and* by dispatched **subagent** Bash shells (the case that matters for
  docket's subagent-heavy autonomous flows). Note this is *re-sourcing per call*,
  **not** OS process-tree inheritance — the verification below shows a
  Claude-injected process var (`CLAUDE_EFFORT`) present in the main session but
  absent in a subagent, so we cannot rely on inheritance; the profile re-source is
  what makes it reliable. (2) the **user-level `~/.claude/settings.json` `env`
  block** — confirmed to inject into the main session's Bash tool, as reinforcement
  / for sessions not launched from a profile-sourcing shell. User-level ⇒ every repo
  and every worktree, no per-repo step. *The profile write in (1) is shell-specific
  — see the multi-shell open question.*
- Skills change `scripts/docket-config.sh` → `"${DOCKET_SCRIPTS:?run docket/install.sh}/docket-config.sh"`
  — a uniform, ~one-token-per-call-site edit. The `:?` form makes a missing/incomplete
  install **fail loud with the remedy**, folding the guardrail (was option 5) in for free.
- **Namespaced.** The introduced var is `DOCKET_SCRIPTS` — `DOCKET_`-prefixed,
  joining the scripts' existing `DOCKET_MODE` / `DOCKET_INTEGRATION_BRANCH` /
  `DOCKET_HARNESS_ROOT` seams. **Constraint:** every env var docket introduces is
  `DOCKET_`-namespaced, to avoid collisions in the user's shared shell environment.
  (`AI_AGENT` / `CLAUDE_EFFORT` from the verification are *Claude Code's own* injected
  vars used here only as test probes — not docket's, so not ours to rename. Minor
  adjacent debt: the scripts' non-namespaced `GIT` / `REPO` mock seams predate this
  and could be tightened separately.)
- **No drift.** It points at the single live clone — the same clone the skill
  symlinks already point into — so skills (live) and scripts (live) stay
  version-matched automatically. This is the property that makes it strictly
  better than copying.
- **Reuses existing plumbing.** `scripts/ensure-claude-settings.sh` (change 0027)
  already writes `<repo>/.claude/settings.local.json` idempotently with `jq`; the
  same pattern writes the `env` entry. Low-surface, well-trodden.

### B. Copy/symlink the scripts into a relative `.claude/` dir — alternative

- Place the scripts at a stable relative spot that does **not** collide with the
  project's own `scripts/` (the original confusion): e.g. `<repo>/.claude/docket/scripts/`,
  resolved by skills as `.claude/docket/scripts/docket-config.sh`. Self-contained per repo.
- Trade-off: a **copy drifts** — the scripts are actively developed in lockstep
  with the skills, so a copied script lags a live skill edit; you'd need a re-sync
  on every docket update (an `install`/`migrate --repair` step). A **symlink** at
  that path avoids drift but does not survive a fresh clone on another machine.
  The only thing copying buys is hermetic, version-pinned reproducibility — not
  the goal here.

### Either way

- **Fail loud, not silent.** Whichever resolution is chosen, an unresolvable
  scripts dir must STOP with an actionable message (`DOCKET_SCRIPTS` unset / scripts
  not found → run `docket/install.sh`), never the bare `no such file or directory`
  that the agent silently works around today.
- **Back-fill already-migrated repos** (markhaus has the scripts absent now) — the
  env-var route fixes this automatically once `install.sh` is re-run (user-level,
  repo-agnostic); the copy route needs a per-repo repair pass.

*Also considered (heavier, demoted):* a `docket` CLI dispatcher on `PATH`
(equivalent reach to A but adds a dispatcher layer + a `PATH` install); resolving
the scripts dir from the skill symlink's own realpath at Step 0 (no install change,
but fragile across harnesses and non-symlink installs); a per-repo `link-scripts.sh`
shim. All are strictly more moving parts than A for the same outcome.

## Out of scope

- Rewriting the scripts' internal logic — they work; the defect is *reachability*.
- Removing the convention's "prose is the contract" manual-fallback documentation
  — it stays as a true last resort, but should no longer be the *de facto* path
  in a normal consuming-repo run.
- Re-litigating the symlink-vs-copy model for *skills* (`link-skills.sh`) — this
  change is about the **scripts**, which today have no distribution path at all.

### Verification — Claude Code settings `env` (2026-06-20)

The load-bearing assumption was checked against the official Claude Code docs
([env-vars](https://code.claude.com/docs/en/env-vars.md), [settings](https://code.claude.com/docs/en/settings.md)):

- ✅ **`settings.json` `env` injects into the Bash tool.** Docs: *"Every command
  executed via the Bash tool and every hook script can read these variables…
  Claude Code injects these key-value pairs into the session's environment at
  startup."* So `env.DOCKET_SCRIPTS` is visible to skill Bash calls.
- **Read at session start** — a settings edit takes effect on the next `claude`
  launch, not mid-session. Fine for an install-time write.
- **Literal strings only** — no `${…}`/`~` expansion, no dynamic eval; `install.sh`
  writes the already-resolved absolute path, so this is a non-issue.
- **Precedence (per-key merge, higher wins):** managed > CLI > local
  (`.claude/settings.local.json`) > project (`.claude/settings.json`) > user
  (`~/.claude/settings.json`).
- ✅ **Subagent propagation — empirically confirmed for the shell-profile route.**
  The docs don't cover whether settings `env` reaches spawned Task-agent Bash
  shells, so it was tested directly: each Bash call (main session AND dispatched
  subagent) runs `/bin/zsh` and **sources `~/.zshrc`** — a dispatched subagent saw
  the profile-only vars `ZSH`, `TF_PLUGIN_CACHE_DIR`, `OPENTUI_FORCE_WCWIDTH`. So a
  shell-profile `export` **does** propagate to subagent Bash shells. Conversely,
  Claude-process-injected vars are *not* uniformly inherited (`CLAUDE_EFFORT` was
  set in the main session but **UNSET** in the subagent; `AI_AGENT` was present in
  both) — which confirms why the design leads with the **profile `export`**
  (re-sourced on every Bash call, main or subagent) rather than relying on
  settings-`env`/process injection for docket's script-running subagents. Settings
  `env` stays as main-session reinforcement.
- Minor: a known Windows bug (gh #20112) where settings `env` isn't injected into
  Bash — docket is bash/macOS-Linux, low concern.

## Open questions

Approach **A (env var)** is viable and the injection mechanism is settled: the
shell-profile `export` is the primary (empirically reaches subagents), settings
`env` is main-session reinforcement (verification above). Remaining unknowns:

- **Multi-shell profile injection.** The helper *scripts* are shell-agnostic
  (`#!/usr/bin/env bash` — they run via their shebang regardless of the user's login
  shell, so no per-shell port is needed there). But the **profile `export` in (1) is
  shell-specific** and must match whatever shell the Bash tool actually launches
  (here it ran `/bin/zsh` = the user's `$SHELL`). `install.sh` must therefore detect
  the user's shell(s) and write to the right rc with the right syntax:
  zsh → `~/.zshrc`/`~/.zshenv` (`export`), bash → `~/.bashrc`/`~/.bash_profile`
  (`export`), **fish → `~/.config/fish/config.fish` with `set -gx VAR val`** (not
  `export`), and ideally tolerate others. Open: support which set (zsh+bash+fish as
  the floor?), write-all-present vs detect-`$SHELL`, and prefer an always-sourced
  file where one exists (zsh `~/.zshenv`). The settings-`env` reinforcement is
  shell-agnostic, so it cushions the main session if a shell's profile write is missed.
- **Per-harness injection:** docket targets `.claude`/`.codex`/`.cursor`/… — the
  shell-profile `export` is harness-agnostic, but does each harness also have a
  settings-style `env` worth writing, and does `install.sh` write all present ones
  (mirroring how `link-skills.sh` links into each present harness)?
- **Should the manual prose fallback stay?** Once the scripts are reliably
  reachable + fail loud, is the convention's "prose is the contract" path retired
  to true-last-resort, or kept behind an explicit override?
- **CI / drift guard:** mirror `sync-agents.sh --check` with a check that a
  consuming repo can actually resolve `docket-config.sh` via `DOCKET_SCRIPTS`
  (catch this whole class early).
