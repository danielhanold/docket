---
id: 472
slug: 'rename-change-version-to-revision-version-revision'
title: 'Rename change version to revision (--version → --revision)'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: [468]
stacked_on:
related: [468, 471, 474, 477]
discovered_from: []
adrs: [129]
spec: 'docs/superpowers/specs/2026-09-29-rename-change-version-to-revision-version-revision-design.md'
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
| Spec | [2026-09-29-rename-change-version-to-revision-version-revision-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-29-rename-change-version-to-revision-version-revision-design.md) |
| ADRs | [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0468 settled a collision-free docket vocabulary (ADR-0129). Every mutating docket operation pins the exact record it read, so a concurrent edit is refused instead of overwritten. That pin is spelled `--version <blob>` / `version`, which reads like a software version and clashes with the `docket version` command. This change applies ADR-0129 family (b), rows 39–45. Grooming found gaps in those rows, so the ADR was amended in place (rows 40a, 41a, 41b, 43a and 44a added, rows 40, 41, 43 and 44 corrected).

## What changes

Apply ADR-0129 family (b), as amended at grooming, as one hard cut with no aliases:

- **Flags:** `--version` becomes `--revision` on the 14 operations that take it, and `change repair-identity --expect-version` becomes `--expect-revision`.
- **Keys:** request and read `version` becomes `revision`. This includes `status --records`, and `context.finalize`'s PR facts, where `pr.version` becomes `pr.revision`. `spec_version` becomes `spec_revision`, the ADR requests' `target.version` / `change.version` become `.revision`, and `pr_version` becomes `pr_revision`.
- **Codes:** the ten refusal codes that say "version" (`version-mismatch`, `empty-spec_version`, …) say "revision".
- **One meaning for "revision":** the glossary's "Change version" and "Entity version" entries merge into one Revision entry. It covers record revisions (blob ids), PR revisions (snapshot hashes), and the existing commit-id `*_revision` keys, which stay as they are.
- **Everything that names these terms moves with them:** Go identifiers, tests, golden `capabilities`/`schema` output, skills, embedded copies and docs.
- **Seal:** add the retired spellings to the retired-vocabulary table. The bare `version` key is checked by walking the schema registry, and the bare `--version` flag only where it is bound to a change, finalize or workspace command. The seal is mutation-tested.
- **Kept:** software and format versions, `docket version`, the claim digest's `version` key, `resolver_budget_version`, and the release tools' `--version`.

**Landing (human procedure):** merge with no dispatched run in flight, rebuild the binary immediately, restart coordinator sessions, and re-run `docket install` in consumer repos. No storage reset is needed.

## Out of scope

- Aliases, a deprecation window or dual-spelling support: old spellings are hard-cut (ADR-0129 Decision 2).
- Renaming the commit-id `*_revision` keys, config keys, agent names or frontmatter fields.
- Editing point-in-time records: archived changes, results, specs and plans. ADR-0129 was amended at grooming, and the build edits it only to record a newly found name, with the human's authorization.
- Rows owned by changes 0473, 0474 and 0477, or by change 0469.
