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

Make the deterministic helper scripts **reliably reachable from any consuming
repo**, and make their absence **fail loud** instead of degrading silently. The
actual mechanism is a design decision to be groomed — candidate approaches,
roughly in order of preference:

1. **Ship a `docket` CLI on `PATH` (recommended).** Have `install.sh` symlink a
   single dispatcher (or each `docket-*.sh`) into a `PATH` dir (`~/.local/bin`,
   or alongside the harness install), and change the skills to call
   `docket config --export` / `docket render-board …` / `docket archive-change …`
   etc. The skills are *already* globally available via symlink and run from
   arbitrary consuming-repo CWDs; the scripts should be globally resolvable the
   same way, with no per-repo install and no CWD assumption. One install, every
   repo. (Keep `scripts/` as the implementation behind the dispatcher.)

2. **Resolve an absolute scripts dir at skill Step 0.** Since each installed
   skill is an absolute symlink back into `~/dev/docket/skills/<name>`, a skill
   can resolve its own real path (`readlink`/`realpath`) to derive
   `…/docket/scripts/` and invoke scripts through that absolute dir
   (`"$DOCKET_SCRIPTS"/docket-config.sh …`). No install change, but the
   resolution preamble must be harness-agnostic and robust to non-symlink
   installs.

3. **Vendor the scripts into the consuming repo at migrate time.** Have
   `migrate-to-docket.sh` copy (or symlink) `scripts/docket-*.sh` + `scripts/lib/`
   into the target repo's `scripts/`, **committed**, so every clone/agent/device
   has them (the same reproducibility argument the convention makes for committed
   config). Makes the bare `scripts/…` path resolve as-written. Costs: copies
   drift from the source repo, symlinks don't survive other clones/machines, and
   the consuming repo gets pinned to a docket version.

4. **Symlink a per-repo `scripts/`/`.docket/bin` shim via an install step.** A
   `link-scripts.sh` (sibling of `link-skills.sh`) that points a known per-repo
   location at the docket `scripts/`. Less clean than `PATH`, repo-local.

5. **Guardrail (pair with whichever real fix): fail-closed Step 0 presence
   check.** Each skill's Step 0 detects an unresolvable `docket-config.sh` and
   **STOPs with an actionable remediation** ("docket scripts not found — run
   `bash /path/to/docket/install.sh`"), rather than emitting a bare
   `no such file or directory` and silently hand-working the rest. This turns a
   silent guarantee-loss into a loud, fixable error and is cheap to add now.

A one-off **repair for already-migrated consuming repos** (markhaus today has the
scripts absent) is part of the rollout, however the mechanism is chosen.

## Out of scope

- Rewriting the scripts' internal logic — they work; the defect is *reachability*.
- Removing the convention's "prose is the contract" manual-fallback documentation
  — it stays as a true last resort, but should no longer be the *de facto* path
  in a normal consuming-repo run.
- Re-litigating the symlink-vs-copy model for *skills* (`link-skills.sh`) — this
  change is about the **scripts**, which today have no distribution path at all.

## Open questions

- **Which approach** (CLI-on-`PATH` vs absolute-resolve-at-Step-0 vs
  vendor-at-migrate vs per-repo shim) — and is it one mechanism or a
  primary + the Step-0 guardrail?
- **Harness-agnostic resolution:** if we resolve via the skill symlink, how do we
  stay robust across `.claude`/`.codex`/`.cursor`/… and non-symlink installs?
- **Reproducibility vs drift:** a committed per-repo copy guarantees every clone
  has the scripts but pins a docket version and can drift; a `PATH`/symlink model
  avoids drift but depends on machine setup. Which guarantee matters more?
- **Fail-closed vs documented fallback:** should Step 0 hard-STOP on missing
  scripts, or keep the manual prose path available behind an explicit flag?
- **Back-fill:** how do already-migrated repos (markhaus) get repaired — a
  `migrate-to-docket.sh --repair`, an `install.sh` step, or a documented one-liner?
- **CI / drift guard:** mirror `sync-agents.sh --check` with a check that a
  consuming repo can actually resolve `docket-config.sh` (catch this class early).
