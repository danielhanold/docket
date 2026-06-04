---
id: 2
slug: docket-metadata-branch
title: docket metadata branch — separate planning state from code history
status: in-progress
priority: high
created: 2026-06-03
updated: 2026-06-03
depends_on: []
related: [1]
adrs: [1, 2]
spec: docs/superpowers/specs/2026-06-02-docket-metadata-branch-design.md
plan: docs/superpowers/plans/2026-06-03-docket-metadata-branch.md
results:
trivial: false
branch: feat/docket-metadata-branch
pr:
blocked_by:
reconciled: true
---

## Why

docket needs a durable, queryable source of truth for planning state — changes, statuses, ADRs, dependencies, board — shared across agents, machines, and time, with **git as the only persistence mechanism** (no database, no service). The shipped v1 keeps that state on the integration branch (`main`), which guarantees freshness but **pollutes code history**: a continuous stream of planning-only commits (`claim`, `reconcile`, `refresh board`, `archive`) interleaves with production code, mixing project-management churn into the branch people read to understand the software.

A separate `docket` branch is already *named* — `metadata_branch: docket` is a config knob in every skill's convention block and the README documents it — but it ships as a deliberate **v1 rough edge**: `docket-implement-next` tells implementers not to use it ("silent failures"), and the README's *How metadata is stored* enumerates three unsolved cross-branch problems (specs read cross-tree, reconcile pushes land on `docket` while feature branches cut from the integration branch, and no mirror/merge from `docket` to the integration branch). This change closes that rough edge and makes the separate metadata branch the **supported, tested default** — clean code history, a complete git-native planning audit trail, and the "why" still co-located with the code on the integration branch.

## What changes

Make `docket`-mode real and the default (full design + git mechanics in the linked spec):

- An **orphan `docket` branch** becomes the authoritative working surface for all planning metadata (change files, `BOARD.md`, ADRs, specs); the integration branch stays code-only **except for published terminal records**.
- A **persistent `.docket/` worktree** is the read/write surface, so the main working tree never switches branches (the same worktree concept docket already uses for feature branches).
- `metadata_branch` **default flips to `docket`**; a new **`integration_branch`** knob (`auto` | `main` | `develop`) supports trunk and GitFlow; feature branches always cut from `origin/<integration_branch>`.
- On a **terminal transition** (`done` or `killed`), the driving skill publishes the change's terminal records (archived change + its spec + `Accepted` ADRs) onto the integration branch via a shared procedure — co-locating the rationale with the code. The live board never leaves `docket`.
- A **four-state bootstrap guard** refuses to run on an un-migrated repo and points to a one-shot **`migrate-to-docket.sh`**; **`main`-mode remains a clean pinned opt-out** that reproduces today's single-branch behavior exactly.
- Touch-points: the synced convention block, **all five skills**, the README (a full docket-mode section + artifact-location table), `.gitignore`, the migration script, and content/sync **test assertions**.

## Out of scope

- Migrating *this* repo (docket itself) to `docket`-mode — build + document the capability; dogfooding the migration is a **separate follow-up**.
- Migrating other repos' historical metadata beyond what the one-shot script does.
- Any CLI runtime dependency or living-spec layer.
- Rewriting existing `main` history (the planning commits already on `main` stay; only the go-forward surface changes).

## Open questions

None at PM altitude. The design is settled and reviewed; remaining items are git-command-level precision (idempotency guards, conflict handling, exact refspecs) that the build's TDD pins down — see the spec's §12 test assertions.

## Reconcile log

**2026-06-03:** Reconciled at claim time. Spec + change were authored 2026-06-02/03 and `origin/main` has advanced only by the propose/claim commits and a user-added `.gitignore` (`b2a75ae`, contents: `.DS_Store`) — so this is a currency check, not a rewrite. Verified the spec's §12 touch-points against live reality:
- **`.gitignore` now exists** (one fold-in): the change *extends* it with `.docket/` + `.worktrees/` rather than creating it; spec §12 updated.
- **Skills + synced convention block are unchanged** from the v1 state the spec targets (no `skills/` commits since the 0001 results work) — the rewrite targets are all still present (`metadata_branch: main` default, the `docket` v1 caveat in `docket-implement-next`, hard-coded `origin/main`).
- **No ADRs exist yet** (`docs/adrs/` empty) — nothing to cite; the 1–2 ADRs are produced at build.
- **`0001` is `done`** as `related` assumed; its results-artifact work (the `results:` field/`results_dir`/templates) is already in the convention and accounted for in the §3 artifact table — no conflict.

Scope otherwise unchanged; nothing shipped elsewhere to drop. **Build approach — TDD-for-docs** (the model change 0001 used): encode the spec's §12 assertions as content/sync checks in `tests/` first (convention blocks in sync after edits; `metadata_branch: docket` + `integration_branch` defaults present across all five skills; the v1 `docket` caveat removed; terminal-publish copy-set = {change, spec, `Accepted` adrs} + kill-publish wired in producer *and* implementer, not just finalize; `.gitignore` ignores `.docket/`+`.worktrees/`; `main`-mode backward-compat references no `docket`/`.docket/`/`checkout origin/docket`), then edit the canonical convention block, run `sync-convention.sh`, apply per-skill specifics (§7), add `migrate-to-docket.sh`, extend `.gitignore`, rewrite the README docket-mode section (§10), and record the branch-model + docket-as-default ADRs (§11).
