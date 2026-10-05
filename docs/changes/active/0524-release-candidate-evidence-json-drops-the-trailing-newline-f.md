---
id: 524
slug: 'release-candidate-evidence-json-drops-the-trailing-newline-f'
title: 'Release-candidate evidence.json drops the trailing newline from its checksums copy'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [366, 317]
discovered_from: [366]
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

The `release-candidate.yml` workflow builds `evidence.json` with `jq --arg checksums "$checksums"`, and `$checksums` is filled from `checksums.txt` by a shell command substitution. Command substitution strips trailing newlines, so the `checksums_txt` field is not byte-equal to the bundle's `checksums.txt`. The release protocol checks exactly that byte equality.

During the alpha.1 release (change 0366), Phase 2 hit this as a STOP. Every digest was correct, and the human waived it. Each future release will hit the same STOP until the workflow is fixed.

## What changes

Make `checksums_txt` in `evidence.json` byte-equal to the bundle's `checksums.txt`, for example with `jq --rawfile`. Add or extend a test that compares the two byte for byte.

## Out of scope

Any other change to the release-candidate workflow or the evidence schema.
