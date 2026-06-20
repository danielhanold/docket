---
id: 29
slug: sync-integration-after-merge
title: Fast-forward the local integration branch after a docket merge
status: proposed
priority: high
created: 2026-06-20
updated: 2026-06-20
depends_on: []
related: [25, 26, 28]
adrs: [7]
spec: docs/superpowers/specs/2026-06-20-sync-integration-after-merge-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Why

Closing out change 0026, finalize dropped a `status: done` edit onto `main`. The
post-mortem (recorded in the kill of change 0028) found the cause was a **stale skills
source**, not a bug in finalize's logic: `~/.claude/skills/*` are symlinks into the
docket clone's primary checkout, that checkout was 39 commits behind
`origin/<integration_branch>`, and the harness therefore loaded the *pre-0025*
manual-archive skill. The tree drifts because, in docket-mode, the primary checkout is
never fast-forwarded — all work happens in `.docket/` and feature worktrees.

A skill's bytes on the integration branch change **only** via a merged PR, and the only
docket operations that merge are **finalize** and the **`docket-status` sweep**. So
staleness is created in exactly two places — fix it there, with the ordinary
merge-then-sync reflex, instead of detecting drift everywhere after the fact.

## What changes

Per [the spec](../../superpowers/specs/2026-06-20-sync-integration-after-merge-design.md):

- A small **best-effort, FF-only** helper `scripts/sync-integration-branch.sh` (guarded:
  acts only when the checkout is on `<integration_branch>` and clean and a true
  fast-forward is possible; every skip is a normal exit-0 with a note — never aborts or
  alters the close-out, the `github-mirror.sh` posture, not the fail-closed one).
- **Both merge sites invoke it once, at end of run:** `docket-finalize-change` (after the
  board step) and the `docket-status` merge sweep. Omitting the sweep would leave
  swept close-outs stale, so both are required.
- One-line prose per site + a one-sentence pointer in `docket-convention`'s Branch model
  as the single documented source.
- `tests/test_sync_integration_branch.sh`, hermetic (FF / dirty / wrong-branch / non-FF /
  already-current / fetch-failure), in the `test_closeout.sh` style.

## Out of scope

- **No Step 0 staleness guard** (detect-and-warn at config resolution) — the two merge
  sites are the cause; outside-flow drift self-heals at the next finalize/sweep.
- **No auto-restart / no fix for the current run** — new skill bytes load only at process
  start; this keeps *future* sessions fresh.
- **No re-link / `sync-agents.sh`** — a FF refreshes existing skills' content; brand-new
  skills/agents still need those install-time steps (rare, orthogonal).
- **Consumer-repo skill freshness** — there the skills clone is a separate repo a project
  merge doesn't advance; keeping it current is a separate "update docket" workflow. The
  skill-staleness guarantee here is specific to dogfooding docket on itself.

## Open questions

- **Helper vs inline.** The spec proposes a dedicated script (reused by both sites,
  testable); if reconcile finds it small enough, an inline guarded block in each site is
  acceptable — the end behavior is identical.

## Reconcile log
