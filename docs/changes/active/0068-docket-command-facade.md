---
id: 68
slug: docket-command-facade
title: One executable docket facade — finite subcommands, config read from stdout, never eval'd
status: in-progress
priority: high
created: 2026-07-13
updated: 2026-07-13
depends_on: []
related: [48, 63, 65, 71, 72, 73]
adrs: [12, 25, 27]
spec: docs/superpowers/specs/2026-07-13-docket-command-facade-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/docket-command-facade
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-13-docket-command-facade-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-13-docket-command-facade-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0025](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0025-docket-worktrees-disable-git-hooks.md), [ADR-0027](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0027-terminal-publish-repo-scoped-script-gated.md) |
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
