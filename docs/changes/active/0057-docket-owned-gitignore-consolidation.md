---
id: 57
slug: docket-owned-gitignore-consolidation
title: Fold the migration-time .gitignore entries into the managed docket:generated block
status: proposed
priority: low
created: 2026-07-10
updated: 2026-07-10
depends_on: [51]
related: [51]
adrs: [20]
spec: docs/superpowers/specs/2026-07-10-docket-owned-gitignore-consolidation-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-10-docket-owned-gitignore-consolidation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-docket-owned-gitignore-consolidation-design.md) |
| ADRs | [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md) |
<!-- docket:artifacts:end -->

## Why

docket now writes `.gitignore` entries from two unrelated places with two different
lifecycles. `migrate-to-docket.sh` step 5 appends three bare, unmanaged lines once at
migration time (`.docket/`, `.worktrees/`, `.claude/settings.local.json`); change 0051's
managed `# docket:generated:start/end` block (owned by `sync-agents.sh`) covers the
machine-local generated agent artifacts and is self-healing — regenerated whenever missing
or stale. The bare entries have no such repair: if someone deletes the `.docket/` line,
nothing restores it, and an accidentally committed `.docket/` worktree is an ugly failure
mode. Daniel raised this while live-testing 0051 (2026-07-10): all docket-owned ignores
should plausibly live in the one marker-bounded, self-healing place.

## What changes

Full consolidation: the managed block becomes the single home for ALL docket-owned ignores.
The block's emitted content is a pure constant (core entries + the static harness roster),
so a shared emitter gives byte-identical output from every writer — the "second roster"
fear dissolves.

- New sourceable `scripts/lib/docket-gitignore-block.sh` owns the mechanics: the canonical
  harness roster (moved from `sync-agents.sh`, which sources it), the marker constants, the
  constant emitter (`.docket/`, `.worktrees/`, `.claude/settings.local.json`,
  `.docket.local.yml`, roster patterns), and the hardened ensure (closed-block guard on both
  marker spellings, legacy upgrade, outside-bytes invariant, dedup advisory). Trigger policy
  stays with the callers.
- Markers renamed — `# docket:start (managed by docket — do not hand-edit)` /
  `# docket:end` — with a one-time in-place upgrade of the day-old 0051
  `docket:generated` block; a dangling marker of either spelling refuses-and-warns.
- Three writers: `migrate-to-docket.sh` step 5 seeds the block instead of bare lines (and
  removes the three bare entries it historically wrote); `docket-config.sh --bootstrap`
  seeds it on `CREATE_ORPHAN` (write + loud COMMIT-THIS notice, no auto-commit — closes the
  fresh-repo gap where bootstrapped repos got no ignores at all); `sync-agents.sh`
  self-heals with a widened trigger (opted-in, `.docket.local.yml`, the bootstrap guard's
  `DOCKET` branch probe, or heal-if-present).
- In already-migrated repos the healer never deletes the old bare lines outside the block
  (they could be user-authored) — it logs a safe-to-delete advisory; duplicates are
  harmless.
- Prose sweep: every `docket:generated` reference in docket-convention, README,
  migrate header comments, and `docket-config.md` updates; ADR-0020 decision 3 gets a dated
  `## Update` via this change's `adrs:` listing.

## Out of scope

- Any change to which agent artifacts are generated or where (ADR-0020 semantics stay).
- Ignoring anything beyond the three existing migration entries + `.docket.local.yml` +
  the 0051 block patterns.
- `ensure-claude-settings.sh` (the block now guarantees its ignore in docket-mode).
- Tracking-only main-mode repos still get no ignores — status quo preserved, accepted gap.
- `link-skills.sh`'s duplicate harness-dir list — noted divergence, not consolidated here.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
