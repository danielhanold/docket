---
id: 383
slug: 'remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts'
title: 'Remove or plumb the dead metadata-fetch diagnostic append in augmentCheckFacts'
status: proposed
priority: low
type: fix
created: '2026-08-31'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [377, 378]
discovered_from: [377]
adrs: []
spec: 'docs/superpowers/specs/2026-09-07-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts-design.md'
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts-design.md) |
<!-- docket:artifacts:end -->

## Why

`augmentCheckFacts` appends a metadata-fetch diagnostic to a copied context that neither it nor `RunRepositoryCheck` subsequently consumes. Change 0377 reported this dead append during review of code introduced by 0378; both changes are now done. Tracing the current code confirms that repository preparation's diagnostic consumer does not read this context copy.

The failed-fetch path already sets metadata ownership to unknown and forbids stale-object ownership proof. The unused append contributes nothing to that behavior and should be removed.

## What changes

Remove the dead metadata-fetch diagnostic append in `augmentCheckFacts`, preserving the existing unknown-ownership assignment, fetched-tip handling, subsequent checks, and all public output. The linked spec records the evidence, the decision against adding reporting behavior in this change, and the implementation verification requirements.

## Out of scope

- New user-visible diagnostics, findings, notices, result values, or exit-code behavior.
- Broader changes to the check facts pipeline or shared diagnostic handling.
- Changes to the ownership verifier, stale-object safety, or repository preparation.

## Reconcile log
