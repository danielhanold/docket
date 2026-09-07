---
id: 383
slug: 'remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts'
title: 'Remove or plumb the dead metadata-fetch diagnostic append in augmentCheckFacts'
status: 'implemented'
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
plan: 'docs/superpowers/plans/2026-09-07-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts.md'
results:
trivial: false
auto_groomable:
branch: 'fix/remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts'
pr: 'https://github.com/danielhanold/docket/pull/281'
blocked_by:
reconciled: true
claimed_at: '2026-09-07T01:52:36Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts-design.md) |
| Plan | [2026-09-07-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-07-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts.md) |
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

### 2026-09-07

2026-09-06: Reconciled against current internal/app code. Confirmed the dead append is still live at repository_check.go:193 inside augmentCheckFacts, which receives setupContext by value (line 168) and is called by value from RunRepositoryCheck (line 92); sc.diagnostics is written only at line 193 in that file and never read there. The consumed setupContext.diagnostics producer lives in repository_facts.go and feeds prepareNotices independently of the augmentCheckFacts value-copy, so the append remains dead — not made live by change 0403's separate config-error path. Design remains accurate: delete only the metadata-fetch append, preserving the RootUnknown assignment, fetch control flow, and all public output. related:[377,378] and discovered_from:[377] remain correct; depends_on/adrs stay empty. No scope change.
