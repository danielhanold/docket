---
id: 382
slug: 'changecreaterequest-typed-auto-groomable-branch-prefix-scalars'
title: 'ChangeCreateRequest should accept typed auto_groomable / branch_prefix scalars'
status: 'implemented'
priority: medium
type: feat
created: '2026-08-31'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [377]
discovered_from: [377]
adrs: []
spec: 'docs/superpowers/specs/2026-09-27-changecreaterequest-typed-auto-groomable-branch-prefix-scalars-design.md'
plan: 'docs/superpowers/plans/2026-09-27-0382-typed-auto-groomable-branch-prefix-and-abstain-rearm.md'
results: 'docs/results/2026-09-27-changecreaterequest-typed-auto-groomable-branch-prefix-scalars-results.md'
trivial: false
auto_groomable:
branch: 'feat/changecreaterequest-typed-auto-groomable-branch-prefix-scalars'
pr: 'https://github.com/danielhanold/docket/pull/342'
blocked_by:
reconciled: true
claimed_at: '2026-09-27T17:49:58Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-27-changecreaterequest-typed-auto-groomable-branch-prefix-scalars-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-27-changecreaterequest-typed-auto-groomable-branch-prefix-scalars-design.md) |
| Plan | [2026-09-27-0382-typed-auto-groomable-branch-prefix-and-abstain-rearm.md](https://github.com/danielhanold/docket/blob/feat/changecreaterequest-typed-auto-groomable-branch-prefix-scalars/docs/superpowers/plans/2026-09-27-0382-typed-auto-groomable-branch-prefix-and-abstain-rearm.md) |
| Results | [2026-09-27-changecreaterequest-typed-auto-groomable-branch-prefix-scalars-results.md](https://github.com/danielhanold/docket/blob/feat/changecreaterequest-typed-auto-groomable-branch-prefix-scalars/docs/results/2026-09-27-changecreaterequest-typed-auto-groomable-branch-prefix-scalars-results.md) |
<!-- docket:artifacts:end -->

## Why

The native `docket change create` operation (`ChangeCreateRequest`) has no typed field for a
change's `auto_groomable` tri-state or its `branch_prefix` scalar. Today `docket-new-change` sets
those two frontmatter fields by editing the change file with plain git after the record is minted,
rather than by passing them through the create op — a bridge that works but leaves the two fields
outside the typed, validated create path (and outside whatever quoting/validation the op applies to
every other frontmatter scalar). Surfaced during change [[0377]]'s Bash-facade → Go cutover, which
routed change creation through the native op.

## What changes

Every write of a change's `auto_groomable` and `branch_prefix` scalars moves onto a typed, validated, board-refreshing operation, and the plain-git frontmatter edits in the skills are retired:

- **Create.** `change.create` accepts optional `auto_groomable` (tri-state) and `branch_prefix`. `docket-new-change` passes them in the create request instead of hand-editing the record afterwards.
- **Branch-prefix normalization moves into Go.** A new `domain.NormalizeBranchPrefix` trims whitespace, strips one trailing slash, and lowercases the value (branch prefixes are lowercase-only), then applies the existing `ValidBranchComponent` rules. A bad prefix is refused at create time (`invalid-branch_prefix`) instead of failing later at claim inside an autonomous run. The skill no longer carries its own normalization rules.
- **Auto-groom abstain and re-arm become typed.** `change.groom` gains `outcome: abstain` (sets `auto_groomable: false` and appends a dated `## Auto-groom blocked` entry) and `outcome: rearm` (sets `auto_groomable: true`, removes the section, and optionally applies owned-section edits). Both run under the pinned-version CAS and re-render the board in the same commit. This fixes today's stale-board defect: a plain-git abstain or re-arm flips the board's "auto-groom blocked — needs you" cell without re-rendering `BOARD.md`.
- **Skills and docs updated.** `docket-auto-groom`, the convention's *Autonomous grooming* section, and `docket-groom-next` are updated to match, including removing the false claim that an abstain "changes no board-visible cell".

Detailed design: the linked spec.

## Out of scope

- Any change to the *meaning* of `auto_groomable` or `branch_prefix` (tri-state inheritance, how claim consumes the prefix).
- Moving auto-groom selection or eligibility into Go.
- Rewriting `refs/heads/<x>` to `<x>`.
- Normalizing (including lowercasing) prefixes on existing records, or loosening or case-folding claim-time validation.
- Other frontmatter fields `change.create` does not accept today.

## Reconcile log

### 2026-09-27

2026-09-27 — Reconciled against origin/main e244dfd26. Spec was authored today; traced current code: ChangeCreateRequest still lacks auto_groomable/branch_prefix, no NormalizeBranchPrefix exists, change.groom outcomes are still spec|trivial|revise (0445 recently landed revise refinements, including dropping spec_path — compatible with this design). No scope change.

