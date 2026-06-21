---
id: 34
slug: consuming-repo-script-resolution
title: Helper scripts unreachable in consuming repos — skills call repo-relative `scripts/…` that exists only in the docket source repo
status: done
priority: high
created: 2026-06-20
updated: 2026-06-21
depends_on: []
related: [37]
adrs: [12, 14]
spec: docs/superpowers/specs/2026-06-21-consuming-repo-script-resolution-design.md
plan: docs/superpowers/plans/2026-06-21-consuming-repo-script-resolution.md
results: docs/results/2026-06-21-consuming-repo-script-resolution-results.md
trivial: false
auto_groomable: false
branch: feat/consuming-repo-script-resolution
pr: https://github.com/danielhanold/docket/pull/45
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-06-21-consuming-repo-script-resolution-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-06-21-consuming-repo-script-resolution-design.md) |
| Plan | [2026-06-21-consuming-repo-script-resolution.md](https://github.com/danielhanold/docket/blob/feat/consuming-repo-script-resolution/docs/superpowers/plans/2026-06-21-consuming-repo-script-resolution.md) |
| Results | [2026-06-21-consuming-repo-script-resolution-results.md](https://github.com/danielhanold/docket/blob/feat/consuming-repo-script-resolution/docs/results/2026-06-21-consuming-repo-script-resolution-results.md) |
| PR | [#45](https://github.com/danielhanold/docket/pull/45) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md) |
<!-- docket:artifacts:end -->

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

- Introduce **`DOCKET_SCRIPTS_DIR`** (absolute path to the docket clone's `scripts/`). Skills
  resolve every call as `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}/<name>.sh"` — the `:?`
  makes a missing install fail loud with the remedy.
- **`install.sh` injects it** (it already holds the absolute path): primary = a
  **shell-profile `export`** (re-sourced on every Bash call, so it reaches the
  subagents docket dispatches), reinforcement = user-level `~/.claude/settings.json`
  `env`. Points at the **live clone** the skill symlinks already use → **zero drift**.
- Re-running `install.sh` **back-fills** already-migrated repos (markhaus included).
- The `:?` form **fails loud** (no more silent degradation). The existing per-skill
  manual-fallback prose is **left in place** here — slimming the skills by relocating it to
  on-demand sibling files (progressive disclosure) is **follow-up #37**.
- A **drift-guard** fails when a consuming repo can't resolve the scripts via
  `DOCKET_SCRIPTS_DIR`, or when a skill body still uses a bare `scripts/<name>.sh` path. It
  lands as a **test-suite static audit** (mirroring `tests/test_change_links_coverage.sh`
  and how `sync-agents.sh --check` is exercised by `tests/test_sync_agents.sh`) — the repo
  has no `.github/workflows/` CI today, so the test suite is the de-facto gate.

Full design — injection mechanics, the shell floor (zsh/bash `export`, fish `set -gx`,
POSIX-`export` fallback for others — grounded in how Starship/zoxide/mise/rustup/Homebrew
do it), the verification, the `DOCKET_` namespacing constraint, and the alternatives
weighed (copy-into-`.claude` rejected for drift; PATH-CLI / realpath / per-repo-shim
demoted) — is in the linked **spec**.

## Out of scope

- Copy/symlink vendoring of the scripts into the consuming repo (rejected — drift).
- The heavier resolutions (PATH CLI dispatcher, realpath-from-symlink, per-repo shim).
- Rewriting the scripts' internal logic; **relocating the per-skill manual-fallback prose**
  (deferred to follow-up #37); tightening the non-namespaced `GIT`/`REPO` mock seams;
  Windows profile injection.

## Open questions

Shell floor and the CI drift-guard are now decided (see spec); the manual-fallback
relocation is follow-up #37. One build-time question remains:

- Whether `install.sh` writes the settings-`env` *reinforcement* for **each present
  harness** (`.claude`/`.codex`/`.cursor`/…, looping like `link-skills.sh`) or only Claude
  Code — low-stakes, since the shell-profile `export` is harness-agnostic and is the actual
  guarantee.

## Reconcile log

### 2026-06-21 — implement-next reconcile (pre-plan)

Verified the spec against current `origin/main` (46c4520, in sync). The change is **valid and
the design holds** — not obsolete, not invalidated. Every premise checks out against live code:

- **`install.sh`** runs only `link-skills.sh` + `sync-agents.sh` (no script injection) and
  already exposes the docket clone's absolute path as `SCRIPT_DIR` → inject
  `DOCKET_SCRIPTS_DIR="$SCRIPT_DIR/scripts"`. **`link-skills.sh`** symlinks skill dirs only;
  **`migrate-to-docket.sh`** resolves only its own `$MIGRATE_DIR/scripts/ensure-claude-settings.sh`
  and vendors nothing else. No `DOCKET_SCRIPTS_DIR` exists anywhere yet. Defect confirmed current.
- **Rewrite scope measured:** ~65 bare `scripts/<name>.sh` call sites across **9 files** — the 8
  skill/agent bodies (`docket-status` 13, `docket-implement-next` 11, `docket-finalize-change` 10,
  `docket-convention` 10, `docket-new-change` 7, `docket-adr` 7, `docket-groom-next` 3,
  `docket-auto-groom` 3) plus `docket-convention/github-board-mirror.md` (1) — all switching to
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh`.
- **Folded in (new constraint): wiring-sentinel updates are in-scope.** The rewrite removes the
  literal `scripts/<name>.sh` substring those skill bodies carry, breaking ~8 existing sentinel
  tests that grep for it (`test_change_links_coverage`, `test_docket_config`, `test_render_board`,
  `test_closeout`, `test_adr_checks`, `test_render_adr_index`, `test_board_checks`). They must be
  updated in lockstep to the `DOCKET_SCRIPTS_DIR`-resolved form. Doc-reference greps that point at
  the docket clone's own path (e.g. `test_ensure_claude_settings.sh`'s README check) stay as-is.
- **Drift-guard realization adjusted:** no `.github/workflows/` CI exists; it lands as a
  test-suite static audit (the de-facto gate), per the body edit above.
- **Settings-`env` reinforcement** targets user-level `~/.claude/settings.json` `.env.DOCKET_SCRIPTS_DIR`
  via the idempotent-`jq` pattern from `ensure-claude-settings.sh` (a distinct file from that
  script's per-repo `settings.local.json` permissions write). Open question (per-harness vs
  Claude-only) stays low-stakes — default to Claude Code only; the harness-agnostic profile
  `export` is the actual guarantee. Final call deferred to the plan.

No scope dropped; no work already done elsewhere. Proceeding to plan.
