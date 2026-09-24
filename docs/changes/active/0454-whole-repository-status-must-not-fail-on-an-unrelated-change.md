---
id: 454
slug: 'whole-repository-status-must-not-fail-on-an-unrelated-change'
title: 'Whole-repository status must not fail on an unrelated change''s invalid branch name'
status: 'implemented'
priority: 'critical'
type: 'fix'
created: '2026-09-24'
updated: '2026-09-24'
depends_on: []
stacked_on:
related: [449]
discovered_from: [449]
adrs: [127]
spec: 'docs/superpowers/specs/2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change-design.md'
plan: 'docs/superpowers/plans/2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change.md'
results: 'docs/results/2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/whole-repository-status-must-not-fail-on-an-unrelated-change'
pr: 'https://github.com/danielhanold/docket/pull/331'
blocked_by:
reconciled: true
claimed_at: '2026-09-24T20:32:08Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change-design.md) |
| Plan | [2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change.md](https://github.com/danielhanold/docket/blob/fix/whole-repository-status-must-not-fail-on-an-unrelated-change/docs/superpowers/plans/2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change.md) |
| Results | [2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change-results.md](https://github.com/danielhanold/docket/blob/fix/whole-repository-status-must-not-fail-on-an-unrelated-change/docs/results/2026-09-24-whole-repository-status-must-not-fail-on-an-unrelated-change-results.md) |
| ADRs | [ADR-0127](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0127-scoped-metadata-validation-for-named-operations.md) |
<!-- docket:artifacts:end -->

## Why

Change 0449 stopped one broken change record from blocking named operations on other changes. It left `docket status`, the whole-repository read, strict. `status` probes live branch facts for every change, so when any unrelated change records an invalid branch name the whole read fails with an external error. The one read a human or agent uses to see and repair a broken record then fails because of that same record. This is the last known way a single unrelated record can take down a shared read, and it is confirmed in 0449's tests (see its results file, "Known issues and follow-ups").

## What changes

Make the three whole-repository reads that share one live branch probe (`docket status`, `maintenance.preflight`, and automatic `context.implementation` selection) survive a change whose recorded `branch:` is not a valid git ref name.

- Complete gitcli's ref-name check to git's own `check-ref-format` rules and export it, so the probe and its caller use the same rule.
- The whole-corpus probe (`stackBranches`) skips any name that fails the check. Such a name cannot exist on the remote, so it counts as absent. A child stacked on that parent is then not build-ready through the existing `stack-base-unresolved` path; no new selection logic.
- `docket status` reports an error-severity `branch-malformed` finding on every displayed active change with a malformed branch, alongside its existing artifact health findings, and still renders the rest of the backlog. The finding reaches the preflight envelope with no format change.
- 0449's integration test goes through the real `Status` read instead of routing around it.

This applies ADR-0127's existing report-per-record model to the read it left out. No new ADR.

## Out of scope

- Named-operation validation scoping, done by 0449. The named probe (`stackBranchesFor`) is unchanged.
- Repairing, rewriting, or auto-clearing the malformed `branch:` value.
- Making a malformed branch a snapshot-validation error, or refusing writes because of it.
- Merging the narrower branch-shape checks used elsewhere into the gitcli one.
- `repository check`, which does not probe branch facts.

## Reconcile log

### 2026-09-24

2026-09-24 — Reconciled against main 4bfc4485 (0449 merged). Anchors verified present: stackBranches/stackBranchesFor and artifactChecks (internal/app/status.go), validateRefName (internal/gitcli/types.go), recordedBranch (internal/app/branch_identity.go), parsePRRef (internal/app/finalize_context.go). No FCBranchMalformed constant exists yet. Scope unchanged.
