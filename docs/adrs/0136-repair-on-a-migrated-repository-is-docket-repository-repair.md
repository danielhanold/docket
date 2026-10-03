---
id: 136
slug: 'repair-on-a-migrated-repository-is-docket-repository-repair'
title: 'Repair on a migrated repository is `docket repository repair`; migrate only migrates'
status: 'Accepted'
date: '2026-10-03'
supersedes: []
reverses: []
relates_to: [99, 104]
change: 496
---

## Context

`repository check` reports mechanically repairable findings on an already-migrated repository (frontmatter roster findings such as a final change still carrying `claimed_at:`, and derived-view drift: the board, `## Artifacts` blocks, the ADR index), but nothing could apply the repair. Change 0377's spec placed derived-view repair on `docket repository migrate`'s already-migrated path behind `--repair-frontmatter`; no ADR recorded that placement. Overloading migrate made a migration command responsible for routine repair of migrated repos, and the repair planner treated an empty `claimed_at:` as a claim stamp where the record validator does not. Spec: docs/superpowers/specs/2026-10-03-drop-final-claimed-at-is-reported-repairable-but-nothing-can-design.md (docket branch). Relates to ADR-0099 (migrate is the only legacy exit) and ADR-0104 (skills name catalog ids).

## Decision

Repair on an already-migrated repository is `docket repository repair` (catalog operation `repository.repair`). It owns every mechanically repairable finding `repository check` reports: the frontmatter roster and the derived views (board, `## Artifacts` blocks, ADR index), applied in one lease-guarded commit on `docket`. `docket repository migrate` only migrates: on a migrated repository it never writes and points at `repository repair`. `--repair-frontmatter` keeps only its legacy-migration meaning. An empty `claimed_at:` is not a claim stamp anywhere: the repair planner aligns with the record validator.

## Consequences

Commands are named for their job: migrate stays the sole legacy exit (ADR-0099) and repair is a dedicated, catalog-addressable operation skills can name (ADR-0104). Every repairable finding has one writer and lands atomically. Skills and docs that pointed at `migrate --repair-frontmatter` for derived-view drift must point at `repository.repair` instead. A migrate invocation on a migrated repo becomes a read-only no-op with a remedy pointer.

## Alternatives considered

Keep repair on migrate's already-migrated path behind `--repair-frontmatter` (change 0377's placement): rejected because it overloads a migration command with routine repair and leaves the flag with two meanings. Repair findings ad hoc per command (separate board/index renderers): rejected because it splits one finding class across writers and loses the single lease-guarded commit.
