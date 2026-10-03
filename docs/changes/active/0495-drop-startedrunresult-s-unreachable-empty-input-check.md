---
id: 495
slug: 'drop-startedrunresult-s-unreachable-empty-input-check'
title: 'Drop startedRunResult''s unreachable empty-input check'
status: 'proposed'
priority: 'low'
type: 'refactor'
created: '2026-10-03'
updated: '2026-10-03'
depends_on: [491]
stacked_on:
related: []
discovered_from: [491]
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

Change 0491's review fix F2 (commit e139a0dfb) moved the empty-capability check in `run.start` ahead of minting the run record, so an empty run context is now refused as `run-untracked scope-failed` before anything is minted. Both callers of `startedRunResult` (in `internal/app/runtracker_start.go`) run only after a successful mint, with a non-empty key and run context. That makes its `key == "" || runContext == ""` branch, which returns `run-untracked mint-failed`, unreachable. Its own comment already calls it "unreachable defense in depth". The branch stays only because two tests assert it directly. 0491's results file lists this as a follow-up ("`startedRunResult` keeps an unreachable fail-closed check").

## What changes

Remove the unreachable empty-input branch from `startedRunResult` and update its comment. Remove the two test assertions that exercise it: the empty-input table cases in `TestIntegrationRunStartStartedResultRequiresKeyAndContext` (`internal/app/runtracker_start_resume_integration_test.go`) and the empty-key assertion in `TestRunStartResultCarriesNoRunID` (`internal/app/runtracker_start_result_json_test.go`). Keep those tests' assertions for the started case. Grooming should confirm that the scope-failed refusal before the mint is already covered by a test, so no fail-closed coverage is lost.

## Out of scope

Any other change to `run.start` behavior, its refusal reasons, or the `run-started` report format. Reworking the run-start refusal ordering that 0491 settled.
