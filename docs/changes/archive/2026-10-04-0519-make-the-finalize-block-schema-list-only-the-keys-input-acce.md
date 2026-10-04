---
id: 519
slug: 'make-the-finalize-block-schema-list-only-the-keys-input-acce'
title: 'Make the finalize.block schema list only the keys --input accepts'
status: 'killed'
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

`finalize.block --input` accepts only `report` and `remedy`; the other six fields (`id`, `revision`, `pr_number`, `attempt`, `reason`, `head`) come from flags. But `internal/app/schema_registry.go` registers the whole `BlockRequest` struct, so `docket schema --operation finalize.block` lists all eight as request fields. An agent following the schema sends keys the input refuses. Seen during the 0502 finalize on 2026-10-04.

## What changes

Make the published schema for `finalize.block` match what `--input` actually accepts (for example, register an input-only type with `report` and `remedy`), and add a test that the schema's request keys equal the keys the input decoder accepts. Check the sibling operations that split a request between flags and `--input` (such as `finalize.clear-block`) for the same mismatch.

## Out of scope

Changing which values are passed as flags versus input. The broader CLI schema items bundled in change 0360.

## Why killed

Consolidated into #520 (with #518), which fixes both finalize schema mismatches together under one --input/schema guard test.
