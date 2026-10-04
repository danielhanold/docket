---
id: 520
slug: 'make-the-published-finalize-request-schemas-match-what-input'
title: 'Make the published finalize request schemas match what --input accepts'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [360, 502]
discovered_from: [518, 519]
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

Consolidates #518 and #519. Both are the same bug: `docket schema` tells an agent the wrong input shape for a finalize operation, so the agent sends keys that `--input` then refuses. Both were seen during the 0502 finalize on 2026-10-04.

- `finalize.rebase-continue` (#518): `internal/app/schema_registry.go` registers it with `Request: nil`, so `docket schema` gives the `docket-rebase-resolver` no request shape to copy. Two resolver reports carried a `schema_version` key, probably copied from the schema envelope. `finalize rebase-continue --input` rejected it, and the controller had to strip the key by hand and re-run the continue step.
- `finalize.block` (#519): `--input` accepts only `report` and `remedy`, and the other six fields (`id`, `revision`, `pr_number`, `attempt`, `reason`, `head`) come from flags. The registry still binds the whole `BlockRequest` struct, so `docket schema --operation finalize.block` lists all eight as request fields.

## What changes

- Register the resolver report's request type for `finalize.rebase-continue` so `docket schema --operation finalize.rebase-continue` prints the accepted keys.
- Make the published schema for `finalize.block` list only the keys its `--input` accepts (for example, register an input-only type with `report` and `remedy`).
- Add one guard test: for each operation that reads `--input`, the schema's request keys must equal the keys its input decoder accepts. Derive that operation set from the code instead of hand-listing it, so the guard also covers sibling flag/input splits such as `finalize.clear-block`. Mutation-test it. Fix any other mismatch it finds.
- Grooming decides whether `rebase-continue` should accept an envelope-level `schema_version` or name it in its refusal, and whether the resolver agent text should point at the schema.

## Out of scope

Changing which values are passed as flags versus input. The resolver attempt budget (`finalize.resolver_max_attempts`) and how the rebase replays commits. The broader CLI schema items bundled in change 0360. The other finalize fixes found during 0502 stay separate on purpose: #515 (a named-id merge skips the repair sign-off block) and #517 (evidence for a finalize re-test).
