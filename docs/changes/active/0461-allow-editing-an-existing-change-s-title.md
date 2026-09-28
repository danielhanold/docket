---
id: 461
slug: 'allow-editing-an-existing-change-s-title'
title: 'Allow editing an existing change''s title'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-09-27'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [366, 447]
discovered_from: [366]
adrs: [71]
spec: 'docs/superpowers/specs/2026-09-28-allow-editing-an-existing-change-s-title-design.md'
plan: 'docs/superpowers/plans/2026-09-28-0461-allow-editing-an-existing-change-s-title.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/allow-editing-an-existing-change-s-title'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-28T20:53:10Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-28-allow-editing-an-existing-change-s-title-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-28-allow-editing-an-existing-change-s-title-design.md) |
| Plan | [2026-09-28-0461-allow-editing-an-existing-change-s-title.md](https://github.com/danielhanold/docket/blob/feat/allow-editing-an-existing-change-s-title/docs/superpowers/plans/2026-09-28-0461-allow-editing-an-existing-change-s-title.md) |
| ADRs | [ADR-0071](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0071-writer-guarantees-yaml-validity-by-construction.md) |
<!-- docket:artifacts:end -->

## Why

There is no typed operation that can change the `title:` of an existing change. `change.groom` (including `outcome: revise`) edits only the owned body sections, the spec, and the relationship fields. `change.create` sets the title once, and no other operation touches it after that. When a change's scope is renamed, the title goes stale. Example: 0366's spec renamed the release from `v1.0.0-rc1` to `v1.0.0-beta1`, but the record is still titled "…v1.0.0-rc1…". The only workaround is a hand-edit of the frontmatter plus a human-typed `docket repository migrate` to fix the board. That route bypasses the writer's quoting guarantee (ADR-0071) and leaves `BOARD.md` stale until the migrate runs.

### Title and path are decoupled after creation (checked 2026-09-27)

The title determines the path only once. `slugifyTitle` runs in `change.create` (and the ADR
create/supersede operations) and nowhere else. After that the slug is stored as its own
`slug:` field and is never re-derived from the title. A grep found no validation that compares
slug to title, and the frontmatter repair path reads `slug` and `title` as independent strings.
So a retitle that leaves the slug alone should not need to touch the path. This was a grep,
not a trace of every reader, so the groom should confirm that no reader, check, or renderer
assumes `slug == slugify(title)`.

Renaming the slug is the hard part, and it is excluded on purpose. The slug is used in the
record filename (`active/` and later the dated `archive/` name), `branch:` (`<type>/<slug>`),
the `.worktrees/<slug>` directory, the spec/plan/results filenames, cross-record and backlink
URLs, the board, and any open PR's head branch. The accepted cost is a slug that no longer
matches the title. 0366 already has this (slug `…-v1-0-0-rc1-…`, spec renamed to beta1), and
its spec accepts it deliberately.

## What changes

- Extend `change.groom` with an optional `title` field, accepted on the `spec`, `trivial`, `revise`, and `rearm` outcomes and refused on `abstain`. A title alone is a valid revise. The existing gates keep every retitle on `proposed` changes.
- In one metadata transaction, a title edit rewrites `title:` and `updated:` through the writer (ADR-0071 quoting), re-renders the change's `## Artifacts` block and the board, and re-stamps the linked spec's `docket:backlink` block over the spec's current bytes.
- The slug, filename, spec path, and any `branch:` never change. A title edit renames nothing.
- One shared title validator (non-empty, single line, no control characters) runs in both `change.create` and `change.groom`. Board title cells escape `|`, which closes an existing table-corruption gap for titles from either entry point.
- `docket-groom-next` gets a one-line pointer telling it to pass `title` when a groom renames the change.

## Out of scope

- Retitling non-`proposed` changes. That would need the feature-branch plan/results backlinks and the PR title updated.
- Changing a slug or renaming a change file, spec, or branch.
- Retitling terminal (archived) records.
- Editing other scalars such as priority or type.

## Reconcile log

### 2026-09-28

2026-09-28 — Reconciled against origin/main ef4a341d2. change_groom.go still refuses a revise lacking spec_markdown/section edits (FCEmptyRevise); no title field exists; board.go still writes titles unescaped with boardRepairCell as the only cell replacer. No intervening change touched retitling. Scope and spec unchanged.
