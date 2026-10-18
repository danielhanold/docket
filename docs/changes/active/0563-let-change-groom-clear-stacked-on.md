---
id: 563
slug: 'let-change-groom-clear-stacked-on'
title: 'change.groom: clear stacked_on, accept relationship-only revises, insert missing fields safely'
status: 'implemented'
priority: 'low'
type: 'fix'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [564, 565]
discovered_from: [559]
adrs: []
spec: 'docs/superpowers/specs/2026-10-17-let-change-groom-clear-stacked-on-design.md'
plan: 'docs/superpowers/plans/2026-10-17-let-change-groom-clear-stacked-on.md'
results: 'docs/results/2026-10-17-let-change-groom-clear-stacked-on-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/let-change-groom-clear-stacked-on'
pr: 'https://github.com/danielhanold/docket/pull/427'
blocked_by:
reconciled: true
claimed_at: '2026-10-17T21:58:50Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-17-let-change-groom-clear-stacked-on-design.md](../../superpowers/specs/2026-10-17-let-change-groom-clear-stacked-on-design.md) |
| Plan | [2026-10-17-let-change-groom-clear-stacked-on.md](../../superpowers/plans/2026-10-17-let-change-groom-clear-stacked-on.md) |
| Results | [2026-10-17-let-change-groom-clear-stacked-on-results.md](../../results/2026-10-17-let-change-groom-clear-stacked-on-results.md) |
<!-- docket:artifacts:end -->

## Why

`change.groom` has three gaps in how it edits a proposed change's frontmatter:

- It can set `stacked_on` but never clear it, so un-stacking a proposed change (typically one whose stack parent was killed) needs a hand edit of the record.
- A revise that only edits relationship fields (`depends_on`, `related`, `discovered_from`, `adrs`, `stacked_on`) is refused as `empty-revise`.
- Records missing frontmatter keys break it: relationship fields fail with an internal error when the key is absent (ten active records lack `stacked_on:`), and two absent upserted fields collide in one patch pass (`overlapping-edit`). This last gap was #564, consolidated here.

Found during change 559.

## What changes

- `stacked_on: 0` clears the stack (writes the valueless `stacked_on:`); omitted or `null` still means unchanged; negative values refuse. `deliverable: results-artifact` plus `stacked_on: 0` is accepted.
- New refusal `not-restackable`: changing `stacked_on` (re-parent or clear) on a change that already has a `branch:` refuses, mirroring `not-retitleable`.
- Any relationship field alone is a valid revise.
- Every field groom writes tolerates a missing key, inserted in template shape, one insert per patch pass (extending #559's per-field upsert cycle).
- The stacking refusal remedies, the groom-next skill, and the stacked-changes reference name the typed un-stack path.

## Out of scope

- `change.reconcile`'s identical gaps (un-stacking is a human decision; reconcile is autonomous).
- Killed-parent descendant handling and its doc/code mismatch (#565).
- How stacks resolve their base; the `document` package's same-point insert rule.

## Reconcile log

### 2026-10-17

Re-read against current main (a52c2d9c9). change_groom.go still writes relationship fields and trivial with bare SetField, stacked_on is still *int with no clear path, empty-revise still ignores relationship fields, and #559's upsertFieldBytes/reqFieldEdits cycle is in place to extend. #564 is killed (consolidated here); #565 stays separate. Scope unchanged.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: ecff8e6da0ae673bd20f7e3f404c917f2784914d
ran_at:   2026-10-18T00:48:16Z
<!-- docket:build-evidence:end -->
