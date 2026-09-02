<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0395 — Migrate CLAUDE.md/AGENTS.md agent-executed blocks to the capability-first idiom](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0395-migrate-claude-md-agents-md-agent-executed-blocks-to-the-cap.md)**
<!-- docket:backlink:end -->

# Spec — Migrate CLAUDE.md/AGENTS.md agent-executed blocks to the capability-first idiom

## Problem

Change 0394 ("Give Docket skills an authoritative compact CLI capability catalog", ADR-0104)
established the capability catalog as the authoritative executable CLI surface and migrated the
maintained workflow surfaces to fetch it and resolve executable spellings from it. Its Task-6
migration **inventory** was scoped to `skills/`, `agents/`, `cursor-rules/`, `scripts/`, `README`,
and `.docket.example.yml`; its **repoguard** enforcement guard was scoped to `skills/`, `agents/`,
`cursor-rules/`.

The two repo-root instruction files — `CLAUDE.md` and `AGENTS.md` — fell outside both scopes. They
still contain agent-executed blocks that hard-code CLI argv, which is exactly the class the catalog
exists to eliminate, and because no guard covers them the divergence from the migrated Cursor mirror
is silent. Change 0394's own results file recorded this as a deferred, out-of-scope minor.

Two blocks are affected in each file:

1. **"Run gate — bracket a dispatched implement-next run with the gate facade"** — hard-codes
   `docket run gate-before <workflow>` and `docket run gate-verdict <key>` (and the
   `--unattributed` / `gate-*` variants) as literal argv.
2. **"Rebuild the binary after a merge to main"** — hard-codes `docket development install --source …`
   as literal argv.

## Goals

- Migrate both blocks in **both** `CLAUDE.md` and `AGENTS.md` to the capability-first idiom, so the
  executable spellings are resolved from the catalog (`run.gate-before`, `run.gate-verdict`,
  `development.install`) rather than hard-coded, while preserving each block's operational contract
  (the gate sequencing, the retry-authorization semantics, the schema-bump deadlock caveat).
- Extend the repoguard enforcement guard's scope to cover `CLAUDE.md` and `AGENTS.md` so the parity
  cannot silently regress — a re-introduced hard-coded argv in these files must redden the guard.
- Regenerate any embedded/derived assets the guard-scope extension touches.

## Approach

### 1. Migrate the two blocks (both files)

For each of `CLAUDE.md` and `AGENTS.md`, rewrite the "Run gate" and "Rebuild the binary" blocks to
the same capability-first idiom change 0394 applied to the maintained surfaces (use the migrated
Cursor mirror and the migrated `skills/`/`agents/` blocks as the reference pattern — match their
house form exactly rather than inventing a new one). The migration is a **prose/idiom** change: the
step ordering, the "keep the printed key in your own notes", the `gate-*` report-line-not-exit-code
discipline, the "only `gate-retry-once` authorizes another dispatch" rule, and the schema-extending
install-deadlock caveat all survive unchanged — only the way the executable spelling is named
changes.

### 2. Extend the guard scope

Bring `CLAUDE.md` and `AGENTS.md` into the repoguard guard's file set. Reconcile the guard's
**exemption model** with these files: they are human-and-agent-facing instruction prose, which is a
different shape from the generated/guarded workflow surfaces the guard was built for, so some
human-remedy or documentation argv may legitimately need honestly-pinned exemptions (as 0394 did for
`repository migrate` / `repository init` / `repository configure-tests` / `change create`). Follow
0394's precedent: key the guard on syntactic **shape**, pin exemptions by count with a distinguishing
comment, and never launder an un-migrated agent-executed site into an exemption. Prefer extending the
existing guard over adding a parallel one.

### 3. Coverage

- A mutation-style test proving the guard **reddens** when a hard-coded `docket run gate-*` or
  `docket development install` argv is re-introduced into `CLAUDE.md` or `AGENTS.md`, and is green on
  the migrated files (mutation-test the guard, per the repo's guard-is-code rule).
- Regenerate and verify any embedded copy of these files or of the guard's file-set manifest.

## Out of scope

- Introducing new catalog leaves or changing the catalog protocol.
- Migrating any surface not named here.
- JSON-schema / request-shape discovery (owned by change 0360).
- Rewriting historical changes, specs, plans, results, or Accepted ADR prose.

## Open questions

- Does the guard's current file-set abstraction take repo-root paths cleanly, or does covering files
  that sit outside `skills/`/`agents/`/`cursor-rules/` require a structural change to how the guard
  enumerates its surfaces? Resolve during reconcile/build against the shipped 0394 guard.
- Are there agent-executed argv in `CLAUDE.md`/`AGENTS.md` **beyond** the two named blocks that the
  guard-shape check will surface once these files are in scope? If so, migrate them in the same pass
  rather than exempting them.

## Acceptance criteria

1. Neither `CLAUDE.md` nor `AGENTS.md` hard-codes `docket run gate-before`, `docket run gate-verdict`,
   or `docket development install` argv on an agent-executed surface; both blocks resolve the
   executable spelling from the catalog in the same idiom as the migrated Cursor mirror.
2. Each block's operational contract is preserved verbatim in meaning (gate sequencing, retry
   authorization, schema-bump install-deadlock caveat).
3. The repoguard guard covers `CLAUDE.md` and `AGENTS.md`; a re-introduced hard-coded argv in either
   file reddens the guard, and the migrated files are green.
4. Any remaining exemptions on these files are honestly pinned by shape/count with a distinguishing
   comment, with zero laundered agent-executed sites.
5. Embedded/derived assets touched by the scope extension are regenerated and consistent.
