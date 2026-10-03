---
id: 500
slug: 'committed-ignore-invalid-hint-for-an-absent-gitignore-names'
title: 'committed-ignore-invalid hint for an absent .gitignore names a no-op migrate'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-03'
depends_on: []
stacked_on:
related: []
discovered_from: []
adrs: []
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
<!-- docket:artifacts:end -->

## Why

`docket repository check` reports `committed-ignore-invalid` when the committed integration tree has no `.gitignore` file at all (the `IgnoreDefectFileAbsent` case in `committedIgnoreFinding`, internal/reposetup/health.go). Its remedy still says "Restore the managed block (e.g. re-run `docket repository migrate`, or add it by hand from the canonical block)".

Since change 0496, `migrate` only migrates: on an already-migrated repository it is a no-op under every flag and points to `docket repository repair`. But `repository repair` does not restore the managed `.gitignore` block either. A human following the hint on a migrated repo runs a command that does nothing, then gets sent to a second command that also does nothing. The hint names a remedy that cannot work.

Surfaced by the 0496 build, which retargeted every other repair remedy to `repository repair` but left this one alone.

## What changes

Give the absent-`.gitignore` finding a remedy that actually works on a migrated repository: either reword it to the hand-restore path (the canonical block, then review/commit/push, matching the sibling `committed-ignore-invalid` remedies), or name a command that really writes the block. Keep the 0496 guard that stops repair remedies naming `migrate` consistent with whatever is chosen.

## Out of scope

The other `migrate` remedies in health.go (`local-metadata-missing`, `docket-worktree-missing`, the legacy and half-migrated findings). They remain valid: on a migrated repo with an incomplete local attachment, `migrate` still runs its resume-local phase. Teaching `repository repair` to write `.gitignore` is out of scope unless grooming finds it is the YAGNI-cheapest working remedy.
