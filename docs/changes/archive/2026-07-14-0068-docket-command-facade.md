---
id: 68
slug: docket-command-facade
title: One executable docket facade — finite subcommands, config read from stdout, never eval'd
status: done
priority: high
created: 2026-07-13
updated: 2026-07-14
depends_on: []
related: [48, 63, 65, 71, 72, 73]
adrs: [12, 25, 27, 29]
spec: docs/superpowers/specs/2026-07-13-docket-command-facade-design.md
plan: docs/superpowers/plans/2026-07-13-docket-command-facade.md
results: docs/results/2026-07-13-docket-command-facade-results.md
trivial: false
auto_groomable:
branch: feat/docket-command-facade
pr: https://github.com/danielhanold/docket/pull/78
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-13-docket-command-facade-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-13-docket-command-facade-design.md) |
| Plan | [2026-07-13-docket-command-facade.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-13-docket-command-facade.md) |
| Results | [2026-07-13-docket-command-facade-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-13-docket-command-facade-results.md) |
| PR | [#78](https://github.com/danielhanold/docket/pull/78) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0025](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0025-docket-worktrees-disable-git-hooks.md), [ADR-0027](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0027-terminal-publish-repo-scoped-script-gated.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md) |
<!-- docket:artifacts:end -->

## Why

Cursor's auto-run classifies every leaf command of a submitted shell program before deciding
whether the program may run outside the sandbox; one non-allowlisted leaf — even an unreachable
one — demotes the whole program. Docket's Step-0 prose asks agents to compose `eval`, branching,
worktree creation, hook setup, and fetch/pull into exactly such programs, so the permission
surface is unboundable.

The deeper problem is not Cursor-specific: `eval "$(docket-config.sh --export)"` stores resolved
config in the agent's **shell**, and shell-state persistence across tool calls is
harness-dependent — Claude Code keeps none, and any harness's shell can restart and silently drop
the exports. The model's context window is the only state guaranteed to persist across tool calls
in every harness — and the model already reads everything the resolver prints before eval'ing it.
The shell round-trip adds nothing but fragility and the un-allowlistable command shapes.

## What changes

- Add one executable facade, `scripts/docket.sh`: a finite table of named operations (operation
  name = daily helper basename), no `run`/`exec`/`shell`/`eval` escape hatch, behavior kept in
  the existing helpers — the facade is a routing boundary, not a second implementation.
- Config flows model-ward: `docket.sh env` prints fully resolved `KEY=value` lines (absolute
  paths, `BOOTSTRAP` verdict) to stdout; agents read the values and interpolate them as literals
  into later commands. Nothing docket emits is ever `eval`'d or `source`d by an agent.
- `docket.sh preflight` performs today's Step-0 side effects (bootstrap verdict fail-closed,
  metadata worktree ensure + hook disable, fetch/pull) as a plain executable op and prints the
  env block on success; re-running it is the sanctioned mid-run re-sync.
- Self-sufficiency invariant: every operation needs only the profile-injected
  `DOCKET_SCRIPTS_DIR`; metadata-touching ops run the shared preflight internally. No operation
  assumes a persistent shell or prior exports. `docket-status.sh` reuses the shared preflight
  implementation instead of its private worktree-sync copy.
- Exactly one canonical spelling of the invocation, enforced by tests; the facade's subcommand
  table (in its `scripts/docket.md` contract) is the permission inventory, guarded by a
  grep-derived, mutation-tested sentinel.
- Hermetic preflight/dispatch/env-output tests per the spec.

## Out of scope

- Rewiring the operating skills and the convention's Step-0 preamble to the facade — change
  0072 (depends on this one).
- The Cursor permissions/sandbox guide, published permission fragment, trust-tier doc, and
  troubleshooting examples — change 0073.
- Automatically editing a user's `~/.cursor/permissions.json` or `~/.cursor/sandbox.json`.
- Blanket approval of every shell file, `bash`, or arbitrary `eval`; any facade operation that
  executes caller-provided shell text.
- Changing helper behavior, close-out semantics, or the existing provenance/fail-closed guards.

## Decisions

- The model's context, not the shell, is where resolved config lives: values are read from
  stdout and interpolated as literals — the `eval` pattern is retired.
- There is no source-only mode; everything is a plain executable op (rejected: source-only
  preflight exporting into "the persistent agent shell" — a harness-dependent premise, false in
  Claude Code, with an undetectable stale-shell failure mode).
- The facade is the trust boundary; individual helper leaves are never allowlisted for docket.
- Operation names are helper basenames so the inventory stays grep-derivable — no alias table
  to drift.
- Split from the original 0068 scope: facade engineering here, skill rewiring in 0072, Cursor
  documentation in 0073.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-13 — reconcile (docket-implement-next)

Verified the same-day design against current `origin/docket` and the integration branch; no
scope change needed, no design invalidation. Findings:

- **Net-new, no collision.** `scripts/docket.sh` and `scripts/docket.md` do not exist yet — this
  is greenfield facade engineering.
- **Inventory is complete and current.** All 16 `scripts/*.sh` map cleanly to the spec's table:
  13 exposed operations (`env`, `docket-status`, `board-refresh`, `archive-change`,
  `terminal-publish`, `cleanup-feature-branch`, `github-mirror`, `sync-integration-branch`,
  `render-change-links`, `render-adr-index`, `adr-checks`, `board-checks`, plus the new
  `preflight`), and the not-exposed set (`docket-config.sh` reached via `env`,
  `disable-worktree-hooks.sh`, `render-board.sh` internal to `board-refresh`, and the
  human-initiated `install.sh`/`migrate-to-docket.sh`/`sync-agents.sh`/`ensure-*.sh`). Nothing
  new has been added to `scripts/` that the table would miss.
- **Preflight extraction target confirmed.** `docket-status.sh`'s private
  `ensure_and_sync_worktree` is at lines 42–60 today (spec cited 40–56; the body is unchanged in
  substance — worktree ensure → `disable-worktree-hooks.sh` → fetch + `pull --rebase`, with the
  `main`-mode `git pull --rebase` degrade). It reuses `CONFIG_EXPORT_CMD`/`GIT`/`GH` mock seams
  the shared preflight should preserve.
- **`env` presentation constraint (build note, not a scope change).** `docket-config.sh`'s
  `emit()` prints `%q`-**shell-quoted** `KEY=value` lines (built for `eval`). The spec's `env` op
  must present *raw*, unquoted `KEY=value` for the model to read as literals — so the build must
  either add a plain-emit path to the resolver or un-`%q` its output; the planner should pick and
  test this explicitly (the resolver stays the single source of resolved values).
- **Related changes.** #48/#63/#65 are `done` (no overlap with this facade); #71
  (board-surfaces-unset-vs-empty), #72 (facade-skill-rewiring), #73 (cursor-guide) remain
  `proposed` follow-ups — #72/#73 correctly still depend on this landing first. Out-of-scope
  boundaries hold.
- **Sentinel guidance intact.** LEARNINGS #64 (derive gated call-site lists by grep, never by
  hand; mutation-test the sentinel) and #64b (an aborting resolver emits nothing → clear the
  asserted var before re-reading) are present and directly govern the inventory sentinel and the
  `env` empty-output test this change ships.
