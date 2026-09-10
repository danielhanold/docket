---
id: 418
slug: 'surface-every-unmet-repository-health-postcondition'
title: 'Surface every unmet repository health postcondition'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-10'
updated: '2026-09-10'
depends_on: []
stacked_on:
related: [352, 378]
discovered_from: []
adrs: [20]
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
| ADRs | [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository check` can emit only `[error] postconditions-unmet`, followed by “The metadata branch exists but not every health postcondition is satisfied” and a remedy to resolve unspecified issues. The reported real-world trigger is a missing `.opencode/agents/docket-*.md` entry in the committed managed `.gitignore` block. Users cannot identify or repair the failure from this output, unlike the command’s more specific diagnostics.

## What changes

- Bubble up every underlying failure currently hidden behind `postconditions-unmet`, with a concrete condition, affected path or setting where applicable, and an actionable remedy. Report all simultaneous failures, not just the first.
- For a missing or invalid committed managed `.gitignore` block, identify `.gitignore` and the specific missing entries or structural defect; explicitly cover the missing `.opencode/agents/docket-*.md` entry. Derive missing entries from the canonical block definition.
- Carry the same diagnostic detail through human-readable and JSON findings. Keep unknown or failed probes distinguishable from proven absence and preserve existing classification, exit-status, and read-only behavior.
- Add regression coverage for the reported missing-entry case, each health postcondition that can reach the generic fallback, multiple simultaneous failures, unresolved probes, and an unchanged healthy result. Validate that a generic summary never stands alone without its underlying reasons.

This is a small diagnostic propagation fix using the existing facts, classifier, and findings pipeline (`Classify`, `findingFor`, and `committedIgnorePresence`), with no new workflow or architecture decision required.

## Out of scope

Automatic repairs, changing the required health postconditions, redesigning repository initialization or migration, and unrelated diagnostic cleanup.
