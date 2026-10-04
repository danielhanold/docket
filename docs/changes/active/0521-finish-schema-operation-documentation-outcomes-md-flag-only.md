---
id: 521
slug: 'finish-schema-operation-documentation-outcomes-md-flag-only'
title: 'Mark every nested required request field in the schema, and fix the stale schema docs'
status: 'in-progress'
priority: 'low'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: [520]
stacked_on:
related: [360, 520]
discovered_from: [520]
adrs: [138]
spec: 'docs/superpowers/specs/2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only-design.md'
plan: 'docs/superpowers/plans/2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only.md'
results: 'docs/results/2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/finish-schema-operation-documentation-outcomes-md-flag-only'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-04T16:51:15Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only-design.md) |
| Plan | [2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only.md](https://github.com/danielhanold/docket/blob/fix/finish-schema-operation-documentation-outcomes-md-flag-only/docs/superpowers/plans/2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only.md) |
| Results | [2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only-results.md](https://github.com/danielhanold/docket/blob/fix/finish-schema-operation-documentation-outcomes-md-flag-only/docs/results/2026-10-04-finish-schema-operation-documentation-outcomes-md-flag-only-results.md) |
| ADRs | [ADR-0138](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0138-a-published-request-schema-is-exactly-the-json-file-an-opera.md) |
<!-- docket:artifacts:end -->

## Why

Change 0520 made `docket schema --operation` publish exactly the JSON file each operation reads. Flag-only operations no longer publish a request. It left two loose ends.

First, two docs still say every operation shows a request shape: `docs/reference/outcomes.md` and the docket-convention `close-out.md` reference. That is now false for about 23 flag-only operations.

Second, no nested request field is marked required, even though the validator refuses several of them when empty. Examples: the ADR `target` (id, path, revision) on supersede/reverse, the producing `change` pins on ADR requests, `sections[].heading`/`intent` on groom and learning updates, and `children[]` pins on retarget. An agent reading the schema cannot tell these are mandatory, so it learns only from a refusal.

## What changes

- One rule: a request field at any depth is marked required exactly when the validator always refuses it if empty. A required field inside an optional object is required only when that object is sent.
- Add the missing `docket:"required"` tags on the nested ADR target and producing-change pins, the ADR `target`/`successor` objects, section `heading`/`intent`, and retarget child pins. No validator behavior changes.
- Extend the required-tag test so it checks nested fields. It starts from a known-valid request and blanks each nested field in turn: a tagged field must be refused and an untagged one accepted. The test is mutation-checked.
- Fix the stale wording in `outcomes.md` and `close-out.md` (and its embedded copy).

## Out of scope

Changing validator behavior or finding codes. New schema vocabulary (conditional-required). Bumping `schema_version`. Fixing `successor.request_id`, which is published as required but ignored by supersede/reverse. That mismatch is harmless and is recorded, not fixed.

## Reconcile log

### 2026-10-04

Reconciled against main at 62d67286d. Dependency #520 is done and merged: `declareJSONFile`, the registry bindings, and `TestRequiredTagMatchesValidator` exist on main. The nested request types (`ADRTarget`, `ADRProducingChange`, `ADRReplaceRequest`, `SectionEditRequest`, `AuthorizedChild`) still carry no nested `docket:"required"` tags, and `outcomes.md` plus both copies of `close-out.md` still carry the stale wording. Scope unchanged.
