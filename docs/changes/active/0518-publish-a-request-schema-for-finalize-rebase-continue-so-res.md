---
id: 518
slug: 'publish-a-request-schema-for-finalize-rebase-continue-so-res'
title: 'Publish a request schema for finalize.rebase-continue so resolver reports stop carrying schema_version'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [360, 502]
discovered_from: [502]
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

During the 0502 finalize (2026-10-04), two `docket-rebase-resolver` reports included a `schema_version` key that `finalize rebase-continue --input` rejects, so the controller had to strip the key by hand and re-run the continue step. `internal/app/schema_registry.go` registers `finalize.rebase-continue` with `Request: nil`, so `docket schema` gives the resolver no request shape to copy; the schema envelope itself carries `schema_version`, which is the likely source of the stray key.

## What changes

Register the resolver report's request type for `finalize.rebase-continue` so `docket schema --operation finalize.rebase-continue` prints the accepted keys. Grooming decides whether the continue step should also tolerate (or name in its refusal) an envelope-level `schema_version`, and whether the resolver agent text needs to point at the schema.

## Out of scope

The resolver attempt budget (`finalize.resolver_max_attempts`) and how the rebase replays commits. The broader CLI schema items bundled in change 0360.
