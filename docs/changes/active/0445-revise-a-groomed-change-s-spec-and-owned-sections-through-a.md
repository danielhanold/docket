---
id: 445
slug: 'revise-a-groomed-change-s-spec-and-owned-sections-through-a'
title: 'Revise a groomed change''s spec and owned sections through a typed operation'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-09-23'
updated: '2026-09-24'
depends_on: []
stacked_on:
related: [382, 444]
discovered_from: [444]
adrs: []
spec: 'docs/superpowers/specs/2026-09-24-revise-a-groomed-change-s-spec-and-owned-sections-through-a-design.md'
plan: 'docs/superpowers/plans/2026-09-24-0445-revise-groomed-change-typed-operation.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/revise-a-groomed-change-s-spec-and-owned-sections-through-a'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-24T20:05:01Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-24-revise-a-groomed-change-s-spec-and-owned-sections-through-a-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-24-revise-a-groomed-change-s-spec-and-owned-sections-through-a-design.md) |
| Plan | [2026-09-24-0445-revise-groomed-change-typed-operation.md](https://github.com/danielhanold/docket/blob/feat/revise-a-groomed-change-s-spec-and-owned-sections-through-a/docs/superpowers/plans/2026-09-24-0445-revise-groomed-change-typed-operation.md) |
<!-- docket:artifacts:end -->

## Why

A common workflow is: groom a change to a spec, review the spec, then adjust it. Docket has no typed operation for the adjust step. `change.groom` refuses any change that already has a spec (its gate requires proposed, needs-design, no spec), and no other catalog operation revises a groomed spec or the change record's owned body sections. The only path today is a hand edit plus plain git commit in the `.docket` worktree, which bypasses the transaction, the exact-version CAS, and the `updated:` stamp that every other metadata write gets. Observed on change 0444 (2026-09-23): after a YAGNI review narrowed its spec, both the spec and the change's What changes / Out of scope had to be edited by hand.

## What changes

Add a `revise` outcome to the existing `change.groom` operation (no new catalog operation, no new CLI verb) gated on the complement of the current groom gate — a `proposed` change that already has a spec or is trivial-verdicted, i.e. already groomed. `revise` reuses the existing `spec_markdown` field (whole spec-body replace, targeting the change's existing spec path — never a new one) and the existing `sections` field (the same owned-proposal-section splice groom/trivial already use); it never writes `spec:` or `trivial:`, so flipping a change between spec'd and trivial stays structurally impossible. Repeatable: a change may be revised any number of times while it stays `proposed`, each call a standard exact-version CAS write. `docket-groom-next`'s explicit-id path gains a carve-out to route an already-groomed change to this flow instead of erroring, replacing its documented hand-edit-plus-plain-git workaround; `docket-new-change` gains a pointer to the same path for its own post-groom adjust case.

## Out of scope

Editing frozen build records (merged plans and results), Accepted ADRs, or terminal (done/killed) changes. Changing the spec path or relinking a different spec file. Revising trivial changes into spec'd ones or vice versa (structurally impossible under this design, not just disallowed by convention). Revising an in-progress change — that stays change.reconcile's existing job (its SpecSections already covers it). Section-level (partial) spec patching — revise's spec edit is whole-body replace only, matching change.groom's existing spec outcome. Any automatic or autonomous spec revision. Review tooling or approval workflow for specs.

## Reconcile log

### 2026-09-24

2026-09-24 — Reconciled against main 4bfc4485. internal/app/change_groom.go still matches the spec's baseline: GroomOutcome has only spec/trivial, the not-groomable gate is unchanged, and FCInvalidOutcome still names two outcomes. skills/docket-groom-next/SKILL.md still carries the hand-edit workaround line this change removes. Related 0444 is done and 0382 (create-request scalars) does not touch groom. No scope change; the skill edits land in the repo's skills/ sources (docket-groom-next, docket-new-change).
