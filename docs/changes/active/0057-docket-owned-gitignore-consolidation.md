---
id: 57
slug: docket-owned-gitignore-consolidation
title: Fold the migration-time .gitignore entries into the managed docket:generated block
status: in-progress
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
branch: feat/docket-owned-gitignore-consolidation
pr:
blocked_by:
reconciled: true
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

### 2026-07-10 — reconcile before build

Spec is same-day fresh (groomed 2026-07-10) and every assumption verifies against current
`origin/main` (`d7f4a96`). Design holds, scope unchanged; **not** obsolete, **not** fundamentally
invalidated. Verified against current code:

- `sync-agents.sh` — the 0051 managed block (`# docket:generated:start … sync-agents.sh …` /
  `:end`), `emit_gitignore_block()` looping the static `VALID_HARNESS_TOKENS`
  (claude codex cursor agents kiro windsurf) + `HARNESS_HAS_DISPATCH_RULES` (cursor) with **no**
  core entries yet, `gitignore_block_wanted` (opted-in **or** `.docket.local.yml`),
  `gitignore_block_unterminated` on the single 0051 spelling, and `--check` leg (a) — all present
  exactly as the spec describes. The "constant emitter" load-bearing fact confirmed.
- `migrate-to-docket.sh` step 5 — appends the three bare lines (`.docket/`, `.worktrees/`,
  `.claude/settings.local.json`) in `PRUNE_WT/.gitignore` and commits on the integration branch.
- `docket-config.sh` `create_orphan` (the `--bootstrap`/`CREATE_ORPHAN` path) — worktree-free,
  writes **no** `.gitignore`; the fresh-repo gap is real.
- ADR-0020 — `Accepted`, present on both `docket` and `origin/main`; sections Context/Decision/
  Consequences. The dated `## Update` (decision 3, ownership broadening) appends after
  `## Consequences` as a **metadata edit on the `docket` branch**, delivered to `origin/main` via
  this change's `adrs: [20]` at terminal-publish — **not** feature-branch code (the feature branch
  never modifies ADRs).

**Coordination notes (no scope change):**

- Two open PRs overlap only the *prose-sweep* surface, neither a dependency, both sharing the
  `origin/main` base 0057 builds on: **PR #61 (0052)** rewrites `README.md`; **PR #62 (0053)**
  restructures `docket-convention` SKILL.md (moving the Agent-layer deep-dive into
  `references/agent-layer.md`). 0057's `docket:generated`→new-marker prose edits land against
  *current* `origin/main`; if either merges first, finalize's rebase-retest gate surfaces the
  README/convention conflict for resolution. Keep the prose-sweep edits minimal and marker-scoped
  so that conflict surface stays small.
- The shell-script consolidation (new `scripts/lib/docket-gitignore-block.sh`, the three writers,
  block contents) touches files neither open PR modifies — no code-level overlap.
