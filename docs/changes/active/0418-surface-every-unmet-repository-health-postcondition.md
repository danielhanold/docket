---
id: 418
slug: 'surface-every-unmet-repository-health-postcondition'
title: 'Surface every unmet repository health postcondition'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-10'
updated: '2026-09-20'
depends_on: []
stacked_on:
related: [352, 378, 383, 403]
discovered_from: []
adrs: [20, 25, 99]
spec: 'docs/superpowers/specs/2026-09-20-surface-every-unmet-repository-health-postcondition-design.md'
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
| Spec | [2026-09-20-surface-every-unmet-repository-health-postcondition-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-20-surface-every-unmet-repository-health-postcondition-design.md) |
| ADRs | [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0025](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0025-docket-worktrees-disable-git-hooks.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository check` can report only `postconditions-unmet` with no explanation of which health condition failed. The reported missing `.opencode/agents/docket-*.md` entry is one example: the committed-ignore probe currently discards the detail needed to identify it.

The classifier also stops at earlier specific diagnoses. A dirty metadata worktree can hide a missing committed ignore entry, and pending setup edits can hide enabled metadata-worktree hooks. Users have to fix one issue and rerun the command to discover another even though the check already gathered the relevant facts.

Tracing the existing implementation and changes 0352, 0378, 0383, and 0403 confirms that the current classifier, canonical ignore block, and findings pipeline provide the foundation. The fix needs fuller reporting over those observations, without changing repository-health policy.

## What changes

- Report every applicable unmet health condition, including independent failures alongside existing specific diagnoses and the causes behind `postconditions-unmet`. Preserve classification precedence, reason tokens, exit codes, and read-only behavior.
- Reuse the existing health-condition evaluation and findings pipeline. Avoid duplicate explanations and misleading cascades from missing prerequisites; unknown evidence remains distinguishable from proven absence.
- Explain committed managed `.gitignore` defects, including all missing canonical entries, malformed markers, and other failures of the existing exact-block check. Derive expected entries from the canonical definition and use the pinned integration commit as authority.
- Carry the same condition, affected path or setting, and state-appropriate remedy through human and JSON findings.
- Cover both mixed-failure examples, every existing health conjunct, simultaneous failures, applicability, deduplication, unknown probes, committed-file authority, and unchanged healthy behavior in the established test suites.

The human selected the broader reporting scope on 2026-09-20. This is a non-trivial diagnostic change using established machinery; the linked spec defines the design and acceptance criteria.

## Out of scope

Automatic repairs; new health requirements, configuration, commands, or probe infrastructure; stricter ignore validation; new surface-drift detection; redesigning initialization, migration, or ownership verification; changing operation-specific authorization; and unrelated diagnostic cleanup. No new ADR or dependency is required.
