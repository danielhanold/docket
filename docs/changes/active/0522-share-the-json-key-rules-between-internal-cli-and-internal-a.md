---
id: 522
slug: 'share-the-json-key-rules-between-internal-cli-and-internal-a'
title: 'Share the JSON-key rules between internal/cli and internal/app'
status: 'in-progress'
priority: 'low'
type: 'refactor'
created: '2026-10-04'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [520]
discovered_from: [521]
adrs: []
spec:
plan:
results:
trivial: true
auto_groomable:
branch_prefix:
branch: 'refactor/share-the-json-key-rules-between-internal-cli-and-internal-a'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-10-05T09:47:42Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 521 merged the app-side JSON-key rules into one helper, `jsonFieldKey` in `internal/app/schema_tags.go`. But `requestJSONKeys` in `internal/cli/requestkeys.go`, which builds the "accepted keys: …" list in an unknown-key refusal, still keeps its own copy of the same walk, and two app-side comments point readers at that copy.

This is a tidy-up, not a bug fix. The copies agree today. `TestPublishedRequestIsTheDecodedJSONFile` (`internal/cli/jsonfile_production_test.go`) already checks, for every JSON-reading operation, that the CLI copy's keys equal the keys `docket schema` publishes, so any drift a real request type hits turns that test red. Grooming found no design question, so this is trivial: one copy of the walk instead of two.

## What changes

- In `internal/app/schema_tags.go`, export one function returning the sorted top-level JSON keys a request struct accepts (keep the name `RequestJSONKeys`). It walks the struct with `jsonFieldKey` and pulls in the fields of embedded structs. `requiredJSONKeys` becomes the same walk filtered to `docket:"required"` fields, so there is a single walk.
- `internal/cli` calls `app.RequestJSONKeys`. Delete `internal/cli/requestkeys.go`, and move its two unit tests (the reconcile-request key set, and skip/promote behavior) to `internal/app`.
- Update the comments that say the app code "mirrors" the CLI's `requestJSONKeys` (`requiredJSONKeys` in `schema_tags.go`, `reflectFields` in `schema.go`) so they name the shared function.
- The existing cross-check tests in `internal/cli/jsonfile_production_test.go` keep running against the shared function, unchanged in intent.

## Out of scope

Changing the JSON-key rules themselves, the schema tags, or validator behavior. The `jsonTagName` helper in `internal/repoguard/testexec_boundary_test.go` is test-only and answers a different question (a config field's bare tag, with no field-name fallback), so it stays as is.
