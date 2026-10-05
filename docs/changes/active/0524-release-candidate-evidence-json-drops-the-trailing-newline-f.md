---
id: 524
slug: 'release-candidate-evidence-json-drops-the-trailing-newline-f'
title: 'Release-candidate evidence.json drops the trailing newline from its checksums copy'
status: 'implemented'
priority: 'low'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [317, 366, 512]
discovered_from: [366]
adrs: []
spec:
plan: 'docs/superpowers/plans/2026-10-05-release-candidate-evidence-json-drops-the-trailing-newline-f.md'
results: 'docs/results/2026-10-05-release-candidate-evidence-json-drops-the-trailing-newline-f-results.md'
trivial: true
auto_groomable:
branch_prefix:
branch: 'fix/release-candidate-evidence-json-drops-the-trailing-newline-f'
pr: 'https://github.com/danielhanold/docket/pull/391'
blocked_by:
reconciled: true
claimed_at: '2026-10-05T09:52:20Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-10-05-release-candidate-evidence-json-drops-the-trailing-newline-f.md](https://github.com/danielhanold/docket/blob/fix/release-candidate-evidence-json-drops-the-trailing-newline-f/docs/superpowers/plans/2026-10-05-release-candidate-evidence-json-drops-the-trailing-newline-f.md) |
| Results | [2026-10-05-release-candidate-evidence-json-drops-the-trailing-newline-f-results.md](https://github.com/danielhanold/docket/blob/fix/release-candidate-evidence-json-drops-the-trailing-newline-f/docs/results/2026-10-05-release-candidate-evidence-json-drops-the-trailing-newline-f-results.md) |
<!-- docket:artifacts:end -->

## Why

The `release-candidate.yml` workflow builds `evidence.json` with `jq --arg checksums "$checksums"`, and `$checksums` is filled from `checksums.txt` by a shell command substitution. Command substitution strips trailing newlines, so the `checksums_txt` field is not byte-equal to the bundle's `checksums.txt`. The release protocol checks exactly that byte equality.

During the alpha.1 release (change 0366), Phase 2 hit this as a STOP. Every digest was correct, and the human waived it. Each future release will hit the same STOP until the workflow is fixed.

## What changes

In the `summary` job's "Assemble the evidence JSON" step of `.github/workflows/release-candidate.yml`, read the checksums with `jq -n --rawfile checksums candidate-head/checksums.txt …` and delete the `checksums="$(cat candidate-head/checksums.txt)"` line. `--rawfile` keeps the file's exact bytes, trailing newline included. The ubuntu-24.04 runner ships jq 1.7; `--rawfile` exists since 1.6.

Trivial: there is no design question. No test exercises any workflow step today, so building one for this line is not worth it. The build proves the fix once instead: run the same jq invocation locally on a sample `checksums.txt`, extract the field with `jq -j .checksums_txt`, and `cmp` it against the file; record the result in the results file. After that, the release protocol's existing Phase 2 byte-equality check covers it on every release. No new CI gate.

## Out of scope

Any other change to the release-candidate workflow or the evidence schema.

## Reconcile log

### 2026-10-05

Re-read against current main (7d2fa3e1a). The `checksums="$(cat candidate-head/checksums.txt)"` line and `--arg checksums "$checksums"` still sit in the summary job's "Assemble the evidence JSON" step; no other change touched them. No test or script references the line. Scope unchanged.
