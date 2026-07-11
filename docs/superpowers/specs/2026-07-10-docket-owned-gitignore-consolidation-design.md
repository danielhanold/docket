# Docket-owned .gitignore consolidation — design

**Date:** 2026-07-10
**Change:** #0057 (`docket-owned-gitignore-consolidation`)
**Status:** Approved (groomed interactively with Daniel, 2026-07-10)

## Problem

docket writes `.gitignore` entries from two unrelated places with two different lifecycles:

1. **`migrate-to-docket.sh` step 5** appends three bare, unmanaged lines once at migration
   time: `.docket/`, `.worktrees/`, `.claude/settings.local.json`. No repair exists — delete
   the `.docket/` line and nothing restores it; an accidentally committed `.docket/` worktree
   is an ugly failure mode.
2. **`sync-agents.sh`** (change 0051, ADR-0020 decision 3) owns a marker-bounded
   `# docket:generated:start/end` block that is self-healing — regenerated whenever missing or
   stale, CI-gated by `--check` leg (a).

Two gaps beyond the stub's framing, found during grooming:

- **Fresh-repo gap:** `docket-config.sh --bootstrap` (the `CREATE_ORPHAN` path for repos born
  in docket-mode) writes no `.gitignore` entries at all — only *migrated* repos ever get the
  three lines. A fresh docket-mode repo can commit its `.docket/` worktree today.
- **`ensure-claude-settings.sh` assumes its output is gitignored** but nothing it runs ensures
  that; the guarantee currently rests entirely on migrate's one-time append.

## Decision

**Full consolidation.** The managed block becomes the single home for ALL docket-owned
ignores. A shared lib owns the mechanics; three writers call it; `sync-agents.sh` keeps
self-healing it with a widened trigger. The markers are renamed to reflect the broadened
contents and ownership, with a one-time in-place upgrade of the day-old 0051 markers.

### Load-bearing fact discovered during grooming

`emit_gitignore_block()`'s output is a **pure constant**: the roster patterns loop over the
static harness table (`HARNESS_AGENT_DIRS` → claude, codex, cursor, agents, kiro, windsurf;
`HARNESS_HAS_DISPATCH_RULES` → cursor), not the enabled config. Adding the three core entries
keeps it constant — so every writer sharing the emitter emits **identical bytes by
construction**, and the stub's "second roster" fear dissolves. This is what makes the
three-writer design safe.

## Design

### 1. The shared lib — `scripts/lib/docket-gitignore-block.sh`

Sourceable helper following the `docket-frontmatter.sh` discipline (no side effects on source
beyond declaring functions/constants). It owns:

- **The canonical harness roster.** The token table (and the dispatch-rule harness list) MOVES
  here from `sync-agents.sh`, which then sources the lib and derives its generation targets
  from it — one roster, genuinely shared rather than duplicated between generation and
  emission. (`link-skills.sh`'s own `HARNESS_SKILL_DIRS` mirror stays put — a noted, tracked
  divergence, out of scope here.)
- **Marker constants, new and legacy.** New: `# docket:start (managed by docket — do not
  hand-edit)` / `# docket:end`. The legacy 0051 spellings
  (`# docket:generated:start (managed by sync-agents.sh — do not hand-edit)` /
  `# docket:generated:end`) are kept as constants **only for upgrade detection**.
- **`emit_docket_gitignore_block()`** — constant bytes, in order: the core entries
  `.docket/`, `.worktrees/`, `.claude/settings.local.json`, then `.docket.local.yml`, then the
  per-harness generated-artifact patterns (`.<H>/agents/docket-*.md`,
  `.<H>/rules/docket-dispatch.mdc`). Config-independent.
- **`ensure_docket_gitignore_block <repo-root>`** — the full hardened mechanics:
  - **Closed-block guard (both spellings):** a dangling start marker — new *or* legacy —
    with no matching end marker means the block's true extent is unknowable; refuse the edit,
    warn loudly, touch nothing (the LEARNINGS #51 rule, extended to the legacy spelling).
  - **One-time legacy upgrade:** a *closed* legacy block is removed outright — inside-markers
    content is docket-owned by definition, so removing it does not breach the outside-bytes
    invariant — and the new block written.
  - **Idempotence:** want-vs-have comparison; no write when current.
  - **Outside-bytes invariant:** bytes outside the markers are never modified.
  - **Dedup advisory:** when any of the three exact bare literals (`^\.docket/?$`,
    `^\.worktrees/?$`, `^\.claude/settings\.local\.json$`) sits outside the markers, log one
    advisory line — "old docket bare entries found outside the block — safe to delete by
    hand" — and write nothing. The literals could be user-authored (e.g. a pre-docket
    `.worktrees/`), so they are never deleted by the healer.

**Trigger policy stays with the callers** — the lib does mechanics only.

### 2. Three writers, three triggers

| Writer | When it calls ensure | Behavior |
|---|---|---|
| `migrate-to-docket.sh` step 5 | Unconditionally (it *is* the migration) | First removes the three bare lines it historically wrote (its own provenance; the same `^${entry%/}/?$` match it used to add them), then ensures the block — both in the prune worktree (`PRUNE_WT`), committed in step 5's existing integration-branch commit. Re-run idempotent. |
| `docket-config.sh --bootstrap` | On the `CREATE_ORPHAN` path, after creating/pushing the orphan | Ensures the block in the **primary working tree**, prints a loud **COMMIT THIS** notice, does **not** auto-commit — bootstrap runs inside a skill's startup; auto-committing to the user's integration branch from a config script crosses a write-scope line docket has held. `--export` stays strictly read-only. Closes the fresh-repo gap. |
| `sync-agents.sh` (every run) | Widened `gitignore_block_wanted`: per-repo opted-in **or** `.docket.local.yml` present **or** the docket branch exists (the bootstrap guard's `DOCKET` probe: `refs/remotes/origin/docket` or `refs/heads/docket`) **or** block markers (either spelling) already present in `.gitignore` (heal-if-present) | The ongoing self-heal. |

**Zero-surprise-writes justification (LEARNINGS #48):** the `DOCKET` probe is an explicit
repo-level signal — a docket branch only exists by deliberate migration or bootstrap — unlike
0048's bug, which keyed generation on mere `.docket.yml` presence. A repo with no docket
signal (no opt-in, no local config, no docket branch, no existing block) is untouched;
regression-tested.

### 3. `--check` and the upgrade path

`--check` leg (a)'s semantics are unchanged, evaluated against the **new** markers: a repo
still carrying the legacy block reads as missing/stale → CI-meaningful fail whose remedy is
"run `sync-agents.sh`" — which performs the upgrade; the human commits the result. Legs (b)
and (c) untouched.

### 4. Prose, contracts, docs (the LEARNINGS #49 end-to-end rule)

Every `docket:generated` reference updates to the new marker name:

- `docket-convention` SKILL.md — the Agent layer, the `--check` legs, and every
  `.gitignore`-block passage (grep for `docket:generated` across skills/).
- README — the block mentions and any migrate-step description.
- `migrate-to-docket.sh` header comments (step 5 description).
- `scripts/docket-config.md` — the `--bootstrap` section gains the seed-and-notice behavior;
  the `--export` read-only guarantee is restated untouched.

**ADR posture:** ADR-0020's decision 3 (block name, sole-`sync-agents.sh` ownership) gets a
dated `## Update` note delivered via this change's `adrs: [20]` listing at terminal-publish
(the LEARNINGS #17 atomic-delivery pattern); a new ADR only if the implementer judges the
ownership broadening non-obvious at build time.

### 5. Tests

- **Constant-emitter equivalence:** a migrate-seeded fixture and a sync-agents-healed fixture
  end with byte-identical blocks.
- **Legacy upgrade:** a 0051-marker block + surrounding user bytes → new-marker block, user
  bytes preserved verbatim, legacy block gone.
- **Dangling legacy marker** → refuse, warn, file untouched; the existing dangling-marker test
  re-pointed at the new spelling.
- **Widened trigger, positive:** a tracking-only repo *with* a docket branch heals the block.
- **Widened trigger, negative (the 0048 regression):** a repo with no docket signal → zero
  writes, `--check` stays a no-op.
- **Migrate end-state:** fresh migration ends with the block present and the bare entries
  absent; re-run idempotent.
- **Bootstrap seed:** `CREATE_ORPHAN` writes the block in the primary tree, prints the notice,
  commits nothing.
- **Dedup advisory:** a bare literal outside the block → advisory logged, line preserved.
- Existing 0051 marker tests updated to the new spelling.

## Rejected alternatives

- **Consolidate, current writers only** (no trigger widening, no bootstrap seed): less
  surgery, but leaves tracking-only repos with no self-heal and the fresh-repo gap open —
  rejected in favor of closing every gap found.
- **Don't consolidate — record why:** cheapest, but the "delete a line and nothing restores
  it" failure mode remains for exactly the highest-stakes entry (`.docket/`).
- **Keep markers verbatim:** zero migration code, but the block name permanently misdescribes
  its contents ("generated" covering hand-ensured settings and worktree dirs) and its
  ownership (three writers, one named). The legacy block is a day old; upgrade cost is tiny.
- **sync-agents.sh `--ensure-gitignore` flag** (migrate/bootstrap shell out to it): smallest
  diff and keeps the hardened code in place, but conflates agent generation with repo-ignore
  mechanics in one entrypoint; the sourceable-lib split is the cleaner ownership boundary
  (Daniel's call at groom time).
- **Healer deletes the bare duplicates outside the block:** breaches the outside-the-markers
  invariant and cannot distinguish docket-written lines from identical user-authored ones —
  advisory-only chosen instead.

## Out of scope / accepted gaps

- ADR-0020's generation semantics unchanged: *what* generates and *where* the artifacts land
  stay exactly as decided.
- No ignores beyond the enumerated set (core three + `.docket.local.yml` + roster patterns).
- `ensure-claude-settings.sh` unchanged — the block now guarantees its ignore in docket-mode.
- **Accepted gap, recorded:** tracking-only *main-mode* repos still get no ignores from
  anywhere — no docket branch, no migration, no opt-in. That is the status quo preserved, not
  a regression; main-mode is the backward-compat path.
- `link-skills.sh`'s duplicate harness-dir list — noted divergence, not consolidated here.
