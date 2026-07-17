---
id: 90
slug: discovered-from-provenance
title: discovered-from provenance links — record which change's build surfaced a new stub
status: proposed
priority: medium
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [35, 91]
adrs: []
spec: docs/superpowers/specs/2026-07-17-discovered-from-provenance-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-17-discovered-from-provenance-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-17-discovered-from-provenance-design.md) |
<!-- docket:artifacts:end -->

## Why

Synthesized from the beads (gastownhall/beads) competitive review (2026-07-17). Beads has a
`discovered-from` dependency type — "Found during work on another issue" — which its FAQ calls
"agent-specific": it exists because agents constantly surface follow-up work mid-task, and the
origin of that work is valuable graph data (what fraction of the backlog is planned vs
discovered? which changes spawn the most follow-ups?).

docket already generates discovered work — implement-next's reconcile and review passes, groom
brainstorms, and close-out findings all propose follow-up changes (this repo's own board shows
chains like 0075→0076, 0086/0087 from 0062) — but the provenance lives only in prose. `related:`
is symmetric and unordered; nothing says "this stub exists *because* building #NN surfaced it."
One frontmatter field fixes that.

## What changes

This change delivers the provenance **field** and its documentation/population as a standalone data
layer (see the linked spec for the full design + audit trail).

- A new optional manifest field `discovered_from: [62]` — a **list of change ids** (parallel to
  `related:` / `depends_on:`) recording which change(s)' work surfaced this one; empty/absent for
  deliberately planned work. Purely **informational**, like `related:` — never a readiness gate,
  never introduces blocking. No automatic `related:` back-link on the origin change.
- The convention (docket-convention manifest section) documents it and the change template seeds it
  empty. Frontmatter is parsed field-by-field with no schema, so the addition is backward-compatible.
- Population in the flow that mints change stubs **today**: `docket-new-change` records it when a
  human names the originating change (extending its existing "scan related context" step). No
  autonomous skill mints change stubs yet — that is #0091, which will populate the field once it
  lands; the convention documents the field generically so #0091 slots in without rework.

## Out of scope

- Auto-creating stubs / autonomous mid-run population — that is #0091, a **separate consumer** of
  this field (being groomed concurrently). #0090 lands standalone; whether to fold #0091 in is a
  human decision this change does not foreclose.
- A new render surface (Artifacts-block row, board column, or mermaid provenance edge) — deferred:
  the Artifacts block is document-only (`related:`/`depends_on:` are absent from it by precedent),
  and a board surface is heavier than an informational field warrants. The field is queryable in
  raw frontmatter.
- New blocking/readiness semantics — `discovered_from` is informational, like `related:`.
- Analytics over the provenance graph (#0010's territory).
- Back-filling existing stubs and a dangling-id health check — optional human follow-ups.

## Open questions

Resolved at groom (2026-07-17); rationale in the linked spec's `## Assumptions`:

- **Field shape** → list of ids, parallel to `related:` (multi-origin expressible; parsed for free
  by `list_field()`). **No** automatic `related:` back-link on the origin (avoids cross-file /
  archived-file mutation and preserves directionality).
- **Merge with #0091** → land #0090 standalone (the field is coherent alone; #0091 is a separate
  consumer being groomed concurrently). The fold-in remains a human decision, not foreclosed here.

## Reconcile log
