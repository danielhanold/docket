---
id: 469
slug: 'replace-opaque-docket-terms-with-clearer-names'
title: 'Replace opaque docket terms with clearer names'
status: 'done'
priority: 'medium'
type: 'refactor'
created: '2026-09-28'
updated: '2026-10-01'
depends_on: []
stacked_on:
related: [402, 468, 471, 474]
discovered_from: []
adrs: [129]
spec: 'docs/superpowers/specs/2026-09-30-replace-opaque-docket-terms-with-clearer-names-design.md'
plan: 'docs/superpowers/plans/2026-09-30-replace-opaque-docket-terms-with-clearer-names.md'
results: 'docs/results/2026-09-30-replace-opaque-docket-terms-with-clearer-names-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/replace-opaque-docket-terms-with-clearer-names'
pr: 'https://github.com/danielhanold/docket/pull/358'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-30-replace-opaque-docket-terms-with-clearer-names-design.md](../../superpowers/specs/2026-09-30-replace-opaque-docket-terms-with-clearer-names-design.md) |
| Plan | [2026-09-30-replace-opaque-docket-terms-with-clearer-names.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-09-30-replace-opaque-docket-terms-with-clearer-names.md) |
| Results | [2026-09-30-replace-opaque-docket-terms-with-clearer-names-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-09-30-replace-opaque-docket-terms-with-clearer-names-results.md) |
| ADRs | [ADR-0129](../../adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

A review of `docs/reference/glossary.md` (2026-09-28) found docket terms that don't collide with anything but are hard for a newcomer, human or agent, to understand without reading the entry: "admission slot", "unmet conjuncts", "presence-encoded section", "Step-0 preamble", `identity-mismatch`, `unresolved-execution`, `change repair-identity`, and others. Each has a plainer name that says what the thing is or does. Renaming them lowers the reading cost of the docs, run logs and verdict lines. "needs-brainstorm" also conflicts with the project's own vocabulary, which uses "groom" for this step.

Two names no longer match any code at all: the Bash-era bootstrap verdicts (`STOP_MIGRATE`, `CREATE_ORPHAN`) and `docket status --digest-only`, which a guide still tells readers to run.

## What changes

Deliver ADR-0129's family (e), rows 67-86, which this change's grooming added to that ADR. It is a hard cut with no aliases (ADR-0129 Decision 2):

- **Wire renames (rows 67-73):** readiness `needs-brainstorm` → `needs-grooming`; gate-drive halt causes `identity-mismatch` → `worktree-changed` and `unresolved-execution` → `launch-unconfirmed`; `change repair-identity` → `change relink` (result tokens `relinked-branch` / `relinked-pr`); finalize's `pr-identity-mismatch` → `pr-link-mismatch`; recertify's `identity-drift` → `certified-input-changed`. Each retired spelling is sealed through the `internal/repoguard` retired-vocabulary table.
- **Prose renames (rows 74-84):** unmet conditions, worktree slot, moved to background, gate supervisor, gate run, marker section, startup check, allowed values, conflict-checked write, read on demand, relink / link check, across skills, agent wrappers, guide, concept and reference pages, comments, and the generated dispatch material.
- **Retirements (rows 85-86):** rewrite the bootstrap guard around `repository.prepare`'s dispositions, fix the nonexistent `--digest-only` command, and move both names to the glossary's "Obsolete terms" section.

Consumer repos re-run `docket install`. The change lands with no gate drive in flight.

## Out of scope

- The stub's other rows (abstain, dummy mode, metadata branch, reconcile, inert, disposition, owner generation, continuation id, sync integration, coordination key, scope tag): dropped for the reasons ADR-0129 family (e) records.
- Config keys, agent names and frontmatter fields (ADR-0129 Decision 9).
- Alias or deprecation-window machinery (ADR-0129 Decision 2).
- Rewriting frozen build records, archived changes, specs, or Accepted ADRs other than ADR-0129's table.
- Migrating persisted gate-drive records that carry a renamed halt cause.

## Reconcile log

### 2026-09-30

Reconciled against origin/main 366827eb5 and origin/docket 686ad1e. Every wire token in ADR-0129 rows 67-73 is still present in maintained source (needs-brainstorm, identity-mismatch, unresolved-execution, repair-identity, pr-identity-mismatch, identity-drift, MergeConjuncts), and the retired Bash-era names of rows 85-86 still appear in prose. Related changes 0402, 0468, 0471, 0474 are merged; nothing else absorbed this work. Scope, spec and relations unchanged.

