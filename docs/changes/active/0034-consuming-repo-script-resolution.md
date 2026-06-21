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
  *holds* the value. Have it write `env.DOCKET_SCRIPTS = "$SCRIPT_DIR/scripts"`
  into the **user-level** `~/.claude/settings.json` `env` block (and a shell-profile
  `export` as the cross-harness fallback). User-level ⇒ every repo and every
  worktree sees it, no per-repo step.
- Skills change `scripts/docket-config.sh` → `"${DOCKET_SCRIPTS:?run docket/install.sh}/docket-config.sh"`
  — a uniform, ~one-token-per-call-site edit. The `:?` form makes a missing/incomplete
  install **fail loud with the remedy**, folding the guardrail (was option 5) in for free.
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

## Open questions

Leaning toward **approach A (env var)**; the remaining unknowns are mostly about
the injection point:

- **Does the var reach the agent's non-interactive shell?** Confirm Claude Code
  injects `~/.claude/settings.json` `env` into the Bash-tool environment (the
  load-bearing assumption). If yes, that is the primary injector; a shell-profile
  `export` is the cross-harness fallback.
- **Per-harness injection:** docket targets `.claude`/`.codex`/`.cursor`/… — what
  is the equivalent `env` mechanism in each, and does `install.sh` write all the
  present ones (mirroring how `link-skills.sh` links into each present harness)?
- **User-level vs per-repo settings:** user-level `~/.claude/settings.json` is
  repo-agnostic (one write, every worktree) — preferred — but is committed/global;
  per-repo `.claude/settings.local.json` (the `ensure-claude-settings.sh` precedent)
  is gitignored/per-user and would need re-running per clone. Pick the level.
- **Should the manual prose fallback stay?** Once the scripts are reliably
  reachable + fail loud, is the convention's "prose is the contract" path retired
  to true-last-resort, or kept behind an explicit override?
- **CI / drift guard:** mirror `sync-agents.sh --check` with a check that a
  consuming repo can actually resolve `docket-config.sh` via `DOCKET_SCRIPTS`
  (catch this whole class early).
