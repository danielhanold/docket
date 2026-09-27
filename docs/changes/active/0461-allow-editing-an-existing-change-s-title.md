---
id: 461
slug: 'allow-editing-an-existing-change-s-title'
title: 'Allow editing an existing change''s title'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-09-27'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [366, 447]
discovered_from: [366]
adrs: [71]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0071](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0071-writer-guarantees-yaml-validity-by-construction.md) |
<!-- docket:artifacts:end -->

## Why

There is no typed operation that can change the `title:` of an existing change. `change.groom` (including `outcome: revise`) edits only the owned body sections, the spec, and the relationship fields. `change.create` sets the title once, and no other operation touches it after that. When a change's scope is renamed, the title goes stale. Example: 0366's spec renamed the release from `v1.0.0-rc1` to `v1.0.0-beta1`, but the record is still titled "…v1.0.0-rc1…". The only workaround is a hand-edit of the frontmatter plus a human-typed `docket repository migrate` to fix the board. That route bypasses the writer's quoting guarantee (ADR-0071) and leaves `BOARD.md` stale until the migrate runs.

## What changes

- Add an optional title field to an existing edit path. Extending `change.groom` revise is the likely candidate, but the groom should confirm this. A `title` change would then rewrite `title:` and `updated:` through the writer, re-render the change's `## Artifacts` block and the board, and re-stamp the reciprocal `docket:backlink` blocks, which quote the title, in one metadata transaction.
- The slug, filename, and any minted `branch:` stay unchanged. The slug is an identifier and a title edit never renames anything.
- Validate the title the same way `change.create` does, and keep the existing version-pin/contended semantics.

## Out of scope

- Changing a slug or renaming a change file or branch.
- Retitling terminal (archived) records.
- Editing other scalars such as priority or type. Those can be follow-ups if wanted.
