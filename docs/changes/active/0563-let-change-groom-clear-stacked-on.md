---
id: 563
slug: 'let-change-groom-clear-stacked-on'
title: 'change.groom: clear stacked_on, accept relationship-only revises, insert missing fields safely'
status: 'proposed'
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
| Spec | [2026-10-17-let-change-groom-clear-stacked-on-design.md](../../superpowers/specs/2026-10-17-let-change-groom-clear-stacked-on-design.md) |
<!-- docket:artifacts:end -->

## Why

 has three gaps in how it edits a proposed change's frontmatter:

- It can set  but never clear it, so un-stacking a proposed change (typically one whose stack parent was killed) needs a hand edit of the record.
- A revise that only edits relationship fields (, , , , ) is refused as .
- Records missing frontmatter keys break it: relationship fields fail with an internal error when the key is absent (ten active records lack ), and two absent upserted fields collide in one patch pass (). This last gap was #564, consolidated here.

Found during change 559.

## What changes

-  clears the stack (writes the valueless ); omitted or  still means unchanged; negative values refuse.  plus  is accepted.
- New refusal : changing  (re-parent or clear) on a change that already has a  refuses, mirroring .
- Any relationship field alone is a valid revise.
- Every field groom writes tolerates a missing key, inserted in template shape, one insert per patch pass (extending #559's per-field upsert cycle).
- The stacking refusal remedies, the groom-next skill, and the stacked-changes reference name the typed un-stack path.

## Out of scope

- 's identical gaps (un-stacking is a human decision; reconcile is autonomous).
- Killed-parent descendant handling and its doc/code mismatch (#565).
- How stacks resolve their base; the  package's same-point insert rule.
